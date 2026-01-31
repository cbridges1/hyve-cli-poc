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
	Use:   "set-token [provider]",
	Short: "Store an API token for a cloud provider",
	Long: `Store an encrypted API token for a cloud provider in the local database.

Supported providers:
  - civo: Civo Cloud API token

Example:
  hyve config set-token civo
  hyve config set-token civo --token YOUR_TOKEN_HERE`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		provider := args[0]
		tokenFlag, _ := cmd.Flags().GetString("token")
		setAPIToken(provider, tokenFlag)
	},
}

var configGetTokenCmd = &cobra.Command{
	Use:   "get-token [provider]",
	Short: "Retrieve the stored API token for a provider",
	Long:  "Display the decrypted API token stored for a cloud provider",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		provider := args[0]
		getAPIToken(provider)
	},
}

var configClearTokenCmd = &cobra.Command{
	Use:   "clear-token [provider]",
	Short: "Remove the stored API token for a provider",
	Long:  "Delete the API token stored for a cloud provider",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		provider := args[0]
		clearAPIToken(provider)
	},
}

var configListTokensCmd = &cobra.Command{
	Use:   "list-tokens",
	Short: "List all providers with stored tokens",
	Long:  "Show which cloud providers have API tokens stored in the database",
	Run: func(cmd *cobra.Command, args []string) {
		listAPITokens()
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

	configCmd.AddCommand(configSetTokenCmd)
	configCmd.AddCommand(configGetTokenCmd)
	configCmd.AddCommand(configClearTokenCmd)
	configCmd.AddCommand(configListTokensCmd)
	configCmd.AddCommand(configSetGitBackendCmd)
	configCmd.AddCommand(configGetGitBackendCmd)
}

func setAPIToken(provider, token string) {
	credsMgr, err := credentials.NewManager()
	if err != nil {
		log.Fatalf("Failed to create credentials manager: %v", err)
	}
	defer credsMgr.Close()

	// If token not provided via flag, prompt for it
	if token == "" {
		fmt.Printf("Enter API token for %s (input will be hidden): ", provider)
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
	if err := credsMgr.StoreAPIToken(provider, token); err != nil {
		log.Fatalf("Failed to store token: %v", err)
	}

	log.Printf("✅ API token for '%s' stored successfully", provider)
	log.Println()
	log.Println("💡 The token is encrypted and stored in ~/.hyve/credentials.db")
	log.Printf("💡 Hyve will now use this token automatically for %s operations", provider)
}

func getAPIToken(provider string) {
	credsMgr, err := credentials.NewManager()
	if err != nil {
		log.Fatalf("Failed to create credentials manager: %v", err)
	}
	defer credsMgr.Close()

	token, err := credsMgr.GetAPIToken(provider)
	if err != nil {
		log.Fatalf("Failed to get token: %v", err)
	}

	if token == "" {
		log.Printf("❌ No token stored for provider '%s'", provider)
		log.Println()
		log.Printf("💡 Store a token with: hyve config set-token %s", provider)
		return
	}

	fmt.Printf("🔑 API token for '%s':\n", provider)
	fmt.Println(token)
}

func clearAPIToken(provider string) {
	credsMgr, err := credentials.NewManager()
	if err != nil {
		log.Fatalf("Failed to create credentials manager: %v", err)
	}
	defer credsMgr.Close()

	// Check if token exists
	hasToken, err := credsMgr.HasAPIToken(provider)
	if err != nil {
		log.Fatalf("Failed to check for token: %v", err)
	}

	if !hasToken {
		log.Printf("ℹ️  No token stored for provider '%s'", provider)
		return
	}

	// Clear the token
	if err := credsMgr.ClearAPIToken(provider); err != nil {
		log.Fatalf("Failed to clear token: %v", err)
	}

	log.Printf("✅ API token for '%s' removed successfully", provider)
}

func listAPITokens() {
	credsMgr, err := credentials.NewManager()
	if err != nil {
		log.Fatalf("Failed to create credentials manager: %v", err)
	}
	defer credsMgr.Close()

	providers, err := credsMgr.ListAPITokens()
	if err != nil {
		log.Fatalf("Failed to list tokens: %v", err)
	}

	if len(providers) == 0 {
		log.Println("❌ No API tokens stored")
		log.Println()
		log.Println("💡 Store a token with: hyve config set-token <provider>")
		log.Println("   Supported providers: civo")
		return
	}

	log.Printf("🔑 Stored API tokens (%d):\n", len(providers))
	for _, provider := range providers {
		log.Printf("  ✓ %s", provider)
	}
	log.Println()
	log.Println("💡 Commands:")
	log.Println("  hyve config get-token <provider>    # View token")
	log.Println("  hyve config clear-token <provider>  # Remove token")
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
