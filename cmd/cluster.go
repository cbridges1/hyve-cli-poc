package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"civo-cluster-deploy/internal/cluster"
	"civo-cluster-deploy/internal/config"
	"civo-cluster-deploy/internal/credentials"
	"civo-cluster-deploy/internal/ingress"
	"civo-cluster-deploy/internal/kubeconfig"
	"civo-cluster-deploy/internal/provider"
	"civo-cluster-deploy/internal/repository"
	"civo-cluster-deploy/internal/state"
	"civo-cluster-deploy/internal/types"
)

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Manage clusters",
	Long:  "Commands to add, modify, or delete cluster configurations",
}

var addCmd = &cobra.Command{
	Use:   "add [cluster-name]",
	Short: "Add a new cluster",
	Long:  "Create a new cluster configuration YAML file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		clusterName := args[0]

		region, _ := cmd.Flags().GetString("region")
		provider, _ := cmd.Flags().GetString("provider")
		nodes, _ := cmd.Flags().GetStringSlice("nodes")
		clusterType, _ := cmd.Flags().GetString("cluster-type")
		addClusterFromCLI(clusterName, region, provider, nodes, clusterType)
	},
}

var modifyCmd = &cobra.Command{
	Use:   "modify [cluster-name]",
	Short: "Modify an existing cluster",
	Long:  "Update an existing cluster configuration YAML file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		clusterName := args[0]

		modifyClusterFromCLI(cmd, clusterName)
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete [cluster-name]",
	Short: "Delete a cluster",
	Long: `Delete a cluster both from the cloud provider and from configuration.
This command will:
1. Explicitly search for and delete the cluster from the cloud provider
2. Remove the cluster configuration YAML file (if it exists)
3. Run reconciliation to clean up any remaining resources

Use --config-only to only remove the configuration file without touching the cloud resources.
Use --force-cloud to delete from cloud even if no configuration file exists.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		clusterName := args[0]
		configOnly, _ := cmd.Flags().GetBool("config-only")
		forceCloud, _ := cmd.Flags().GetBool("force-cloud")
		deleteClusterFromCLI(clusterName, configOnly, forceCloud)
	},
}

var forceDeleteCmd = &cobra.Command{
	Use:   "force-delete [cluster-name]",
	Short: "Force delete a cluster by name from cloud provider",
	Long: `Force delete a cluster by name directly from the cloud provider without requiring a configuration file.
This command will search all regions for the cluster and delete it if found.
Useful when configuration files are lost or corrupted.

Note: This command does not remove configuration files or run reconciliation.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		clusterName := args[0]
		region, _ := cmd.Flags().GetString("region")
		forceDeleteClusterFromCloud(clusterName, region)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all stored cluster kubeconfigs",
	Long:  "Display all kubeconfigs stored for clusters in the current repository",
	Run: func(cmd *cobra.Command, args []string) {
		listClusters()
	},
}

func init() {
	addCmd.Flags().StringP("region", "r", "PHX1", "Region for the cluster")
	addCmd.Flags().StringP("provider", "p", "civo", "Cloud provider (e.g., civo, aws, gcp, azure)")
	addCmd.Flags().StringSliceP("nodes", "n", []string{"g4s.kube.small"}, "Node sizes")
	addCmd.Flags().StringP("cluster-type", "t", "k3s", "Type of Kubernetes cluster")

	modifyCmd.Flags().StringP("region", "r", "", "Region for the cluster")
	modifyCmd.Flags().StringP("provider", "p", "", "Cloud provider")
	modifyCmd.Flags().StringSliceP("nodes", "n", nil, "Node sizes")
	modifyCmd.Flags().StringP("cluster-type", "t", "", "Type of Kubernetes cluster")

	deleteCmd.Flags().Bool("config-only", false, "Only remove configuration file, skip cloud provider deletion")
	deleteCmd.Flags().Bool("force-cloud", false, "Delete from cloud even if no configuration file exists")

	forceDeleteCmd.Flags().StringP("region", "r", "", "Specific region to search (optional, will search common regions if not provided)")

	clusterCmd.AddCommand(addCmd)
	clusterCmd.AddCommand(listCmd)
	clusterCmd.AddCommand(modifyCmd)
	clusterCmd.AddCommand(deleteCmd)
	clusterCmd.AddCommand(forceDeleteCmd)
}

// createStateManager creates state manager from current repository
func createStateManager(ctx context.Context) (*state.Manager, string) {
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		log.Fatalf("❌ No Git repository configured. Hyve requires a Git repository for state management.\n\n" +
			"To get started:\n" +
			"  1. hyve git add <name> --repo-url <repository-url>\n" +
			"  2. hyve cluster add <cluster-name> --region <region>\n\n" +
			"Example:\n" +
			"  hyve git add production --repo-url https://github.com/company/hyve-state.git")
	}

	log.Printf("Using Git repository '%s': %s", currentRepo.Name, currentRepo.RepoURL)

	// Get authentication - prefer global credentials, fallback to environment token
	credsMgr, err := credentials.NewManager()
	var authToken string
	var authUsername = currentRepo.Username

	if err == nil {
		defer credsMgr.Close()
		if creds, _ := credsMgr.GetCredentials(); creds != nil {
			if password, err := creds.GetPassword(); err == nil && password != "" {
				authToken = password
				if authUsername == "" {
					authUsername = creds.Username
				}
			}
		}
	}

	if authToken == "" {
		authToken = os.Getenv("HYVE_GIT_TOKEN")
	}

	stateMgr, err := state.NewManager(currentRepo.RepoURL, currentRepo.LocalPath, authUsername, authToken)
	if err != nil {
		log.Fatalf("Failed to create state manager: %v", err)
	}

	// Initialize and sync Git repository
	if err := stateMgr.InitializeGitRepo(ctx); err != nil {
		log.Fatalf("Failed to initialize Git repository: %v", err)
	}

	if err := stateMgr.SyncWithRemote(ctx); err != nil {
		log.Fatalf("Failed to sync with remote repository: %v", err)
	}

	log.Println("Git repository synchronized")

	// Get the state directory path
	stateDir := filepath.Join(currentRepo.LocalPath, "clusters")
	return stateMgr, stateDir
}

// commitStateChanges commits changes to Git repository and pushes to remote
func commitStateChanges(ctx context.Context, stateMgr *state.Manager, message string) {
	log.Println("📝 Committing and pushing changes to Git repository...")

	if err := stateMgr.CommitAndPush(ctx, message); err != nil {
		log.Printf("❌ Failed to commit and push: %v", err)

		// Provide helpful hints based on error type
		if strings.Contains(err.Error(), "failed to push") {
			log.Println("💡 Changes were committed locally but push failed")
			log.Println("💡 Check your Git credentials and network connection")
			log.Println("💡 You can manually push with: cd <repo-path> && git push")
		} else if strings.Contains(err.Error(), "failed to commit") {
			log.Println("💡 Commit operation failed - changes may still be in working directory")
		}
		return
	}

	log.Println("✅ Changes committed and pushed to remote repository successfully")
}

func addClusterFromCLI(clusterName, region, provider string, nodes []string, clusterType string) {
	ctx := context.Background()
	stateMgr, stateDir := createStateManager(ctx)

	if err := os.MkdirAll(stateDir, 0755); err != nil {
		log.Fatalf("Failed to create state directory: %v", err)
	}

	filePath := filepath.Join(stateDir, clusterName+".yaml")

	if _, err := os.Stat(filePath); err == nil {
		log.Fatalf("Cluster %s already exists. Use 'modify' action to update it.", clusterName)
	}

	clusterDef := types.ClusterDefinition{
		APIVersion: "v1",
		Kind:       "Cluster",
		Metadata: types.ClusterMetadata{
			Name:   clusterName,
			Region: region,
		},
		Spec: types.ClusterSpec{
			Provider:    provider,
			Nodes:       nodes,
			ClusterType: clusterType,
			Ingress: types.IngressSpec{
				Enabled:      true,
				LoadBalancer: true,
			},
		},
	}

	data, err := yaml.Marshal(&clusterDef)
	if err != nil {
		log.Fatalf("Failed to marshal cluster definition: %v", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		log.Fatalf("Failed to write cluster definition file: %v", err)
	}

	log.Printf("Created cluster definition file: %s", filePath)
	log.Printf("Cluster %s configuration:", clusterName)
	log.Printf("  Region: %s", region)
	log.Printf("  Provider: %s", provider)
	log.Printf("  Nodes: %v", nodes)
	log.Printf("  Cluster Type: %s", clusterType)

	// Commit changes to Git if configured
	commitStateChanges(ctx, stateMgr, fmt.Sprintf("Add cluster %s", clusterName))

	log.Printf("Exporting cluster information...")
	configMgr := config.NewManager()
	if apiKey := configMgr.GetCivoToken(); apiKey != "" {
		err := exportClusterInfo(ctx, apiKey, clusterDef)
		if err != nil {
			log.Printf("Warning: Failed to export cluster info: %v", err)
		}
	}

	runReconciliation()
}

func modifyClusterFromCLI(cmd *cobra.Command, clusterName string) {
	ctx := context.Background()
	stateMgr, stateDir := createStateManager(ctx)
	filePath := filepath.Join(stateDir, clusterName+".yaml")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Fatalf("Cluster %s does not exist. Use 'add' action to create it.", clusterName)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to read existing cluster file: %v", err)
	}

	var clusterDef types.ClusterDefinition
	if err := yaml.Unmarshal(data, &clusterDef); err != nil {
		log.Fatalf("Failed to parse existing cluster definition: %v", err)
	}

	if cmd.Flags().Changed("region") {
		region, _ := cmd.Flags().GetString("region")
		clusterDef.Metadata.Region = region
	}
	if cmd.Flags().Changed("provider") {
		provider, _ := cmd.Flags().GetString("provider")
		clusterDef.Spec.Provider = provider
	}
	if cmd.Flags().Changed("nodes") {
		nodes, _ := cmd.Flags().GetStringSlice("nodes")
		clusterDef.Spec.Nodes = nodes
	}
	if cmd.Flags().Changed("cluster-type") {
		clusterType, _ := cmd.Flags().GetString("cluster-type")
		clusterDef.Spec.ClusterType = clusterType
	}

	updatedData, err := yaml.Marshal(&clusterDef)
	if err != nil {
		log.Fatalf("Failed to marshal updated cluster definition: %v", err)
	}

	if err := os.WriteFile(filePath, updatedData, 0644); err != nil {
		log.Fatalf("Failed to write updated cluster definition file: %v", err)
	}

	log.Printf("Updated cluster definition file: %s", filePath)
	log.Printf("Cluster %s updated configuration:", clusterName)
	log.Printf("  Region: %s", clusterDef.Metadata.Region)
	log.Printf("  Provider: %s", clusterDef.Spec.Provider)
	log.Printf("  Nodes: %v", clusterDef.Spec.Nodes)
	log.Printf("  Cluster Type: %s", clusterDef.Spec.ClusterType)

	// Commit changes to Git if configured
	commitStateChanges(ctx, stateMgr, fmt.Sprintf("Modify cluster %s", clusterName))

	log.Printf("Exporting cluster information...")
	configMgr := config.NewManager()
	if apiKey := configMgr.GetCivoToken(); apiKey != "" {
		err := exportClusterInfo(ctx, apiKey, clusterDef)
		if err != nil {
			log.Printf("Warning: Failed to export cluster info: %v", err)
		}
	}
}

func deleteClusterFromCLI(clusterName string, configOnly bool, forceCloud bool) {
	ctx := context.Background()
	stateMgr, stateDir := createStateManager(ctx)
	filePath := filepath.Join(stateDir, clusterName+".yaml")

	var clusterDef types.ClusterDefinition
	configExists := false

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if !forceCloud {
			log.Fatalf("Cluster %s configuration does not exist. Use --force-cloud to delete from cloud provider anyway.", clusterName)
		}
		log.Printf("⚠️ Configuration file not found, but --force-cloud specified")
		// Use default region for force cloud deletion
		clusterDef.Metadata.Region = "PHX1" // Default region
	} else {
		configExists = true
		// Read the cluster definition to get the region for proper provider initialization
		data, err := os.ReadFile(filePath)
		if err != nil {
			log.Fatalf("Failed to read cluster definition: %v", err)
		}

		if err := yaml.Unmarshal(data, &clusterDef); err != nil {
			log.Fatalf("Failed to parse cluster definition: %v", err)
		}
	}

	// Explicitly delete the cluster by name before removing the YAML file (unless config-only mode)
	if !configOnly {
		log.Printf("🗑️ Deleting cluster '%s' from cloud provider...", clusterName)
		err := deleteClusterExplicitly(ctx, clusterName, clusterDef.Metadata.Region)
		if err != nil {
			log.Printf("Warning: Failed to delete cluster %s from cloud provider: %v", clusterName, err)
			log.Printf("Continuing with configuration file removal...")
		}
	} else {
		log.Printf("📝 Skipping cloud provider deletion (config-only mode)")
	}

	// Remove configuration file if it exists
	if configExists {
		if err := os.Remove(filePath); err != nil {
			log.Fatalf("Failed to delete cluster definition file: %v", err)
		}

		// Commit changes to Git if configured
		commitStateChanges(ctx, stateMgr, fmt.Sprintf("Delete cluster %s", clusterName))

		log.Printf("Deleted cluster definition file: %s", filePath)
		log.Printf("Cluster %s has been removed from configuration", clusterName)
	} else {
		log.Printf("📝 No configuration file to remove")
	}

	// Run reconciliation to clean up any remaining resources
	runReconciliation()
}

// deleteClusterExplicitly deletes a cluster by name directly from the provider
// This ensures deletion even if the cluster doesn't appear in provider API listings
func deleteClusterExplicitly(ctx context.Context, clusterName, region string) error {
	configMgr := config.NewManager()
	apiKey := configMgr.GetCivoToken()
	if apiKey == "" {
		return fmt.Errorf("CIVO API token not found. Please run 'hyve config set-token civo' or set CIVO_TOKEN environment variable")
	}

	// Create provider factory and provider for the cluster's region
	providerFactory := provider.NewFactory()
	prov, err := providerFactory.CreateProvider("civo", apiKey, region)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Create cluster and ingress managers
	clusterMgr := cluster.NewManager(prov)
	ingressMgr := ingress.NewManager(prov)

	log.Printf("🔍 Explicitly searching for cluster '%s' in region %s...", clusterName, region)

	// Try to find the cluster by name
	existingCluster, err := clusterMgr.FindByName(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to search for cluster: %w", err)
	}

	if existingCluster == nil {
		log.Printf("✅ Cluster '%s' not found in cloud provider, may already be deleted", clusterName)
		return nil
	}

	log.Printf("🗑️ Found cluster '%s' with ID %s, explicitly deleting...", clusterName, existingCluster.ID)

	// If cluster has ingress enabled, try to remove it first
	// We assume it might have ingress based on common patterns
	log.Printf("🔌 Attempting to remove any ingress controllers...")
	err = ingressMgr.RemoveIngressController(ctx, existingCluster.ID)
	if err != nil {
		log.Printf("Warning: Failed to remove ingress controller (may not exist): %v", err)
	}

	// Delete the cluster explicitly by ID
	err = clusterMgr.Delete(ctx, existingCluster.ID)
	if err != nil {
		return fmt.Errorf("failed to delete cluster %s (ID: %s): %w", clusterName, existingCluster.ID, err)
	}

	log.Printf("✅ Successfully deleted cluster '%s' from cloud provider", clusterName)
	return nil
}

// forceDeleteClusterFromCloud deletes a cluster by name from the cloud provider across multiple regions
func forceDeleteClusterFromCloud(clusterName, region string) {
	ctx := context.Background()

	configMgr := config.NewManager()
	apiKey := configMgr.GetCivoToken()
	if apiKey == "" {
		log.Fatalf("CIVO API token not found. Please run 'hyve config set-token civo' or set CIVO_TOKEN environment variable")
	}

	regions := []string{region}
	if region == "" {
		// Search common regions if none specified
		regions = []string{"PHX1", "NYC1", "FRA1", "LON1"}
		log.Printf("🔍 No region specified, searching common regions: %v", regions)
	}

	providerFactory := provider.NewFactory()
	found := false

	for _, r := range regions {
		log.Printf("🔍 Searching for cluster '%s' in region %s...", clusterName, r)

		prov, err := providerFactory.CreateProvider("civo", apiKey, r)
		if err != nil {
			log.Printf("Failed to create provider for region %s: %v", r, err)
			continue
		}

		clusterMgr := cluster.NewManager(prov)
		ingressMgr := ingress.NewManager(prov)

		existingCluster, err := clusterMgr.FindByName(ctx, clusterName)
		if err != nil {
			log.Printf("Failed to search for cluster in region %s: %v", r, err)
			continue
		}

		if existingCluster == nil {
			log.Printf("❌ Cluster '%s' not found in region %s", clusterName, r)
			continue
		}

		found = true
		log.Printf("✅ Found cluster '%s' in region %s with ID %s", clusterName, r, existingCluster.ID)
		log.Printf("🗑️ Force deleting cluster '%s'...", clusterName)

		// Remove ingress controller if present
		log.Printf("🔌 Attempting to remove any ingress controllers...")
		err = ingressMgr.RemoveIngressController(ctx, existingCluster.ID)
		if err != nil {
			log.Printf("Warning: Failed to remove ingress controller (may not exist): %v", err)
		}

		// Delete the cluster
		err = clusterMgr.Delete(ctx, existingCluster.ID)
		if err != nil {
			log.Fatalf("Failed to delete cluster %s (ID: %s): %v", clusterName, existingCluster.ID, err)
		}

		log.Printf("✅ Successfully force deleted cluster '%s' from region %s", clusterName, r)
		break
	}

	if !found {
		log.Printf("❌ Cluster '%s' not found in any searched regions", clusterName)
		log.Printf("💡 Try specifying a specific region with --region flag")
	}
}

func listClusters() {
	// Get current repository
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		log.Fatalf("No Git repository configured. Use 'hyve git add' to configure a repository")
	}

	// Create kubeconfig manager
	kubeconfigMgr, err := kubeconfig.NewManager(currentRepo.Name)
	if err != nil {
		log.Fatalf("Failed to create kubeconfig manager: %v", err)
	}
	defer kubeconfigMgr.Close()

	kubeconfigs, err := kubeconfigMgr.ListKubeconfigs()
	if err != nil {
		log.Fatalf("Failed to list kubeconfigs: %v", err)
	}

	if len(kubeconfigs) == 0 {
		log.Printf("❌ No clusters found for repository '%s'", currentRepo.Name)
		log.Println("\n💡 Run 'hyve kubeconfig sync' to retrieve kubeconfigs from active clusters")
		return
	}

	log.Printf("🔑 Clusters in repository '%s' (%d):\n", currentRepo.Name, len(kubeconfigs))

	for _, kc := range kubeconfigs {
		log.Printf("  %s", kc.ClusterName)
		log.Printf("    Repository: %s", kc.RepositoryName)
		log.Printf("    Stored: %s", kc.UpdatedAt.Format("2006-01-02 15:04:05"))
		log.Println()
	}

	log.Println("💡 Commands:")
	log.Println("  eval $(hyve use <cluster-name>)              # Quickly set kubeconfig (recommended)")
	log.Println("  hyve kubeconfig get <cluster-name>           # Display kubeconfig")
	log.Println("  hyve kubeconfig get <cluster-name> --save    # Save to ~/.kube/config-<cluster-name>")
	log.Println("  hyve kubeconfig get <cluster-name> -o <file> # Save to specific file")
	log.Println("  hyve kubeconfig use <cluster-name>           # Set kubeconfig for current terminal session")
}
