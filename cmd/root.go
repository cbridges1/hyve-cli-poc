package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"hyve/internal/context"
	"hyve/internal/database"
)

var hyveHomeFlagValue string

var rootCmd = &cobra.Command{
	Use:   "hyve",
	Short: "Hyve cluster management CLI",
	Long: `A CLI tool for managing Kubernetes clusters on various cloud providers.
Supports cluster creation, modification, deletion, and reconciliation.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		home := resolvedHyveHome()
		if home != "" {
			database.SetConfigDir(home)
			context.SetHyveHome(home)
		}
		return nil
	},
}

// HyveHome returns the effective Hyve home directory. It respects (in order):
//  1. --home flag
//  2. HYVE_HOME environment variable
//  3. ~/.hyve (default)
func HyveHome() string {
	if home := resolvedHyveHome(); home != "" {
		return home
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	return filepath.Join(homeDir, ".hyve")
}

func resolvedHyveHome() string {
	if hyveHomeFlagValue != "" {
		return hyveHomeFlagValue
	}
	return os.Getenv("HYVE_HOME")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&hyveHomeFlagValue, "home", "", "Hyve home directory (default: ~/.hyve). Also read from HYVE_HOME env var.")

	rootCmd.AddCommand(reconcileCmd)
	rootCmd.AddCommand(clusterCmd)
	rootCmd.AddCommand(gitCmd)
	rootCmd.AddCommand(kubeconfigCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(useCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(workflowCmd)
	rootCmd.AddCommand(templateCmd)
}
