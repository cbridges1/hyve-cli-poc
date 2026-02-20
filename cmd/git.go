package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"hyve/internal/credentials"
	"hyve/internal/git"
	"hyve/internal/repository"
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

var gitCredentialsMigrateCmd = &cobra.Command{
	Use:   "credentials-migrate [old-hostname]",
	Short: "Migrate credentials encryption to new portable format",
	Long: `Migrate credentials encryption from hostname-based keys to portable keys.

This command re-encrypts your stored credentials using a key that doesn't include the hostname,
making the database portable across machines. You need to provide the hostname that was
used when the credentials were originally encrypted.

Example:
  hyve git credentials-migrate "old-macbook.local"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldHostname := args[0]
		return migrateGitCredentialsEncryption(oldHostname)
	},
}

var gitBranchCmd = &cobra.Command{
	Use:   "branch",
	Short: "Manage Git branches",
	Long:  "Create, list, delete, and switch between Git branches in the current repository",
}

var gitBranchListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all branches",
	Long:  "List all branches in the current Git repository",
	Run: func(cmd *cobra.Command, args []string) {
		listGitBranches()
	},
}

var gitBranchCreateCmd = &cobra.Command{
	Use:   "create [branch-name]",
	Short: "Create a new branch",
	Long:  "Create a new branch from the current HEAD",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		branchName := args[0]
		switchToBranch, _ := cmd.Flags().GetBool("switch")
		push, _ := cmd.Flags().GetBool("push")
		createGitBranch(branchName, switchToBranch, push)
	},
}

var gitBranchDeleteCmd = &cobra.Command{
	Use:   "delete [branch-name]",
	Short: "Delete a branch",
	Long:  "Delete a branch from the local repository",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		branchName := args[0]
		force, _ := cmd.Flags().GetBool("force")
		deleteGitBranch(branchName, force)
	},
}

var gitBranchSwitchCmd = &cobra.Command{
	Use:   "switch [branch-name]",
	Short: "Switch to a branch",
	Long:  "Switch to a different branch (git checkout)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		branchName := args[0]
		pull, _ := cmd.Flags().GetBool("pull")
		switchGitBranch(branchName, pull)
	},
}

var gitPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull changes from remote",
	Long:  "Pull the latest changes from the remote repository for the current branch",
	Run: func(cmd *cobra.Command, args []string) {
		pullGitChanges()
	},
}

var gitPushCmd = &cobra.Command{
	Use:   "push [commit-message]",
	Short: "Stage, commit, and push changes",
	Long: `Stage all changes, commit with a message, and push to remote.

This is a convenience command that combines:
  - git add .
  - git commit -m "message"
  - git push

If no commit message is provided, a default message based on the changes will be used.`,
	Run: func(cmd *cobra.Command, args []string) {
		var message string
		if len(args) > 0 {
			message = args[0]
		}
		pushGitChanges(message)
	},
}

var gitSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync with remote (pull and push)",
	Long: `Pull latest changes from remote and push any local changes.

This command:
  1. Pulls latest changes from remote
  2. If there are local changes, prompts for commit message
  3. Commits and pushes local changes`,
	Run: func(cmd *cobra.Command, args []string) {
		var message string
		if len(args) > 0 {
			message = args[0]
		}
		syncGitChanges(message)
	},
}

func init() {
	gitAddCmd.Flags().StringP("repo-url", "r", "", "Git repository URL (required)")
	gitAddCmd.Flags().StringP("username", "u", "", "Git username for authentication (stored in repository config)")
	gitAddCmd.Flags().BoolP("set-current", "c", false, "Set this repository as current after adding")

	gitCredentialsCmd.Flags().StringP("username", "u", "", "Git username for authentication")
	gitCredentialsCmd.Flags().StringP("password", "p", "", "Git password or personal access token for authentication")
	gitCredentialsCmd.Flags().Bool("clear", false, "Clear all stored credentials")

	gitBranchCreateCmd.Flags().BoolP("switch", "s", false, "Switch to the new branch after creating it")
	gitBranchCreateCmd.Flags().BoolP("push", "p", false, "Push the branch to remote after creating it")

	gitBranchDeleteCmd.Flags().BoolP("force", "f", false, "Force delete the branch")

	gitBranchSwitchCmd.Flags().BoolP("pull", "p", false, "Pull latest changes after switching")

	gitBranchCmd.AddCommand(gitBranchListCmd)
	gitBranchCmd.AddCommand(gitBranchCreateCmd)
	gitBranchCmd.AddCommand(gitBranchDeleteCmd)
	gitBranchCmd.AddCommand(gitBranchSwitchCmd)

	gitCmd.AddCommand(gitAddCmd)
	gitCmd.AddCommand(gitListCmd)
	gitCmd.AddCommand(gitUseCmd)
	gitCmd.AddCommand(gitStatusCmd)
	gitCmd.AddCommand(gitRemoveCmd)
	gitCmd.AddCommand(gitResetCmd)
	gitCmd.AddCommand(gitCredentialsCmd)
	gitCmd.AddCommand(gitCredentialsMigrateCmd)
	gitCmd.AddCommand(gitBranchCmd)
	gitCmd.AddCommand(gitPullCmd)
	gitCmd.AddCommand(gitPushCmd)
	gitCmd.AddCommand(gitSyncCmd)
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
	gitMgr, err := git.NewBackend(repoURL, localPath, authUsername, authToken)
	if err != nil {
		log.Fatalf("Failed to create git backend: %v", err)
	}

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
	gitMgr, err := git.NewBackend(currentRepo.RepoURL, currentRepo.LocalPath, authUsername, authToken)
	if err != nil {
		log.Fatalf("Failed to create git backend: %v", err)
	}

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

// migrateGitCredentialsEncryption migrates credentials encryption from hostname-based to portable
func migrateGitCredentialsEncryption(oldHostname string) error {
	credsMgr, err := credentials.NewManager()
	if err != nil {
		return err
	}
	defer credsMgr.Close()

	log.Println("🔄 Starting credentials encryption migration")
	log.Printf("🔑 Old hostname: %s", oldHostname)
	log.Println()

	// Perform migration
	if err := credsMgr.MigrateEncryption(oldHostname); err != nil {
		log.Printf("❌ Migration failed: %v", err)
		return err
	}

	log.Println("✅ Migration completed successfully!")
	log.Println()
	log.Println("📝 Your credentials have been re-encrypted with the new portable key format.")
	log.Println("💡 Your credentials will now work across different machines without hostname dependencies.")
	log.Println()

	return nil
}

func listGitBranches() {
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		log.Println("❌ No Git repository configured")
		log.Println("Add a Git repository first: hyve git add <name> --repo-url <url>")
		return
	}

	// Get authentication
	authToken, authUsername := getGitAuth(currentRepo)

	ctx := context.Background()
	gitMgr, err := git.NewBackend(currentRepo.RepoURL, currentRepo.LocalPath, authUsername, authToken)
	if err != nil {
		log.Fatalf("Failed to create git backend: %v", err)
	}

	// Initialize/open repository
	if err := gitMgr.InitializeRepo(ctx); err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}

	// List branches
	branches, err := gitMgr.ListBranches(ctx)
	if err != nil {
		log.Fatalf("Failed to list branches: %v", err)
	}

	if len(branches) == 0 {
		log.Println("No branches found in repository")
		return
	}

	log.Printf("🌿 Branches in repository '%s':\n", currentRepo.Name)
	for _, branch := range branches {
		marker := "  "
		if branch.IsCurrent {
			marker = "* "
		}
		log.Printf("%s%s (%s)", marker, branch.Name, branch.Hash)
	}

	log.Println("\n💡 Commands:")
	log.Println("  hyve git branch create <name>    # Create new branch")
	log.Println("  hyve git branch switch <name>    # Switch to branch")
	log.Println("  hyve git branch delete <name>    # Delete branch")
}

func createGitBranch(branchName string, switchToBranch, push bool) {
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		log.Println("❌ No Git repository configured")
		return
	}

	authToken, authUsername := getGitAuth(currentRepo)

	ctx := context.Background()
	gitMgr, err := git.NewBackend(currentRepo.RepoURL, currentRepo.LocalPath, authUsername, authToken)
	if err != nil {
		log.Fatalf("Failed to create git backend: %v", err)
	}

	if err := gitMgr.InitializeRepo(ctx); err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}

	// Get current branch for display
	currentBranch, err := gitMgr.GetCurrentBranch(ctx)
	if err != nil {
		log.Fatalf("Failed to get current branch: %v", err)
	}

	// Create branch
	log.Printf("Creating branch '%s' from '%s'...", branchName, currentBranch)
	if err := gitMgr.CreateBranch(ctx, branchName); err != nil {
		log.Fatalf("Failed to create branch: %v", err)
	}

	log.Printf("✅ Branch '%s' created successfully", branchName)

	// Switch if requested
	if switchToBranch {
		if err := gitMgr.SwitchBranch(ctx, branchName); err != nil {
			log.Fatalf("Failed to switch to branch: %v", err)
		}
		log.Printf("✅ Switched to branch '%s'", branchName)
	}

	// Push if requested
	if push {
		log.Printf("Pushing branch '%s' to remote...", branchName)
		if err := gitMgr.PushBranch(ctx, branchName); err != nil {
			log.Printf("⚠️  Failed to push branch: %v", err)
		} else {
			log.Printf("✅ Branch '%s' pushed to remote", branchName)
		}
	}

	if !switchToBranch {
		log.Printf("\n💡 Switch to this branch with: hyve git branch switch %s", branchName)
	}
}

func deleteGitBranch(branchName string, force bool) {
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		log.Println("❌ No Git repository configured")
		return
	}

	authToken, authUsername := getGitAuth(currentRepo)

	ctx := context.Background()
	gitMgr, err := git.NewBackend(currentRepo.RepoURL, currentRepo.LocalPath, authUsername, authToken)
	if err != nil {
		log.Fatalf("Failed to create git backend: %v", err)
	}

	if err := gitMgr.InitializeRepo(ctx); err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}

	// Delete branch
	log.Printf("Deleting branch '%s'...", branchName)
	if err := gitMgr.DeleteBranch(ctx, branchName, force); err != nil {
		log.Fatalf("Failed to delete branch: %v", err)
	}

	log.Printf("✅ Branch '%s' deleted successfully", branchName)
	log.Println("\n💡 The branch has been deleted locally.")
	log.Println("💡 To delete from remote, use: git push origin --delete " + branchName)
}

func switchGitBranch(branchName string, pull bool) {
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		log.Println("❌ No Git repository configured")
		return
	}

	authToken, authUsername := getGitAuth(currentRepo)

	ctx := context.Background()
	gitMgr, err := git.NewBackend(currentRepo.RepoURL, currentRepo.LocalPath, authUsername, authToken)
	if err != nil {
		log.Fatalf("Failed to create git backend: %v", err)
	}

	if err := gitMgr.InitializeRepo(ctx); err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}

	// Get current branch
	currentBranch, err := gitMgr.GetCurrentBranch(ctx)
	if err != nil {
		log.Fatalf("Failed to get current branch: %v", err)
	}

	if currentBranch == branchName {
		log.Printf("Already on branch '%s'", branchName)
		if pull {
			log.Println("Pulling latest changes...")
			if err := gitMgr.Pull(ctx); err != nil {
				log.Printf("⚠️  Failed to pull: %v", err)
			} else {
				log.Println("✅ Pulled latest changes")
			}
		}
		return
	}

	// Switch branch
	log.Printf("Switching from '%s' to '%s'...", currentBranch, branchName)
	if err := gitMgr.SwitchBranch(ctx, branchName); err != nil {
		log.Fatalf("Failed to switch branch: %v", err)
	}

	log.Printf("✅ Switched to branch '%s'", branchName)

	// Pull if requested
	if pull {
		log.Println("Pulling latest changes...")
		if err := gitMgr.Pull(ctx); err != nil {
			log.Printf("⚠️  Failed to pull: %v", err)
		} else {
			log.Println("✅ Pulled latest changes")
		}
	}

	log.Println("\n💡 Your working directory now reflects the '" + branchName + "' branch")
	log.Println("💡 Changes made will be tracked on this branch")
}

// getGitAuth retrieves Git authentication credentials
func getGitAuth(repo *repository.Repository) (token, username string) {
	username = repo.Username

	// Try global credentials first
	credsMgr, err := credentials.NewManager()
	if err == nil {
		defer credsMgr.Close()
		if creds, err := credsMgr.GetCredentials(); err == nil && creds != nil {
			if password, err := creds.GetPassword(); err == nil && password != "" {
				token = password
				if username == "" {
					username = creds.Username
				}
			}
		}
	}

	// Fall back to environment token
	if token == "" {
		token = os.Getenv("HYVE_GIT_TOKEN")
	}

	return token, username
}

func pullGitChanges() {
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		log.Println("❌ No Git repository configured")
		return
	}

	authToken, authUsername := getGitAuth(currentRepo)

	ctx := context.Background()
	gitMgr, err := git.NewBackend(currentRepo.RepoURL, currentRepo.LocalPath, authUsername, authToken)
	if err != nil {
		log.Fatalf("Failed to create git backend: %v", err)
	}

	if err := gitMgr.InitializeRepo(ctx); err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}

	// Get current branch
	currentBranch, err := gitMgr.GetCurrentBranch(ctx)
	if err != nil {
		log.Fatalf("Failed to get current branch: %v", err)
	}

	log.Printf("Pulling latest changes from '%s'...", currentBranch)

	// Pull changes
	if err := gitMgr.Pull(ctx); err != nil {
		log.Fatalf("Failed to pull changes: %v", err)
	}

	log.Println("✅ Successfully pulled latest changes")
	log.Println("\n💡 Your local branch is now up to date with remote")
}

func pushGitChanges(message string) {
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		log.Println("❌ No Git repository configured")
		return
	}

	authToken, authUsername := getGitAuth(currentRepo)

	ctx := context.Background()
	gitMgr, err := git.NewBackend(currentRepo.RepoURL, currentRepo.LocalPath, authUsername, authToken)
	if err != nil {
		log.Fatalf("Failed to create git backend: %v", err)
	}

	if err := gitMgr.InitializeRepo(ctx); err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}

	// Check if there are uncommitted changes
	hasChanges, err := gitMgr.HasUncommittedChanges(ctx)
	if err != nil {
		log.Fatalf("Failed to check for changes: %v", err)
	}

	if !hasChanges {
		log.Println("No changes to commit")
		log.Println("\n💡 Working tree is clean")
		return
	}

	// Get status summary
	statusSummary, err := gitMgr.GetStatusSummary(ctx)
	if err != nil {
		log.Fatalf("Failed to get status: %v", err)
	}

	log.Printf("📝 Changes detected: %s", statusSummary)

	// Use default message if not provided
	if message == "" {
		message = fmt.Sprintf("Update: %s", statusSummary)
		log.Printf("Using default commit message: %s", message)
	}

	// Get current branch
	currentBranch, err := gitMgr.GetCurrentBranch(ctx)
	if err != nil {
		log.Fatalf("Failed to get current branch: %v", err)
	}

	// Stage, commit, and push
	log.Printf("Committing changes to '%s'...", currentBranch)
	if err := gitMgr.Commit(ctx, message); err != nil {
		log.Fatalf("Failed to commit changes: %v", err)
	}

	log.Println("✅ Changes committed successfully")

	log.Printf("Pushing to remote '%s'...", currentBranch)
	if err := gitMgr.Push(ctx); err != nil {
		log.Fatalf("Failed to push changes: %v", err)
	}

	log.Println("✅ Changes pushed successfully")
	log.Printf("\n💡 Branch '%s' is now synchronized with remote", currentBranch)
}

func syncGitChanges(message string) {
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		log.Println("❌ No Git repository configured")
		return
	}

	authToken, authUsername := getGitAuth(currentRepo)

	ctx := context.Background()
	gitMgr, err := git.NewBackend(currentRepo.RepoURL, currentRepo.LocalPath, authUsername, authToken)
	if err != nil {
		log.Fatalf("Failed to create git backend: %v", err)
	}

	if err := gitMgr.InitializeRepo(ctx); err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}

	// Get current branch
	currentBranch, err := gitMgr.GetCurrentBranch(ctx)
	if err != nil {
		log.Fatalf("Failed to get current branch: %v", err)
	}

	log.Printf("🔄 Syncing branch '%s' with remote...", currentBranch)

	// First, pull changes
	log.Println("1. Pulling latest changes from remote...")
	if err := gitMgr.Pull(ctx); err != nil {
		log.Printf("⚠️  Failed to pull changes: %v", err)
	} else {
		log.Println("✅ Pulled latest changes")
	}

	// Check if there are uncommitted changes
	hasChanges, err := gitMgr.HasUncommittedChanges(ctx)
	if err != nil {
		log.Fatalf("Failed to check for changes: %v", err)
	}

	if !hasChanges {
		log.Println("\n✅ Repository is synchronized")
		log.Println("💡 No local changes to push")
		return
	}

	// Get status summary
	statusSummary, err := gitMgr.GetStatusSummary(ctx)
	if err != nil {
		log.Fatalf("Failed to get status: %v", err)
	}

	log.Printf("\n2. Local changes detected: %s", statusSummary)

	// Prompt for commit message if not provided
	if message == "" {
		log.Print("Enter commit message (or press Enter to skip push): ")
		var input string
		fmt.Scanln(&input)
		if input == "" {
			log.Println("⏭️  Skipping commit and push")
			return
		}
		message = input
	}

	// Commit changes
	log.Println("3. Committing local changes...")
	if err := gitMgr.Commit(ctx, message); err != nil {
		log.Fatalf("Failed to commit changes: %v", err)
	}
	log.Println("✅ Changes committed")

	// Push changes
	log.Println("4. Pushing to remote...")
	if err := gitMgr.Push(ctx); err != nil {
		log.Fatalf("Failed to push changes: %v", err)
	}
	log.Println("✅ Changes pushed")

	log.Printf("\n✅ Branch '%s' is now fully synchronized", currentBranch)
}
