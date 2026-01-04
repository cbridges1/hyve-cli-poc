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
}
