package cmd

import (
	"fmt"
	"log"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"civo-cluster-deploy/internal/config"
	"civo-cluster-deploy/internal/credentials"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Hyve configuration",
	Long:  "Commands to manage API tokens and other configuration settings",
}

var configSetTokenCmd = &cobra.Command{
	Use:   "set-token civo",
	Short: "Store a Civo API token",
	Long: `Store an encrypted Civo API token in the local database.

Civo is the only provider that requires storing credentials in Hyve.
Other providers use their native CLI authentication:
  - AWS:   Run 'aws configure' to set up credentials
  - GCP:   Run 'gcloud auth application-default login'
  - Azure: Run 'az login'

The --account flag specifies the Civo account ID. This allows you to store
multiple Civo accounts and switch between them. If not specified, 'default'
is used.

Examples:
  hyve config set-token civo
  hyve config set-token civo --account my-account
  hyve config set-token civo --account production --token YOUR_TOKEN_HERE`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		provider := args[0]
		if provider != "civo" {
			log.Fatalf("Only 'civo' is supported. Other providers use native CLI authentication:\n" +
				"  AWS:   Run 'aws configure'\n" +
				"  GCP:   Run 'gcloud auth application-default login'\n" +
				"  Azure: Run 'az login'")
		}
		tokenFlag, _ := cmd.Flags().GetString("token")
		accountFlag, _ := cmd.Flags().GetString("account")
		setCivoToken(accountFlag, tokenFlag)
	},
}

var configGetTokenCmd = &cobra.Command{
	Use:   "get-token civo",
	Short: "Retrieve the stored Civo API token",
	Long: `Display the decrypted Civo API token stored for an account.

Use --account to specify which account's token to retrieve.
If not specified, 'default' is used.

Examples:
  hyve config get-token civo
  hyve config get-token civo --account production`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		provider := args[0]
		if provider != "civo" {
			log.Fatalf("Only 'civo' tokens are stored. Other providers use native CLI authentication.")
		}
		accountFlag, _ := cmd.Flags().GetString("account")
		getCivoToken(accountFlag)
	},
}

var configClearTokenCmd = &cobra.Command{
	Use:   "clear-token civo",
	Short: "Remove the stored Civo API token",
	Long: `Delete the Civo API token stored for an account.

Use --account to specify which account's token to remove.
If not specified, 'default' is used.

Examples:
  hyve config clear-token civo
  hyve config clear-token civo --account production`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		provider := args[0]
		if provider != "civo" {
			log.Fatalf("Only 'civo' tokens are stored. Other providers use native CLI authentication.")
		}
		accountFlag, _ := cmd.Flags().GetString("account")
		clearCivoToken(accountFlag)
	},
}

var configListTokensCmd = &cobra.Command{
	Use:   "list-tokens",
	Short: "List all stored Civo accounts",
	Long: `Show which Civo accounts have API tokens stored in the database.

Note: AWS, GCP, and Azure use native CLI authentication and don't store
tokens in Hyve. Use their respective CLI tools to manage credentials:
  - AWS:   aws configure list
  - GCP:   gcloud auth list
  - Azure: az account list`,
	Run: func(cmd *cobra.Command, args []string) {
		listCivoAccounts()
	},
}

var configSetGitBackendCmd = &cobra.Command{
	Use:   "set-git-backend [backend]",
	Short: "Set the git backend preference",
	Long: `Set the git backend used for repository operations.

Supported backends:
  - system:  Use system git command (default, requires git in PATH)
  - builtin: Use embedded go-git library (portable)

The preference is stored in ~/.hyve/config.yaml and persists across sessions.

Example:
  hyve config set-git-backend system
  hyve config set-git-backend builtin`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		backend := args[0]
		setGitBackend(backend)
	},
}

var configGetGitBackendCmd = &cobra.Command{
	Use:   "get-git-backend",
	Short: "Get the current git backend preference",
	Long:  "Display the configured git backend (system or builtin)",
	Run: func(cmd *cobra.Command, args []string) {
		getGitBackend()
	},
}

func init() {
	configSetTokenCmd.Flags().StringP("token", "t", "", "API token (if not provided, will prompt securely)")
	configSetTokenCmd.Flags().StringP("account", "a", "default", "Civo account ID (allows multiple accounts)")

	configGetTokenCmd.Flags().StringP("account", "a", "default", "Civo account ID")
	configClearTokenCmd.Flags().StringP("account", "a", "default", "Civo account ID")

	configCmd.AddCommand(configSetTokenCmd)
	configCmd.AddCommand(configGetTokenCmd)
	configCmd.AddCommand(configClearTokenCmd)
	configCmd.AddCommand(configListTokensCmd)
	configCmd.AddCommand(configSetGitBackendCmd)
	configCmd.AddCommand(configGetGitBackendCmd)
}

func setCivoToken(accountID, token string) {
	credsMgr, err := credentials.NewManager()
	if err != nil {
		log.Fatalf("Failed to create credentials manager: %v", err)
	}
	defer credsMgr.Close()

	// If token not provided via flag, prompt for it
	if token == "" {
		fmt.Printf("Enter Civo API token for account '%s' (input will be hidden): ", accountID)
		tokenBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println() // New line after password input
		if err != nil {
			log.Fatalf("Failed to read token: %v", err)
		}
		token = string(tokenBytes)
	}

	if token == "" {
		log.Fatal("Token cannot be empty")
	}

	// Store the token
	if err := credsMgr.StoreCivoToken(accountID, token); err != nil {
		log.Fatalf("Failed to store token: %v", err)
	}

	log.Printf("✅ Civo API token for account '%s' stored successfully", accountID)
	log.Println()
	log.Println("💡 The token is encrypted and stored in ~/.hyve/credentials.db")
	log.Printf("💡 Hyve will now use this token automatically for Civo operations")
	if accountID != "default" {
		log.Printf("💡 To use this account, specify --civo-account %s when creating clusters", accountID)
	}
}

func getCivoToken(accountID string) {
	credsMgr, err := credentials.NewManager()
	if err != nil {
		log.Fatalf("Failed to create credentials manager: %v", err)
	}
	defer credsMgr.Close()

	token, err := credsMgr.GetCivoToken(accountID)
	if err != nil {
		log.Fatalf("Failed to get token: %v", err)
	}

	if token == "" {
		log.Printf("❌ No token stored for Civo account '%s'", accountID)
		log.Println()
		log.Printf("💡 Store a token with: hyve config set-token civo --account %s", accountID)
		return
	}

	fmt.Printf("🔑 Civo API token for account '%s':\n", accountID)
	fmt.Println(token)
}

func clearCivoToken(accountID string) {
	credsMgr, err := credentials.NewManager()
	if err != nil {
		log.Fatalf("Failed to create credentials manager: %v", err)
	}
	defer credsMgr.Close()

	// Check if token exists
	hasToken, err := credsMgr.HasCivoToken(accountID)
	if err != nil {
		log.Fatalf("Failed to check for token: %v", err)
	}

	if !hasToken {
		log.Printf("ℹ️  No token stored for Civo account '%s'", accountID)
		return
	}

	// Clear the token
	if err := credsMgr.ClearCivoToken(accountID); err != nil {
		log.Fatalf("Failed to clear token: %v", err)
	}

	log.Printf("✅ Civo API token for account '%s' removed successfully", accountID)
}

func listCivoAccounts() {
	credsMgr, err := credentials.NewManager()
	if err != nil {
		log.Fatalf("Failed to create credentials manager: %v", err)
	}
	defer credsMgr.Close()

	accounts, err := credsMgr.ListCivoAccounts()
	if err != nil {
		log.Fatalf("Failed to list accounts: %v", err)
	}

	if len(accounts) == 0 {
		log.Println("❌ No Civo accounts configured")
		log.Println()
		log.Println("💡 Store a token with: hyve config set-token civo --account <account-id>")
		log.Println()
		log.Println("📝 Note: AWS, GCP, and Azure use native CLI authentication:")
		log.Println("   AWS:   aws configure")
		log.Println("   GCP:   gcloud auth application-default login")
		log.Println("   Azure: az login")
		return
	}

	log.Printf("🔑 Stored Civo accounts (%d):\n", len(accounts))
	for _, account := range accounts {
		log.Printf("  ✓ %s", account)
	}
	log.Println()
	log.Println("💡 Commands:")
	log.Println("  hyve config get-token civo --account <id>    # View token")
	log.Println("  hyve config clear-token civo --account <id>  # Remove token")
	log.Println()
	log.Println("📝 Note: AWS, GCP, and Azure use native CLI authentication")
}

func setGitBackend(backend string) {
	configMgr := config.NewManager()
	if err := configMgr.SetGitBackend(backend); err != nil {
		log.Fatalf("Failed to set git backend: %v", err)
	}

	log.Printf("✅ Git backend set to '%s'", backend)
	log.Println()
	log.Println("💡 The backend preference is stored in ~/.hyve/config.yaml")

	if backend == "system" {
		log.Println("💡 Hyve will now use your system's git command for repository operations")
		log.Println("   Requirement: git must be in PATH")
	} else if backend == "builtin" {
		log.Println("💡 Hyve will now use the embedded go-git library")
		log.Println("   This works without git installed, but may have limited authentication options")
	}
}

func getGitBackend() {
	configMgr := config.NewManager()
	if err := configMgr.LoadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	backend := configMgr.GetGitBackend()

	log.Printf("🔧 Current git backend: %s\n", backend)
	log.Println()

	if backend == "system" {
		log.Println("Using system git command for repository operations")
		log.Println("Requirement: git must be in PATH")
	} else if backend == "builtin" {
		log.Println("Using embedded go-git library for repository operations")
		log.Println("Works without git installed")
	}

	log.Println()
	log.Println("💡 Change backend with: hyve config set-git-backend [system|builtin]")
}
