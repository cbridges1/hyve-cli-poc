# Hyve - GitOps Kubernetes Cluster Management CLI

A declarative GitOps Kubernetes cluster management tool for Civo Cloud that manages state through Git repositories with SQLite-backed multi-repository support.

## Overview

Hyve is a **GitOps-first** cluster management tool that requires Git repositories for all state management. It supports:
- **Multi-Repository Management**: Manage separate repositories for different environments (dev/staging/prod)
- **SQLite-Backed Storage**: Persistent configuration storage for repository management
- **Automatic Reconciliation**: Clusters are automatically reconciled after add/delete operations
- **Pure GitOps Workflow**: All state changes are committed and pushed to Git repositories

## Features

- **Git Repository Requirement**: All cluster state must be managed through Git repositories
- **Multi-Repository Support**: Configure multiple repositories and switch between them
- **Automatic Reconciliation**: Add/delete commands automatically run reconciliation
- **Modern CLI Interface**: Built with Cobra CLI for intuitive command structure
- **Declarative Configuration**: Define clusters using YAML files in Git repositories
- **Master Cluster Dependencies**: Ensures master clusters are ready before deploying workers
- **Idempotent Operations**: Safe to run multiple times without side effects
- **Firewall Management**: Automatic creation and cleanup of cluster firewalls
- **SQLite Database**: Persistent storage for repository configurations

## Prerequisites

### Required
- **Civo API Token**: Get your API token from [Civo Dashboard](https://dashboard.civo.com/) → Settings → Security → API Keys
- **Git Repository**: A Git repository to store cluster state (GitHub, GitLab, etc.)
- **Go 1.21+**: [Install Go](https://golang.org/doc/install)
- **Git**: For repository operations

### Optional (for authentication)
- **Git Token**: Personal access token or app password for private repositories
- **Git Username**: Username for Git authentication

## Installation

1. **Clone the repository**:
   ```bash
   git clone <repository-url>
   cd hyve-action-poc
   ```

2. **Build the application**:
   ```bash
   go build -o hyve .
   ```

3. **Configure environment**:
   ```bash
   # Create .env file with your Civo API token
   echo "CIVO_TOKEN=your_civo_api_token_here" > .env
   
   # Optional: Set Git token for private repositories
   export HYVE_GIT_TOKEN=your_git_token_here
   ```

## Quick Start

### 1. Add Your First Repository

Before using Hyve, you must configure a Git repository for state management:

```bash
# Add a repository (public)
./hyve git add production --repo-url https://github.com/company/hyve-state.git

# Add a repository (private with authentication)
export HYVE_GIT_TOKEN=your_personal_access_token
./hyve git add production --repo-url https://github.com/company/hyve-state.git --username your-username

# Check status
./hyve git status
```

### 2. Add Your First Cluster

```bash
# Add a master cluster (automatically runs reconciliation)
./hyve cluster add master --region PHX1 --nodes g4s.kube.small --master-cluster

# Add a worker cluster (automatically runs reconciliation)
./hyve cluster add production --region PHX1 --nodes g4s.kube.medium,g4s.kube.medium,g4s.kube.large
```

### 3. Manual Reconciliation

```bash
# Run reconciliation manually if needed
./hyve reconcile
```

## Usage

### Git Repository Management

#### Git Commands

```bash
# Add repositories for different environments
./hyve git add production --repo-url https://github.com/company/hyve-prod.git
./hyve git add development --repo-url https://github.com/company/hyve-dev.git

# List all configured repositories  
./hyve git list

# Switch between repositories
./hyve git use development
./hyve git use production

# Show current repository status
./hyve git status

# Remove a repository
./hyve git remove development

# Reset all repositories (back to no configuration)
./hyve git reset
```

### Cluster Management

#### CLI Architecture

The CLI follows a command-subcommand structure:
- **`hyve git`**: Git repository management
- **`hyve reconcile`**: Manual reconciliation of all clusters in current repository
- **`hyve cluster`**: Cluster operations (with automatic reconciliation)
  - **`add`**: Create cluster + reconcile
  - **`modify`**: Update cluster + reconcile  
  - **`delete`**: Remove cluster + reconcile

#### Basic Commands

```bash
# Show help
./hyve --help

# Run manual reconciliation (deploy all clusters in current repository)
./hyve reconcile

# Add a new cluster (automatically runs reconciliation)
./hyve cluster add production --region PHX1 --nodes g4s.kube.large,g4s.kube.large,g4s.kube.large

# Add a master cluster (automatically runs reconciliation)
./hyve cluster add master --region PHX1 --nodes g4s.kube.small --master-cluster

# Modify existing cluster (automatically runs reconciliation)
./hyve cluster modify production --nodes g4s.kube.medium,g4s.kube.medium,g4s.kube.large,g4s.kube.large,g4s.kube.large

# Delete a cluster (automatically runs reconciliation)
./hyve cluster delete production
```

#### CLI Commands

| Command | Description | Reconciliation |
|---------|-------------|----------------|
| `hyve git add [name]` | Add Git repository for state management | No |
| `hyve git list` | List all configured repositories | No |
| `hyve git use [name]` | Switch to different repository | No |
| `hyve git status` | Show current repository status | No |
| `hyve reconcile` | Deploy all clusters in current repository | Manual |
| `hyve cluster add [name]` | Create cluster configuration | Automatic |
| `hyve cluster modify [name]` | Update cluster configuration | Automatic |
| `hyve cluster delete [name]` | Remove cluster configuration | Automatic |

#### CLI Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--region` | `-r` | Civo region | PHX1 |
| `--provider` | `-p` | Cloud provider | civo |
| `--nodes` | `-n` | Node sizes (comma-separated) | g4s.kube.small |
| `--cluster-type` | `-t` | Kubernetes type | k3s |
| `--master-cluster` | `-m` | Is master cluster | false |

#### Available Regions
- `PHX1` - Phoenix, USA
- `NYC1` - New York, USA  
- `FRA1` - Frankfurt, Germany
- `LON1` - London, UK

#### Available Node Sizes
- `g4s.kube.xsmall` - 1 vCPU, 1GB RAM
- `g4s.kube.small` - 1 vCPU, 2GB RAM
- `g4s.kube.medium` - 2 vCPU, 4GB RAM
- `g4s.kube.large` - 4 vCPU, 8GB RAM
- `g4s.kube.xlarge` - 6 vCPU, 16GB RAM

### Method 2: GitHub Actions (Web UI)

#### Automated Deployment (Push to Main)
1. Make changes to YAML files in `state/clusters/`
2. Commit and push to main branch
3. GitHub Action automatically deploys changes

#### Manual Deployment with Parameters
1. Go to **Actions** tab in your GitHub repository
2. Select **"Deploy Civo Clusters"** workflow
3. Click **"Run workflow"**
4. Fill in the parameters:
   - **Action**: add, modify, or delete
   - **Cluster Name**: Your cluster name
   - **Region**: Select from dropdown
   - **Node Count**: Number of nodes
   - **Size**: Select node size
   - **Cluster Type**: k3s or talos
   - **Master Cluster**: Check if this is a master cluster
5. Click **"Run workflow"**

The workflow will:
- Create/modify the cluster YAML file
- Commit changes with `[skip ci]` to avoid recursion
- Deploy the cluster automatically

### Method 3: GitHub CLI

#### Install and Setup GitHub CLI
```bash
# Install gh CLI (macOS)
brew install gh

# Install gh CLI (Linux)
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
sudo apt update && sudo apt install gh

# Authenticate with GitHub
gh auth login
```

#### Trigger GitHub Actions via CLI

```bash
# Basic reconciliation (no parameters)
gh workflow run "Deploy Civo Clusters"

# Add a new cluster
gh workflow run "Deploy Civo Clusters" \
  -f action=add \
  -f cluster-name=production \
  -f region=PHX1 \
  -f node-count=3 \
  -f size=g4s.kube.large \
  -f cluster-type=k3s

# Add a master cluster
gh workflow run "Deploy Civo Clusters" \
  -f action=add \
  -f cluster-name=master \
  -f region=PHX1 \
  -f node-count=1 \
  -f size=g4s.kube.small \
  -f cluster-type=k3s \
  -f master-cluster=true

# Modify existing cluster
gh workflow run "Deploy Civo Clusters" \
  -f action=modify \
  -f cluster-name=production \
  -f node-count=5

# Delete a cluster
gh workflow run "Deploy Civo Clusters" \
  -f action=delete \
  -f cluster-name=production

# Check workflow status
gh run list --workflow="Deploy Civo Clusters"

# View workflow logs
gh run view --log
```

#### GitHub CLI Parameters

| Flag | Description | Example |
|------|-------------|---------|
| `-f action=<value>` | Action to perform | `add`, `modify`, `delete` |
| `-f cluster-name=<value>` | Cluster name | `production` |
| `-f region=<value>` | Civo region | `PHX1`, `NYC1`, `FRA1`, `LON1` |
| `-f node-count=<value>` | Number of nodes | `1`, `3`, `5` |
| `-f size=<value>` | Node size | `g4s.kube.small` |
| `-f cluster-type=<value>` | Kubernetes type | `k3s`, `talos` |
| `-f master-cluster=<value>` | Master cluster flag | `true`, `false` |

## Configuration

### Git Repository Structure

Each Git repository contains cluster configurations in a `clusters/` directory:

```
repository-root/
├── README.md                    # Repository documentation  
├── clusters/                    # Cluster definitions
│   ├── production.yaml
│   ├── master.yaml
│   └── staging.yaml
└── .gitignore                   # Ignore temporary files
```

### Cluster YAML Structure

```yaml
# clusters/production.yaml
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
    - g4s.kube.large
  clusterType: k3s
  masterCluster: false  # Set to true for master clusters
  firewall:
    enabled: true
    rules:
      - protocol: tcp
        startPort: "6443"
        endPort: "6443"
        cidr:
          - 0.0.0.0/0
        direction: ingress
  ingress:
    enabled: true
    loadBalancer: true
```

### Local Environment Configuration

```bash
# .env file in Hyve CLI directory
CIVO_TOKEN=your_civo_api_token_here

# Environment variable for Git authentication (optional)
export HYVE_GIT_TOKEN=your_personal_access_token
```

### Repository Configuration Storage

Hyve stores repository configurations in SQLite database:

```
~/.hyve/
├── repositories.db              # SQLite database with repository configs
└── config.yaml                 # Legacy config (unused in current version)
```

## Workflow Modes

### 1. GitOps Workflow (Recommended)
- **Repository-Based**: All cluster state stored in Git repositories
- **Multi-Environment**: Separate repositories for dev/staging/prod
- **Automatic Reconciliation**: Add/delete operations trigger immediate reconciliation  
- **Version Controlled**: All changes committed and pushed to Git
- **Audit Trail**: Complete history of all cluster configuration changes

### 2. Manual Reconciliation
- **Explicit Control**: Run `hyve reconcile` when desired
- **Batch Operations**: Make multiple configuration changes before deploying
- **Review Before Deploy**: Inspect changes in Git before reconciliation

### 3. Multi-Repository Management
- **Environment Isolation**: Completely separate state per environment
- **Easy Switching**: `hyve git use <env>` to switch between environments  
- **Independent Operations**: Changes in one repository don't affect others

## Master Cluster Dependencies

The application enforces master cluster dependencies:

1. **Validation**: Ensures exactly one master cluster exists when clusters are defined
2. **Ordering**: Master clusters are always processed first
3. **Readiness Check**: Worker clusters wait for master cluster to be ACTIVE
4. **API-based Status**: Uses live Civo API status instead of stored state

## Monitoring and Troubleshooting

### Local Debugging
```bash
# Run with verbose output
./hyve reconcile 2>&1 | tee deployment.log

# Check cluster status directly via Civo CLI
civo kubernetes list
civo firewall list
```

### GitHub Actions Monitoring

1. **View Logs**:
   - Go to Actions tab → Select workflow run → View logs

2. **Download Artifacts**:
   - Deployment logs and state files are saved as artifacts
   - Available for 30 days after each run

3. **Common Issues**:
   ```bash
   # Missing CIVO_TOKEN
   ❌ CIVO_TOKEN secret is not set!
   
   # No cluster configurations
   No cluster configurations found in state/clusters directory
   
   # Master cluster not ready
   Skipping cluster worker-1 - master cluster is not ready yet
   ```

### GitHub CLI Monitoring
```bash
# List recent workflow runs
gh run list --workflow="Deploy Civo Clusters" --limit 5

# View specific run details
gh run view <run-id>

# View logs for latest run
gh run view --log

# Watch a running workflow
gh run watch
```

## Security Considerations

- **Secret Protection**: CIVO_TOKEN is only accessible to workflows
- **Branch Protection**: Only main branch pushes trigger production deployments  
- **PR Safety**: Pull requests run in dry-run mode only
- **Repository Access**: Minimal `contents: write` permission for configuration updates
- **Token Authentication**: Uses built-in `GITHUB_TOKEN` for secure git operations
- **Audit Trail**: All deployments and configuration changes are logged

## Examples

### Complete GitOps Workflow

1. **Initial Setup**:
   ```bash
   # Add production repository
   ./hyve git add production --repo-url https://github.com/company/hyve-prod.git
   
   # Add development repository  
   ./hyve git add development --repo-url https://github.com/company/hyve-dev.git
   
   # List repositories
   ./hyve git list
   ```

2. **Development Environment**:
   ```bash
   # Switch to development
   ./hyve git use development
   
   # Add clusters (automatic reconciliation)
   ./hyve cluster add dev-master --region PHX1 --nodes g4s.kube.small --master-cluster
   ./hyve cluster add dev-app --region PHX1 --nodes g4s.kube.medium,g4s.kube.medium
   ```

3. **Production Environment**:
   ```bash
   # Switch to production
   ./hyve git use production
   
   # Add production clusters (automatic reconciliation)
   ./hyve cluster add prod-master --region PHX1 --nodes g4s.kube.large --master-cluster
   ./hyve cluster add prod-app --region PHX1 --nodes g4s.kube.large,g4s.kube.large,g4s.kube.large
   ```

4. **Scaling Operations**:
   ```bash
   # Scale production app cluster (automatic reconciliation)
   ./hyve cluster modify prod-app --nodes g4s.kube.large,g4s.kube.large,g4s.kube.large,g4s.kube.large,g4s.kube.large
   ```

### Multi-Region Setup

```bash
# Set up production repository with master cluster
./hyve git use production
./hyve cluster add master-phx --region PHX1 --master-cluster --nodes g4s.kube.medium

# Worker clusters in different regions (automatic reconciliation for each)
./hyve cluster add worker-nyc --region NYC1 --nodes g4s.kube.small,g4s.kube.small,g4s.kube.small
./hyve cluster add worker-fra --region FRA1 --nodes g4s.kube.small,g4s.kube.small

# Manual reconciliation if needed
./hyve reconcile
```

### Repository Management

```bash
# List all environments
./hyve git list

# Check current environment  
./hyve git status

# Switch environments
./hyve git use development
./hyve cluster add test-feature --region PHX1 --nodes g4s.kube.small

./hyve git use production  
./hyve cluster delete old-cluster

# Clean up old environment
./hyve git remove development
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes and test locally
4. Submit a pull request (triggers dry-run validation)
5. Merge to main (triggers production deployment)

## License

[Add your license here]

## Support

- **Issues**: Create issues in the GitHub repository
- **Documentation**: This README and inline help (`--help`)
- **Logs**: Check workflow artifacts for detailed deployment logs