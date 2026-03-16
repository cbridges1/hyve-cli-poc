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
	"gopkg.in/yaml.v3"

	"hyve/internal/cluster"
	"hyve/internal/config"
	"hyve/internal/credentials"
	"hyve/internal/provider"
	"hyve/internal/providerconfig"
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

func addClusterFromCLI(clusterName, region, providerName string, nodes []string, nodeGroups []types.NodeGroup, clusterType, accountName, projectName, subscriptionName, orgName, vpcName, eksRoleName, nodeRoleName string) {
	ctx := gocontext.Background()
	stateMgr, stateDir := createStateManager(ctx)

	if err := os.MkdirAll(stateDir, 0755); err != nil {
		log.Fatalf("Failed to create state directory: %v", err)
	}

	filePath := filepath.Join(stateDir, clusterName+".yaml")

	if _, err := os.Stat(filePath); err == nil {
		log.Fatalf("Cluster %s already exists. Use 'modify' action to update it.", clusterName)
	}

	pcMgr := providerconfig.NewManager(filepath.Dir(stateDir))
	var err error

	// Resolve GCP project alias to project ID
	var gcpProjectID string
	if providerName == "gcp" && projectName != "" {
		gcpProjectID, err = pcMgr.GetGCPProjectID(projectName)
		if err != nil {
			log.Fatalf("GCP project alias '%s' not found in repository configuration.\n"+
				"Use 'hyve config gcp project add --name %s --id <project-id>' to add it.", projectName, projectName)
		}
		log.Printf("Using GCP project '%s' (ID: %s)", projectName, gcpProjectID)
	}

	// Resolve AWS aliases
	var awsAccountID, awsVPCID, awsEKSRoleARN, awsNodeRoleARN string
	if providerName == "aws" {
		// Resolve AWS account alias
		awsAccountID, err = pcMgr.GetAWSAccountID(accountName)
		if err != nil {
			log.Fatalf("AWS account alias '%s' not found in repository configuration.\n"+
				"Use 'hyve config aws account add --name %s --id <account-id>' to add it.", accountName, accountName)
		}
		log.Printf("Using AWS account '%s' (ID: %s)", accountName, awsAccountID)

		// Resolve VPC alias (required for AWS)
		if vpcName != "" {
			awsVPCID, err = pcMgr.GetAWSVPCID(accountName, vpcName)
			if err != nil {
				log.Fatalf("AWS VPC alias '%s' not found in account '%s'.\n"+
					"Use 'hyve config use aws %s' to set the account, then:\n"+
					"  hyve config aws vpc add --name %s --id <vpc-id>\n"+
					"Or use 'hyve config aws vpc create --name %s --region %s' to create one.", vpcName, accountName, accountName, vpcName, vpcName, region)
			}
			log.Printf("Using AWS VPC '%s' (ID: %s)", vpcName, awsVPCID)
		}

		// Resolve EKS role alias (required for AWS)
		if eksRoleName != "" {
			awsEKSRoleARN, err = pcMgr.GetAWSEKSRoleARN(accountName, eksRoleName)
			if err != nil {
				log.Fatalf("AWS EKS role alias '%s' not found in account '%s'.\n"+
					"Use 'hyve config use aws %s' to set the account, then:\n"+
					"  hyve config aws eks-role add --name %s --role-arn <arn>\n"+
					"Or use 'hyve config aws eks-role create --name %s --role-name <name> --region %s' to create one.", eksRoleName, accountName, accountName, eksRoleName, eksRoleName, region)
			}
			log.Printf("Using AWS EKS role '%s' (ARN: %s)", eksRoleName, awsEKSRoleARN)
		}

		// Resolve node role alias (required for AWS)
		if nodeRoleName != "" {
			awsNodeRoleARN, err = pcMgr.GetAWSNodeRoleARN(accountName, nodeRoleName)
			if err != nil {
				log.Fatalf("AWS node role alias '%s' not found in account '%s'.\n"+
					"Use 'hyve config use aws %s' to set the account, then:\n"+
					"  hyve config aws node-role add --name %s --role-arn <arn>\n"+
					"Or use 'hyve config aws node-role create --name %s --role-name <name> --region %s' to create one.", nodeRoleName, accountName, accountName, nodeRoleName, nodeRoleName, region)
			}
			log.Printf("Using AWS node role '%s' (ARN: %s)", nodeRoleName, awsNodeRoleARN)
		}
	}

	// ClusterType is only meaningful for Civo
	if providerName != "civo" {
		clusterType = ""
	}

	clusterDef := types.ClusterDefinition{
		APIVersion: "v1",
		Kind:       "Cluster",
		Metadata: types.ClusterMetadata{
			Name:   clusterName,
			Region: region,
		},
		Spec: types.ClusterSpec{
			Provider:    providerName,
			Nodes:       nodes,
			NodeGroups:  nodeGroups,
			ClusterType: clusterType,
			// GCP-specific
			GCPProject:   projectName,
			GCPProjectID: gcpProjectID,
			// AWS-specific
			AWSAccount:     accountName,
			AWSAccountID:   awsAccountID,
			AWSVPCName:     vpcName,
			AWSVPCID:       awsVPCID,
			AWSEKSRole:     eksRoleName,
			AWSEKSRoleARN:  awsEKSRoleARN,
			AWSNodeRole:    nodeRoleName,
			AWSNodeRoleARN: awsNodeRoleARN,
			// Azure-specific
			AzureSubscription: subscriptionName,
			// Civo-specific
			CivoOrganization: orgName,
			Ingress: types.IngressSpec{
				Enabled:      true,
				LoadBalancer: true,
			},
		},
	}

	data, err := yaml.Marshal(&clusterDef)
	if err != nil {
		log.Fatalf("Failed to marshal cluster definition: %v", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		log.Fatalf("Failed to write cluster definition file: %v", err)
	}

	log.Printf("Created cluster definition file: %s", filePath)
	log.Printf("Cluster %s configuration:", clusterName)
	log.Printf("  Region: %s", region)
	log.Printf("  Provider: %s", providerName)
	log.Printf("  Nodes: %v", nodes)
	log.Printf("  Cluster Type: %s", clusterType)
	if projectName != "" {
		log.Printf("  GCP Project: %s (ID: %s)", projectName, gcpProjectID)
	}
	if awsVPCID != "" {
		log.Printf("  AWS VPC: %s (ID: %s)", vpcName, awsVPCID)
	}
	if awsEKSRoleARN != "" {
		log.Printf("  AWS EKS Role: %s", eksRoleName)
	}
	if awsNodeRoleARN != "" {
		log.Printf("  AWS Node Role: %s", nodeRoleName)
	}

	// Commit changes to Git if configured
	commitStateChanges(ctx, stateMgr, fmt.Sprintf("Add cluster %s", clusterName))

	log.Printf("Exporting cluster information...")
	configMgr := config.NewManager()
	if apiKey := configMgr.GetCivoToken(clusterDef.Spec.CivoOrganization); apiKey != "" {
		err := exportClusterInfo(ctx, apiKey, clusterDef)
		if err != nil {
			log.Printf("Warning: Failed to export cluster info: %v", err)
		}
	}

	runReconciliation("")
}

func modifyClusterFromCLI(cmd *cobra.Command, clusterName string) {
	ctx := gocontext.Background()
	stateMgr, stateDir := createStateManager(ctx)
	filePath := filepath.Join(stateDir, clusterName+".yaml")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Fatalf("Cluster %s does not exist. Use 'add' action to create it.", clusterName)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to read existing cluster file: %v", err)
	}

	var clusterDef types.ClusterDefinition
	if err := yaml.Unmarshal(data, &clusterDef); err != nil {
		log.Fatalf("Failed to parse existing cluster definition: %v", err)
	}

	if cmd.Flags().Changed("region") {
		region, _ := cmd.Flags().GetString("region")
		clusterDef.Metadata.Region = region
	}
	if cmd.Flags().Changed("provider") {
		provider, _ := cmd.Flags().GetString("provider")
		clusterDef.Spec.Provider = strings.ToLower(provider)
	}
	if cmd.Flags().Changed("nodes") {
		nodes, _ := cmd.Flags().GetStringSlice("nodes")
		clusterDef.Spec.Nodes = nodes
	}
	if cmd.Flags().Changed("cluster-type") {
		if clusterDef.Spec.Provider != "civo" {
			log.Printf("⚠️  --cluster-type is only supported for the Civo provider and will be ignored for '%s'", clusterDef.Spec.Provider)
		} else {
			clusterType, _ := cmd.Flags().GetString("cluster-type")
			clusterDef.Spec.ClusterType = clusterType
		}
	}
	if cmd.Flags().Changed("node-group") {
		nodeGroupStrs, _ := cmd.Flags().GetStringArray("node-group")
		var nodeGroups []types.NodeGroup
		for _, s := range nodeGroupStrs {
			ng, err := parseNodeGroup(s)
			if err != nil {
				log.Fatalf("Invalid --node-group value '%s': %v", s, err)
			}
			nodeGroups = append(nodeGroups, ng)
		}
		clusterDef.Spec.NodeGroups = nodeGroups
	}

	updatedData, err := yaml.Marshal(&clusterDef)
	if err != nil {
		log.Fatalf("Failed to marshal updated cluster definition: %v", err)
	}

	if err := os.WriteFile(filePath, updatedData, 0644); err != nil {
		log.Fatalf("Failed to write updated cluster definition file: %v", err)
	}

	log.Printf("Updated cluster definition file: %s", filePath)
	log.Printf("Cluster %s updated configuration:", clusterName)
	log.Printf("  Region: %s", clusterDef.Metadata.Region)
	log.Printf("  Provider: %s", clusterDef.Spec.Provider)
	log.Printf("  Nodes: %v", clusterDef.Spec.Nodes)
	log.Printf("  Cluster Type: %s", clusterDef.Spec.ClusterType)

	// Commit changes to Git if configured
	commitStateChanges(ctx, stateMgr, fmt.Sprintf("Modify cluster %s", clusterName))

	log.Printf("Exporting cluster information...")
	configMgr := config.NewManager()
	if apiKey := configMgr.GetCivoToken(clusterDef.Spec.CivoOrganization); apiKey != "" {
		err := exportClusterInfo(ctx, apiKey, clusterDef)
		if err != nil {
			log.Printf("Warning: Failed to export cluster info: %v", err)
		}
	}
}

func deleteClusterFromCLI(clusterName string, forceCloud bool, force bool) {
	ctx := gocontext.Background()
	stateMgr, stateDir := createStateManager(ctx)
	filePath := filepath.Join(stateDir, clusterName+".yaml")

	var clusterDef types.ClusterDefinition
	configExists := false

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if force && forceCloud {
			// --force --force-cloud: allow cloud deletion even without a config file
			log.Printf("⚠️ Configuration file not found, but --force --force-cloud specified")
			clusterDef.Metadata.Region = "PHX1" // Default region
		} else {
			log.Fatalf("Cluster %s configuration does not exist. Use --force --force-cloud to delete from cloud provider anyway.", clusterName)
		}
	} else {
		configExists = true
		data, err := os.ReadFile(filePath)
		if err != nil {
			log.Fatalf("Failed to read cluster definition: %v", err)
		}
		if err := yaml.Unmarshal(data, &clusterDef); err != nil {
			log.Fatalf("Failed to parse cluster definition: %v", err)
		}
	}

	if force {
		// Force path: delete the cluster from the cloud immediately, then clean up YAML.
		log.Printf("🗑️ Force-deleting cluster '%s' from cloud provider...", clusterName)
		if err := deleteClusterExplicitly(ctx, clusterDef); err != nil {
			log.Fatalf("❌ Failed to delete cluster %s from cloud provider: %v\n\n"+
				"Configuration file was NOT removed to prevent orphaned cluster state.\n"+
				"Please resolve the issue and try again.", clusterName, err)
		}
	} else {
		// Default path: remove the YAML and let reconciliation handle cloud deletion.
		// In CI/CD mode the push triggers the pipeline; with strictDelete enabled the
		// pipeline (or local reconcile) will delete the orphaned cloud cluster.
		log.Printf("📝 Removing cluster YAML and reconciling — cloud deletion will be handled by reconciliation")
	}

	// Remove configuration file if it exists
	if configExists {
		if err := os.Remove(filePath); err != nil {
			log.Fatalf("Failed to delete cluster definition file: %v", err)
		}
		commitStateChanges(ctx, stateMgr, fmt.Sprintf("Delete cluster %s", clusterName))
		log.Printf("Deleted cluster definition file: %s", filePath)
		log.Printf("Cluster %s has been removed from configuration", clusterName)
	} else {
		log.Printf("📝 No configuration file to remove")
	}

	// Remove kubeconfig from ~/.kube/config and from Hyve's database
	cleanupClusterKubeconfig(clusterName)

	runReconciliation("")
}

// cleanupClusterKubeconfig removes a cluster's kubeconfig from both the Hyve
// database and the active ~/.kube/config context list.
func cleanupClusterKubeconfig(clusterName string) {
	// Remove from the Hyve encrypted database
	kubeconfigMgr, _, err := createKubeconfigManager()
	if err != nil {
		log.Printf("⚠️  Could not open kubeconfig database: %v", err)
	} else {
		defer kubeconfigMgr.Close()
		if err := kubeconfigMgr.DeleteKubeconfig(clusterName); err != nil {
			log.Printf("⚠️  Failed to remove kubeconfig from database: %v", err)
		} else {
			log.Printf("🗑️  Removed kubeconfig for '%s' from Hyve database", clusterName)
		}
	}

	// Remove from ~/.kube/config
	removeKubeconfig(clusterName)
}

// deleteClusterExplicitly deletes a cluster by name directly from the provider
// This ensures deletion even if the cluster doesn't appear in provider API listings
func deleteClusterExplicitly(ctx gocontext.Context, clusterDef types.ClusterDefinition) error {
	clusterName := clusterDef.Metadata.Name
	region := clusterDef.Metadata.Region
	providerName := clusterDef.Spec.Provider
	if providerName == "" {
		providerName = "civo" // default for backward compatibility
	}

	// Create provider with appropriate options
	prov, err := createProviderForClusterDef(clusterDef)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Create cluster manager
	clusterMgr := cluster.NewManager(prov)

	log.Printf("🔍 Explicitly searching for cluster '%s' in region %s (provider: %s)...", clusterName, region, providerName)

	// Try to find the cluster by name
	existingCluster, err := clusterMgr.FindByName(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to search for cluster: %w", err)
	}

	if existingCluster == nil {
		log.Printf("✅ Cluster '%s' not found in cloud provider, may already be deleted", clusterName)
		return nil
	}

	log.Printf("🗑️ Found cluster '%s' with ID %s, explicitly deleting...", clusterName, existingCluster.ID)

	// Delete the cluster explicitly by ID
	err = clusterMgr.Delete(ctx, existingCluster.ID)
	if err != nil {
		return fmt.Errorf("failed to delete cluster %s (ID: %s): %w", clusterName, existingCluster.ID, err)
	}

	log.Printf("✅ Successfully deleted cluster '%s' from cloud provider", clusterName)
	return nil
}

// createProviderForClusterDef creates a provider with appropriate options for a cluster definition
func createProviderForClusterDef(clusterDef types.ClusterDefinition) (provider.Provider, error) {
	providerName := clusterDef.Spec.Provider
	if providerName == "" {
		providerName = "civo" // default
	}

	providerFactory := provider.NewFactory()

	opts := provider.ProviderOptions{
		Region: clusterDef.Metadata.Region,
	}

	// Populate AccountName so the factory can resolve named env vars.
	switch strings.ToLower(providerName) {
	case "civo":
		opts.AccountName = clusterDef.Spec.CivoOrganization
	case "aws":
		opts.AccountName = clusterDef.Spec.AWSAccount
	case "gcp":
		opts.AccountName = clusterDef.Spec.GCPProject
	case "azure":
		opts.AccountName = clusterDef.Spec.AzureSubscription
	}

	// Handle Civo-specific configuration
	if providerName == "civo" {
		configMgr := config.NewManager()
		apiKey := configMgr.GetCivoToken(clusterDef.Spec.CivoOrganization)
		if apiKey == "" {
			apiKey = os.Getenv("CIVO_TOKEN")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("Civo API token not found. Please run 'hyve config civo token set --org %s' or set CIVO_TOKEN environment variable", clusterDef.Spec.CivoOrganization)
		}
		opts.APIKey = apiKey
	}

	// Handle GCP-specific configuration
	if providerName == "gcp" {
		// Use stored project ID if available, otherwise resolve from alias
		if clusterDef.Spec.GCPProjectID != "" {
			opts.ProjectID = clusterDef.Spec.GCPProjectID
			log.Printf("Using GCP project ID '%s'", clusterDef.Spec.GCPProjectID)
		} else if clusterDef.Spec.GCPProject != "" {
			gcpRepoMgr, err := repository.NewManager()
			if err == nil {
				defer gcpRepoMgr.Close()
				if currentRepo, err := gcpRepoMgr.GetCurrentRepository(); err == nil {
					pcMgr := providerconfig.NewManager(currentRepo.LocalPath)
					projectID, err := pcMgr.GetGCPProjectID(clusterDef.Spec.GCPProject)
					if err != nil {
						return nil, fmt.Errorf("failed to resolve GCP project '%s': %w", clusterDef.Spec.GCPProject, err)
					}
					opts.ProjectID = projectID
					log.Printf("Using GCP project '%s' (ID: %s)", clusterDef.Spec.GCPProject, projectID)
				}
			}
		}
	}

	// Handle Azure-specific configuration
	if providerName == "azure" {
		opts.AzureResourceGroup = clusterDef.Spec.AzureResourceGroup
		if clusterDef.Spec.AzureSubscriptionID != "" {
			opts.AzureSubscriptionID = clusterDef.Spec.AzureSubscriptionID
		} else if clusterDef.Spec.AzureSubscription != "" {
			azureRepoMgr, err := repository.NewManager()
			if err == nil {
				defer azureRepoMgr.Close()
				if currentRepo, err := azureRepoMgr.GetCurrentRepository(); err == nil {
					pcMgr := providerconfig.NewManager(currentRepo.LocalPath)
					subscriptionID, err := pcMgr.GetAzureSubscriptionID(clusterDef.Spec.AzureSubscription)
					if err != nil {
						return nil, fmt.Errorf("failed to resolve Azure subscription '%s': %w", clusterDef.Spec.AzureSubscription, err)
					}
					opts.AzureSubscriptionID = subscriptionID
					log.Printf("Using Azure subscription '%s' (ID: %s)", clusterDef.Spec.AzureSubscription, subscriptionID)
				}
			}
		}
	}

	return providerFactory.CreateProviderWithOptions(providerName, opts)
}

// forceDeleteClusterFromCloud deletes a cluster by name from the cloud provider across multiple regions
func forceDeleteClusterFromCloud(clusterName, region, providerName, projectName string) {
	ctx := gocontext.Background()

	regions := []string{region}
	if region == "" {
		// Search common regions based on provider
		switch providerName {
		case "civo":
			regions = []string{"PHX1", "NYC1", "FRA1", "LON1"}
		case "gcp":
			regions = []string{"us-central1", "us-east1", "us-west1", "europe-west1"}
		case "aws":
			regions = []string{"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1"}
		case "azure":
			regions = []string{"eastus", "westus2", "westeurope", "southeastasia"}
		default:
			regions = []string{"PHX1"}
		}
		log.Printf("🔍 No region specified, searching common %s regions: %v", providerName, regions)
	}

	// Build provider options
	opts := provider.ProviderOptions{}

	// Handle Civo-specific configuration
	if providerName == "civo" {
		configMgr := config.NewManager()
		apiKey := configMgr.GetCivoToken(projectName)
		if apiKey == "" {
			apiKey = os.Getenv("CIVO_TOKEN")
		}
		if apiKey == "" {
			log.Fatalf("Civo API token not found. Please run 'hyve config civo token set --org %s' or set CIVO_TOKEN environment variable", projectName)
		}
		opts.APIKey = apiKey
	}

	// Handle GCP-specific configuration
	if providerName == "gcp" && projectName != "" {
		fdRepoMgr, err := repository.NewManager()
		if err != nil {
			log.Fatalf("Failed to create repository manager: %v", err)
		}
		defer fdRepoMgr.Close()
		fdCurrentRepo, err := fdRepoMgr.GetCurrentRepository()
		if err != nil {
			log.Fatalf("Failed to get current repository: %v", err)
		}
		pcMgr := providerconfig.NewManager(fdCurrentRepo.LocalPath)
		projectID, err := pcMgr.GetGCPProjectID(projectName)
		if err != nil {
			log.Fatalf("GCP project alias '%s' not found in repository configuration.\n"+
				"Use 'hyve config gcp project add --name %s --id <project-id>' to add it.", projectName, projectName)
		}
		opts.ProjectID = projectID
		log.Printf("Using GCP project '%s' (ID: %s)", projectName, projectID)
		// GKE's list/get API with a specific region misses zonal clusters. The
		// wildcard "-" searches all zones and regions in a single API call.
		regions = []string{"-"}
	}

	providerFactory := provider.NewFactory()
	found := false

	for _, r := range regions {
		log.Printf("🔍 Searching for cluster '%s' in region %s (provider: %s)...", clusterName, r, providerName)

		opts.Region = r
		prov, err := providerFactory.CreateProviderWithOptions(providerName, opts)
		if err != nil {
			log.Printf("Failed to create provider for region %s: %v", r, err)
			continue
		}

		clusterMgr := cluster.NewManager(prov)

		existingCluster, err := clusterMgr.FindByName(ctx, clusterName)
		if err != nil {
			log.Printf("Failed to search for cluster in region %s: %v", r, err)
			continue
		}

		if existingCluster == nil {
			log.Printf("❌ Cluster '%s' not found in region %s", clusterName, r)
			continue
		}

		found = true
		log.Printf("✅ Found cluster '%s' in region %s with ID %s", clusterName, r, existingCluster.ID)
		log.Printf("🗑️ Force deleting cluster '%s'...", clusterName)

		// Delete the cluster
		err = clusterMgr.Delete(ctx, existingCluster.ID)
		if err != nil {
			log.Fatalf("Failed to delete cluster %s (ID: %s): %v", clusterName, existingCluster.ID, err)
		}

		log.Printf("✅ Successfully force deleted cluster '%s' from region %s", clusterName, r)
		break
	}

	if !found {
		log.Printf("❌ Cluster '%s' not found in any searched regions", clusterName)
		log.Printf("💡 Try specifying a specific region with --region flag")
	}
}

func listClusters() {
	// Get local path from current repository
	listRepoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer listRepoMgr.Close()

	listCurrentRepo, err := listRepoMgr.GetCurrentRepository()
	if err != nil {
		log.Fatalf("No Git repository configured. Add one with: hyve git add <name> --repo-url <url>")
	}

	// Read cluster definitions from the repository's clusters directory
	clustersDir := filepath.Join(listCurrentRepo.LocalPath, "clusters")

	// Check if clusters directory exists
	if _, err := os.Stat(clustersDir); os.IsNotExist(err) {
		log.Println("❌ No clusters found")
		log.Println("\n💡 Run 'hyve cluster add <name>' to create a cluster")
		return
	}

	// Read all YAML files from the clusters directory
	entries, err := os.ReadDir(clustersDir)
	if err != nil {
		log.Fatalf("Failed to read clusters directory: %v", err)
	}

	var clusters []types.ClusterDefinition
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .yaml and .yml files
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		filePath := filepath.Join(clustersDir, name)
		data, err := os.ReadFile(filePath)
		if err != nil {
			log.Printf("Warning: Failed to read %s: %v", name, err)
			continue
		}

		var clusterDef types.ClusterDefinition
		if err := yaml.Unmarshal(data, &clusterDef); err != nil {
			log.Printf("Warning: Failed to parse %s: %v", name, err)
			continue
		}

		// Only include files with Kind: Cluster
		if clusterDef.Kind == "Cluster" {
			clusters = append(clusters, clusterDef)
		}
	}

	if len(clusters) == 0 {
		log.Println("❌ No clusters found")
		log.Println("\n💡 Run 'hyve cluster add <name>' to create a cluster")
		return
	}

	log.Printf("📦 Clusters (%d):\n", len(clusters))

	for _, cluster := range clusters {
		log.Printf("  %s", cluster.Metadata.Name)
		log.Printf("    Provider: %s", cluster.Spec.Provider)
		log.Printf("    Region: %s", cluster.Metadata.Region)
		if len(cluster.Spec.NodeGroups) > 0 {
			log.Printf("    NodeGroups: %d", len(cluster.Spec.NodeGroups))
			for _, ng := range cluster.Spec.NodeGroups {
				log.Printf("      - %s: %s x%d", ng.Name, ng.InstanceType, ng.Count)
			}
		} else {
			log.Printf("    Nodes: %d (%s)", len(cluster.Spec.Nodes), strings.Join(cluster.Spec.Nodes, ", "))
		}
		if cluster.Spec.Ingress.Enabled {
			log.Printf("    Ingress: enabled")
		}
		log.Println()
	}

	log.Println("💡 Commands:")
	log.Println("  hyve cluster add <name>       # Add a new cluster")
	log.Println("  hyve cluster modify <name>    # Modify an existing cluster")
	log.Println("  hyve cluster delete <name>    # Delete a cluster")
	log.Println("  hyve reconcile                # Apply cluster changes to cloud")
}

// importClusterFromCLI records an already-running cloud cluster in the hyve
// repository without provisioning it. Reconciliation is intentionally skipped.
func importClusterFromCLI(clusterName, region, providerName string, nodes []string, nodeGroups []types.NodeGroup, accountName, projectName, subscriptionName, orgName, vpcName, eksRoleName, nodeRoleName string) {
	ctx := gocontext.Background()
	stateMgr, stateDir := createStateManager(ctx)

	if repoCfg, err := stateMgr.LoadRepoConfig(); err == nil && repoCfg.Reconcile.StrictDelete {
		log.Fatalf("❌ Import is disabled: this repository has strictDelete enabled. " +
			"In strict-delete mode hyve owns the full desired-state; importing an unmanaged cluster would cause it to be deleted on the next reconciliation.")
	}

	if err := os.MkdirAll(stateDir, 0755); err != nil {
		log.Fatalf("Failed to create state directory: %v", err)
	}

	filePath := filepath.Join(stateDir, clusterName+".yaml")
	if _, err := os.Stat(filePath); err == nil {
		log.Fatalf("Cluster '%s' already exists in the repository. Use 'modify' to update it.", clusterName)
	}

	pcMgr := providerconfig.NewManager(filepath.Dir(stateDir))
	var err error

	// Resolve GCP project alias
	var gcpProjectID string
	if providerName == "gcp" && projectName != "" {
		gcpProjectID, err = pcMgr.GetGCPProjectID(projectName)
		if err != nil {
			log.Fatalf("GCP project alias '%s' not found.", projectName)
		}
	}

	// Resolve AWS aliases
	var awsAccountID, awsVPCID, awsEKSRoleARN, awsNodeRoleARN string
	if providerName == "aws" && accountName != "" {
		awsAccountID, _ = pcMgr.GetAWSAccountID(accountName)
		if vpcName != "" {
			awsVPCID, _ = pcMgr.GetAWSVPCID(accountName, vpcName)
		}
		if eksRoleName != "" {
			awsEKSRoleARN, _ = pcMgr.GetAWSEKSRoleARN(accountName, eksRoleName)
		}
		if nodeRoleName != "" {
			awsNodeRoleARN, _ = pcMgr.GetAWSNodeRoleARN(accountName, nodeRoleName)
		}
	}

	clusterDef := types.ClusterDefinition{
		APIVersion: "v1",
		Kind:       "Cluster",
		Metadata: types.ClusterMetadata{
			Name:   clusterName,
			Region: region,
		},
		Spec: types.ClusterSpec{
			Provider:   providerName,
			Nodes:      nodes,
			NodeGroups: nodeGroups,
			// ClusterType intentionally omitted — not provisioned by hyve
			GCPProject:        projectName,
			GCPProjectID:      gcpProjectID,
			AWSAccount:        accountName,
			AWSAccountID:      awsAccountID,
			AWSVPCName:        vpcName,
			AWSVPCID:          awsVPCID,
			AWSEKSRole:        eksRoleName,
			AWSEKSRoleARN:     awsEKSRoleARN,
			AWSNodeRole:       nodeRoleName,
			AWSNodeRoleARN:    awsNodeRoleARN,
			AzureSubscription: subscriptionName,
			CivoOrganization:  orgName,
		},
	}

	data, err := yaml.Marshal(&clusterDef)
	if err != nil {
		log.Fatalf("Failed to marshal cluster definition: %v", err)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		log.Fatalf("Failed to write cluster definition: %v", err)
	}

	commitStateChanges(ctx, stateMgr, fmt.Sprintf("Import cluster %s", clusterName))
	log.Printf("✅ Cluster '%s' imported into hyve repository (cloud cluster untouched)", clusterName)
}

// releaseClusterFromCLI removes a cluster from the hyve repository without
// deleting it from the cloud. Reconciliation is intentionally skipped.
func releaseClusterFromCLI(clusterName string) {
	ctx := gocontext.Background()
	stateMgr, stateDir := createStateManager(ctx)

	if repoCfg, err := stateMgr.LoadRepoConfig(); err == nil && repoCfg.Reconcile.StrictDelete {
		log.Fatalf("❌ Release is disabled: this repository has strictDelete enabled. " +
			"In strict-delete mode removing a cluster definition would cause the cloud cluster to be deleted on the next reconciliation. " +
			"Use 'hyve cluster delete' instead.")
	}

	filePath := filepath.Join(stateDir, clusterName+".yaml")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Fatalf("Cluster '%s' not found in repository.", clusterName)
	}

	if err := os.Remove(filePath); err != nil {
		log.Fatalf("Failed to remove cluster definition: %v", err)
	}

	commitStateChanges(ctx, stateMgr, fmt.Sprintf("Release cluster %s", clusterName))
	log.Printf("✅ Cluster '%s' released from hyve management. The cloud cluster continues to run.", clusterName)
}
