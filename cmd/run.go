package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"civo-cluster-deploy/internal/kubeconfig"
	"civo-cluster-deploy/internal/repository"
)

var runCmd = &cobra.Command{
	Use:   "run [command] [args...]",
	Short: "Run a command with kubeconfig set for specified cluster",
	Long: `Execute a command with the kubeconfig environment variable set to a specific cluster.
If no cluster is specified with --cluster, uses the current repository's default or prompts for selection.

Examples:
  # Run kubectl with production cluster
  ./hyve run --cluster production kubectl get nodes
  
  # Run any command with staging cluster  
  ./hyve run --cluster staging kubectl get pods
  
  # Interactive cluster selection
  ./hyve run kubectl get services
  
  # Run complex commands
  ./hyve run --cluster production sh -c "kubectl get pods | grep nginx"`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		clusterName, _ := cmd.Flags().GetString("cluster")
		runCommandWithKubeconfig(clusterName, args)
	},
}

func init() {
	runCmd.Flags().StringP("cluster", "c", "", "Cluster name to use for kubeconfig")
}

func runCommandWithKubeconfig(clusterName string, cmdArgs []string) {
	// Get current repository
	repoManager, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	currentRepo, err := repoManager.GetCurrentRepository()
	if err != nil {
		log.Fatalf("No current repository configured. Please add a repository first: %v", err)
	}

	// Initialize kubeconfig manager
	manager, err := kubeconfig.NewManager(currentRepo.Name)
	if err != nil {
		log.Fatalf("Failed to initialize kubeconfig manager: %v", err)
	}
	defer manager.Close()

	// If no cluster specified, list available clusters and prompt
	if clusterName == "" {
		kubeconfigs, err := manager.ListKubeconfigs()
		if err != nil {
			log.Fatalf("Failed to list kubeconfigs: %v", err)
		}

		if len(kubeconfigs) == 0 {
			log.Fatal("No kubeconfigs found. Run 'hyve kubeconfig sync' first.")
		}

		if len(kubeconfigs) == 1 {
			clusterName = kubeconfigs[0].ClusterName
			fmt.Printf("Using cluster: %s\n", clusterName)
		} else {
			fmt.Println("Available clusters:")
			for i, kc := range kubeconfigs {
				fmt.Printf("  %d. %s\n", i+1, kc.ClusterName)
			}
			log.Fatal("Please specify a cluster with --cluster flag")
		}
	}

	// Get kubeconfig for the specified cluster
	kc, err := manager.GetKubeconfig(clusterName)
	if err != nil {
		log.Fatalf("Failed to get kubeconfig for cluster '%s': %v", clusterName, err)
	}

	// Get decrypted kubeconfig data
	kubeconfigData, err := kc.GetConfig()
	if err != nil {
		log.Fatalf("Failed to decrypt kubeconfig data: %v", err)
	}

	// Create temporary kubeconfig file
	tempDir := filepath.Join(os.Getenv("HOME"), ".hyve", "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}

	tempFile := filepath.Join(tempDir, fmt.Sprintf("kubeconfig-run-%s", kc.ClusterName))
	if err := os.WriteFile(tempFile, []byte(kubeconfigData), 0600); err != nil {
		log.Fatalf("Failed to write temporary kubeconfig: %v", err)
	}

	// Clean up temp file when done
	defer func() {
		if err := os.Remove(tempFile); err != nil {
			log.Printf("Warning: Failed to clean up temporary kubeconfig: %v", err)
		}
	}()

	// Prepare the command
	command := cmdArgs[0]
	args := cmdArgs[1:]

	cmd := exec.Command(command, args...)

	// Set environment variables
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", tempFile))

	// Connect stdin/stdout/stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("🚀 Running command with cluster '%s' kubeconfig...\n", kc.ClusterName)
	fmt.Printf("💡 Command: %s %v\n", command, args)
	fmt.Println()

	// Execute the command
	if err := cmd.Run(); err != nil {
		// Check if it's an exit error to get the exit code
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
		log.Fatalf("Command failed: %v", err)
	}
}
