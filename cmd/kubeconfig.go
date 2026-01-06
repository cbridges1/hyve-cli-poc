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
	"civo-cluster-deploy/internal/repository"
	"civo-cluster-deploy/internal/state"
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

var kubeconfigListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all stored kubeconfigs",
	Long:  "Display all kubeconfigs stored for clusters in the current repository",
	Run: func(cmd *cobra.Command, args []string) {
		listKubeconfigs()
	},
}

func init() {
	kubeconfigGetCmd.Flags().BoolP("save", "s", false, "Save kubeconfig to ~/.kube/config-<cluster-name>")
	kubeconfigGetCmd.Flags().BoolP("merge", "m", false, "Merge kubeconfig into ~/.kube/config")
	kubeconfigGetCmd.Flags().StringP("output", "o", "", "Output file path for kubeconfig")

	kubeconfigCmd.AddCommand(kubeconfigSyncCmd)
	kubeconfigCmd.AddCommand(kubeconfigGetCmd)
	kubeconfigCmd.AddCommand(kubeconfigListCmd)
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
		return nil, fmt.Errorf("CIVO_TOKEN environment variable is required")
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
	stateMgr := state.NewManager(currentRepo.RepoURL, currentRepo.LocalPath, authUsername, authToken)

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

func syncKubeconfigs() {
	ctx := context.Background()

	// Create kubeconfig manager
	kubeconfigMgr, repoName, err := createKubeconfigManager()
	if err != nil {
		log.Fatalf("Failed to create kubeconfig manager: %v", err)
	}
	defer kubeconfigMgr.Close()

	// Create provider
	prov, err := createProviderFromCurrentRepo(ctx)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	// Create state manager to load cluster definitions
	stateMgr, _ := createStateManager(ctx)
	clusterDefs, err := stateMgr.LoadClusterDefinitions()
	if err != nil {
		log.Fatalf("Failed to load cluster definitions: %v", err)
	}

	log.Printf("📁 Syncing kubeconfigs for repository '%s'", repoName)

	// Create syncer and sync kubeconfigs
	syncer := kubeconfig.NewSyncer(kubeconfigMgr, prov)
	err = syncer.SyncKubeconfigs(ctx, clusterDefs)
	if err != nil {
		log.Fatalf("Failed to sync kubeconfigs: %v", err)
	}

	log.Println("✅ Kubeconfig sync completed")
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

func listKubeconfigs() {
	kubeconfigMgr, repoName, err := createKubeconfigManager()
	if err != nil {
		log.Fatalf("Failed to create kubeconfig manager: %v", err)
	}
	defer kubeconfigMgr.Close()

	kubeconfigs, err := kubeconfigMgr.ListKubeconfigs()
	if err != nil {
		log.Fatalf("Failed to list kubeconfigs: %v", err)
	}

	if len(kubeconfigs) == 0 {
		log.Printf("❌ No kubeconfigs stored for repository '%s'", repoName)
		log.Println("\n💡 Run 'hyve kubeconfig sync' to retrieve kubeconfigs from active clusters")
		return
	}

	log.Printf("🔑 Stored kubeconfigs for repository '%s' (%d):\n", repoName, len(kubeconfigs))

	for _, kc := range kubeconfigs {
		log.Printf("  %s", kc.ClusterName)
		log.Printf("    Repository: %s", kc.RepositoryName)
		log.Printf("    Stored: %s", kc.UpdatedAt.Format("2006-01-02 15:04:05"))
		log.Println()
	}

	log.Println("💡 Commands:")
	log.Println("  hyve kubeconfig get <cluster-name>           # Display kubeconfig")
	log.Println("  hyve kubeconfig get <cluster-name> --save    # Save to ~/.kube/config-<cluster-name>")
	log.Println("  hyve kubeconfig get <cluster-name> -o <file> # Save to specific file")
}
