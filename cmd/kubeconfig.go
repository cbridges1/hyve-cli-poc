package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"hyve/internal/config"
	"hyve/internal/kubeconfig"
	"hyve/internal/provider"
	"hyve/internal/providerconfig"
	"hyve/internal/repository"
	"hyve/internal/types"
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
	Short: "Merge cluster into ~/.kube/config and set as active context",
	Long:  "Merge the cluster's kubeconfig into ~/.kube/config and set it as the active kubectl context",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		clusterName := args[0]
		useKubeconfig(clusterName)
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

// createProviderForCluster creates a provider with the appropriate options for a specific cluster
func createProviderForCluster(factory *provider.Factory, clusterDef types.ClusterDefinition) (provider.Provider, error) {
	providerName := clusterDef.Spec.Provider
	if providerName == "" {
		providerName = "civo" // default
	}

	opts := provider.ProviderOptions{
		Region: clusterDef.Metadata.Region,
	}

	// Populate AccountName so the factory can resolve named env vars.
	switch strings.ToLower(providerName) {
	case "civo":
		opts.AccountName = clusterDef.Spec.CivoOrganization
	case "aws":
		opts.AccountName = clusterDef.Spec.AWSAccount
	case "gcp":
		opts.AccountName = clusterDef.Spec.GCPProject
	case "azure":
		opts.AccountName = clusterDef.Spec.AzureSubscription
	}

	// Handle Civo-specific configuration
	if providerName == "civo" {
		configMgr := config.NewManager()
		apiKey := configMgr.GetCivoToken(clusterDef.Spec.CivoOrganization)
		if apiKey == "" {
			apiKey = os.Getenv("CIVO_TOKEN")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("Civo API token not found. Please run 'hyve config civo token set --org %s' or set CIVO_TOKEN environment variable", clusterDef.Spec.CivoOrganization)
		}
		opts.APIKey = apiKey
	}

	// Handle GCP-specific configuration
	if providerName == "gcp" {
		if clusterDef.Spec.GCPProjectID != "" {
			opts.ProjectID = clusterDef.Spec.GCPProjectID
		} else if clusterDef.Spec.GCPProject != "" {
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

	// Handle Azure-specific configuration
	if providerName == "azure" {
		opts.AzureResourceGroup = clusterDef.Spec.AzureResourceGroup
		if clusterDef.Spec.AzureSubscriptionID != "" {
			opts.AzureSubscriptionID = clusterDef.Spec.AzureSubscriptionID
		} else if clusterDef.Spec.AzureSubscription != "" {
			azRepoMgr, err := repository.NewManager()
			if err == nil {
				defer azRepoMgr.Close()
				if currentRepo, err := azRepoMgr.GetCurrentRepository(); err == nil {
					pcMgr := providerconfig.NewManager(currentRepo.LocalPath)
					if subscriptionID, err := pcMgr.GetAzureSubscriptionID(clusterDef.Spec.AzureSubscription); err == nil {
						opts.AzureSubscriptionID = subscriptionID
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

	// Collect all active cluster names for a single orphan-cleanup pass at the end.
	// Calling SyncKubeconfigs (which runs CleanupOrphanedKubeconfigs internally) once
	// per cluster would delete every other cluster's kubeconfig on each iteration.
	activeClusterNames := make([]string, 0, len(clusterDefs))
	for _, cd := range clusterDefs {
		activeClusterNames = append(activeClusterNames, cd.Metadata.Name)
	}

	for _, clusterDef := range clusterDefs {
		prov, err := createProviderForCluster(providerFactory, clusterDef)
		if err != nil {
			log.Printf("Failed to create provider for cluster %s: %v", clusterDef.Metadata.Name, err)
			continue
		}

		syncer := kubeconfig.NewSyncer(kubeconfigMgr, prov)
		if err := syncer.SyncSingleKubeconfig(ctx, clusterDef.Metadata.Name); err != nil {
			log.Printf("Failed to sync kubeconfig for cluster %s: %v", clusterDef.Metadata.Name, err)
			continue
		}
		successCount++
	}

	if err := kubeconfigMgr.CleanupOrphanedKubeconfigs(activeClusterNames); err != nil {
		log.Printf("Failed to cleanup orphaned kubeconfigs: %v", err)
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

func useKubeconfig(clusterName string) {
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

	// Merge kubeconfig into ~/.kube/config
	log.Printf("🔀 Merging cluster '%s' into %s", clusterName, kubeConfigPath)

	existingConfig := ""
	if existingData, err := os.ReadFile(kubeConfigPath); err == nil {
		existingConfig = string(existingData)
	}

	if existingConfig == "" {
		if err := os.WriteFile(kubeConfigPath, []byte(config), 0600); err != nil {
			log.Fatalf("Failed to write kubeconfig: %v", err)
		}
	} else {
		backupPath := fmt.Sprintf("%s.backup", kubeConfigPath)
		if err := os.WriteFile(backupPath, []byte(existingConfig), 0600); err != nil {
			log.Printf("⚠️  Warning: Failed to create backup at %s", backupPath)
		} else {
			log.Printf("📦 Backup created at %s", backupPath)
		}

		mergedContent, err := kubeconfig.MergeKubeconfigs(existingConfig, config)
		if err != nil {
			log.Fatalf("Failed to merge kubeconfigs: %v", err)
		}

		if err := os.WriteFile(kubeConfigPath, []byte(mergedContent), 0600); err != nil {
			log.Fatalf("Failed to write merged kubeconfig: %v", err)
		}
	}

	log.Printf("✅ Merged cluster '%s' into %s", clusterName, kubeConfigPath)

	// Set the active context
	useCtxCmd := exec.Command("kubectl", "config", "use-context", clusterName)
	useCtxCmd.Stdout = os.Stdout
	useCtxCmd.Stderr = os.Stderr
	if err := useCtxCmd.Run(); err != nil {
		log.Printf("⚠️  Failed to set context: %v", err)
		log.Printf("   Run manually: kubectl config use-context %s", clusterName)
	} else {
		log.Printf("✅ Active context set to '%s'", clusterName)
		log.Println()
		log.Println("💡 Test your connection:")
		log.Println("   kubectl get nodes")
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
