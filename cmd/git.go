package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"civo-cluster-deploy/internal/git"
	"civo-cluster-deploy/internal/repository"
)

var gitCmd = &cobra.Command{
	Use:   "git",
	Short: "Manage Git repositories",
	Long:  "Configure and manage multiple Git repositories for state management",
}

var gitAddCmd = &cobra.Command{
	Use:   "add [repository-name]",
	Short: "Add a new Git repository",
	Long: `Add a new Git repository configuration for state management.
The repository name is used as a friendly identifier for switching between repositories.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repoName := args[0]
		repoURL, _ := cmd.Flags().GetString("repo-url")
		localPath, _ := cmd.Flags().GetString("local-path")
		username, _ := cmd.Flags().GetString("username")
		setCurrent, _ := cmd.Flags().GetBool("set-current")

		if repoURL == "" {
			log.Fatal("Repository URL is required. Use --repo-url flag.")
		}

		addGitRepository(repoName, repoURL, localPath, username, setCurrent)
	},
}

var gitListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured Git repositories",
	Long:  "Display all configured Git repositories and their status",
	Run: func(cmd *cobra.Command, args []string) {
		listGitRepositories()
	},
}

var gitUseCmd = &cobra.Command{
	Use:   "use [repository-name]",
	Short: "Switch to a different Git repository",
	Long:  "Set the specified repository as the current active repository",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repoName := args[0]
		switchToRepository(repoName)
	},
}

var gitStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current Git configuration status",
	Long:  "Display the current Git repository configuration and connection status",
	Run: func(cmd *cobra.Command, args []string) {
		showGitStatus()
	},
}

var gitRemoveCmd = &cobra.Command{
	Use:   "remove [repository-name]",
	Short: "Remove a Git repository configuration",
	Long:  "Remove the specified repository configuration from storage",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repoName := args[0]
		removeGitRepository(repoName)
	},
}

var gitResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset to local state management",
	Long:  "Remove all Git repository configurations and revert to local state directory",
	Run: func(cmd *cobra.Command, args []string) {
		resetGitConfiguration()
	},
}

func init() {
	gitAddCmd.Flags().StringP("repo-url", "r", "", "Git repository URL (required)")
	gitAddCmd.Flags().StringP("local-path", "l", "", "Local path to clone/store the repository (default: .hyve-state-[repo-name])")
	gitAddCmd.Flags().StringP("username", "u", "", "Git username for authentication")
	gitAddCmd.Flags().BoolP("set-current", "c", false, "Set this repository as current after adding")

	gitCmd.AddCommand(gitAddCmd)
	gitCmd.AddCommand(gitListCmd)
	gitCmd.AddCommand(gitUseCmd)
	gitCmd.AddCommand(gitStatusCmd)
	gitCmd.AddCommand(gitRemoveCmd)
	gitCmd.AddCommand(gitResetCmd)
}

func addGitRepository(name, repoURL, localPath, username string, setCurrent bool) {
	if localPath == "" {
		localPath = fmt.Sprintf(".hyve-state-%s", strings.ToLower(name))
	}

	log.Printf("Adding Git repository '%s': %s", name, repoURL)

	// Create repository manager
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	// Add repository
	repo, err := repoMgr.AddRepository(name, repoURL, localPath, username)
	if err != nil {
		log.Fatalf("Failed to add repository: %v", err)
	}

	// Set as current if requested or if it's the first repository
	if setCurrent {
		if err := repoMgr.SetCurrentRepository(name); err != nil {
			log.Fatalf("Failed to set current repository: %v", err)
		}
		repo.IsCurrent = true
	}

	// Test the connection
	log.Println("Testing Git repository connection...")

	token := os.Getenv("HYVE_GIT_TOKEN")
	ctx := context.Background()
	gitMgr := git.NewManager(repoURL, localPath, username, token)

	if err := gitMgr.InitializeRepo(ctx); err != nil {
		log.Printf("⚠️  Failed to connect to Git repository: %v", err)
		log.Println("Repository added but connection failed. Check your credentials and network.")
	} else {
		log.Println("✅ Git repository connected successfully!")
	}

	log.Printf("Repository '%s' added successfully!", name)
	log.Printf("Repository URL: %s", repoURL)
	log.Printf("Local path: %s", localPath)
	if username != "" {
		log.Printf("Username: %s", username)
	}
	if repo.IsCurrent {
		log.Println("✅ This repository is now current")
	}

	log.Println("\n💡 Tips:")
	log.Println("  - Use 'hyve git list' to see all repositories")
	log.Println("  - Use 'hyve git use <name>' to switch repositories")
	log.Println("  - Set HYVE_GIT_TOKEN env var for authentication")
}

func listGitRepositories() {
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	repos, err := repoMgr.ListRepositories()
	if err != nil {
		log.Fatalf("Failed to list repositories: %v", err)
	}

	if len(repos) == 0 {
		log.Println("❌ No Git repositories configured")
		log.Println("Using local state directory: state/clusters")
		log.Println("\nTo add a Git repository, use:")
		log.Println("  hyve git add <name> --repo-url <repository-url>")
		return
	}

	log.Printf("📁 Configured Git repositories (%d):\n", len(repos))

	for _, repo := range repos {
		status := ""
		if repo.IsCurrent {
			status = " (current) ⭐"
		}

		log.Printf("  %s%s", repo.Name, status)
		log.Printf("    URL: %s", repo.RepoURL)
		log.Printf("    Local: %s", repo.LocalPath)
		if repo.Username != "" {
			log.Printf("    User: %s", repo.Username)
		}
		log.Printf("    Added: %s", repo.CreatedAt.Format("2006-01-02 15:04"))
		log.Println()
	}

	hasToken := os.Getenv("HYVE_GIT_TOKEN") != ""
	if hasToken {
		log.Println("🔑 Authentication: ✅ HYVE_GIT_TOKEN configured")
	} else {
		log.Println("🔑 Authentication: ⚠️  HYVE_GIT_TOKEN not set")
	}
}

func switchToRepository(name string) {
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	if err := repoMgr.SetCurrentRepository(name); err != nil {
		log.Fatalf("Failed to switch repository: %v", err)
	}

	log.Printf("✅ Switched to repository '%s'", name)

	// Show current status
	repo, err := repoMgr.GetRepositoryByName(name)
	if err != nil {
		log.Fatalf("Failed to get repository details: %v", err)
	}

	log.Printf("Repository URL: %s", repo.RepoURL)
	log.Printf("Local path: %s", repo.LocalPath)
}

func showGitStatus() {
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		log.Println("❌ No current Git repository configured")
		log.Println("Using local state directory: state/clusters")
		log.Println("\nTo add a Git repository, use:")
		log.Println("  hyve git add <name> --repo-url <repository-url>")
		return
	}

	log.Printf("✅ Current repository: %s", currentRepo.Name)
	log.Printf("Repository URL: %s", currentRepo.RepoURL)
	log.Printf("Local path: %s", currentRepo.LocalPath)
	if currentRepo.Username != "" {
		log.Printf("Username: %s", currentRepo.Username)
	}

	hasToken := os.Getenv("HYVE_GIT_TOKEN") != ""
	if hasToken {
		log.Println("Authentication: ✅ Token configured")
	} else {
		log.Println("Authentication: ⚠️  No token configured")
	}

	// Test connection
	log.Println("\nTesting connection...")
	token := os.Getenv("HYVE_GIT_TOKEN")
	ctx := context.Background()
	gitMgr := git.NewManager(currentRepo.RepoURL, currentRepo.LocalPath, currentRepo.Username, token)

	if err := gitMgr.Clone(ctx); err != nil {
		log.Printf("❌ Connection failed: %v", err)
	} else {
		log.Println("✅ Connection successful")
	}
}

func removeGitRepository(name string) {
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	if err := repoMgr.DeleteRepository(name); err != nil {
		log.Fatalf("Failed to remove repository: %v", err)
	}

	log.Printf("✅ Repository '%s' removed successfully", name)

	// Show remaining repositories
	repos, err := repoMgr.ListRepositories()
	if err == nil && len(repos) > 0 {
		current, err := repoMgr.GetCurrentRepository()
		if err == nil {
			log.Printf("Current repository is now: %s", current.Name)
		}
	} else {
		log.Println("No repositories remaining. Using local state directory: state/clusters")
	}
}

func resetGitConfiguration() {
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	// List all repositories for confirmation
	repos, err := repoMgr.ListRepositories()
	if err != nil {
		log.Fatalf("Failed to list repositories: %v", err)
	}

	if len(repos) == 0 {
		log.Println("No Git repositories configured to reset")
		return
	}

	// Remove all repositories
	for _, repo := range repos {
		if err := repoMgr.DeleteRepository(repo.Name); err != nil {
			log.Printf("Failed to remove repository '%s': %v", repo.Name, err)
		}
	}

	log.Println("✅ All Git configurations reset")
	log.Println("Hyve will now use local state directory: state/clusters")
}
