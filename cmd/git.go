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
		password, _ := cmd.Flags().GetString("password")
		setCurrent, _ := cmd.Flags().GetBool("set-current")

		if repoURL == "" {
			log.Fatal("Repository URL is required. Use --repo-url flag.")
		}

		addGitRepository(repoName, repoURL, localPath, username, password, setCurrent)
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

var gitCredentialsCmd = &cobra.Command{
	Use:   "credentials [repository-name]",
	Short: "Update stored credentials for a Git repository",
	Long:  "Update the username and password stored for the specified repository",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repoName := args[0]
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")
		updateGitCredentials(repoName, username, password)
	},
}

func init() {
	gitAddCmd.Flags().StringP("repo-url", "r", "", "Git repository URL (required)")
	gitAddCmd.Flags().StringP("local-path", "l", "", "Local path to clone/store the repository (default: .hyve-state-[repo-name])")
	gitAddCmd.Flags().StringP("username", "u", "", "Git username for authentication")
	gitAddCmd.Flags().StringP("password", "p", "", "Git password or personal access token for authentication")
	gitAddCmd.Flags().BoolP("set-current", "c", false, "Set this repository as current after adding")

	gitCredentialsCmd.Flags().StringP("username", "u", "", "Git username for authentication")
	gitCredentialsCmd.Flags().StringP("password", "p", "", "Git password or personal access token for authentication")

	gitCmd.AddCommand(gitAddCmd)
	gitCmd.AddCommand(gitListCmd)
	gitCmd.AddCommand(gitUseCmd)
	gitCmd.AddCommand(gitStatusCmd)
	gitCmd.AddCommand(gitRemoveCmd)
	gitCmd.AddCommand(gitResetCmd)
	gitCmd.AddCommand(gitCredentialsCmd)
}

func addGitRepository(name, repoURL, localPath, username, password string, setCurrent bool) {
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
	repo, err := repoMgr.AddRepository(name, repoURL, localPath, username, password)
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

	// Use stored password if provided, otherwise fall back to token
	var authToken string
	if password != "" {
		authToken = password
	} else {
		authToken = os.Getenv("HYVE_GIT_TOKEN")
	}
	ctx := context.Background()
	gitMgr := git.NewManager(repoURL, localPath, username, authToken)

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
	if password != "" {
		log.Printf("Authentication: ✅ Password stored securely")
	}
	if repo.IsCurrent {
		log.Println("✅ This repository is now current")
	}

	log.Println("\n💡 Tips:")
	log.Println("  - Use 'hyve git list' to see all repositories")
	log.Println("  - Use 'hyve git use <name>' to switch repositories")
	log.Println("  - Use 'hyve git credentials <name>' to update stored credentials")
	log.Println("  - Set HYVE_GIT_TOKEN env var as fallback authentication")
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
		log.Println("\nHyve requires at least one Git repository for state management.")
		log.Println("To add a Git repository, use:")
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
			// Check if password is stored
			if storedPassword, _ := repo.GetPassword(); storedPassword != "" {
				log.Printf("    Auth: ✅ Stored credentials")
			} else {
				log.Printf("    Auth: ⚠️  Using environment token")
			}
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
		log.Println("❌ No Git repository configured")
		log.Println("\nHyve requires a Git repository for state management.")
		log.Println("To add a Git repository, use:")
		log.Println("  hyve git add <name> --repo-url <repository-url>")
		return
	}

	log.Printf("✅ Current repository: %s", currentRepo.Name)
	log.Printf("Repository URL: %s", currentRepo.RepoURL)
	log.Printf("Local path: %s", currentRepo.LocalPath)
	if currentRepo.Username != "" {
		log.Printf("Username: %s", currentRepo.Username)
	}

	// Check authentication options
	storedPassword, _ := currentRepo.GetPassword()
	envToken := os.Getenv("HYVE_GIT_TOKEN")

	if storedPassword != "" {
		log.Println("Authentication: ✅ Stored credentials configured")
	} else if envToken != "" {
		log.Println("Authentication: ✅ Environment token configured")
	} else {
		log.Println("Authentication: ⚠️  No authentication configured")
	}

	// Test connection
	log.Println("\nTesting connection...")
	var authToken string
	if storedPassword != "" {
		authToken = storedPassword
	} else {
		authToken = envToken
	}
	ctx := context.Background()
	gitMgr := git.NewManager(currentRepo.RepoURL, currentRepo.LocalPath, currentRepo.Username, authToken)

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
		log.Println("No repositories remaining. Add a new repository to continue using Hyve.")
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
	log.Println("Add a Git repository to continue using Hyve: hyve git add <name> --repo-url <url>")
}

func updateGitCredentials(repoName, username, password string) {
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	// Get current repository details
	repo, err := repoMgr.GetRepositoryByName(repoName)
	if err != nil {
		log.Fatalf("Failed to get repository: %v", err)
	}

	// Use current values if not provided
	if username == "" {
		username = repo.Username
	}
	if password == "" {
		// Prompt for password if not provided
		log.Print("Enter password/token (input will be hidden): ")
		// For now, just show a message - in a real implementation you'd use a secure input method
		log.Println("⚠️  Password must be provided via --password flag for security")
		return
	}

	// Update the repository
	_, err = repoMgr.UpdateRepository(repoName, repo.RepoURL, repo.LocalPath, username, password)
	if err != nil {
		log.Fatalf("Failed to update credentials: %v", err)
	}

	log.Printf("✅ Credentials updated for repository '%s'", repoName)
	if username != "" {
		log.Printf("Username: %s", username)
	}
	log.Println("Password: ✅ Stored securely")

	// Test connection with new credentials
	log.Println("\nTesting connection with updated credentials...")
	ctx := context.Background()
	gitMgr := git.NewManager(repo.RepoURL, repo.LocalPath, username, password)

	if err := gitMgr.InitializeRepo(ctx); err != nil {
		log.Printf("⚠️  Connection test failed: %v", err)
		log.Println("Credentials saved but connection failed. Check your credentials and network.")
	} else {
		log.Println("✅ Connection successful with new credentials!")
	}
}
