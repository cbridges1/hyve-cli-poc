package cmd

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"hyve/internal/config"
	"hyve/internal/credentials"
	"hyve/internal/reconcile"
	"hyve/internal/repository"
	"hyve/internal/state"
)

var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Reconcile clusters based on YAML files in Git repository",
	Long: `Reconcile clusters by reading cluster definitions from YAML files in the current Git repository
and ensuring the actual infrastructure matches the desired state.`,
	Run: func(cmd *cobra.Command, args []string) {
		runReconciliation()
	},
}

func runReconciliation() {
	ctx := context.Background()

	// Create state manager from current repository configuration
	stateMgr := createStateManagerFromRepository(ctx)

	clusterDefs, err := stateMgr.LoadClusterDefinitions()
	if err != nil {
		log.Fatalf("Failed to load cluster definitions: %v", err)
	}

	err = stateMgr.ValidateClusterDefinitions(clusterDefs)
	if err != nil {
		log.Fatalf("Invalid cluster configuration: %v", err)
	}

	clusterDefs = stateMgr.OrderClusters(clusterDefs)

	// Check if any clusters require Civo - only then require the token
	configMgr := config.NewManager()
	apiKey := configMgr.GetCivoToken()

	hasCivoClusters := false
	for _, clusterDef := range clusterDefs {
		if clusterDef.Spec.Provider == "" || clusterDef.Spec.Provider == "civo" {
			hasCivoClusters = true
			break
		}
	}

	if hasCivoClusters && apiKey == "" {
		log.Fatal("CIVO API token not found. Please run 'hyve config set-token civo' or set CIVO_TOKEN environment variable")
	}

	reconciler := reconcile.NewReconciler(apiKey, stateMgr)
	err = reconciler.ReconcileAll(ctx, clusterDefs)
	if err != nil {
		log.Fatalf("Reconciliation failed: %v", err)
	}

	// Commit and push changes to Git repository
	log.Println("📝 Committing and pushing reconciliation changes to Git repository...")
	if err := stateMgr.CommitAndPush(ctx, "Update cluster state after reconciliation"); err != nil {
		log.Printf("❌ Failed to commit and push: %v", err)

		// Provide helpful hints based on error type
		if strings.Contains(err.Error(), "failed to push") {
			log.Println("💡 Changes were committed locally but push failed")
			log.Println("💡 Check your Git credentials and network connection")
			log.Println("💡 You can manually push with: cd <repo-path> && git push")
		} else if strings.Contains(err.Error(), "failed to commit") {
			log.Println("💡 Commit operation failed - changes may still be in working directory")
		}
	} else {
		log.Println("✅ Changes committed and pushed to remote repository successfully")
	}

	log.Println("Cluster reconciliation completed")
}

// createStateManagerFromRepository creates state manager from current repository configuration
func createStateManagerFromRepository(ctx context.Context) *state.Manager {
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
			"  2. hyve reconcile\n\n" +
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
	return stateMgr
}
