package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	"civo-cluster-deploy/internal/config"
	"civo-cluster-deploy/internal/credentials"
	"civo-cluster-deploy/internal/kubeconfig"
	"civo-cluster-deploy/internal/provider"
	"civo-cluster-deploy/internal/providerconfig"
	"civo-cluster-deploy/internal/repository"
	"civo-cluster-deploy/internal/state"
	"civo-cluster-deploy/internal/types"
)

var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig",
	Short: "Manage kubeconfigs for clusters",
	Long:  "Commands to sync, retrieve, and manage kubeconfigs for clusters in the current repository",
}

var kubeconfigSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync kubeconfigs from all active clusters",
	Long:  "Retrieve and store kubeconfigs from all active clusters in the current repository",
	Run: func(cmd *cobra.Command, args []string) {
		syncKubeconfigs()
	},
}

var kubeconfigGetCmd = &cobra.Command{
	Use:   "get [cluster-name]",
	Short: "Get kubeconfig for a specific cluster",
	Long:  "Retrieve and display the kubeconfig for a specific cluster",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		clusterName := args[0]
		getKubeconfig(cmd, clusterName)
	},
}

var kubeconfigUseCmd = &cobra.Command{
	Use:   "use [cluster-name]",
	Short: "Set kubeconfig for current terminal session",
	Long:  "Create a temporary kubeconfig and provide export command to use it in the current terminal session",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		clusterName := args[0]
		evalMode, _ := cmd.Flags().GetBool("eval")
		useKubeconfig(clusterName, evalMode)
	},
}

var kubeconfigMergeCmd = &cobra.Command{
	Use:   "merge [cluster-name]",
	Short: "Merge cluster context into local ~/.kube/config",
	Long:  "Merge the cluster's kubeconfig context into your local ~/.kube/config file for easy kubectl access",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		clusterName := args[0]
		mergeKubeconfig(clusterName)
	},
}

var kubeconfigRemoveCmd = &cobra.Command{
	Use:   "remove [cluster-name]",
	Short: "Remove cluster context from local ~/.kube/config",
	Long:  "Remove the cluster's context, cluster, and user entries from your local ~/.kube/config file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		clusterName := args[0]
		removeKubeconfig(clusterName)
	},
}

var kubeconfigMigrateCmd = &cobra.Command{
	Use:   "migrate [old-hostname]",
	Short: "Migrate kubeconfig encryption to new portable format",
	Long: `Migrate kubeconfig encryption from hostname-based keys to portable keys.

This command re-encrypts all kubeconfigs using a key that doesn't include the hostname,
making the database portable across machines. You need to provide the hostname that was
used when the kubeconfigs were originally encrypted.

Example:
  hyve kubeconfig migrate "old-macbook.local"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldHostname := args[0]
		return migrateKubeconfigEncryption(oldHostname)
	},
}

func init() {
	kubeconfigGetCmd.Flags().BoolP("save", "s", false, "Save kubeconfig to ~/.kube/config-<cluster-name>")
	kubeconfigGetCmd.Flags().BoolP("merge", "m", false, "Merge kubeconfig into ~/.kube/config")
	kubeconfigGetCmd.Flags().StringP("output", "o", "", "Output file path for kubeconfig")

	kubeconfigUseCmd.Flags().BoolP("eval", "e", false, "Output shell commands for evaluation (use with eval)")

	kubeconfigCmd.AddCommand(kubeconfigSyncCmd)
	kubeconfigCmd.AddCommand(kubeconfigGetCmd)
	kubeconfigCmd.AddCommand(kubeconfigUseCmd)
	kubeconfigCmd.AddCommand(kubeconfigMergeCmd)
	kubeconfigCmd.AddCommand(kubeconfigRemoveCmd)
	kubeconfigCmd.AddCommand(kubeconfigMigrateCmd)
}

// createKubeconfigManager creates a kubeconfig manager for the current repository
func createKubeconfigManager() (*kubeconfig.Manager, string, error) {
	repoMgr, err := repository.NewManager()
	if err != nil {
		return nil, "", fmt.Errorf("failed to create repository manager: %w", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		return nil, "", fmt.Errorf("no Git repository configured. Use 'hyve git add' to configure a repository")
	}

	kubeconfigMgr, err := kubeconfig.NewManager(currentRepo.Name)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create kubeconfig manager: %w", err)
	}

	return kubeconfigMgr, currentRepo.Name, nil
}

// createProviderFromCurrentRepo creates a provider instance from current repository configuration
func createProviderFromCurrentRepo(ctx context.Context) (provider.Provider, error) {
	// Get current repository
	repoMgr, err := repository.NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create repository manager: %w", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		return nil, fmt.Errorf("no Git repository configured")
	}

	// Get API key
	configMgr := config.NewManager()
	apiKey := configMgr.GetCivoToken()
	if apiKey == "" {
		return nil, fmt.Errorf("CIVO API token not found. Please run 'hyve config set-token civo' or set CIVO_TOKEN environment variable")
	}

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

	// Create state manager to get cluster definitions (to determine regions)
	stateMgr, err := state.NewManager(currentRepo.RepoURL, currentRepo.LocalPath, authUsername, authToken)
	if err != nil {
		log.Fatalf("Failed to create state manager: %v", err)
	}

	// Initialize and sync Git repository
	if err := stateMgr.InitializeGitRepo(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize Git repository: %w", err)
	}

	if err := stateMgr.SyncWithRemote(ctx); err != nil {
		return nil, fmt.Errorf("failed to sync with remote repository: %w", err)
	}

	// Load cluster definitions to get regions
	clusterDefs, err := stateMgr.LoadClusterDefinitions()
	if err != nil {
		return nil, fmt.Errorf("failed to load cluster definitions: %w", err)
	}

	// For now, use the first cluster's region or default to PHX1
	region := "PHX1"
	if len(clusterDefs) > 0 {
		region = clusterDefs[0].Metadata.Region
	}

	// Create provider factory and provider
	providerFactory := provider.NewFactory()
	prov, err := providerFactory.CreateProvider("civo", apiKey, region)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	return prov, nil
}

// createProviderForCluster creates a provider with the appropriate options for a specific cluster
func createProviderForCluster(factory *provider.Factory, clusterDef types.ClusterDefinition) (provider.Provider, error) {
	providerName := clusterDef.Spec.Provider
	if providerName == "" {
		providerName = "civo" // default
	}

	opts := provider.ProviderOptions{
		Region: clusterDef.Metadata.Region,
	}

	// Handle Civo-specific configuration
	if providerName == "civo" {
		configMgr := config.NewManager()
		opts.APIKey = configMgr.GetCivoToken()
		if opts.APIKey == "" {
			return nil, fmt.Errorf("CIVO API token not found. Please run 'hyve config set-token civo' or set CIVO_TOKEN environment variable")
		}
	}

	// Handle GCP-specific configuration
	if providerName == "gcp" {
		if clusterDef.Spec.GCPProjectID != "" {
			opts.ProjectID = clusterDef.Spec.GCPProjectID
		} else if clusterDef.Spec.GCPProject != "" {
			// Resolve from alias
			repoMgr, err := repository.NewManager()
			if err == nil {
				defer repoMgr.Close()
				if currentRepo, err := repoMgr.GetCurrentRepository(); err == nil {
					pcMgr := providerconfig.NewManager(currentRepo.LocalPath)
					if projectID, err := pcMgr.GetGCPProjectID(clusterDef.Spec.GCPProject); err == nil {
						opts.ProjectID = projectID
					}
				}
			}
		}
	}

	return factory.CreateProviderWithOptions(providerName, opts)
}

func syncKubeconfigs() {
	ctx := context.Background()

	// Create kubeconfig manager
	kubeconfigMgr, repoName, err := createKubeconfigManager()
	if err != nil {
		log.Fatalf("Failed to create kubeconfig manager: %v", err)
	}
	defer kubeconfigMgr.Close()

	// Create state manager to load cluster definitions
	stateMgr, _ := createStateManager(ctx)
	clusterDefs, err := stateMgr.LoadClusterDefinitions()
	if err != nil {
		log.Fatalf("Failed to load cluster definitions: %v", err)
	}

	log.Printf("📁 Syncing kubeconfigs for repository '%s'", repoName)

	// Create a provider factory for creating per-cluster providers
	providerFactory := provider.NewFactory()
	successCount := 0

	// Sync kubeconfigs for each cluster using the appropriate provider
	for _, clusterDef := range clusterDefs {
		// Create provider with appropriate options for this cluster
		prov, err := createProviderForCluster(providerFactory, clusterDef)
		if err != nil {
			log.Printf("Failed to create provider for cluster %s: %v", clusterDef.Metadata.Name, err)
			continue
		}

		// Create syncer for this cluster
		syncer := kubeconfig.NewSyncer(kubeconfigMgr, prov)
		err = syncer.SyncKubeconfigs(ctx, []types.ClusterDefinition{clusterDef})
		if err != nil {
			log.Printf("Failed to sync kubeconfig for cluster %s: %v", clusterDef.Metadata.Name, err)
			continue
		}
		successCount++
	}

	log.Printf("✅ Kubeconfig sync completed: %d/%d clusters synced successfully", successCount, len(clusterDefs))
}

func getKubeconfig(cmd *cobra.Command, clusterName string) {
	kubeconfigMgr, _, err := createKubeconfigManager()
	if err != nil {
		log.Fatalf("Failed to create kubeconfig manager: %v", err)
	}
	defer kubeconfigMgr.Close()

	// Get kubeconfig
	kc, err := kubeconfigMgr.GetKubeconfig(clusterName)
	if err != nil {
		log.Fatalf("Failed to get kubeconfig: %v", err)
	}

	if kc == nil {
		log.Fatalf("Kubeconfig not found for cluster %s. Run 'hyve kubeconfig sync' first.", clusterName)
	}

	config, err := kc.GetConfig()
	if err != nil {
		log.Fatalf("Failed to decrypt kubeconfig: %v", err)
	}

	// Handle output options
	saveFlag, _ := cmd.Flags().GetBool("save")
	mergeFlag, _ := cmd.Flags().GetBool("merge")
	outputPath, _ := cmd.Flags().GetString("output")

	if outputPath != "" {
		// Save to specified file
		err := os.WriteFile(outputPath, []byte(config), 0600)
		if err != nil {
			log.Fatalf("Failed to write kubeconfig to %s: %v", outputPath, err)
		}
		log.Printf("✅ Kubeconfig saved to %s", outputPath)
	} else if saveFlag {
		// Save to ~/.kube/config-<cluster-name>
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to get user home directory: %v", err)
		}
		kubeDir := fmt.Sprintf("%s/.kube", homeDir)
		if err := os.MkdirAll(kubeDir, 0755); err != nil {
			log.Fatalf("Failed to create .kube directory: %v", err)
		}

		outputPath := fmt.Sprintf("%s/config-%s", kubeDir, clusterName)
		err = os.WriteFile(outputPath, []byte(config), 0600)
		if err != nil {
			log.Fatalf("Failed to write kubeconfig to %s: %v", outputPath, err)
		}
		log.Printf("✅ Kubeconfig saved to %s", outputPath)
		log.Printf("💡 To use: export KUBECONFIG=%s", outputPath)
	} else if mergeFlag {
		log.Println("⚠️  Merge functionality not yet implemented")
		log.Println("💡 Use --save flag to save to a separate file, or redirect output:")
		log.Printf("   hyve kubeconfig get %s > ~/.kube/config-%s", clusterName, clusterName)
		fmt.Print(config)
	} else {
		// Output to stdout
		fmt.Print(config)
	}
}

func useKubeconfig(clusterName string, evalMode bool) {
	kubeconfigMgr, repoName, err := createKubeconfigManager()
	if err != nil {
		log.Fatalf("Failed to create kubeconfig manager: %v", err)
	}
	defer kubeconfigMgr.Close()

	// Get kubeconfig
	kc, err := kubeconfigMgr.GetKubeconfig(clusterName)
	if err != nil {
		log.Fatalf("Failed to get kubeconfig: %v", err)
	}

	if kc == nil {
		log.Fatalf("Kubeconfig not found for cluster %s. Run 'hyve kubeconfig sync' first.", clusterName)
	}

	config, err := kc.GetConfig()
	if err != nil {
		log.Fatalf("Failed to decrypt kubeconfig: %v", err)
	}

	// Create temporary kubeconfig file in ~/.hyve/temp/
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get user home directory: %v", err)
	}

	tempDir := fmt.Sprintf("%s/.hyve/temp", homeDir)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create a unique temporary file for this cluster and repository
	tempFile := fmt.Sprintf("%s/kubeconfig-%s-%s", tempDir, repoName, clusterName)
	err = os.WriteFile(tempFile, []byte(config), 0600)
	if err != nil {
		log.Fatalf("Failed to write temporary kubeconfig: %v", err)
	}

	if evalMode {
		// Output only the shell commands for evaluation
		fmt.Printf("export KUBECONFIG='%s'", tempFile)
		fmt.Printf("; echo '✅ Kubeconfig set for cluster %s (repository: %s)'", clusterName, repoName)
		fmt.Printf("; echo '💡 Use \"unset KUBECONFIG\" to revert'")
	} else {
		// Regular informational output
		log.Printf("✅ Temporary kubeconfig created for cluster '%s' (repository: %s)", clusterName, repoName)
		log.Printf("📁 Temporary file: %s", tempFile)
		log.Println()
		log.Println("🔧 To use this kubeconfig in your current terminal session, run:")
		log.Printf("   export KUBECONFIG='%s'", tempFile)
		log.Println()
		log.Println("💡 This will only affect your current terminal session.")
		log.Println("💡 To revert, use: unset KUBECONFIG")
		log.Println()
		log.Println("🧪 Test your connection:")
		log.Printf("   export KUBECONFIG='%s' && kubectl get nodes", tempFile)
		log.Println()
		log.Println()
		log.Println("⚡ For automatic setup, use:")
		log.Printf("   eval $(./hyve kubeconfig use %s --eval)", clusterName)
	}
}

func mergeKubeconfig(clusterName string) {
	kubeconfigMgr, _, err := createKubeconfigManager()
	if err != nil {
		log.Fatalf("Failed to create kubeconfig manager: %v", err)
	}
	defer kubeconfigMgr.Close()

	// Get kubeconfig from Hyve storage
	kc, err := kubeconfigMgr.GetKubeconfig(clusterName)
	if err != nil {
		log.Fatalf("Failed to get kubeconfig: %v", err)
	}

	if kc == nil {
		log.Fatalf("Kubeconfig not found for cluster %s. Run 'hyve kubeconfig sync' first.", clusterName)
	}

	config, err := kc.GetConfig()
	if err != nil {
		log.Fatalf("Failed to decrypt kubeconfig: %v", err)
	}

	// Get ~/.kube/config path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get user home directory: %v", err)
	}

	kubeDir := fmt.Sprintf("%s/.kube", homeDir)
	if err := os.MkdirAll(kubeDir, 0755); err != nil {
		log.Fatalf("Failed to create .kube directory: %v", err)
	}

	kubeConfigPath := fmt.Sprintf("%s/config", kubeDir)

	// Merge kubeconfig
	log.Printf("🔀 Merging cluster '%s' into %s", clusterName, kubeConfigPath)

	// Read existing config if it exists
	existingConfig := ""
	if existingData, err := os.ReadFile(kubeConfigPath); err == nil {
		existingConfig = string(existingData)
	}

	// Merge configs
	if existingConfig == "" {
		// No existing config, just use the new one
		if err := os.WriteFile(kubeConfigPath, []byte(config), 0600); err != nil {
			log.Fatalf("Failed to write kubeconfig: %v", err)
		}
	} else {
		// Create backup
		backupPath := fmt.Sprintf("%s.backup", kubeConfigPath)
		if err := os.WriteFile(backupPath, []byte(existingConfig), 0600); err != nil {
			log.Printf("⚠️  Warning: Failed to create backup at %s", backupPath)
		} else {
			log.Printf("📦 Backup created at %s", backupPath)
		}

		// Merge the kubeconfigs
		mergedContent, err := kubeconfig.MergeKubeconfigs(existingConfig, config)
		if err != nil {
			log.Fatalf("Failed to merge kubeconfigs: %v", err)
		}

		if err := os.WriteFile(kubeConfigPath, []byte(mergedContent), 0600); err != nil {
			log.Fatalf("Failed to write merged kubeconfig: %v", err)
		}
	}

	log.Printf("✅ Successfully merged cluster '%s' into %s", clusterName, kubeConfigPath)
	log.Println()
	log.Println("💡 Next steps:")
	log.Printf("   kubectl config use-context %s", clusterName)
	log.Println("   kubectl get nodes")
}

func removeKubeconfig(clusterName string) {
	// Get ~/.kube/config path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get user home directory: %v", err)
	}

	kubeConfigPath := fmt.Sprintf("%s/.kube/config", homeDir)

	// Check if config file exists
	if _, err := os.Stat(kubeConfigPath); os.IsNotExist(err) {
		log.Printf("❌ No kubeconfig found at %s", kubeConfigPath)
		return
	}

	// Read existing config
	existingData, err := os.ReadFile(kubeConfigPath)
	if err != nil {
		log.Fatalf("Failed to read kubeconfig: %v", err)
	}

	// Create backup
	backupPath := fmt.Sprintf("%s.backup", kubeConfigPath)
	if err := os.WriteFile(backupPath, existingData, 0600); err != nil {
		log.Printf("⚠️  Warning: Failed to create backup at %s", backupPath)
	} else {
		log.Printf("📦 Backup created at %s", backupPath)
	}

	log.Printf("🗑️  Removing cluster '%s' from %s", clusterName, kubeConfigPath)

	// Remove using kubectl commands
	removed := false

	// Try to delete context
	if err := kubeconfig.RemoveKubeconfigContext(string(existingData), clusterName, kubeConfigPath); err != nil {
		log.Printf("⚠️  Warning: Failed to remove context: %v", err)
	} else {
		removed = true
	}

	if removed {
		log.Printf("✅ Successfully removed cluster '%s' from %s", clusterName, kubeConfigPath)
		log.Println()
		log.Println("💡 View remaining contexts:")
		log.Println("   kubectl config get-contexts")
	} else {
		log.Printf("⚠️  Context '%s' not found in kubeconfig", clusterName)
	}
}

// migrateKubeconfigEncryption migrates kubeconfig encryption from hostname-based to portable
func migrateKubeconfigEncryption(oldHostname string) error {
	// Create kubeconfig manager
	kubeconfigMgr, repoName, err := createKubeconfigManager()
	if err != nil {
		return err
	}

	log.Printf("🔄 Starting migration for repository: %s", repoName)
	log.Printf("🔑 Old hostname: %s", oldHostname)
	log.Println()

	// Perform migration
	if err := kubeconfigMgr.MigrateEncryption(oldHostname); err != nil {
		log.Printf("❌ Migration failed: %v", err)
		return err
	}

	log.Println("✅ Migration completed successfully!")
	log.Println()
	log.Println("📝 All kubeconfigs have been re-encrypted with the new portable key format.")
	log.Println("💡 Your kubeconfigs will now work across different machines without hostname dependencies.")
	log.Println()

	return nil
}
