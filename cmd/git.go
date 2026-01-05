package cmd

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"civo-cluster-deploy/internal/credentials"
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
		username, _ := cmd.Flags().GetString("username")
		setCurrent, _ := cmd.Flags().GetBool("set-current")

		if repoURL == "" {
			log.Fatal("Repository URL is required. Use --repo-url flag.")
		}

		addGitRepository(repoName, repoURL, username, setCurrent)
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
	Use:   "credentials",
	Short: "Manage global Git credentials",
	Long:  "Store, update, or view global Git credentials used for authentication",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")
		shouldClear, _ := cmd.Flags().GetBool("clear")

		if shouldClear {
			clearGitCredentials()
		} else if username != "" || password != "" {
			updateGitCredentials(username, password)
		} else {
			showGitCredentials()
		}
	},
}

func init() {
	gitAddCmd.Flags().StringP("repo-url", "r", "", "Git repository URL (required)")
	gitAddCmd.Flags().StringP("username", "u", "", "Git username for authentication (stored in repository config)")
	gitAddCmd.Flags().BoolP("set-current", "c", false, "Set this repository as current after adding")

	gitCredentialsCmd.Flags().StringP("username", "u", "", "Git username for authentication")
	gitCredentialsCmd.Flags().StringP("password", "p", "", "Git password or personal access token for authentication")
	gitCredentialsCmd.Flags().Bool("clear", false, "Clear all stored credentials")

	gitCmd.AddCommand(gitAddCmd)
	gitCmd.AddCommand(gitListCmd)
	gitCmd.AddCommand(gitUseCmd)
	gitCmd.AddCommand(gitStatusCmd)
	gitCmd.AddCommand(gitRemoveCmd)
	gitCmd.AddCommand(gitResetCmd)
	gitCmd.AddCommand(gitCredentialsCmd)
}

func addGitRepository(name, repoURL, username string, setCurrent bool) {
	// Generate local path in centralized repositories directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	repositoriesDir := filepath.Join(homeDir, ".hyve", "repositories")
	localPath := filepath.Join(repositoriesDir, strings.ToLower(name))

	// Ensure repositories directory exists
	if err := os.MkdirAll(repositoriesDir, 0755); err != nil {
		log.Printf("Warning: Failed to create repositories directory: %v", err)
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

	// Get global credentials or fall back to environment token
	credsMgr, err := credentials.NewManager()
	if err == nil {
		defer credsMgr.Close()
	}

	var authToken string
	var authUsername string = username

	if credsMgr != nil {
		if creds, err := credsMgr.GetCredentials(); err == nil && creds != nil {
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

	ctx := context.Background()
	gitMgr := git.NewManager(repoURL, localPath, authUsername, authToken)

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
	// Check if global credentials are configured
	if credsMgr != nil {
		if creds, _ := credsMgr.GetCredentials(); creds != nil {
			log.Printf("Authentication: ✅ Global credentials configured")
		}
	}
	if repo.IsCurrent {
		log.Println("✅ This repository is now current")
	}

	log.Println("\n💡 Tips:")
	log.Println("  - Use 'hyve git list' to see all repositories")
	log.Println("  - Use 'hyve git use <name>' to switch repositories")
	log.Println("  - Use 'hyve git credentials' to manage global Git authentication")
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
		}
		log.Printf("    Added: %s", repo.CreatedAt.Format("2006-01-02 15:04"))
		log.Println()
	}

	// Check global credentials
	credsMgr, err := credentials.NewManager()
	if err == nil {
		defer credsMgr.Close()
		if creds, _ := credsMgr.GetCredentials(); creds != nil {
			log.Printf("🔑 Authentication: ✅ Global credentials configured (%s)", creds.Username)
		} else {
			log.Println("🔑 Authentication: ⚠️  No global credentials stored")
		}
	}

	hasToken := os.Getenv("HYVE_GIT_TOKEN") != ""
	if hasToken {
		log.Println("🔑 Environment Fallback: ✅ HYVE_GIT_TOKEN configured")
	} else {
		log.Println("🔑 Environment Fallback: ⚠️  HYVE_GIT_TOKEN not set")
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
	credsMgr, err := credentials.NewManager()
	var globalCreds *credentials.Credentials
	if err == nil {
		defer credsMgr.Close()
		globalCreds, _ = credsMgr.GetCredentials()
	}

	envToken := os.Getenv("HYVE_GIT_TOKEN")

	if globalCreds != nil {
		log.Printf("Authentication: ✅ Global credentials configured (%s)", globalCreds.Username)
	} else if envToken != "" {
		log.Println("Authentication: ✅ Environment token configured")
	} else {
		log.Println("Authentication: ⚠️  No authentication configured")
	}

	// Test connection
	log.Println("\nTesting connection...")
	var authToken string
	var authUsername = currentRepo.Username

	if globalCreds != nil {
		if password, err := globalCreds.GetPassword(); err == nil && password != "" {
			authToken = password
			if authUsername == "" {
				authUsername = globalCreds.Username
			}
		}
	}

	if authToken == "" {
		authToken = envToken
	}
	ctx := context.Background()
	gitMgr := git.NewManager(currentRepo.RepoURL, currentRepo.LocalPath, authUsername, authToken)

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

func updateGitCredentials(username, password string) {
	credsMgr, err := credentials.NewManager()
	if err != nil {
		log.Fatalf("Failed to create credentials manager: %v", err)
	}
	defer credsMgr.Close()

	// Get existing credentials if any
	existing, _ := credsMgr.GetCredentials()

	// Use existing values if not provided
	if username == "" && existing != nil {
		username = existing.Username
	}
	if password == "" {
		log.Println("⚠️  Password must be provided via --password flag for security")
		return
	}
	if username == "" {
		log.Println("⚠️  Username must be provided via --username flag")
		return
	}

	// Store the credentials
	_, err = credsMgr.StoreCredentials(username, password)
	if err != nil {
		log.Fatalf("Failed to store credentials: %v", err)
	}

	log.Println("✅ Global Git credentials stored securely")
	log.Printf("Username: %s", username)
	log.Println("Password: ✅ Stored and encrypted")
}

func showGitCredentials() {
	credsMgr, err := credentials.NewManager()
	if err != nil {
		log.Fatalf("Failed to create credentials manager: %v", err)
	}
	defer credsMgr.Close()

	creds, err := credsMgr.GetCredentials()
	if err != nil {
		log.Fatalf("Failed to get credentials: %v", err)
	}

	if creds == nil {
		log.Println("❌ No Git credentials stored")
		log.Println("\nTo store credentials:")
		log.Println("  hyve git credentials --username <user> --password <token>")
		log.Println("\nOr use environment variable as fallback:")
		log.Println("  export HYVE_GIT_TOKEN=<your-token>")
		return
	}

	log.Println("✅ Global Git credentials:")
	log.Printf("Username: %s", creds.Username)
	log.Println("Password: ✅ Stored and encrypted")
	log.Printf("Updated: %s", creds.UpdatedAt.Format("2006-01-02 15:04:05"))

	// Check environment token as well
	envToken := os.Getenv("HYVE_GIT_TOKEN")
	if envToken != "" {
		log.Println("\n💡 Environment token also available as fallback")
	}
}

func clearGitCredentials() {
	credsMgr, err := credentials.NewManager()
	if err != nil {
		log.Fatalf("Failed to create credentials manager: %v", err)
	}
	defer credsMgr.Close()

	err = credsMgr.ClearCredentials()
	if err != nil {
		log.Fatalf("Failed to clear credentials: %v", err)
	}

	log.Println("✅ All Git credentials cleared")
	log.Println("\n💡 You can still use HYVE_GIT_TOKEN environment variable for authentication")
}
