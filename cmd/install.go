package cmd

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install shell integration for seamless kubeconfig switching",
	Long: `Install shell functions that enable seamless kubeconfig switching without eval commands.
This adds functions like 'hyve-use', 'hyve-unset', and 'hyve-status' to your shell.`,
	Run: func(cmd *cobra.Command, args []string) {
		installShellIntegration()
	},
}

func installShellIntegration() {
	// Get the directory where the hyve binary is located
	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}

	execDir := filepath.Dir(execPath)
	installScript := filepath.Join(execDir, "install-shell-integration.sh")

	// Check if install script exists
	if _, err := os.Stat(installScript); os.IsNotExist(err) {
		log.Printf("❌ Shell integration installer not found at: %s", installScript)
		log.Println()
		log.Println("📦 Manual installation:")
		log.Println("   1. Download hyve-shell-integration.sh")
		log.Println("   2. Add to your ~/.bashrc or ~/.zshrc:")
		log.Println("      source /path/to/hyve-shell-integration.sh")
		log.Println()
		log.Println("🚀 This will provide commands like:")
		log.Println("   hyve-use <cluster>     # Switch to cluster (no eval needed)")
		log.Println("   hyve-unset            # Revert to default kubeconfig")
		log.Println("   hyve-status           # Show current cluster")
		log.Println("   hyve-list             # List available clusters")
		return
	}

	log.Println("🚀 Installing Hyve shell integration...")
	log.Println("📁 Found installer at:", installScript)
	log.Println()

	// Make sure the script is executable
	if err := os.Chmod(installScript, 0755); err != nil {
		log.Fatalf("Failed to make installer executable: %v", err)
	}

	// Run the installer
	cmd := exec.Command("bash", installScript)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalf("Installation failed: %v", err)
	}

	log.Println()
	log.Println("✅ Installation complete!")
	log.Println()
	log.Println("🎯 You can now use:")
	log.Println("   hyve-use production    # Instantly switch to production cluster")
	log.Println("   hyve-unset            # Go back to default kubeconfig")
	log.Println("   hyve-status           # Check which cluster you're using")
}
