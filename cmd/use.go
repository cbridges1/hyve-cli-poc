package cmd

import (
	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:   "use [cluster-name]",
	Short: "Quickly set kubeconfig for current terminal session",
	Long:  "Convenience command that automatically sets KUBECONFIG for the specified cluster. Equivalent to 'hyve kubeconfig use --eval'",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		clusterName := args[0]
		// Always use eval mode for the convenience command
		useKubeconfig(clusterName, true)
	},
}
