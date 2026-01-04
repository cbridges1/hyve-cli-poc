package cmd

import (
	"context"
	"log"
	"os"

	"github.com/spf13/cobra"

	"civo-cluster-deploy/internal/config"
	"civo-cluster-deploy/internal/reconcile"
	"civo-cluster-deploy/internal/repository"
	"civo-cluster-deploy/internal/state"
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
	configMgr := config.NewManager()
	apiKey := configMgr.GetCivoToken()
	if apiKey == "" {
		log.Fatal("CIVO_TOKEN environment variable is required")
	}

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

	reconciler := reconcile.NewReconciler(apiKey, stateMgr)
	err = reconciler.ReconcileAll(ctx, clusterDefs)
	if err != nil {
		log.Fatalf("Reconciliation failed: %v", err)
	}

	// Commit and push changes to Git repository
	if err := stateMgr.CommitAndPush(ctx, "Update cluster state after reconciliation"); err != nil {
		log.Printf("Warning: Failed to commit changes to Git repository: %v", err)
	} else {
		log.Println("Changes committed and pushed to Git repository")
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

	// Get token from environment
	token := os.Getenv("HYVE_GIT_TOKEN")
	stateMgr := state.NewManager(currentRepo.RepoURL, currentRepo.LocalPath, currentRepo.Username, token)

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
