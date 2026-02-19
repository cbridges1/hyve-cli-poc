package cmd

import (
	gocontext "context"
	"fmt"
	"log"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"hyve/internal/config"
	"hyve/internal/context"
	"hyve/internal/credentials"
	"hyve/internal/provider/aws"
	"hyve/internal/providerconfig"
	"hyve/internal/repository"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Hyve configuration",
	Long:  "Commands to manage API tokens and other configuration settings",
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

// GCP provider config commands
var configGCPCmd = &cobra.Command{
	Use:   "gcp",
	Short: "Manage GCP provider configuration",
	Long: `Manage GCP-specific configuration stored in the current repository.

These configurations are stored in the repository under provider-configs/gcp.yaml
and are committed to Git for team sharing.`,
}

var configGCPAddProjectCmd = &cobra.Command{
	Use:   "add-project",
	Short: "Add a GCP project with an alias to the repository configuration",
	Long: `Add a GCP project ID with a friendly name/alias to the repository's provider configuration.

The project is stored in provider-configs/gcp.yaml in the current repository.
The name can then be used as an alias when creating clusters.

Examples:
  hyve config gcp add-project --name dev --id my-dev-project-123
  hyve config gcp add-project --name prod --id my-prod-project-456

Then use with cluster create:
  hyve cluster add my-cluster --provider gcp --gcp-project dev --region us-central1`,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		projectID, _ := cmd.Flags().GetString("id")
		addGCPProject(name, projectID)
	},
}

var configGCPRemoveProjectCmd = &cobra.Command{
	Use:   "remove-project [name]",
	Short: "Remove a GCP project from the repository configuration",
	Long: `Remove a GCP project by its alias/name from the repository's provider configuration.

Examples:
  hyve config gcp remove-project dev
  hyve config gcp remove-project prod`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		removeGCPProject(args[0])
	},
}

var configGCPListProjectsCmd = &cobra.Command{
	Use:   "list-projects",
	Short: "List configured GCP projects",
	Long:  "Display all GCP projects configured in the current repository with their aliases.",
	Run: func(cmd *cobra.Command, args []string) {
		listGCPProjects()
	},
}

var configGCPGetProjectCmd = &cobra.Command{
	Use:   "get-project [name]",
	Short: "Get the project ID for a GCP project alias",
	Long: `Display the GCP project ID associated with a given alias/name.

Examples:
  hyve config gcp get-project dev`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		getGCPProject(args[0])
	},
}

// AWS provider config commands
var configAWSCmd = &cobra.Command{
	Use:   "aws",
	Short: "Manage AWS provider configuration",
	Long: `Manage AWS-specific configuration stored in the current repository.

These configurations are stored in the repository under provider-configs/aws.yaml
and are committed to Git for team sharing.`,
}

var configAWSAccountAddCmd = &cobra.Command{
	Use:   "account-add",
	Short: "Add an AWS account to the repository configuration",
	Long: `Add an AWS account with a friendly name/alias to the repository's provider configuration.

The account is stored in provider-configs/aws.yaml in the current repository.
The name can then be used as an alias when creating EKS clusters.

Examples:
  hyve config aws account-add --name prod --id 123456789012
  hyve config aws account-add --name dev --id 987654321098`,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		accountID, _ := cmd.Flags().GetString("id")
		addAWSAccount(name, accountID)
	},
}

var configAWSAccountRemoveCmd = &cobra.Command{
	Use:   "account-remove [name]",
	Short: "Remove an AWS account from the repository configuration",
	Long: `Remove an AWS account by its alias/name from the repository's provider configuration.

Examples:
  hyve config aws account-remove prod`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		removeAWSAccount(args[0])
	},
}

var configAWSAccountListCmd = &cobra.Command{
	Use:   "account-list",
	Short: "List configured AWS accounts",
	Long:  "Display all AWS accounts configured in the current repository with their aliases.",
	Run: func(cmd *cobra.Command, args []string) {
		listAWSAccounts()
	},
}

var configAWSAccountGetCmd = &cobra.Command{
	Use:   "account-get [name]",
	Short: "Get the account ID for an AWS account alias",
	Long: `Display the AWS account ID associated with a given alias/name.

Examples:
  hyve config aws account-get prod`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		getAWSAccount(args[0])
	},
}

// AWS EKS Role commands
var configAWSEKSRoleAddCmd = &cobra.Command{
	Use:   "eks-role-add",
	Short: "Add an EKS IAM role to the repository configuration",
	Long: `Add an EKS IAM role with a friendly name/alias to the repository's provider configuration.

The role is stored in provider-configs/aws.yaml in the current repository.
The name can then be used as an alias when creating EKS clusters.

Examples:
  hyve config aws eks-role-add --name default-role --role-arn arn:aws:iam::123456789012:role/my-eks-cluster-role
  hyve config aws eks-role-add --name prod-role --role-arn arn:aws:iam::123456789012:role/prod-eks-role`,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		roleARN, _ := cmd.Flags().GetString("role-arn")
		addAWSEKSRole(name, roleARN)
	},
}

var configAWSEKSRoleRemoveCmd = &cobra.Command{
	Use:   "eks-role-remove [name]",
	Short: "Remove an EKS IAM role from the repository configuration",
	Long: `Remove an EKS IAM role by its alias/name from the repository's provider configuration.

Examples:
  hyve config aws eks-role-remove default-role`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		removeAWSEKSRole(args[0])
	},
}

var configAWSEKSRoleListCmd = &cobra.Command{
	Use:   "eks-role-list",
	Short: "List configured EKS IAM roles",
	Long:  "Display all EKS IAM roles configured in the current repository with their aliases.",
	Run: func(cmd *cobra.Command, args []string) {
		listAWSEKSRoles()
	},
}

var configAWSEKSRoleGetCmd = &cobra.Command{
	Use:   "eks-role-get [name]",
	Short: "Get the role ARN for an EKS IAM role alias",
	Long: `Display the EKS IAM role ARN associated with a given alias/name.

Examples:
  hyve config aws eks-role-get default-role`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		getAWSEKSRole(args[0])
	},
}

// AWS Node Role commands
var configAWSNodeRoleAddCmd = &cobra.Command{
	Use:   "node-role-add",
	Short: "Add an EKS node IAM role to the repository configuration",
	Long: `Add an EKS node IAM role with a friendly name/alias to the repository's provider configuration.

The role is stored in provider-configs/aws.yaml in the current repository.
The name can then be used as an alias when creating EKS clusters.

Examples:
  hyve config aws node-role-add --name default-node-role --role-arn arn:aws:iam::123456789012:role/my-eks-node-role
  hyve config aws node-role-add --name prod-node-role --role-arn arn:aws:iam::123456789012:role/prod-eks-node-role`,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		roleARN, _ := cmd.Flags().GetString("role-arn")
		addAWSNodeRole(name, roleARN)
	},
}

var configAWSNodeRoleRemoveCmd = &cobra.Command{
	Use:   "node-role-remove [name]",
	Short: "Remove an EKS node IAM role from the repository configuration",
	Long: `Remove an EKS node IAM role by its alias/name from the repository's provider configuration.

Examples:
  hyve config aws node-role-remove default-node-role`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		removeAWSNodeRole(args[0])
	},
}

var configAWSNodeRoleListCmd = &cobra.Command{
	Use:   "node-role-list",
	Short: "List configured EKS node IAM roles",
	Long:  "Display all EKS node IAM roles configured in the current repository with their aliases.",
	Run: func(cmd *cobra.Command, args []string) {
		listAWSNodeRoles()
	},
}

var configAWSNodeRoleGetCmd = &cobra.Command{
	Use:   "node-role-get [name]",
	Short: "Get the role ARN for an EKS node IAM role alias",
	Long: `Display the EKS node IAM role ARN associated with a given alias/name.

Examples:
  hyve config aws node-role-get default-node-role`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		getAWSNodeRole(args[0])
	},
}

// AWS Node Role create/delete commands (actual AWS operations)
var configAWSNodeRoleCreateCmd = &cobra.Command{
	Use:   "node-role-create",
	Short: "Create an EKS node IAM role in AWS",
	Long: `Create an IAM role for EKS worker nodes in AWS and store the alias in the repository configuration.

This command creates an actual IAM role in AWS with the EC2 assume role policy and
attaches the required EKS node policies (AmazonEKSWorkerNodePolicy, AmazonEKS_CNI_Policy,
AmazonEC2ContainerRegistryReadOnly). The role ARN is then stored with the given alias.

Requires AWS credentials configured via 'aws configure' or environment variables.

Examples:
  hyve config aws node-role-create --name default-node-role --role-name my-eks-node-role --region us-east-1
  hyve config aws node-role-create --name prod-node-role --role-name prod-eks-node-role --region us-west-2`,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		roleName, _ := cmd.Flags().GetString("role-name")
		region, _ := cmd.Flags().GetString("region")
		createAWSNodeRole(name, roleName, region)
	},
}

var configAWSNodeRoleDeleteCmd = &cobra.Command{
	Use:   "node-role-delete [name]",
	Short: "Delete an EKS node IAM role from AWS",
	Long: `Delete an EKS node IAM role from AWS and remove it from the repository configuration.

This command deletes the actual IAM role from AWS (detaching all policies first),
then removes the alias from the repository configuration.

Use --config-only to remove only the configuration without deleting the AWS role.

Examples:
  hyve config aws node-role-delete default-node-role
  hyve config aws node-role-delete default-node-role --region us-east-1
  hyve config aws node-role-delete default-node-role --config-only`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		region, _ := cmd.Flags().GetString("region")
		configOnly, _ := cmd.Flags().GetBool("config-only")
		deleteAWSNodeRole(args[0], region, configOnly)
	},
}

// AWS VPC commands
var configAWSVPCAddCmd = &cobra.Command{
	Use:   "vpc-add",
	Short: "Add a VPC to the repository configuration",
	Long: `Add a VPC with a friendly name/alias to the repository's provider configuration.

The VPC is stored in provider-configs/aws.yaml in the current repository.
The name can then be used as an alias when creating EKS clusters.

Examples:
  hyve config aws vpc-add --name default-vpc --id vpc-0123456789abcdef0
  hyve config aws vpc-add --name prod-vpc --id vpc-abcdef0123456789`,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		vpcID, _ := cmd.Flags().GetString("id")
		addAWSVPC(name, vpcID)
	},
}

var configAWSVPCRemoveCmd = &cobra.Command{
	Use:   "vpc-remove [name]",
	Short: "Remove a VPC from the repository configuration",
	Long: `Remove a VPC by its alias/name from the repository's provider configuration.

Examples:
  hyve config aws vpc-remove default-vpc`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		removeAWSVPC(args[0])
	},
}

var configAWSVPCListCmd = &cobra.Command{
	Use:   "vpc-list",
	Short: "List configured VPCs",
	Long:  "Display all VPCs configured in the current repository with their aliases.",
	Run: func(cmd *cobra.Command, args []string) {
		listAWSVPCs()
	},
}

var configAWSVPCGetCmd = &cobra.Command{
	Use:   "vpc-get [name]",
	Short: "Get the VPC ID for a VPC alias",
	Long: `Display the VPC ID associated with a given alias/name.

Examples:
  hyve config aws vpc-get default-vpc`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		getAWSVPC(args[0])
	},
}

// AWS EKS Role create/delete commands (actual AWS operations)
var configAWSEKSRoleCreateCmd = &cobra.Command{
	Use:   "eks-role-create",
	Short: "Create an EKS IAM role in AWS",
	Long: `Create an IAM role for EKS clusters in AWS and store the alias in the repository configuration.

This command creates an actual IAM role in AWS with the EKS assume role policy and
attaches the AmazonEKSClusterPolicy. The role ARN is then stored with the given alias.

Requires AWS credentials configured via 'aws configure' or environment variables.

Examples:
  hyve config aws eks-role-create --name default-role --role-name my-eks-cluster-role --region us-east-1
  hyve config aws eks-role-create --name prod-role --role-name prod-eks-role --region us-west-2`,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		roleName, _ := cmd.Flags().GetString("role-name")
		region, _ := cmd.Flags().GetString("region")
		createAWSEKSRole(name, roleName, region)
	},
}

var configAWSEKSRoleDeleteCmd = &cobra.Command{
	Use:   "eks-role-delete [name]",
	Short: "Delete an EKS IAM role from AWS",
	Long: `Delete an EKS IAM role from AWS and remove it from the repository configuration.

This command deletes the actual IAM role from AWS (detaching all policies first),
then removes the alias from the repository configuration.

Use --config-only to remove only the configuration without deleting the AWS role.

Examples:
  hyve config aws eks-role-delete default-role
  hyve config aws eks-role-delete default-role --region us-east-1
  hyve config aws eks-role-delete default-role --config-only`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		region, _ := cmd.Flags().GetString("region")
		configOnly, _ := cmd.Flags().GetBool("config-only")
		deleteAWSEKSRole(args[0], region, configOnly)
	},
}

// AWS VPC create/delete commands (actual AWS operations)
var configAWSVPCCreateCmd = &cobra.Command{
	Use:   "vpc-create",
	Short: "Create a VPC in AWS",
	Long: `Create a VPC in AWS and store the alias in the repository configuration.

This command creates an actual VPC in AWS with optional subnets and DNS settings.
The VPC ID is then stored with the given alias.

Requires AWS credentials configured via 'aws configure' or environment variables.

Examples:
  hyve config aws vpc-create --name default-vpc --region us-east-1
  hyve config aws vpc-create --name prod-vpc --region us-west-2 --cidr 10.1.0.0/16
  hyve config aws vpc-create --name dev-vpc --region us-east-1 --cidr 10.0.0.0/16 --subnets 10.0.1.0/24,10.0.2.0/24`,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		region, _ := cmd.Flags().GetString("region")
		cidr, _ := cmd.Flags().GetString("cidr")
		subnets, _ := cmd.Flags().GetString("subnets")
		enableDNS, _ := cmd.Flags().GetBool("enable-dns")
		createAWSVPC(name, region, cidr, subnets, enableDNS)
	},
}

var configAWSVPCDeleteCmd = &cobra.Command{
	Use:   "vpc-delete [name]",
	Short: "Delete a VPC from AWS",
	Long: `Delete a VPC from AWS and remove it from the repository configuration.

This command deletes the actual VPC from AWS (including subnets and internet gateways),
then removes the alias from the repository configuration.

Use --config-only to remove only the configuration without deleting the AWS VPC.

Examples:
  hyve config aws vpc-delete default-vpc
  hyve config aws vpc-delete default-vpc --region us-east-1
  hyve config aws vpc-delete default-vpc --config-only`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		region, _ := cmd.Flags().GetString("region")
		configOnly, _ := cmd.Flags().GetBool("config-only")
		deleteAWSVPC(args[0], region, configOnly)
	},
}

// Context commands
var configUseCmd = &cobra.Command{
	Use:   "use [provider] [account]",
	Short: "Set the current account/project/subscription for a provider",
	Long: `Set the current context for a cloud provider. This determines which account's
resources will be used by default when running commands.

The context is stored locally in ~/.hyve/context.yaml and is NOT committed to Git.

Providers:
  aws   - Set the current AWS account
  gcp   - Set the current GCP project
  azure - Set the current Azure subscription
  civo  - Set the current Civo organization

Examples:
  hyve config use aws prod
  hyve config use gcp my-project
  hyve config use azure my-subscription
  hyve config use civo my-org`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		setContext(args[0], args[1])
	},
}

var configContextCmd = &cobra.Command{
	Use:   "context",
	Short: "Show current context for all providers",
	Long: `Display the currently selected account/project/subscription for each provider.

The context is stored locally in ~/.hyve/context.yaml and is NOT committed to Git.`,
	Run: func(cmd *cobra.Command, args []string) {
		showContext()
	},
}

var configContextClearCmd = &cobra.Command{
	Use:   "context-clear [provider]",
	Short: "Clear the current context",
	Long: `Clear the current context for a provider or all providers.

Examples:
  hyve config context-clear        # Clear all
  hyve config context-clear aws    # Clear only AWS`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			clearAllContext()
		} else {
			clearContext(args[0])
		}
	},
}

// Azure provider config commands
var configAzureCmd = &cobra.Command{
	Use:   "azure",
	Short: "Manage Azure provider configuration",
	Long: `Manage Azure-specific configuration stored in the current repository.

These configurations are stored in the repository under provider-configs/azure.yaml
and are committed to Git for team sharing.`,
}

var configAzureAddSubscriptionIDsCmd = &cobra.Command{
	Use:   "add-subscription-ids [subscription-id,...]",
	Short: "Add Azure subscription IDs to the repository configuration",
	Long: `Add one or more Azure subscription IDs to the repository's provider configuration.

The subscription IDs are stored in provider-configs/azure.yaml in the current repository.
Multiple subscription IDs can be specified as comma-separated values or as separate arguments.

Examples:
  hyve config azure add-subscription-ids 12345678-1234-1234-1234-123456789012
  hyve config azure add-subscription-ids sub-1,sub-2`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		addAzureSubscriptionIDs(args)
	},
}

var configAzureRemoveSubscriptionIDsCmd = &cobra.Command{
	Use:   "remove-subscription-ids [subscription-id,...]",
	Short: "Remove Azure subscription IDs from the repository configuration",
	Long: `Remove one or more Azure subscription IDs from the repository's provider configuration.

Examples:
  hyve config azure remove-subscription-ids 12345678-1234-1234-1234-123456789012`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		removeAzureSubscriptionIDs(args)
	},
}

var configAzureListSubscriptionIDsCmd = &cobra.Command{
	Use:   "list-subscription-ids",
	Short: "List configured Azure subscription IDs",
	Long:  "Display all Azure subscription IDs configured in the current repository.",
	Run: func(cmd *cobra.Command, args []string) {
		listAzureSubscriptionIDs()
	},
}

// Civo provider config commands
var configCivoCmd = &cobra.Command{
	Use:   "civo",
	Short: "Manage Civo provider configuration",
	Long: `Manage Civo-specific configuration stored in the current repository.

These configurations are stored in the repository under provider-configs/civo.yaml
and are committed to Git for team sharing.`,
}

var configCivoOrgAddCmd = &cobra.Command{
	Use:   "org-add",
	Short: "Add a Civo organization to the repository configuration",
	Long: `Add a Civo organization with a friendly name/alias to the repository's provider configuration.

The organization is stored in provider-configs/civo.yaml in the current repository.
The name can then be used as an alias when creating clusters.

Examples:
  hyve config civo org-add --name prod --id org-abc123
  hyve config civo org-add --name dev --id org-def456`,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		orgID, _ := cmd.Flags().GetString("id")
		addCivoOrganization(name, orgID)
	},
}

var configCivoOrgRemoveCmd = &cobra.Command{
	Use:   "org-remove [name]",
	Short: "Remove a Civo organization from the repository configuration",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		removeCivoOrganization(args[0])
	},
}

var configCivoOrgListCmd = &cobra.Command{
	Use:   "org-list",
	Short: "List configured Civo organizations",
	Run: func(cmd *cobra.Command, args []string) {
		listCivoOrganizations()
	},
}

var configCivoOrgGetCmd = &cobra.Command{
	Use:   "org-get [name]",
	Short: "Get the organization ID for a Civo organization alias",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		getCivoOrganization(args[0])
	},
}

var configCivoSetTokenCmd = &cobra.Command{
	Use:   "set-token",
	Short: "Store a Civo API token for the current organization",
	Long: `Store an encrypted Civo API token in the local database.

The token is stored under the name "<org>-token" where <org> is the current
Civo organization set via 'hyve config use civo <org-name>'.

Examples:
  hyve config civo set-token
  hyve config civo set-token --token YOUR_TOKEN_HERE`,
	Run: func(cmd *cobra.Command, args []string) {
		tokenFlag, _ := cmd.Flags().GetString("token")
		setCivoToken(tokenFlag)
	},
}

var configCivoGetTokenCmd = &cobra.Command{
	Use:   "get-token",
	Short: "Retrieve the stored Civo API token for the current organization",
	Long: `Display the decrypted Civo API token for the current organization.

Examples:
  hyve config civo get-token`,
	Run: func(cmd *cobra.Command, args []string) {
		getCivoToken()
	},
}

var configCivoClearTokenCmd = &cobra.Command{
	Use:   "clear-token",
	Short: "Remove the stored Civo API token for the current organization",
	Long: `Delete the Civo API token for the current organization.

Examples:
  hyve config civo clear-token`,
	Run: func(cmd *cobra.Command, args []string) {
		clearCivoToken()
	},
}

func init() {

	// GCP subcommands
	configGCPAddProjectCmd.Flags().String("name", "", "Friendly name/alias for the project (required)")
	configGCPAddProjectCmd.Flags().String("id", "", "GCP project ID (required)")
	configGCPAddProjectCmd.MarkFlagRequired("name")
	configGCPAddProjectCmd.MarkFlagRequired("id")

	configGCPCmd.AddCommand(configGCPAddProjectCmd)
	configGCPCmd.AddCommand(configGCPRemoveProjectCmd)
	configGCPCmd.AddCommand(configGCPListProjectsCmd)
	configGCPCmd.AddCommand(configGCPGetProjectCmd)

	// AWS Account subcommands
	configAWSAccountAddCmd.Flags().String("name", "", "Friendly name/alias for the account (required)")
	configAWSAccountAddCmd.Flags().String("id", "", "AWS account ID (required)")
	configAWSAccountAddCmd.MarkFlagRequired("name")
	configAWSAccountAddCmd.MarkFlagRequired("id")
	configAWSCmd.AddCommand(configAWSAccountAddCmd)
	configAWSCmd.AddCommand(configAWSAccountRemoveCmd)
	configAWSCmd.AddCommand(configAWSAccountListCmd)
	configAWSCmd.AddCommand(configAWSAccountGetCmd)

	// AWS EKS Role subcommands
	configAWSEKSRoleAddCmd.Flags().String("name", "", "Friendly name/alias for the EKS role (required)")
	configAWSEKSRoleAddCmd.Flags().String("role-arn", "", "IAM role ARN for EKS (required)")
	configAWSEKSRoleAddCmd.MarkFlagRequired("name")
	configAWSEKSRoleAddCmd.MarkFlagRequired("role-arn")
	configAWSCmd.AddCommand(configAWSEKSRoleAddCmd)
	configAWSCmd.AddCommand(configAWSEKSRoleRemoveCmd)
	configAWSCmd.AddCommand(configAWSEKSRoleListCmd)
	configAWSCmd.AddCommand(configAWSEKSRoleGetCmd)

	// AWS Node Role subcommands
	configAWSNodeRoleAddCmd.Flags().String("name", "", "Friendly name/alias for the node role (required)")
	configAWSNodeRoleAddCmd.Flags().String("role-arn", "", "IAM role ARN for EKS nodes (required)")
	configAWSNodeRoleAddCmd.MarkFlagRequired("name")
	configAWSNodeRoleAddCmd.MarkFlagRequired("role-arn")
	configAWSCmd.AddCommand(configAWSNodeRoleAddCmd)
	configAWSCmd.AddCommand(configAWSNodeRoleRemoveCmd)
	configAWSCmd.AddCommand(configAWSNodeRoleListCmd)
	configAWSCmd.AddCommand(configAWSNodeRoleGetCmd)

	// AWS Node Role create/delete subcommands (actual AWS operations)
	configAWSNodeRoleCreateCmd.Flags().String("name", "", "Friendly name/alias for the node role (required)")
	configAWSNodeRoleCreateCmd.Flags().String("role-name", "", "IAM role name to create in AWS (required)")
	configAWSNodeRoleCreateCmd.Flags().String("region", "us-east-1", "AWS region")
	configAWSNodeRoleCreateCmd.MarkFlagRequired("name")
	configAWSNodeRoleCreateCmd.MarkFlagRequired("role-name")
	configAWSCmd.AddCommand(configAWSNodeRoleCreateCmd)

	configAWSNodeRoleDeleteCmd.Flags().String("region", "us-east-1", "AWS region")
	configAWSNodeRoleDeleteCmd.Flags().Bool("config-only", false, "Only remove from configuration, don't delete from AWS")
	configAWSCmd.AddCommand(configAWSNodeRoleDeleteCmd)

	// AWS VPC subcommands
	configAWSVPCAddCmd.Flags().String("name", "", "Friendly name/alias for the VPC (required)")
	configAWSVPCAddCmd.Flags().String("id", "", "VPC ID (required)")
	configAWSVPCAddCmd.MarkFlagRequired("name")
	configAWSVPCAddCmd.MarkFlagRequired("id")
	configAWSCmd.AddCommand(configAWSVPCAddCmd)
	configAWSCmd.AddCommand(configAWSVPCRemoveCmd)
	configAWSCmd.AddCommand(configAWSVPCListCmd)
	configAWSCmd.AddCommand(configAWSVPCGetCmd)

	// AWS EKS Role create/delete subcommands (actual AWS operations)
	configAWSEKSRoleCreateCmd.Flags().String("name", "", "Friendly name/alias for the EKS role (required)")
	configAWSEKSRoleCreateCmd.Flags().String("role-name", "", "IAM role name to create in AWS (required)")
	configAWSEKSRoleCreateCmd.Flags().String("region", "us-east-1", "AWS region")
	configAWSEKSRoleCreateCmd.MarkFlagRequired("name")
	configAWSEKSRoleCreateCmd.MarkFlagRequired("role-name")
	configAWSCmd.AddCommand(configAWSEKSRoleCreateCmd)

	configAWSEKSRoleDeleteCmd.Flags().String("region", "us-east-1", "AWS region")
	configAWSEKSRoleDeleteCmd.Flags().Bool("config-only", false, "Only remove from configuration, don't delete from AWS")
	configAWSCmd.AddCommand(configAWSEKSRoleDeleteCmd)

	// AWS VPC create/delete subcommands (actual AWS operations)
	configAWSVPCCreateCmd.Flags().String("name", "", "Friendly name/alias for the VPC (required)")
	configAWSVPCCreateCmd.Flags().String("region", "us-east-1", "AWS region")
	configAWSVPCCreateCmd.Flags().String("cidr", "10.0.0.0/16", "CIDR block for the VPC")
	configAWSVPCCreateCmd.Flags().String("subnets", "", "Comma-separated subnet CIDRs to create (e.g., 10.0.1.0/24,10.0.2.0/24)")
	configAWSVPCCreateCmd.Flags().Bool("enable-dns", true, "Enable DNS support and hostnames")
	configAWSVPCCreateCmd.MarkFlagRequired("name")
	configAWSCmd.AddCommand(configAWSVPCCreateCmd)

	configAWSVPCDeleteCmd.Flags().String("region", "us-east-1", "AWS region")
	configAWSVPCDeleteCmd.Flags().Bool("config-only", false, "Only remove from configuration, don't delete from AWS")
	configAWSCmd.AddCommand(configAWSVPCDeleteCmd)

	// Azure subcommands
	configAzureCmd.AddCommand(configAzureAddSubscriptionIDsCmd)
	configAzureCmd.AddCommand(configAzureRemoveSubscriptionIDsCmd)
	configAzureCmd.AddCommand(configAzureListSubscriptionIDsCmd)

	// Civo subcommands
	configCivoOrgAddCmd.Flags().String("name", "", "Friendly name/alias for the organization (required)")
	configCivoOrgAddCmd.Flags().String("id", "", "Civo organization ID (required)")
	configCivoOrgAddCmd.MarkFlagRequired("name")
	configCivoOrgAddCmd.MarkFlagRequired("id")
	configCivoCmd.AddCommand(configCivoOrgAddCmd)
	configCivoCmd.AddCommand(configCivoOrgRemoveCmd)
	configCivoCmd.AddCommand(configCivoOrgListCmd)
	configCivoCmd.AddCommand(configCivoOrgGetCmd)
	configCivoSetTokenCmd.Flags().StringP("token", "t", "", "API token (if not provided, will prompt securely)")
	configCivoCmd.AddCommand(configCivoSetTokenCmd)
	configCivoCmd.AddCommand(configCivoGetTokenCmd)
	configCivoCmd.AddCommand(configCivoClearTokenCmd)
	configCmd.AddCommand(configSetGitBackendCmd)
	configCmd.AddCommand(configGetGitBackendCmd)
	configCmd.AddCommand(configGCPCmd)
	configCmd.AddCommand(configAWSCmd)
	configCmd.AddCommand(configAzureCmd)
	configCmd.AddCommand(configCivoCmd)
	configCmd.AddCommand(configUseCmd)
	configCmd.AddCommand(configContextCmd)
	configCmd.AddCommand(configContextClearCmd)
}

// Context helper functions
func setContext(provider, account string) {
	ctxMgr, err := context.NewManager()
	if err != nil {
		log.Fatalf("Failed to create context manager: %v", err)
	}

	// Validate the account exists in config
	repoPath := getRepoPath()
	pcMgr := providerconfig.NewManager(repoPath)

	switch provider {
	case "aws":
		exists, err := pcMgr.HasAWSAccount(account)
		if err != nil {
			log.Fatalf("Failed to check AWS account: %v", err)
		}
		if !exists {
			log.Fatalf("AWS account '%s' not found. Add it with: hyve config aws account-add --name %s --id <account-id>", account, account)
		}
	case "gcp":
		exists, err := pcMgr.HasGCPProject(account)
		if err != nil {
			log.Fatalf("Failed to check GCP project: %v", err)
		}
		if !exists {
			log.Fatalf("GCP project '%s' not found. Add it with: hyve config gcp add-project --name %s --id <project-id>", account, account)
		}
	case "azure":
		exists, err := pcMgr.HasAzureSubscription(account)
		if err != nil {
			log.Fatalf("Failed to check Azure subscription: %v", err)
		}
		if !exists {
			log.Fatalf("Azure subscription '%s' not found. Add it with: hyve config azure subscription-add --name %s --id <subscription-id>", account, account)
		}
	case "civo":
		exists, err := pcMgr.HasCivoOrganization(account)
		if err != nil {
			log.Fatalf("Failed to check Civo organization: %v", err)
		}
		if !exists {
			log.Fatalf("Civo organization '%s' not found. Add it with: hyve config civo org-add --name %s --id <org-id>", account, account)
		}
	default:
		log.Fatalf("Unknown provider: %s. Valid providers: aws, gcp, azure, civo", provider)
	}

	if err := ctxMgr.SetCurrentAccount(provider, account); err != nil {
		log.Fatalf("Failed to set context: %v", err)
	}

	log.Printf("✅ Set current %s account to '%s'", provider, account)
	log.Println()
	log.Println("💡 Context is stored locally in ~/.hyve/context.yaml")
}

func showContext() {
	ctxMgr, err := context.NewManager()
	if err != nil {
		log.Fatalf("Failed to create context manager: %v", err)
	}

	ctx := ctxMgr.GetContext()

	log.Println("🔧 Current Context:")
	log.Println()

	if ctx.AWS.Account != "" {
		log.Printf("   AWS:   %s", ctx.AWS.Account)
	} else {
		log.Println("   AWS:   (not set)")
	}

	if ctx.GCP.Account != "" {
		log.Printf("   GCP:   %s", ctx.GCP.Account)
	} else {
		log.Println("   GCP:   (not set)")
	}

	if ctx.Azure.Account != "" {
		log.Printf("   Azure: %s", ctx.Azure.Account)
	} else {
		log.Println("   Azure: (not set)")
	}

	if ctx.Civo.Account != "" {
		log.Printf("   Civo:  %s", ctx.Civo.Account)
	} else {
		log.Println("   Civo:  (not set)")
	}

	log.Println()
	log.Println("💡 Set context with: hyve config use <provider> <account>")
}

func clearContext(provider string) {
	ctxMgr, err := context.NewManager()
	if err != nil {
		log.Fatalf("Failed to create context manager: %v", err)
	}

	if err := ctxMgr.ClearProvider(provider); err != nil {
		log.Fatalf("Failed to clear context: %v", err)
	}

	log.Printf("✅ Cleared %s context", provider)
}

func clearAllContext() {
	ctxMgr, err := context.NewManager()
	if err != nil {
		log.Fatalf("Failed to create context manager: %v", err)
	}

	if err := ctxMgr.Clear(); err != nil {
		log.Fatalf("Failed to clear context: %v", err)
	}

	log.Println("✅ Cleared all context")
}

// getCurrentAWSAccount returns the current AWS account from context or fatally exits
func getCurrentAWSAccount() string {
	ctxMgr, err := context.NewManager()
	if err != nil {
		log.Fatalf("Failed to create context manager: %v", err)
	}

	account := ctxMgr.GetAWSAccount()
	if account == "" {
		log.Fatalf("❌ No AWS account selected.\n\n" +
			"Set the current account with:\n" +
			"  hyve config use aws <account-name>\n\n" +
			"Or list available accounts:\n" +
			"  hyve config aws account-list")
	}

	return account
}

// getCivoOrgFromContext returns the current Civo organization name or fatally exits
func getCivoOrgFromContext() string {
	ctxMgr, err := context.NewManager()
	if err != nil {
		log.Fatalf("Failed to create context manager: %v", err)
	}
	orgName := ctxMgr.GetCivoOrganization()
	if orgName == "" {
		log.Fatalf("❌ No Civo organization selected.\n\n" +
			"Set the current organization with:\n" +
			"  hyve config use civo <org-name>\n\n" +
			"Or add an organization first:\n" +
			"  hyve config civo org-add --name <name> --id <org-id>")
	}
	return orgName
}

func setCivoToken(token string) {
	orgName := getCivoOrgFromContext()

	credsMgr, err := credentials.NewManager()
	if err != nil {
		log.Fatalf("Failed to create credentials manager: %v", err)
	}
	defer credsMgr.Close()

	// If token not provided via flag, prompt for it
	if token == "" {
		fmt.Print("Enter Civo API token (input will be hidden): ")
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

	// Store the token under "<orgName>-token"
	if err := credsMgr.StoreCivoToken(orgName, token); err != nil {
		log.Fatalf("Failed to store token: %v", err)
	}

	log.Printf("✅ Civo API token stored for organization '%s'", orgName)
	log.Println()
	log.Println("💡 The token is encrypted and stored in ~/.hyve/hyve.db")
	log.Println("💡 Hyve will now use this token automatically for Civo operations")
}

func getCivoToken() {
	orgName := getCivoOrgFromContext()

	credsMgr, err := credentials.NewManager()
	if err != nil {
		log.Fatalf("Failed to create credentials manager: %v", err)
	}
	defer credsMgr.Close()

	token, err := credsMgr.GetCivoToken(orgName)
	if err != nil {
		log.Fatalf("Failed to get token: %v", err)
	}

	if token == "" {
		log.Printf("❌ No Civo token stored for organization '%s'", orgName)
		log.Println()
		log.Println("💡 Store a token with: hyve config civo set-token")
		return
	}

	fmt.Println("🔑 Civo API token:")
	fmt.Println(token)
}

func clearCivoToken() {
	orgName := getCivoOrgFromContext()

	credsMgr, err := credentials.NewManager()
	if err != nil {
		log.Fatalf("Failed to create credentials manager: %v", err)
	}
	defer credsMgr.Close()

	// Check if token exists
	hasToken, err := credsMgr.HasCivoToken(orgName)
	if err != nil {
		log.Fatalf("Failed to check for token: %v", err)
	}

	if !hasToken {
		log.Printf("ℹ️  No Civo token stored for organization '%s'", orgName)
		return
	}

	// Clear the token
	if err := credsMgr.ClearCivoToken(orgName); err != nil {
		log.Fatalf("Failed to clear token: %v", err)
	}

	log.Printf("✅ Civo API token removed for organization '%s'", orgName)
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

// getRepoPath returns the current repository's local path
func getRepoPath() string {
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		log.Fatalf("❌ No Git repository configured.\n\n" +
			"Provider configurations are stored in the repository.\n" +
			"Please configure a Git repository first:\n" +
			"  hyve git add <name> --repo-url <repository-url>")
	}

	return currentRepo.LocalPath
}

// parseProjectIDs parses project IDs from arguments (supports comma-separated and space-separated)
func parseProjectIDs(args []string) []string {
	var projectIDs []string
	for _, arg := range args {
		// Split by comma for comma-separated values
		parts := strings.Split(arg, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				projectIDs = append(projectIDs, trimmed)
			}
		}
	}
	return projectIDs
}

func addGCPProject(name, projectID string) {
	if name == "" {
		log.Fatal("Project name is required (--name)")
	}
	if projectID == "" {
		log.Fatal("Project ID is required (--id)")
	}

	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	// Check if name already exists
	exists, err := mgr.HasGCPProject(name)
	if err != nil {
		log.Fatalf("Failed to check GCP config: %v", err)
	}

	if err := mgr.AddGCPProject(name, projectID); err != nil {
		log.Fatalf("Failed to add GCP project: %v", err)
	}

	if exists {
		log.Printf("✅ Updated GCP project '%s':\n", name)
	} else {
		log.Printf("✅ Added GCP project '%s':\n", name)
	}
	log.Printf("   Name:       %s", name)
	log.Printf("   Project ID: %s", projectID)
	log.Println()
	log.Println("💡 The configuration is stored in provider-configs/gcp.yaml")
	log.Println("💡 Use this project when creating clusters:")
	log.Printf("   hyve cluster add my-cluster --provider gcp --gcp-project %s --region us-central1", name)
}

func removeGCPProject(name string) {
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	// Get project info before removing for display
	projectID, err := mgr.GetGCPProjectID(name)
	if err != nil {
		log.Fatalf("❌ GCP project '%s' not found", name)
	}

	if err := mgr.RemoveGCPProject(name); err != nil {
		log.Fatalf("Failed to remove GCP project: %v", err)
	}

	log.Printf("✅ Removed GCP project '%s' (project ID: %s)", name, projectID)
}

func listGCPProjects() {
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	projects, err := mgr.ListGCPProjects()
	if err != nil {
		log.Fatalf("Failed to list GCP projects: %v", err)
	}

	if len(projects) == 0 {
		log.Println("❌ No GCP projects configured")
		log.Println()
		log.Println("💡 Add a project with:")
		log.Println("   hyve config gcp add-project --name dev --id my-project-id")
		return
	}

	log.Printf("🌐 GCP Projects (%d):\n", len(projects))
	log.Println()
	for _, p := range projects {
		log.Printf("   %s", p.Name)
		log.Printf("      Project ID: %s", p.ProjectID)
		log.Println()
	}
	log.Println("💡 Commands:")
	log.Println("   hyve config gcp add-project --name <name> --id <id>  # Add/update project")
	log.Println("   hyve config gcp remove-project <name>                 # Remove project")
	log.Println("   hyve config gcp get-project <name>                    # Get project ID")
	log.Println()
	log.Println("💡 Use with cluster create:")
	log.Println("   hyve cluster add my-cluster --provider gcp --gcp-project <name> --region us-central1")
}

func getGCPProject(name string) {
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	projectID, err := mgr.GetGCPProjectID(name)
	if err != nil {
		log.Fatalf("❌ GCP project '%s' not found", name)
	}

	fmt.Printf("%s\n", projectID)
}

// AWS Account helper functions
func addAWSAccount(name, accountID string) {
	if name == "" {
		log.Fatal("Account name is required (--name)")
	}
	if accountID == "" {
		log.Fatal("Account ID is required (--id)")
	}

	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	exists, err := mgr.HasAWSAccount(name)
	if err != nil {
		log.Fatalf("Failed to check AWS config: %v", err)
	}

	if err := mgr.AddAWSAccount(name, accountID); err != nil {
		log.Fatalf("Failed to add AWS account: %v", err)
	}

	if exists {
		log.Printf("✅ Updated AWS account '%s':\n", name)
	} else {
		log.Printf("✅ Added AWS account '%s':\n", name)
	}
	log.Printf("   Name:       %s", name)
	log.Printf("   Account ID: %s", accountID)
	log.Println()
	log.Println("💡 The configuration is stored in provider-configs/aws.yaml")
}

func removeAWSAccount(name string) {
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	accountID, err := mgr.GetAWSAccountID(name)
	if err != nil {
		log.Fatalf("❌ AWS account '%s' not found", name)
	}

	if err := mgr.RemoveAWSAccount(name); err != nil {
		log.Fatalf("Failed to remove AWS account: %v", err)
	}

	log.Printf("✅ Removed AWS account '%s' (ID: %s)", name, accountID)
}

func listAWSAccounts() {
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	accounts, err := mgr.ListAWSAccounts()
	if err != nil {
		log.Fatalf("Failed to list AWS accounts: %v", err)
	}

	if len(accounts) == 0 {
		log.Println("❌ No AWS accounts configured")
		log.Println()
		log.Println("💡 Add an account with:")
		log.Println("   hyve config aws account-add --name prod --id 123456789012")
		return
	}

	log.Printf("☁️  AWS Accounts (%d):\n", len(accounts))
	log.Println()
	for _, a := range accounts {
		log.Printf("   %s", a.Name)
		log.Printf("      Account ID: %s", a.AccountID)
		log.Println()
	}
	log.Println("💡 Commands:")
	log.Println("   hyve config aws account-add --name <name> --id <id>  # Add/update account")
	log.Println("   hyve config aws account-remove <name>                # Remove account")
	log.Println("   hyve config aws account-get <name>                   # Get account ID")
}

func getAWSAccount(name string) {
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	accountID, err := mgr.GetAWSAccountID(name)
	if err != nil {
		log.Fatalf("❌ AWS account '%s' not found", name)
	}

	fmt.Printf("%s\n", accountID)
}

// AWS EKS Role helper functions
func addAWSEKSRole(name, roleARN string) {
	if name == "" {
		log.Fatal("Role name is required (--name)")
	}
	if roleARN == "" {
		log.Fatal("Role ARN is required (--role-arn)")
	}

	accountName := getCurrentAWSAccount()
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	exists, err := mgr.HasAWSEKSRole(accountName, name)
	if err != nil {
		log.Fatalf("Failed to check AWS config: %v", err)
	}

	if err := mgr.AddAWSEKSRole(accountName, name, roleARN); err != nil {
		log.Fatalf("Failed to add EKS role: %v", err)
	}

	if exists {
		log.Printf("✅ Updated EKS role '%s' in account '%s':\n", name, accountName)
	} else {
		log.Printf("✅ Added EKS role '%s' to account '%s':\n", name, accountName)
	}
	log.Printf("   Name:     %s", name)
	log.Printf("   Role ARN: %s", roleARN)
	log.Println()
	log.Println("💡 The configuration is stored in provider-configs/aws.yaml")
}

func removeAWSEKSRole(name string) {
	accountName := getCurrentAWSAccount()
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	roleARN, err := mgr.GetAWSEKSRoleARN(accountName, name)
	if err != nil {
		log.Fatalf("❌ EKS role '%s' not found in account '%s'", name, accountName)
	}

	if err := mgr.RemoveAWSEKSRole(accountName, name); err != nil {
		log.Fatalf("Failed to remove EKS role: %v", err)
	}

	log.Printf("✅ Removed EKS role '%s' from account '%s' (ARN: %s)", name, accountName, roleARN)
}

func listAWSEKSRoles() {
	accountName := getCurrentAWSAccount()
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	roles, err := mgr.ListAWSEKSRoles(accountName)
	if err != nil {
		log.Fatalf("Failed to list EKS roles: %v", err)
	}

	if len(roles) == 0 {
		log.Printf("❌ No EKS roles configured for account '%s'", accountName)
		log.Println()
		log.Println("💡 Add an EKS role with:")
		log.Println("   hyve config aws eks-role-add --name default-role --role-arn arn:aws:iam::123456789012:role/my-role")
		return
	}

	log.Printf("🔐 EKS IAM Roles for account '%s' (%d):\n", accountName, len(roles))
	log.Println()
	for _, r := range roles {
		log.Printf("   %s", r.Name)
		log.Printf("      Role ARN: %s", r.RoleARN)
		log.Println()
	}
	log.Println("💡 Commands:")
	log.Println("   hyve config aws eks-role-add --name <name> --role-arn <arn>  # Add/update role")
	log.Println("   hyve config aws eks-role-remove <name>                       # Remove role")
	log.Println("   hyve config aws eks-role-get <name>                          # Get role ARN")
}

func getAWSEKSRole(name string) {
	accountName := getCurrentAWSAccount()
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	roleARN, err := mgr.GetAWSEKSRoleARN(accountName, name)
	if err != nil {
		log.Fatalf("❌ EKS role '%s' not found in account '%s'", name, accountName)
	}

	fmt.Printf("%s\n", roleARN)
}

// AWS Node Role helper functions
func addAWSNodeRole(name, roleARN string) {
	if name == "" {
		log.Fatal("Role name is required (--name)")
	}
	if roleARN == "" {
		log.Fatal("Role ARN is required (--role-arn)")
	}

	accountName := getCurrentAWSAccount()
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	exists, err := mgr.HasAWSNodeRole(accountName, name)
	if err != nil {
		log.Fatalf("Failed to check AWS config: %v", err)
	}

	if err := mgr.AddAWSNodeRole(accountName, name, roleARN); err != nil {
		log.Fatalf("Failed to add node role: %v", err)
	}

	if exists {
		log.Printf("✅ Updated node role '%s' in account '%s':\n", name, accountName)
	} else {
		log.Printf("✅ Added node role '%s' to account '%s':\n", name, accountName)
	}
	log.Printf("   Name:     %s", name)
	log.Printf("   Role ARN: %s", roleARN)
	log.Println()
	log.Println("💡 The configuration is stored in provider-configs/aws.yaml")
}

func removeAWSNodeRole(name string) {
	accountName := getCurrentAWSAccount()
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	roleARN, err := mgr.GetAWSNodeRoleARN(accountName, name)
	if err != nil {
		log.Fatalf("❌ Node role '%s' not found in account '%s'", name, accountName)
	}

	if err := mgr.RemoveAWSNodeRole(accountName, name); err != nil {
		log.Fatalf("Failed to remove node role: %v", err)
	}

	log.Printf("✅ Removed node role '%s' from account '%s' (ARN: %s)", name, accountName, roleARN)
}

func listAWSNodeRoles() {
	accountName := getCurrentAWSAccount()
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	roles, err := mgr.ListAWSNodeRoles(accountName)
	if err != nil {
		log.Fatalf("Failed to list node roles: %v", err)
	}

	if len(roles) == 0 {
		log.Printf("❌ No node roles configured for account '%s'", accountName)
		log.Println()
		log.Println("💡 Add a node role with:")
		log.Println("   hyve config aws node-role-add --name default-node-role --role-arn arn:aws:iam::123456789012:role/my-node-role")
		return
	}

	log.Printf("🔐 EKS Node IAM Roles for account '%s' (%d):\n", accountName, len(roles))
	log.Println()
	for _, r := range roles {
		log.Printf("   %s", r.Name)
		log.Printf("      Role ARN: %s", r.RoleARN)
		log.Println()
	}
	log.Println("💡 Commands:")
	log.Println("   hyve config aws node-role-add --name <name> --role-arn <arn>  # Add/update role")
	log.Println("   hyve config aws node-role-remove <name>                       # Remove role")
	log.Println("   hyve config aws node-role-get <name>                          # Get role ARN")
}

func getAWSNodeRole(name string) {
	accountName := getCurrentAWSAccount()
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	roleARN, err := mgr.GetAWSNodeRoleARN(accountName, name)
	if err != nil {
		log.Fatalf("❌ Node role '%s' not found in account '%s'", name, accountName)
	}

	fmt.Printf("%s\n", roleARN)
}

// AWS Node Role create/delete helper functions (actual AWS operations)
func createAWSNodeRole(name, roleName, region string) {
	if name == "" {
		log.Fatal("Role alias name is required (--name)")
	}
	if roleName == "" {
		log.Fatal("IAM role name is required (--role-name)")
	}
	if region == "" {
		region = "us-east-1"
	}

	accountName := getCurrentAWSAccount()
	repoPath := getRepoPath()
	configMgr := providerconfig.NewManager(repoPath)

	// Check if alias already exists
	exists, err := configMgr.HasAWSNodeRole(accountName, name)
	if err != nil {
		log.Fatalf("Failed to check AWS config: %v", err)
	}
	if exists {
		log.Fatalf("❌ Node role alias '%s' already exists in account '%s'. Use 'node-role-remove' first or choose a different name.", name, accountName)
	}

	log.Printf("🔐 Creating EKS node IAM role '%s' in AWS region %s...", roleName, region)

	// Create the AWS resource manager
	resourceMgr, err := aws.NewResourceManager(region)
	if err != nil {
		log.Fatalf("Failed to create AWS resource manager: %v", err)
	}

	// Create the IAM role for nodes
	ctx := gocontext.Background()
	roleInfo, err := resourceMgr.CreateNodeRole(ctx, roleName)
	if err != nil {
		log.Fatalf("Failed to create node IAM role in AWS: %v", err)
	}

	log.Printf("✅ Created IAM role '%s' in AWS", roleInfo.Name)
	log.Printf("   Role ARN: %s", roleInfo.ARN)

	// Store the alias in configuration
	if err := configMgr.AddAWSNodeRole(accountName, name, roleInfo.ARN); err != nil {
		log.Printf("⚠️  Warning: Role created in AWS but failed to save alias: %v", err)
		log.Printf("   You can manually add it with: hyve config aws node-role-add --name %s --role-arn %s", name, roleInfo.ARN)
		return
	}

	log.Printf("✅ Stored alias '%s' in account '%s'", name, accountName)
	log.Println()
	log.Println("💡 The configuration is stored in provider-configs/aws.yaml")
	log.Printf("💡 Use this role when creating EKS clusters with: --node-role-name %s", name)
}

func deleteAWSNodeRole(name, region string, configOnly bool) {
	if region == "" {
		region = "us-east-1"
	}

	accountName := getCurrentAWSAccount()
	repoPath := getRepoPath()
	configMgr := providerconfig.NewManager(repoPath)

	// Get the role ARN from config
	roleARN, err := configMgr.GetAWSNodeRoleARN(accountName, name)
	if err != nil {
		log.Fatalf("❌ Node role alias '%s' not found in account '%s'", name, accountName)
	}

	if configOnly {
		// Only remove from configuration
		if err := configMgr.RemoveAWSNodeRole(accountName, name); err != nil {
			log.Fatalf("Failed to remove node role from configuration: %v", err)
		}
		log.Printf("✅ Removed node role alias '%s' from account '%s'", name, accountName)
		log.Printf("   Note: The IAM role still exists in AWS (ARN: %s)", roleARN)
		return
	}

	// Extract role name from ARN (format: arn:aws:iam::123456789012:role/role-name)
	roleName := extractRoleNameFromARN(roleARN)
	if roleName == "" {
		log.Fatalf("❌ Could not extract role name from ARN: %s", roleARN)
	}

	log.Printf("🗑️  Deleting node IAM role '%s' from AWS...", roleName)

	// Create the AWS resource manager
	resourceMgr, err := aws.NewResourceManager(region)
	if err != nil {
		log.Fatalf("Failed to create AWS resource manager: %v", err)
	}

	// Delete the IAM role
	ctx := gocontext.Background()
	if err := resourceMgr.DeleteNodeRole(ctx, roleName); err != nil {
		log.Fatalf("Failed to delete node IAM role from AWS: %v\n\n"+
			"Configuration was NOT updated to prevent inconsistent state.\n"+
			"Use --config-only to remove only the configuration.", err)
	}

	log.Printf("✅ Deleted IAM role '%s' from AWS", roleName)

	// Remove from configuration
	if err := configMgr.RemoveAWSNodeRole(accountName, name); err != nil {
		log.Printf("⚠️  Warning: Role deleted from AWS but failed to remove alias: %v", err)
		return
	}

	log.Printf("✅ Removed alias '%s' from account '%s'", name, accountName)
}

// AWS VPC helper functions
func addAWSVPC(name, vpcID string) {
	if name == "" {
		log.Fatal("VPC name is required (--name)")
	}
	if vpcID == "" {
		log.Fatal("VPC ID is required (--id)")
	}

	accountName := getCurrentAWSAccount()
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	exists, err := mgr.HasAWSVPC(accountName, name)
	if err != nil {
		log.Fatalf("Failed to check AWS config: %v", err)
	}

	if err := mgr.AddAWSVPC(accountName, name, vpcID); err != nil {
		log.Fatalf("Failed to add VPC: %v", err)
	}

	if exists {
		log.Printf("✅ Updated VPC '%s' in account '%s':\n", name, accountName)
	} else {
		log.Printf("✅ Added VPC '%s' to account '%s':\n", name, accountName)
	}
	log.Printf("   Name:   %s", name)
	log.Printf("   VPC ID: %s", vpcID)
	log.Println()
	log.Println("💡 The configuration is stored in provider-configs/aws.yaml")
}

func removeAWSVPC(name string) {
	accountName := getCurrentAWSAccount()
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	vpcID, err := mgr.GetAWSVPCID(accountName, name)
	if err != nil {
		log.Fatalf("❌ VPC '%s' not found in account '%s'", name, accountName)
	}

	if err := mgr.RemoveAWSVPC(accountName, name); err != nil {
		log.Fatalf("Failed to remove VPC: %v", err)
	}

	log.Printf("✅ Removed VPC '%s' from account '%s' (ID: %s)", name, accountName, vpcID)
}

func listAWSVPCs() {
	accountName := getCurrentAWSAccount()
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	vpcs, err := mgr.ListAWSVPCs(accountName)
	if err != nil {
		log.Fatalf("Failed to list VPCs: %v", err)
	}

	if len(vpcs) == 0 {
		log.Printf("❌ No VPCs configured for account '%s'", accountName)
		log.Println()
		log.Println("💡 Add a VPC with:")
		log.Println("   hyve config aws vpc-add --name default-vpc --id vpc-0123456789abcdef0")
		return
	}

	log.Printf("🌐 VPCs for account '%s' (%d):\n", accountName, len(vpcs))
	log.Println()
	for _, v := range vpcs {
		log.Printf("   %s", v.Name)
		log.Printf("      VPC ID: %s", v.VPCID)
		log.Println()
	}
	log.Println("💡 Commands:")
	log.Println("   hyve config aws vpc-add --name <name> --id <vpc-id>  # Add/update VPC")
	log.Println("   hyve config aws vpc-remove <name>                    # Remove VPC")
	log.Println("   hyve config aws vpc-get <name>                       # Get VPC ID")
}

func getAWSVPC(name string) {
	accountName := getCurrentAWSAccount()
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	vpcID, err := mgr.GetAWSVPCID(accountName, name)
	if err != nil {
		log.Fatalf("❌ VPC '%s' not found in account '%s'", name, accountName)
	}

	fmt.Printf("%s\n", vpcID)
}

// AWS EKS Role create/delete helper functions (actual AWS operations)
func createAWSEKSRole(name, roleName, region string) {
	if name == "" {
		log.Fatal("Role alias name is required (--name)")
	}
	if roleName == "" {
		log.Fatal("IAM role name is required (--role-name)")
	}
	if region == "" {
		region = "us-east-1"
	}

	accountName := getCurrentAWSAccount()
	repoPath := getRepoPath()
	configMgr := providerconfig.NewManager(repoPath)

	// Check if alias already exists
	exists, err := configMgr.HasAWSEKSRole(accountName, name)
	if err != nil {
		log.Fatalf("Failed to check AWS config: %v", err)
	}
	if exists {
		log.Fatalf("❌ EKS role alias '%s' already exists in account '%s'. Use 'eks-role-remove' first or choose a different name.", name, accountName)
	}

	log.Printf("🔐 Creating EKS IAM role '%s' in AWS region %s...", roleName, region)

	// Create the AWS resource manager
	resourceMgr, err := aws.NewResourceManager(region)
	if err != nil {
		log.Fatalf("Failed to create AWS resource manager: %v", err)
	}

	// Create the IAM role
	ctx := gocontext.Background()
	roleInfo, err := resourceMgr.CreateEKSRole(ctx, roleName)
	if err != nil {
		log.Fatalf("Failed to create EKS IAM role in AWS: %v", err)
	}

	log.Printf("✅ Created IAM role '%s' in AWS", roleInfo.Name)
	log.Printf("   Role ARN: %s", roleInfo.ARN)

	// Store the alias in configuration
	if err := configMgr.AddAWSEKSRole(accountName, name, roleInfo.ARN); err != nil {
		log.Printf("⚠️  Warning: Role created in AWS but failed to save alias: %v", err)
		log.Printf("   You can manually add it with: hyve config aws eks-role-add --name %s --role-arn %s", name, roleInfo.ARN)
		return
	}

	log.Printf("✅ Stored alias '%s' in account '%s'", name, accountName)
	log.Println()
	log.Println("💡 The configuration is stored in provider-configs/aws.yaml")
	log.Printf("💡 Use this role when creating EKS clusters with: --eks-role %s", name)
}

func deleteAWSEKSRole(name, region string, configOnly bool) {
	if region == "" {
		region = "us-east-1"
	}

	accountName := getCurrentAWSAccount()
	repoPath := getRepoPath()
	configMgr := providerconfig.NewManager(repoPath)

	// Get the role ARN from config
	roleARN, err := configMgr.GetAWSEKSRoleARN(accountName, name)
	if err != nil {
		log.Fatalf("❌ EKS role alias '%s' not found in account '%s'", name, accountName)
	}

	if configOnly {
		// Only remove from configuration
		if err := configMgr.RemoveAWSEKSRole(accountName, name); err != nil {
			log.Fatalf("Failed to remove EKS role from configuration: %v", err)
		}
		log.Printf("✅ Removed EKS role alias '%s' from account '%s'", name, accountName)
		log.Printf("   Note: The IAM role still exists in AWS (ARN: %s)", roleARN)
		return
	}

	// Extract role name from ARN (format: arn:aws:iam::123456789012:role/role-name)
	roleName := extractRoleNameFromARN(roleARN)
	if roleName == "" {
		log.Fatalf("❌ Could not extract role name from ARN: %s", roleARN)
	}

	log.Printf("🗑️  Deleting EKS IAM role '%s' from AWS...", roleName)

	// Create the AWS resource manager
	resourceMgr, err := aws.NewResourceManager(region)
	if err != nil {
		log.Fatalf("Failed to create AWS resource manager: %v", err)
	}

	// Delete the IAM role
	ctx := gocontext.Background()
	if err := resourceMgr.DeleteEKSRole(ctx, roleName); err != nil {
		log.Fatalf("Failed to delete EKS IAM role from AWS: %v\n\n"+
			"Configuration was NOT updated to prevent inconsistent state.\n"+
			"Use --config-only to remove only the configuration.", err)
	}

	log.Printf("✅ Deleted IAM role '%s' from AWS", roleName)

	// Remove from configuration
	if err := configMgr.RemoveAWSEKSRole(accountName, name); err != nil {
		log.Printf("⚠️  Warning: Role deleted from AWS but failed to remove alias: %v", err)
		return
	}

	log.Printf("✅ Removed alias '%s' from account '%s'", name, accountName)
}

// extractRoleNameFromARN extracts the role name from an IAM role ARN
func extractRoleNameFromARN(arn string) string {
	// ARN format: arn:aws:iam::123456789012:role/role-name
	parts := strings.Split(arn, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return ""
}

// AWS VPC create/delete helper functions (actual AWS operations)
func createAWSVPC(name, region, cidr, subnets string, enableDNS bool) {
	if name == "" {
		log.Fatal("VPC alias name is required (--name)")
	}
	if region == "" {
		region = "us-east-1"
	}
	if cidr == "" {
		cidr = "10.0.0.0/16"
	}

	accountName := getCurrentAWSAccount()
	repoPath := getRepoPath()
	configMgr := providerconfig.NewManager(repoPath)

	// Check if alias already exists
	exists, err := configMgr.HasAWSVPC(accountName, name)
	if err != nil {
		log.Fatalf("Failed to check AWS config: %v", err)
	}
	if exists {
		log.Fatalf("❌ VPC alias '%s' already exists in account '%s'. Use 'vpc-remove' first or choose a different name.", name, accountName)
	}

	log.Printf("🌐 Creating VPC '%s' in AWS region %s...", name, region)
	log.Printf("   CIDR: %s", cidr)

	// Create the AWS resource manager
	resourceMgr, err := aws.NewResourceManager(region)
	if err != nil {
		log.Fatalf("Failed to create AWS resource manager: %v", err)
	}

	// Parse subnet CIDRs
	var subnetCIDRs []string
	if subnets != "" {
		parts := strings.Split(subnets, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				subnetCIDRs = append(subnetCIDRs, trimmed)
			}
		}
	}

	// Create the VPC
	ctx := gocontext.Background()
	vpcInput := &aws.CreateVPCInput{
		Name:              name,
		CIDR:              cidr,
		EnableDNSSupport:  enableDNS,
		EnableDNSHostname: enableDNS,
		CreateSubnets:     len(subnetCIDRs) > 0,
		SubnetCIDRs:       subnetCIDRs,
	}

	vpcInfo, err := resourceMgr.CreateVPC(ctx, vpcInput)
	if err != nil {
		log.Fatalf("Failed to create VPC in AWS: %v", err)
	}

	log.Printf("✅ Created VPC '%s' in AWS", vpcInfo.Name)
	log.Printf("   VPC ID: %s", vpcInfo.ID)
	log.Printf("   CIDR:   %s", vpcInfo.CIDR)
	log.Printf("   State:  %s", vpcInfo.State)

	if len(vpcInfo.Subnets) > 0 {
		log.Printf("   Subnets:")
		for _, subnet := range vpcInfo.Subnets {
			log.Printf("      %s (%s) - %s", subnet.ID, subnet.CIDR, subnet.AvailabilityZone)
		}
	}

	// Store the alias in configuration
	if err := configMgr.AddAWSVPC(accountName, name, vpcInfo.ID); err != nil {
		log.Printf("⚠️  Warning: VPC created in AWS but failed to save alias: %v", err)
		log.Printf("   You can manually add it with: hyve config aws vpc-add --name %s --id %s", name, vpcInfo.ID)
		return
	}

	log.Printf("✅ Stored alias '%s' in account '%s'", name, accountName)
	log.Println()
	log.Println("💡 The configuration is stored in provider-configs/aws.yaml")
	log.Printf("💡 Use this VPC when creating EKS clusters with: --vpc %s", name)
}

func deleteAWSVPC(name, region string, configOnly bool) {
	if region == "" {
		region = "us-east-1"
	}

	accountName := getCurrentAWSAccount()
	repoPath := getRepoPath()
	configMgr := providerconfig.NewManager(repoPath)

	// Get the VPC ID from config
	vpcID, err := configMgr.GetAWSVPCID(accountName, name)
	if err != nil {
		log.Fatalf("❌ VPC alias '%s' not found in account '%s'", name, accountName)
	}

	if configOnly {
		// Only remove from configuration
		if err := configMgr.RemoveAWSVPC(accountName, name); err != nil {
			log.Fatalf("Failed to remove VPC from configuration: %v", err)
		}
		log.Printf("✅ Removed VPC alias '%s' from account '%s'", name, accountName)
		log.Printf("   Note: The VPC still exists in AWS (ID: %s)", vpcID)
		return
	}

	log.Printf("🗑️  Deleting VPC '%s' from AWS...", vpcID)

	// Create the AWS resource manager
	resourceMgr, err := aws.NewResourceManager(region)
	if err != nil {
		log.Fatalf("Failed to create AWS resource manager: %v", err)
	}

	// Delete the VPC
	ctx := gocontext.Background()
	if err := resourceMgr.DeleteVPC(ctx, vpcID); err != nil {
		log.Fatalf("Failed to delete VPC from AWS: %v\n\n"+
			"Configuration was NOT updated to prevent inconsistent state.\n"+
			"Use --config-only to remove only the configuration.", err)
	}

	log.Printf("✅ Deleted VPC '%s' from AWS", vpcID)

	// Remove from configuration
	if err := configMgr.RemoveAWSVPC(accountName, name); err != nil {
		log.Printf("⚠️  Warning: VPC deleted from AWS but failed to remove alias: %v", err)
		return
	}

	log.Printf("✅ Removed alias '%s' from account '%s'", name, accountName)
}

// Azure helper functions
func addAzureSubscriptionIDs(args []string) {
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	// Parse args as name=id pairs or just IDs
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		var name, subscriptionID string
		if len(parts) == 2 {
			name = parts[0]
			subscriptionID = parts[1]
		} else {
			// Use the subscription ID as the name
			name = arg
			subscriptionID = arg
		}

		exists, err := mgr.HasAzureSubscription(name)
		if err != nil {
			log.Fatalf("Failed to check Azure config: %v", err)
		}

		if err := mgr.AddAzureSubscription(name, subscriptionID); err != nil {
			log.Fatalf("Failed to add Azure subscription: %v", err)
		}

		if exists {
			log.Printf("✅ Updated Azure subscription '%s'", name)
		} else {
			log.Printf("✅ Added Azure subscription '%s' (ID: %s)", name, subscriptionID)
		}
	}

	log.Println()
	log.Println("💡 The configuration is stored in provider-configs/azure.yaml")
}

func removeAzureSubscriptionIDs(args []string) {
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	for _, name := range args {
		if err := mgr.RemoveAzureSubscription(name); err != nil {
			log.Printf("❌ Failed to remove subscription '%s': %v", name, err)
			continue
		}
		log.Printf("✅ Removed Azure subscription '%s'", name)
	}
}

func listAzureSubscriptionIDs() {
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	subscriptions, err := mgr.ListAzureSubscriptions()
	if err != nil {
		log.Fatalf("Failed to load Azure config: %v", err)
	}

	if len(subscriptions) == 0 {
		log.Println("❌ No Azure subscriptions configured")
		log.Println()
		log.Println("💡 Add subscriptions with:")
		log.Println("   hyve config azure add-subscription-ids name=<subscription-id>")
		return
	}

	log.Printf("🔷 Azure Subscriptions (%d):\n", len(subscriptions))
	log.Println()
	for _, s := range subscriptions {
		log.Printf("   %s", s.Name)
		log.Printf("      Subscription ID: %s", s.SubscriptionID)
		log.Println()
	}
	log.Println("💡 Commands:")
	log.Println("   hyve config azure add-subscription-ids name=<id>     # Add subscription")
	log.Println("   hyve config azure remove-subscription-ids <name>     # Remove subscription")
}

// Civo helper functions
func addCivoOrganization(name, orgID string) {
	if name == "" {
		log.Fatal("Organization name is required (--name)")
	}
	if orgID == "" {
		log.Fatal("Organization ID is required (--id)")
	}

	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	exists, err := mgr.HasCivoOrganization(name)
	if err != nil {
		log.Fatalf("Failed to check Civo config: %v", err)
	}

	if err := mgr.AddCivoOrganization(name, orgID); err != nil {
		log.Fatalf("Failed to add Civo organization: %v", err)
	}

	if exists {
		log.Printf("✅ Updated Civo organization '%s':\n", name)
	} else {
		log.Printf("✅ Added Civo organization '%s':\n", name)
	}
	log.Printf("   Name:  %s", name)
	log.Printf("   Org ID: %s", orgID)
	log.Println()
	log.Println("💡 The configuration is stored in provider-configs/civo.yaml")
	log.Println("💡 Set this as the current organization with:")
	log.Printf("   hyve config use civo %s", name)
}

func removeCivoOrganization(name string) {
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	orgID, err := mgr.GetCivoOrgID(name)
	if err != nil {
		log.Fatalf("❌ Civo organization '%s' not found", name)
	}

	if err := mgr.RemoveCivoOrganization(name); err != nil {
		log.Fatalf("Failed to remove Civo organization: %v", err)
	}

	log.Printf("✅ Removed Civo organization '%s' (ID: %s)", name, orgID)
}

func listCivoOrganizations() {
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	orgs, err := mgr.ListCivoOrganizations()
	if err != nil {
		log.Fatalf("Failed to list Civo organizations: %v", err)
	}

	if len(orgs) == 0 {
		log.Println("❌ No Civo organizations configured")
		log.Println()
		log.Println("💡 Add an organization with:")
		log.Println("   hyve config civo org-add --name prod --id org-abc123")
		return
	}

	log.Printf("🟢 Civo Organizations (%d):\n", len(orgs))
	log.Println()
	for _, o := range orgs {
		log.Printf("   %s", o.Name)
		log.Printf("      Org ID: %s", o.OrgID)
		if len(o.Regions) > 0 {
			log.Printf("      Regions: %v", o.Regions)
		}
		log.Println()
	}
	log.Println("💡 Commands:")
	log.Println("   hyve config civo org-add --name <name> --id <org-id>  # Add/update organization")
	log.Println("   hyve config civo org-remove <name>                     # Remove organization")
	log.Println("   hyve config civo org-get <name>                        # Get organization ID")
	log.Println()
	log.Println("💡 Set the current organization with:")
	log.Println("   hyve config use civo <name>")
}

func getCivoOrganization(name string) {
	repoPath := getRepoPath()
	mgr := providerconfig.NewManager(repoPath)

	orgID, err := mgr.GetCivoOrgID(name)
	if err != nil {
		log.Fatalf("❌ Civo organization '%s' not found", name)
	}

	fmt.Printf("%s\n", orgID)
}
