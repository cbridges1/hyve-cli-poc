package cmd

import (
	gocontext "context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"hyve/internal/credentials"
	"hyve/internal/repository"
	"hyve/internal/state"
	"hyve/internal/types"
)

// syncRepoState performs a git pull to synchronise the local repository with
// the remote before any operation that reads or writes repository state.
// It is non-fatal: if the sync cannot be completed (e.g. no network), a
// warning is logged and the caller continues with local state.
func syncRepoState(ctx gocontext.Context) {
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Printf("⚠️  Skipping git sync: failed to open repository manager: %v", err)
		return
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		// No repository configured — nothing to sync.
		return
	}

	authUsername, authToken := getAuthCredentials(currentRepo)
	stateMgr, err := state.NewManager(currentRepo.RepoURL, currentRepo.LocalPath, authUsername, authToken)
	if err != nil {
		log.Printf("⚠️  Skipping git sync: failed to create state manager: %v", err)
		return
	}

	if err := stateMgr.InitializeGitRepo(ctx); err != nil {
		log.Printf("⚠️  Skipping git sync: failed to initialise git repo: %v", err)
		return
	}

	if err := stateMgr.SyncWithRemote(ctx); err != nil {
		log.Printf("⚠️  Skipping git sync: failed to sync with remote: %v", err)
		return
	}

	log.Println("🔄 Repository synchronised")
}

// ValidProviders is the list of supported cloud providers
var ValidProviders = []string{"civo", "aws", "gcp", "azure"}

// isValidProvider checks if the given provider is in the list of valid providers
func isValidProvider(provider string) bool {
	provider = strings.ToLower(provider)
	for _, p := range ValidProviders {
		if p == provider {
			return true
		}
	}
	return false
}

// validProvidersString returns a formatted string of valid providers for error messages
func validProvidersString() string {
	return strings.Join(ValidProviders, ", ")
}

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Manage clusters",
	Long:  "Commands to add, modify, or delete cluster configurations",
}

var addCmd = &cobra.Command{
	Use:   "add [cluster-name]",
	Short: "Add a new cluster",
	Long: `Create a new cluster configuration YAML file.

Supported cloud providers:
  - civo    Civo Cloud (K3s/Talos clusters)
  - aws     Amazon Web Services (EKS)
  - gcp     Google Cloud Platform (GKE)
  - azure   Microsoft Azure (AKS)

Use --account-name, --project-name, --subscription-name, or --org-name to specify the account.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		clusterName := args[0]

		region, _ := cmd.Flags().GetString("region")
		providerName, _ := cmd.Flags().GetString("provider")
		nodes, _ := cmd.Flags().GetStringSlice("nodes")
		clusterType, _ := cmd.Flags().GetString("cluster-type")

		// cluster-type is only meaningful for the Civo provider
		if strings.ToLower(providerName) != "civo" {
			if cmd.Flags().Changed("cluster-type") {
				log.Printf("⚠️  --cluster-type is only supported for the Civo provider and will be ignored for '%s'", providerName)
			}
			clusterType = ""
		}

		// Provider-specific account/project override flags
		accountName, _ := cmd.Flags().GetString("account-name")
		projectName, _ := cmd.Flags().GetString("project-name")
		subscriptionName, _ := cmd.Flags().GetString("subscription-name")
		orgName, _ := cmd.Flags().GetString("org-name")

		// AWS-specific flags
		vpcName, _ := cmd.Flags().GetString("vpc-name")
		eksRoleName, _ := cmd.Flags().GetString("eks-role-name")
		nodeRoleName, _ := cmd.Flags().GetString("node-role-name")

		// Validate provider
		if !isValidProvider(providerName) {
			log.Fatalf("Invalid provider '%s'. Valid providers are: %s", providerName, validProvidersString())
		}

		// Normalize provider to lowercase
		providerName = strings.ToLower(providerName)

		// Validate required account/project flags per provider
		switch providerName {
		case "aws":
			if accountName == "" {
				log.Fatalf("AWS provider requires --account-name flag. Use 'hyve config aws account list' to see available accounts.")
			}
			if vpcName == "" {
				log.Fatalf("AWS provider requires --vpc-name flag. Use 'hyve config aws vpc list --account %s' to see available VPCs.", accountName)
			}
			if eksRoleName == "" {
				log.Fatalf("AWS provider requires --eks-role-name flag. Use 'hyve config aws eks-role list --account %s' to see available roles.", accountName)
			}
			if nodeRoleName == "" {
				log.Fatalf("AWS provider requires --node-role-name flag. Use 'hyve config aws node-role list --account %s' to see available roles.", accountName)
			}
		case "gcp":
			if projectName == "" {
				log.Fatalf("GCP provider requires --project-name flag. Use 'hyve config gcp project list' to see available projects.")
			}
		case "azure":
			if subscriptionName == "" {
				log.Fatalf("Azure provider requires --subscription-name flag. Use 'hyve config azure subscription list' to see available subscriptions.")
			}
		case "civo":
			if orgName == "" {
				log.Fatalf("Civo provider requires --org-name flag. Use 'hyve config civo org list' to see available organizations.")
			}
		}

		// Parse --node-group flags
		nodeGroupStrs, _ := cmd.Flags().GetStringArray("node-group")
		var nodeGroups []types.NodeGroup
		for _, s := range nodeGroupStrs {
			ng, err := parseNodeGroup(s)
			if err != nil {
				log.Fatalf("Invalid --node-group value '%s': %v", s, err)
			}
			nodeGroups = append(nodeGroups, ng)
		}

		addClusterFromCLI(clusterName, region, providerName, nodes, nodeGroups, clusterType, accountName, projectName, subscriptionName, orgName, vpcName, eksRoleName, nodeRoleName)
	},
}

var modifyCmd = &cobra.Command{
	Use:   "modify [cluster-name]",
	Short: "Modify an existing cluster",
	Long: `Update an existing cluster configuration YAML file.

Supported cloud providers (if changing provider):
  - civo    Civo Cloud (K3s/Talos clusters)
  - aws     Amazon Web Services (EKS)
  - gcp     Google Cloud Platform (GKE)
  - azure   Microsoft Azure (AKS)`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		clusterName := args[0]

		// Validate provider if it's being changed
		if cmd.Flags().Changed("provider") {
			providerName, _ := cmd.Flags().GetString("provider")
			if !isValidProvider(providerName) {
				log.Fatalf("Invalid provider '%s'. Valid providers are: %s", providerName, validProvidersString())
			}
		}

		modifyClusterFromCLI(cmd, clusterName)
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete [cluster-name]",
	Short: "Delete a cluster",
	Long: `Delete a cluster by removing its YAML definition and reconciling.

Default behaviour:
  1. Remove the cluster configuration YAML file
  2. Commit and push the removal to the state repository
  3. Run reconciliation

  In CI/CD mode with strictDelete enabled the push triggers the pipeline,
  which then deletes the cloud cluster. In local mode with strictDelete enabled
  the local reconcile deletes the cloud cluster immediately.

Use --force to delete the cluster from the cloud provider immediately before
removing the configuration file. This is useful when you want to bypass CI/CD
and destroy the cluster right now.

Use --force-cloud together with --force to delete from cloud even if no
configuration file exists.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		clusterName := args[0]
		forceCloud, _ := cmd.Flags().GetBool("force-cloud")
		force, _ := cmd.Flags().GetBool("force")
		deleteClusterFromCLI(clusterName, forceCloud, force)
	},
}

var forceDeleteCmd = &cobra.Command{
	Use:   "force-delete [cluster-name]",
	Short: "Force delete a cluster by name from cloud provider",
	Long: `Force delete a cluster by name directly from the cloud provider without requiring a configuration file.
This command will search all regions for the cluster and delete it if found.
Useful when configuration files are lost or corrupted.

Note: This command does not remove configuration files or run reconciliation.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		clusterName := args[0]
		region, _ := cmd.Flags().GetString("region")
		providerName, _ := cmd.Flags().GetString("provider")
		projectName, _ := cmd.Flags().GetString("project-name")

		// Default to civo for backward compatibility
		if providerName == "" {
			providerName = "civo"
		}

		// Validate provider
		if !isValidProvider(providerName) {
			log.Fatalf("Invalid provider '%s'. Valid providers are: %s", providerName, validProvidersString())
		}

		// Validate project-name is provided for GCP provider
		if providerName == "gcp" && projectName == "" {
			log.Fatalf("GCP provider requires --project-name flag. Use 'hyve config gcp project list' to see available projects.")
		}

		forceDeleteClusterFromCloud(clusterName, region, providerName, projectName)
	},
}

var importCmd = &cobra.Command{
	Use:   "import <name>",
	Short: "Import an existing cloud cluster into hyve",
	Long:  "Record an already-running cluster in the hyve repository without provisioning it.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		clusterName := args[0]
		region, _ := cmd.Flags().GetString("region")
		providerName, _ := cmd.Flags().GetString("provider")
		nodes, _ := cmd.Flags().GetStringSlice("nodes")
		accountName, _ := cmd.Flags().GetString("account-name")
		projectName, _ := cmd.Flags().GetString("project-name")
		subscriptionName, _ := cmd.Flags().GetString("subscription-name")
		orgName, _ := cmd.Flags().GetString("org-name")
		vpcName, _ := cmd.Flags().GetString("vpc-name")
		eksRoleName, _ := cmd.Flags().GetString("eks-role-name")
		nodeRoleName, _ := cmd.Flags().GetString("node-role-name")
		importClusterFromCLI(clusterName, region, providerName, nodes, []types.NodeGroup{}, accountName, projectName, subscriptionName, orgName, vpcName, eksRoleName, nodeRoleName)
	},
}

var releaseCmd = &cobra.Command{
	Use:   "release <name>",
	Short: "Release a cluster from hyve management without deleting it from the cloud",
	Long:  "Remove the cluster definition from the hyve repository. The cloud cluster is left running.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		releaseClusterFromCLI(args[0])
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all cluster definitions",
	Long:  "Display all cluster definitions from the current repository",
	Run: func(cmd *cobra.Command, args []string) {
		listClusters()
	},
}

func init() {
	addCmd.Flags().StringP("region", "r", "PHX1", "Region for the cluster")
	addCmd.Flags().StringP("provider", "p", "", "Cloud provider (civo, aws, gcp, azure)")
	addCmd.MarkFlagRequired("provider")
	addCmd.Flags().StringSliceP("nodes", "n", []string{"g4s.kube.small"}, "Node sizes")
	addCmd.Flags().StringP("cluster-type", "t", "k3s", "Type of Kubernetes cluster")

	// Provider account/project override flags (uses current context if not specified)
	addCmd.Flags().StringP("account-name", "a", "", "AWS account name (required for AWS provider)")
	addCmd.Flags().String("project-name", "", "GCP project name (required for GCP provider)")
	addCmd.Flags().StringP("subscription-name", "s", "", "Azure subscription name (required for Azure provider)")
	addCmd.Flags().StringP("org-name", "o", "", "Civo organization name (required for Civo provider)")

	// AWS-specific flags
	addCmd.Flags().StringP("vpc-name", "v", "", "AWS VPC name alias (required for AWS provider)")
	addCmd.Flags().StringP("eks-role-name", "e", "", "AWS EKS IAM role name alias (required for AWS provider)")
	addCmd.Flags().String("node-role-name", "", "AWS EKS node IAM role name alias (required for AWS provider)")

	addCmd.Flags().StringArrayP("node-group", "g", nil, `Node group spec (repeatable): name=workers,type=t3.medium,count=3[,min=1,max=5,disk=50,spot=true,mode=System]`)

	modifyCmd.Flags().StringP("region", "r", "", "Region for the cluster")
	modifyCmd.Flags().StringP("provider", "p", "", "Cloud provider")
	modifyCmd.Flags().StringSliceP("nodes", "n", nil, "Node sizes")
	modifyCmd.Flags().StringP("cluster-type", "t", "", "Type of Kubernetes cluster")
	modifyCmd.Flags().StringArrayP("node-group", "g", nil, `Node group spec (repeatable): name=workers,type=t3.medium,count=3[,min=1,max=5,disk=50,spot=true,mode=System]`)

	deleteCmd.Flags().Bool("force-cloud", false, "With --force: delete from cloud even if no configuration file exists")
	deleteCmd.Flags().Bool("force", false, "Delete cluster from cloud immediately before removing configuration (bypasses CI/CD)")

	forceDeleteCmd.Flags().StringP("region", "r", "", "Specific region to search (optional, will search common regions if not provided)")
	forceDeleteCmd.Flags().StringP("provider", "p", "civo", "Cloud provider (civo, aws, gcp, azure)")
	forceDeleteCmd.Flags().String("project-name", "", "Project/account name alias (required for GCP provider)")

	importCmd.Flags().StringP("region", "r", "", "Region where the cluster is running")
	importCmd.Flags().StringP("provider", "p", "", "Cloud provider (civo, aws, gcp, azure)")
	importCmd.MarkFlagRequired("provider")
	importCmd.Flags().StringSliceP("nodes", "n", nil, "Node sizes")
	importCmd.Flags().StringP("account-name", "a", "", "AWS account name alias")
	importCmd.Flags().String("project-name", "", "GCP project name alias")
	importCmd.Flags().StringP("subscription-name", "s", "", "Azure subscription name alias")
	importCmd.Flags().StringP("org-name", "o", "", "Civo organization name alias")
	importCmd.Flags().StringP("vpc-name", "v", "", "AWS VPC name alias")
	importCmd.Flags().StringP("eks-role-name", "e", "", "AWS EKS IAM role name alias")
	importCmd.Flags().String("node-role-name", "", "AWS EKS node IAM role name alias")

	clusterCmd.AddCommand(addCmd)
	clusterCmd.AddCommand(importCmd)
	clusterCmd.AddCommand(releaseCmd)
	clusterCmd.AddCommand(listCmd)
	clusterCmd.AddCommand(modifyCmd)
	clusterCmd.AddCommand(deleteCmd)
	clusterCmd.AddCommand(forceDeleteCmd)
}

// createStateManager creates state manager from current repository
func createStateManager(ctx gocontext.Context) (*state.Manager, string) {
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		log.Fatalf("❌ No Git repository configured. Hyve requires a Git repository for state management.\n\n" +
			"Add a Git repository with: hyve git add <name> --repo-url <url>")
	}
	log.Printf("Using Git repository: %s", currentRepo.RepoURL)

	credsMgr, err := credentials.NewManager()
	var authToken string
	var authUsername = currentRepo.Username
	if err == nil {
		defer credsMgr.Close()
		if creds, _ := credsMgr.GetCredentials(); creds != nil {
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

	stateMgr, err := state.NewManager(currentRepo.RepoURL, currentRepo.LocalPath, authUsername, authToken)
	if err != nil {
		log.Fatalf("Failed to create state manager: %v", err)
	}
	if err := stateMgr.InitializeGitRepo(ctx); err != nil {
		log.Fatalf("Failed to initialize Git repository: %v", err)
	}
	if err := stateMgr.SyncWithRemote(ctx); err != nil {
		log.Fatalf("Failed to sync with remote repository: %v", err)
	}
	log.Println("Git repository synchronized")
	stateDir := filepath.Join(currentRepo.LocalPath, "clusters")
	return stateMgr, stateDir
}

// commitStateChanges commits changes to Git repository and pushes to remote
func commitStateChanges(ctx gocontext.Context, stateMgr *state.Manager, message string) {
	log.Println("📝 Committing and pushing changes to Git repository...")

	if err := stateMgr.CommitAndPush(ctx, message); err != nil {
		log.Printf("❌ Failed to commit and push: %v", err)

		// Provide helpful hints based on error type
		if strings.Contains(err.Error(), "failed to push") {
			log.Println("💡 Changes were committed locally but push failed")
			log.Println("💡 Check your Git credentials and network connection")
			log.Println("💡 You can manually push with: cd <repo-path> && git push")
		} else if strings.Contains(err.Error(), "failed to commit") {
			log.Println("💡 Commit operation failed - changes may still be in working directory")
		}
		return
	}

	log.Println("✅ Changes committed and pushed to remote repository successfully")
}

// parseNodeGroup parses a node group spec string into a types.NodeGroup.
// Format: name=workers,type=t3.medium,count=3[,min=1,max=5,disk=50,spot=true,mode=System]
func parseNodeGroup(s string) (types.NodeGroup, error) {
	ng := types.NodeGroup{}
	for _, part := range strings.Split(s, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		k, v := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
		switch k {
		case "name":
			ng.Name = v
		case "type", "instanceType":
			ng.InstanceType = v
		case "count":
			n, err := strconv.Atoi(v)
			if err != nil {
				return ng, fmt.Errorf("invalid count '%s': %w", v, err)
			}
			ng.Count = n
		case "min":
			n, err := strconv.Atoi(v)
			if err != nil {
				return ng, fmt.Errorf("invalid min '%s': %w", v, err)
			}
			ng.MinCount = n
		case "max":
			n, err := strconv.Atoi(v)
			if err != nil {
				return ng, fmt.Errorf("invalid max '%s': %w", v, err)
			}
			ng.MaxCount = n
		case "disk":
			n, err := strconv.Atoi(v)
			if err != nil {
				return ng, fmt.Errorf("invalid disk '%s': %w", v, err)
			}
			ng.DiskSize = n
		case "spot":
			ng.Spot = strings.EqualFold(v, "true")
		case "mode":
			ng.Mode = v
		}
	}
	if ng.Name == "" {
		return ng, fmt.Errorf("node group must have a name (name=<value>)")
	}
	if ng.InstanceType == "" {
		return ng, fmt.Errorf("node group '%s' must have a type (type=<value>)", ng.Name)
	}
	if ng.Count < 1 {
		ng.Count = 1
	}
	return ng, nil
}
