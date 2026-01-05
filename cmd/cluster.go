package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"civo-cluster-deploy/internal/config"
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
	Long:  "Remove a cluster configuration YAML file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		clusterName := args[0]
		deleteClusterFromCLI(clusterName)
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

	clusterCmd.AddCommand(addCmd)
	clusterCmd.AddCommand(modifyCmd)
	clusterCmd.AddCommand(deleteCmd)
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

	// Get authentication - prefer stored password, fallback to environment token
	var authToken string
	if storedPassword, err := currentRepo.GetPassword(); err == nil && storedPassword != "" {
		authToken = storedPassword
	} else {
		authToken = os.Getenv("HYVE_GIT_TOKEN")
	}
	stateMgr := state.NewManager(currentRepo.RepoURL, currentRepo.LocalPath, currentRepo.Username, authToken)

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

// commitStateChanges commits changes to Git repository
func commitStateChanges(ctx context.Context, stateMgr *state.Manager, message string) {
	if err := stateMgr.CommitAndPush(ctx, message); err != nil {
		log.Printf("Warning: Failed to commit changes to Git repository: %v", err)
	} else {
		log.Println("Changes committed and pushed to Git repository")
	}
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
			Firewall: types.FirewallSpec{
				Enabled: true,
				Rules: []types.FirewallRule{
					{
						Protocol:  "tcp",
						StartPort: "6443",
						EndPort:   "6443",
						Cidr:      []string{"0.0.0.0/0"},
						Direction: "ingress",
					},
				},
			},
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

func deleteClusterFromCLI(clusterName string) {
	ctx := context.Background()
	stateMgr, stateDir := createStateManager(ctx)
	filePath := filepath.Join(stateDir, clusterName+".yaml")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Fatalf("Cluster %s does not exist.", clusterName)
	}

	if err := os.Remove(filePath); err != nil {
		log.Fatalf("Failed to delete cluster definition file: %v", err)
	}

	// Commit changes to Git if configured
	commitStateChanges(ctx, stateMgr, fmt.Sprintf("Delete cluster %s", clusterName))

	log.Printf("Deleted cluster definition file: %s", filePath)
	log.Printf("Cluster %s has been removed from configuration", clusterName)

	runReconciliation()
}
