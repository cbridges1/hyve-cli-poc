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

# 4. Create a cluster (specify provider)
./hyve cluster add my-cluster --provider civo --region PHX1 --nodes g4s.kube.medium
# or
./hyve cluster add my-cluster --provider aws --region us-east-1 --nodes t3.medium
# or (GCP requires project alias configured first)
./hyve config gcp add-project --name dev --id my-gcp-project-id
./hyve cluster add my-cluster --provider gcp --gcp-project dev --region us-central1 --nodes e2-medium
# or
./hyve cluster add my-cluster --provider azure --region eastus --nodes Standard_D2s_v3

# 5. Run a workflow
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

# Configure provider-specific settings (stored in repository)
# GCP: Add project aliases
./hyve config gcp add-project --name dev --id my-gcp-project-id

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
| `hyve config gcp` | Manage GCP provider configuration (projects) |
| `hyve config aws` | Manage AWS provider configuration (accounts, EKS roles, VPCs) |
| `hyve config azure` | Manage Azure provider configuration (subscriptions) |
| `hyve reconcile` | Reconcile cluster state |

### AWS Resource Commands

Hyve can create and manage actual AWS resources:

| Command | Description |
|---------|-------------|
| `hyve config aws eks-role-create` | Create an EKS IAM role in AWS |
| `hyve config aws eks-role-delete` | Delete an EKS IAM role from AWS |
| `hyve config aws vpc-create` | Create a VPC in AWS (with optional subnets) |
| `hyve config aws vpc-delete` | Delete a VPC from AWS |

These commands use native AWS SDK authentication (via `aws configure` or environment variables).

### Provider Configuration Commands

Store provider-specific configurations in your repository for team sharing. All provider configs support aliases for easier reference.

#### GCP Configuration

```bash
# Add GCP projects with aliases
hyve config gcp add-project --name dev --id my-dev-project-123
hyve config gcp add-project --name prod --id my-prod-project-456

# List configured projects
hyve config gcp list-projects

# Get project ID by alias
hyve config gcp get-project dev

# Remove a project
hyve config gcp remove-project dev

# Use alias when creating clusters
hyve cluster add my-cluster --provider gcp --gcp-project dev --region us-central1
```

#### AWS Configuration

```bash
# Account management (with aliases)
hyve config aws account-add --name prod --id 123456789012
hyve config aws account-list
hyve config aws account-get prod
hyve config aws account-remove prod

# EKS IAM Role management (configuration only)
hyve config aws eks-role-add --name default-role --role-arn arn:aws:iam::123456789012:role/my-eks-role
hyve config aws eks-role-list
hyve config aws eks-role-get default-role
hyve config aws eks-role-remove default-role

# EKS IAM Role creation (creates actual AWS resources)
hyve config aws eks-role-create --name default-role --role-name my-eks-cluster-role --region us-east-1
hyve config aws eks-role-delete default-role --region us-east-1
hyve config aws eks-role-delete default-role --config-only  # Remove from config only

# VPC management (configuration only)
hyve config aws vpc-add --name default-vpc --id vpc-0123456789abcdef0
hyve config aws vpc-list
hyve config aws vpc-get default-vpc
hyve config aws vpc-remove default-vpc

# VPC creation (creates actual AWS resources)
hyve config aws vpc-create --name dev-vpc --region us-east-1 --cidr 10.0.0.0/16
hyve config aws vpc-create --name dev-vpc --region us-east-1 --subnets 10.0.1.0/24,10.0.2.0/24
hyve config aws vpc-delete dev-vpc --region us-east-1
hyve config aws vpc-delete dev-vpc --config-only  # Remove from config only
```

#### Azure Configuration

```bash
# Add subscription IDs
hyve config azure add-subscription-ids sub-id-1,sub-id-2
hyve config azure list-subscription-ids
hyve config azure remove-subscription-ids sub-id-1
```

Provider configurations are stored in `provider-configs/` in your repository and can be committed to Git.

See [CLI Reference](https://docs.hyve.dev/cli/overview) for complete command documentation.

## Storage

Hyve stores all data in `~/.hyve/`:

```
~/.hyve/
├── config.yaml          # Global configuration (git backend preference)
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
    │       ├── aws.yaml      # AWS accounts, EKS roles, VPCs (with aliases)
    │       └── azure.yaml    # Azure subscription IDs
    └── development/
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
accounts:
  - name: prod
    account_id: "123456789012"
  - name: dev
    account_id: "987654321098"
eks_roles:
  - name: default-role
    role_arn: arn:aws:iam::123456789012:role/my-eks-role
vpcs:
  - name: default-vpc
    vpc_id: vpc-0123456789abcdef0
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
