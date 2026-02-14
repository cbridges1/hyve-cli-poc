<p align="center">
  <img src="images/banner.svg" alt="Hyve Banner" width="800">
</p>

# Hyve - GitOps Kubernetes Cluster Management CLI

A declarative GitOps Kubernetes cluster management tool with multi-cloud support (Civo, AWS EKS, GCP GKE, Azure AKS), multi-repository support, automated workflows, and secure credential management.

[![Documentation](https://img.shields.io/badge/docs-hyve.dev-green)](https://docs.hyve.dev)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Features

- **GitOps Native** - All cluster state managed through Git repositories
- **Multi-Cloud Support** - Civo, AWS EKS, GCP GKE, and Azure AKS
- **Native Cloud Authentication** - Uses AWS CLI, gcloud, and Azure CLI for authentication
- **Multi-Repository** - Separate repos for dev/staging/prod environments
- **Automated Workflows** - Define deployment pipelines with requirements validation
- **Cluster Templates** - Reusable cluster patterns with automated workflows
- **Secure Credentials** - AES-GCM encrypted storage for Civo tokens and kubeconfigs
- **AWS Resource Management** - Create and manage EKS IAM roles and VPCs directly
- **Provider Aliases** - Named aliases for GCP projects, AWS accounts, roles, and VPCs
- **Variable Substitution** - Full shell support with workflow and environment variables

## Quick Start

```bash
# 1. Build Hyve
go build -o hyve .

# 2. Configure authentication for your cloud provider
# For Civo (stores encrypted token in Hyve):
./hyve config set-token civo --account default

# For AWS, GCP, or Azure (use native CLI authentication):
# aws configure                              # AWS
# gcloud auth application-default login      # GCP
# az login                                   # Azure

# 3. Set up Git repository
./hyve git add production --repo-url https://github.com/company/hyve-prod.git

# 4. Configure provider accounts (required before creating clusters)
# Add an account and set it as the current context
./hyve config aws account-add --name prod --id 123456789012
./hyve config use aws prod

# 5. Create a cluster (specify provider)
./hyve cluster add my-cluster --provider civo --region PHX1 --nodes g4s.kube.medium
# or
./hyve cluster add my-cluster --provider aws --region us-east-1 --nodes t3.medium
# or (GCP requires project configured first)
./hyve config gcp project-add --name dev --id my-gcp-project-id
./hyve config use gcp dev
./hyve cluster add my-cluster --provider gcp --region us-central1 --nodes e2-medium
# or
./hyve cluster add my-cluster --provider azure --region eastus --nodes Standard_D2s_v3

# 6. Run a workflow
./hyve workflow run deploy-app --cluster my-cluster
```

## Installation

### Prerequisites

- Go 1.21 or higher
- Git (required - Hyve uses system git by default for easier onboarding)
- One or more cloud provider accounts:
  - **Civo**: API token (stored encrypted in Hyve)
  - **AWS**: AWS CLI configured (`aws configure`)
  - **GCP**: gcloud CLI authenticated (`gcloud auth application-default login`)
  - **Azure**: Azure CLI authenticated (`az login`)

### Build from Source

```bash
git clone <repository-url>
cd hyve
go build -o hyve .
```

### Configure

```bash
# Verify git is available (required for default backend)
git --version

# Configure cloud provider authentication:

# Option 1: Civo (token stored encrypted in Hyve)
./hyve config set-token civo --account default
# Or for multiple Civo accounts:
./hyve config set-token civo --account production
./hyve config set-token civo --account development

# Option 2: AWS (uses native AWS CLI authentication)
aws configure
# Or use environment variables:
# - AWS_ACCESS_KEY_ID
# - AWS_SECRET_ACCESS_KEY

# Option 3: GCP (uses Application Default Credentials)
gcloud auth application-default login

# Option 4: Azure (uses Azure CLI authentication)
az login

# Configure Git credentials (for private repos)
./hyve git credentials --username your-username --password your-token

# Add your first repository
./hyve git add production --repo-url https://github.com/company/hyve-state.git

# Configure provider accounts (REQUIRED before adding resources)
# Each provider requires an account/project/subscription to be configured and selected

# AWS: Add account and set as current context
./hyve config aws account-add --name prod --id 123456789012
./hyve config use aws prod

# GCP: Add project and set as current context
./hyve config gcp project-add --name dev --id my-gcp-project-id
./hyve config use gcp dev

# Azure: Add subscription and set as current context
./hyve config azure subscription-add --name prod --id 12345678-1234-1234-1234-123456789012
./hyve config use azure prod

# Civo: Add organization and set as current context
./hyve config civo org-add --name default --id org-123456
./hyve config use civo default

# View current context for all providers
./hyve config context

# AWS: Create EKS IAM role (optional - creates actual AWS resource)
./hyve config aws eks-role-create --name default-role --role-name hyve-eks-role --region us-east-1

# AWS: Create VPC (optional - creates actual AWS resource)
./hyve config aws vpc-create --name dev-vpc --region us-east-1 --cidr 10.0.0.0/16
```

<details>
<summary>Optional: Use built-in git library</summary>

By default, Hyve uses your system's git command for easier onboarding. If you prefer the built-in go-git library:

```bash
# Set to built-in git (persisted in config)
./hyve config set-git-backend builtin

# Or switch back to system git
./hyve config set-git-backend system

# Check current backend
./hyve config get-git-backend
```

The preference is stored in `~/.hyve/config.yaml` and persists across sessions.
</details>

## Documentation

📚 **[Full Documentation](https://docs.hyve.dev)**

- [Quick Start Guide](https://docs.hyve.dev/quickstart)
- [Installation](https://docs.hyve.dev/installation)
- [Configuration](https://docs.hyve.dev/configuration)
- [Git Management](https://docs.hyve.dev/guides/git-management)
- [Cluster Management](https://docs.hyve.dev/guides/cluster-management)
- [Workflows](https://docs.hyve.dev/workflows/overview)
- [Templates](https://docs.hyve.dev/guides/template-management)
- [CLI Reference](https://docs.hyve.dev/cli/overview)

## Authentication

Hyve uses different authentication methods depending on the cloud provider:

| Provider | Authentication Method | How to Configure |
|----------|----------------------|------------------|
| **Civo** | API token stored encrypted in Hyve | `hyve config set-token civo` |
| **AWS** | Native AWS CLI credentials | `aws configure` or environment variables |
| **GCP** | Application Default Credentials | `gcloud auth application-default login` |
| **Azure** | Azure CLI authentication | `az login` |

### Why Native CLI Authentication?

For AWS, GCP, and Azure, Hyve uses the native cloud CLI authentication instead of storing credentials. This provides:

- **Security**: Credentials are managed by the official cloud CLIs with their security features
- **Consistency**: Same credentials used by other tools (Terraform, kubectl, etc.)
- **SSO Support**: Works with corporate SSO, MFA, and identity federation
- **No Duplication**: No need to manage credentials in multiple places

Civo is the exception because it doesn't have a widely-used CLI, so Hyve stores Civo API tokens securely using AES-GCM encryption.

## Key Concepts

### Provider Context

Before working with any cloud provider, you must set a current context (account/project/subscription/organization). This tells Hyve which account's resources to use when creating clusters.

```bash
# Set current context
hyve config use aws prod
hyve config use gcp dev
hyve config use azure prod
hyve config use civo default

# View all contexts
hyve config context

# Output:
# Current context:
#   AWS:   prod (Account ID: 123456789012)
#   GCP:   dev (Project ID: my-dev-project-123)
#   Azure: prod (Subscription ID: 12345678-1234-1234-1234-123456789012)
#   Civo:  default (Org ID: org-123456)
```

If you try to work with a provider without setting the context, Hyve will display CLI commands to switch to the required account:

```
❌ No AWS account selected.

Set the current account with:
  hyve config use aws <account-name>

Or switch AWS CLI credentials:
  export AWS_PROFILE=<profile-name>
```

### Repositories

Git repositories store cluster definitions, workflows, and templates:

```bash
hyve git add production --repo-url https://github.com/company/hyve-prod.git
hyve git add development --repo-url https://github.com/company/hyve-dev.git
hyve git use production
```

### Clusters

Define Kubernetes clusters as YAML files:

```yaml
# Civo cluster
apiVersion: v1
kind: Cluster
metadata:
  name: production
  region: PHX1
spec:
  provider: civo
  nodes:
    - g4s.kube.large
    - g4s.kube.large
---
# AWS EKS cluster
apiVersion: v1
kind: Cluster
metadata:
  name: production
  region: us-east-1
spec:
  provider: aws
  nodes:
    - t3.large
    - t3.large
---
# GCP GKE cluster (with project alias and ID)
apiVersion: v1
kind: Cluster
metadata:
  name: production
  region: us-central1
spec:
  provider: gcp
  gcpProject: dev           # Project alias (configured via hyve config gcp add-project)
  gcpProjectId: my-project  # Resolved project ID (auto-populated)
  nodes:
    - e2-standard-4
---
# Azure AKS cluster
apiVersion: v1
kind: Cluster
metadata:
  name: production
  region: eastus
spec:
  provider: azure
  nodes:
    - Standard_D2s_v3
```

### Workflows

Automate deployments with requirements validation:

```yaml
apiVersion: v1
kind: Workflow
metadata:
  name: deploy-app
spec:
  requirements:
    tools:
      - name: kubectl
        version: "1.28"
    secrets:
      - name: DOCKER_TOKEN
        provider: docker
  jobs:
    - name: deploy
      steps:
        - name: apply
          command: kubectl apply -f manifests/
```

### Templates

Reusable cluster configurations with workflows:

```bash
# Create template
hyve template create prod-template \
  --region NYC1 \
  --nodes g4s.kube.large,g4s.kube.large,g4s.kube.large \
  --workflows setup-monitoring,deploy-app

# Execute template
hyve template execute prod-template prod-cluster-01
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `hyve git` | Manage Git repositories |
| `hyve cluster` | Manage cluster definitions |
| `hyve workflow` | Run and manage workflows |
| `hyve template` | Manage cluster templates |
| `hyve kubeconfig` | Manage cluster kubeconfigs |
| `hyve config` | Configure API tokens and provider settings |
| `hyve config use` | Set current account/project/subscription for a provider |
| `hyve config context` | Show current context for all providers |
| `hyve config context-clear` | Clear context for a provider or all providers |
| `hyve config gcp` | Manage GCP provider configuration (projects) |
| `hyve config aws` | Manage AWS provider configuration (accounts, EKS roles, node roles, VPCs) |
| `hyve config azure` | Manage Azure provider configuration (subscriptions) |
| `hyve config civo` | Manage Civo provider configuration (organizations) |
| `hyve reconcile` | Reconcile cluster state |

### Context Commands

Manage the current account/project/subscription context for each provider:

| Command | Description |
|---------|-------------|
| `hyve config use <provider> <name>` | Set current context for a provider |
| `hyve config context` | Show current context for all providers |
| `hyve config context-clear [provider]` | Clear context (all or specific provider) |

**Examples:**
```bash
hyve config use aws prod           # Set AWS account to "prod"
hyve config use gcp my-project     # Set GCP project to "my-project"
hyve config use azure dev          # Set Azure subscription to "dev"
hyve config use civo default       # Set Civo organization to "default"
```

### AWS Resource Commands

Hyve can create and manage actual AWS resources:

| Command | Description |
|---------|-------------|
| `hyve config aws eks-role-create` | Create an EKS IAM role in AWS |
| `hyve config aws eks-role-delete` | Delete an EKS IAM role from AWS |
| `hyve config aws node-role-create` | Create an EKS node IAM role in AWS |
| `hyve config aws node-role-delete` | Delete an EKS node IAM role from AWS |
| `hyve config aws vpc-create` | Create a VPC in AWS (with optional subnets) |
| `hyve config aws vpc-delete` | Delete a VPC from AWS |

These commands use native AWS SDK authentication (via `aws configure` or environment variables).

### Provider Configuration Commands

Store provider-specific configurations in your repository for team sharing. All provider configs support aliases for easier reference.

**Important:** You must set a current context (account/project/subscription/organization) before adding resources for any provider. Use `hyve config use <provider> <name>` to set the current context.

#### Context Management

```bash
# Set current context for a provider
hyve config use aws prod           # Set current AWS account
hyve config use gcp dev            # Set current GCP project
hyve config use azure prod         # Set current Azure subscription
hyve config use civo default       # Set current Civo organization

# View current context for all providers
hyve config context

# Clear context for a specific provider
hyve config context-clear aws

# Clear all context
hyve config context-clear
```

When working with a provider, if the required account/project is not set, Hyve will display CLI commands to switch to the correct account.

#### GCP Configuration

```bash
# Add GCP projects with aliases
hyve config gcp project-add --name dev --id my-dev-project-123
hyve config gcp project-add --name prod --id my-prod-project-456

# Set current GCP project
hyve config use gcp dev

# List configured projects
hyve config gcp project-list

# Get project ID by alias
hyve config gcp project-get dev

# Remove a project
hyve config gcp project-remove dev

# Use alias when creating clusters
hyve cluster add my-cluster --provider gcp --region us-central1
```

#### AWS Configuration

AWS resources (VPCs, EKS roles, node roles) are organized under accounts. You must first add an account and set it as the current context before adding resources.

```bash
# Account management (REQUIRED first step)
hyve config aws account-add --name prod --id 123456789012
hyve config aws account-add --name dev --id 987654321098
hyve config aws account-list
hyve config aws account-get prod
hyve config aws account-remove prod

# Set current AWS account (REQUIRED before adding resources)
hyve config use aws prod

# EKS IAM Role management (configuration only - added to current account)
hyve config aws eks-role-add --name default-role --role-arn arn:aws:iam::123456789012:role/my-eks-role
hyve config aws eks-role-list
hyve config aws eks-role-get default-role
hyve config aws eks-role-remove default-role

# EKS IAM Role creation (creates actual AWS resources - added to current account)
hyve config aws eks-role-create --name default-role --role-name my-eks-cluster-role --region us-east-1
hyve config aws eks-role-delete default-role --region us-east-1
hyve config aws eks-role-delete default-role --config-only  # Remove from config only

# Node Role management (configuration only - added to current account)
hyve config aws node-role-add --name default-node-role --role-arn arn:aws:iam::123456789012:role/my-node-role
hyve config aws node-role-list
hyve config aws node-role-get default-node-role
hyve config aws node-role-remove default-node-role

# Node Role creation (creates actual AWS resources - added to current account)
hyve config aws node-role-create --name default-node-role --role-name my-eks-node-role --region us-east-1
hyve config aws node-role-delete default-node-role --region us-east-1
hyve config aws node-role-delete default-node-role --config-only  # Remove from config only

# VPC management (configuration only - added to current account)
hyve config aws vpc-add --name default-vpc --id vpc-0123456789abcdef0
hyve config aws vpc-list
hyve config aws vpc-get default-vpc
hyve config aws vpc-remove default-vpc

# VPC creation (creates actual AWS resources - added to current account)
hyve config aws vpc-create --name dev-vpc --region us-east-1 --cidr 10.0.0.0/16
hyve config aws vpc-create --name dev-vpc --region us-east-1 --subnets 10.0.1.0/24,10.0.2.0/24
hyve config aws vpc-delete dev-vpc --region us-east-1
hyve config aws vpc-delete dev-vpc --config-only  # Remove from config only
```

#### Azure Configuration

```bash
# Add subscriptions with aliases
hyve config azure subscription-add --name prod --id 12345678-1234-1234-1234-123456789012
hyve config azure subscription-add --name dev --id 87654321-4321-4321-4321-210987654321
hyve config azure subscription-list
hyve config azure subscription-get prod
hyve config azure subscription-remove prod

# Set current Azure subscription
hyve config use azure prod
```

#### Civo Configuration

```bash
# Add organizations with aliases
hyve config civo org-add --name default --id org-123456
hyve config civo org-add --name production --id org-789012
hyve config civo org-list
hyve config civo org-get default
hyve config civo org-remove default

# Set current Civo organization
hyve config use civo default
```

Provider configurations are stored in `provider-configs/` in your repository and can be committed to Git. Current context is stored locally in `~/.hyve/context.yaml` (not committed to Git).

See [CLI Reference](https://docs.hyve.dev/cli/overview) for complete command documentation.

## Storage

Hyve stores all data in `~/.hyve/`:

```
~/.hyve/
├── config.yaml          # Global configuration (git backend preference)
├── context.yaml         # Current provider context (account/project selections - local only)
├── repositories.db      # Repository configurations (SQLite)
├── credentials.db       # Encrypted Civo tokens and Git credentials (AES-GCM)
├── kubeconfigs.db      # Encrypted cluster kubeconfigs (AES-GCM)
├── temp/               # Temporary kubeconfig files
└── repositories/       # Cloned repository storage
    ├── production/
    │   ├── clusters/         # Cluster YAML files
    │   ├── workflows/        # Workflow definitions
    │   ├── templates/        # Cluster templates
    │   └── provider-configs/ # Provider-specific configuration
    │       ├── gcp.yaml      # GCP projects (name/ID aliases)
    │       ├── aws.yaml      # AWS accounts with nested resources
    │       ├── azure.yaml    # Azure subscriptions
    │       └── civo.yaml     # Civo organizations
    └── development/
```

### Local Context File

The `~/.hyve/context.yaml` file tracks the current account/project/subscription for each provider. This file is local-only and not committed to the repository, allowing each developer to work with different accounts.

**~/.hyve/context.yaml:**
```yaml
aws:
  account: prod
gcp:
  account: dev
azure:
  account: prod
civo:
  account: default
```

### Provider Config File Examples

**provider-configs/gcp.yaml:**
```yaml
projects:
  - name: dev
    project_id: my-dev-project-123
  - name: prod
    project_id: my-prod-project-456
```

**provider-configs/aws.yaml:**
```yaml
# AWS resources are nested under accounts
accounts:
  - name: prod
    account_id: "123456789012"
    regions:
      - us-east-1
      - us-west-2
    vpcs:
      - name: default-vpc
        vpc_id: vpc-0123456789abcdef0
      - name: dev-vpc
        vpc_id: vpc-0987654321fedcba0
    eks_roles:
      - name: default-role
        role_arn: arn:aws:iam::123456789012:role/my-eks-role
    node_roles:
      - name: default-node-role
        role_arn: arn:aws:iam::123456789012:role/my-node-role
  - name: dev
    account_id: "987654321098"
    regions:
      - us-east-1
    vpcs:
      - name: dev-vpc
        vpc_id: vpc-abcdef0123456789
    eks_roles:
      - name: dev-role
        role_arn: arn:aws:iam::987654321098:role/dev-eks-role
    node_roles:
      - name: dev-node-role
        role_arn: arn:aws:iam::987654321098:role/dev-node-role
```

**provider-configs/azure.yaml:**
```yaml
subscriptions:
  - name: prod
    subscription_id: "12345678-1234-1234-1234-123456789012"
  - name: dev
    subscription_id: "87654321-4321-4321-4321-210987654321"
```

**provider-configs/civo.yaml:**
```yaml
organizations:
  - name: default
    org_id: "org-123456"
    regions:
      - PHX1
      - NYC1
  - name: production
    org_id: "org-789012"
    regions:
      - LON1
```

## Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test ./... -cover

# Run specific package tests
go test ./internal/credentials -v
go test ./internal/workflow -v
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes and test locally
4. Submit a pull request

## License

[Add your license here]

## Support

- **Documentation**: [https://docs.hyve.dev](https://docs.hyve.dev)
- **Issues**: [GitHub Issues](https://github.com/your-org/hyve/issues)
- **Community**: [Discord](https://discord.gg/your-discord)

---

For detailed documentation, visit **[docs.hyve.dev](https://docs.hyve.dev)**
