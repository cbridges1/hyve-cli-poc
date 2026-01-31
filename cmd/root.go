package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hyve",
	Short: "Hyve cluster management CLI",
	Long: `A CLI tool for managing Kubernetes clusters on various cloud providers.
Supports cluster creation, modification, deletion, and reconciliation.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
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
