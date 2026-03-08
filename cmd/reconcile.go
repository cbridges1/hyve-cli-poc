package cmd

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"hyve/internal/credentials"
	"hyve/internal/reconcile"
	"hyve/internal/repository"
	"hyve/internal/state"
)

var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Reconcile clusters based on YAML files in Git repository",
	Long: `Reconcile clusters by reading cluster definitions from YAML files in the current Git repository
and ensuring the actual infrastructure matches the desired state.

When --path is provided, the given local repository path is used directly and all
reconciliation runs locally, bypassing the cicd mode check in hyve.yaml. This is
intended for use inside CI/CD pipelines that have already checked out the repository.`,
	Run: func(cmd *cobra.Command, args []string) {
		repoPath, _ := cmd.Flags().GetString("path")
		runReconciliation(repoPath)
	},
}

func init() {
	reconcileCmd.Flags().StringP("path", "p", "", "Path to a local repository checkout; bypasses cicd mode check and runs reconciliation directly")
}

func runReconciliation(repoPath string) {
	ctx := context.Background()

	var stateMgr *state.Manager

	if repoPath != "" {
		// --path provided: use the local checkout directly, skip clone/sync and cicd check
		absPath, err := filepath.Abs(repoPath)
		if err != nil {
			log.Fatalf("Invalid path %q: %v", repoPath, err)
		}
		log.Printf("Using local repository path: %s", absPath)
		stateMgr = createStateManagerFromPath(absPath)
		runLocalReconciliation(ctx, stateMgr)
		return
	}

	// No path flag: use the configured repository and honour hyve.yaml mode
	stateMgr, _ = createStateManagerFromRepository(ctx)

	// Load cluster definitions and validate regardless of mode
	clusterDefs, err := stateMgr.LoadClusterDefinitions()
	if err != nil {
		log.Fatalf("Failed to load cluster definitions: %v", err)
	}

	if err = stateMgr.ValidateClusterDefinitions(clusterDefs); err != nil {
		log.Fatalf("Invalid cluster configuration: %v", err)
	}

	// Check the reconcile mode from hyve.yaml in the repository root
	repoCfg, err := stateMgr.LoadRepoConfig()
	if err != nil {
		log.Printf("Warning: Could not load hyve.yaml: %v. Defaulting to local mode.", err)
		repoCfg = &state.RepoConfig{Reconcile: state.ReconcileConfig{Mode: state.ReconcileModeLocal}}
	}

	if repoCfg.Reconcile.Mode == state.ReconcileModeCICD {
		log.Println("Reconcile mode: cicd")
		log.Println("Skipping local reconciliation — cluster provisioning will be handled by the CI/CD pipeline.")
		log.Println("Pushing desired state to repository...")
		if err := stateMgr.CommitAndPush(ctx, "Update desired cluster state"); err != nil {
			log.Printf("❌ Failed to push state: %v", err)
			if strings.Contains(err.Error(), "failed to push") {
				log.Println("💡 Changes were committed locally but push failed")
				log.Println("💡 Check your Git credentials and network connection")
			}
		} else {
			log.Println("✅ Desired state pushed to repository. The CI/CD pipeline will reconcile.")
		}
		return
	}

	runLocalReconciliation(ctx, stateMgr)
}

// runLocalReconciliation performs full local reconciliation and commits the result.
func runLocalReconciliation(ctx context.Context, stateMgr *state.Manager) {
	clusterDefs, err := stateMgr.LoadClusterDefinitions()
	if err != nil {
		log.Fatalf("Failed to load cluster definitions: %v", err)
	}

	if err = stateMgr.ValidateClusterDefinitions(clusterDefs); err != nil {
		log.Fatalf("Invalid cluster configuration: %v", err)
	}

	clusterDefs = stateMgr.OrderClusters(clusterDefs)

	reconciler := reconcile.NewReconciler(stateMgr)
	if err = reconciler.ReconcileAll(ctx, clusterDefs); err != nil {
		log.Fatalf("Reconciliation failed: %v", err)
	}

	//TODO: Consider removing
	//log.Println("📝 Committing and pushing reconciliation changes to Git repository...")
	//if err := stateMgr.CommitAndPush(ctx, "Update cluster state after reconciliation"); err != nil {
	//	log.Printf("❌ Failed to commit and push: %v", err)
	//
	//	if strings.Contains(err.Error(), "failed to push") {
	//		log.Println("💡 Changes were committed locally but push failed")
	//		log.Println("💡 Check your Git credentials and network connection")
	//		log.Println("💡 You can manually push with: cd <repo-path> && git push")
	//	} else if strings.Contains(err.Error(), "failed to commit") {
	//		log.Println("💡 Commit operation failed - changes may still be in working directory")
	//	}
	//} else {
	//	log.Println("✅ Changes committed and pushed to remote repository successfully")
	//}

	log.Println("Cluster reconciliation completed")
}

// createStateManagerFromPath builds a state manager from an already-checked-out
// local repository. No clone or remote sync is performed.
func createStateManagerFromPath(absPath string) *state.Manager {
	stateMgr, err := state.NewManager("", absPath, "", "")
	if err != nil {
		log.Fatalf("Failed to create state manager from path %q: %v", absPath, err)
	}
	return stateMgr
}

// createStateManagerFromRepository creates state manager from current repository configuration
func createStateManagerFromRepository(ctx context.Context) (*state.Manager, string) {
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		log.Fatalf("❌ No Git repository configured. Hyve requires a Git repository for state management.\n\n" +
			"Add a Git repository with: hyve git add <name> --repo-url <url>")
	}
	log.Printf("Using Git repository: %s", currentRepo.RepoURL)

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

	if err := stateMgr.InitializeGitRepo(ctx); err != nil {
		log.Fatalf("Failed to initialize Git repository: %v", err)
	}

	if err := stateMgr.SyncWithRemote(ctx); err != nil {
		log.Fatalf("Failed to sync with remote repository: %v", err)
	}

	log.Println("Git repository synchronized")
	return stateMgr, currentRepo.LocalPath
}
