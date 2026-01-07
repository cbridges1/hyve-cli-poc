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
- **Idempotent Operations**: Safe to run multiple times without side effects
- **SQLite Database**: Persistent storage for repository configurations

## Prerequisites

### Required
- **Civo API Token**: Get your API token from [Civo Dashboard](https://dashboard.civo.com/) → Settings → Security → API Keys
- **Git Repository**: A Git repository to store cluster state (GitHub, GitLab, etc.)
- **Go 1.21+**: [Install Go](https://golang.org/doc/install)
- **Git**: For repository operations

### Optional (for authentication)
- **Global Git Credentials**: Username and password/token stored securely for all repositories
- **Git Token**: Personal access token or app password (environment variable fallback)
- **Git Username**: Username for Git authentication (can be stored globally or per repository)

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
   
   # Optional: Set Git token as fallback (global credentials are preferred)
   export HYVE_GIT_TOKEN=your_git_token_here
   ```

## Quick Start

### 1. Add Your First Repository

Before using Hyve, you must configure a Git repository for state management:

```bash
# Store global Git credentials (recommended for private repositories)
./hyve git credentials --username your-username --password your_personal_access_token

# Add a repository (public)
./hyve git add production --repo-url https://github.com/company/hyve-state.git

# Add a repository (private) - uses global credentials automatically
./hyve git add production --repo-url https://github.com/company/hyve-state.git

# Alternative: Use environment token as fallback
export HYVE_GIT_TOKEN=your_personal_access_token
./hyve git add production --repo-url https://github.com/company/hyve-state.git

# Check status
./hyve git status
```

### 2. Add Your First Cluster

```bash
# Add a cluster (automatically runs reconciliation)
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

#### Global Credential Management

```bash
# Store global Git credentials (used for all repositories)
./hyve git credentials --username your-username --password your-token

# Update global credentials 
./hyve git credentials --username new-username --password new-token

# Update just the password (keeps existing username)
./hyve git credentials --password new-personal-access-token

# Update just the username (keeps existing password)  
./hyve git credentials --username new-username

# View current global credentials
./hyve git credentials

# Clear all stored credentials
./hyve git credentials --clear
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
- **`hyve kubeconfig`**: Kubeconfig management (automatically synced after reconcile)
  - **`sync`**: Manually sync kubeconfigs from active clusters
  - **`list`**: List all stored kubeconfigs for current repository
  - **`get`**: Retrieve and display/save kubeconfig for specific cluster
  - **`use`**: Set kubeconfig for current terminal session (temporary)
- **`hyve use`**: Convenience command to quickly set kubeconfig (equivalent to `hyve kubeconfig use --eval`)
- **`hyve run`**: Execute commands with specific cluster kubeconfig
- **`hyve install`**: Install shell integration for seamless kubeconfig switching (no eval needed)

#### Basic Commands

```bash
# Show help
./hyve --help

# Run manual reconciliation (deploy all clusters in current repository)
./hyve reconcile

# Add a new cluster (automatically runs reconciliation)
./hyve cluster add production --region PHX1 --nodes g4s.kube.large,g4s.kube.large,g4s.kube.large

# Modify existing cluster (automatically runs reconciliation)
./hyve cluster modify production --nodes g4s.kube.medium,g4s.kube.medium,g4s.kube.large,g4s.kube.large,g4s.kube.large

# Delete a cluster (automatically runs reconciliation)
./hyve cluster delete production

# Quickly set kubeconfig for current terminal session
eval $(./hyve use production)

# Run kubectl commands with specific cluster
./hyve run --cluster production kubectl get nodes

# Run any command with cluster context
./hyve run --cluster staging kubectl get pods
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
| `hyve kubeconfig sync` | Sync kubeconfigs from all active clusters | No |
| `hyve kubeconfig list` | List all stored kubeconfigs | No |
| `hyve kubeconfig get [name]` | Get kubeconfig for specific cluster | No |
| `hyve kubeconfig use [name]` | Set kubeconfig for current terminal session | No |
| `hyve use [name]` | Convenience command to quickly set kubeconfig | No |
| `hyve run [cmd] [args...]` | Execute command with cluster kubeconfig | No |
| `hyve install` | Install shell integration for seamless switching | No |

#### CLI Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--region` | `-r` | Civo region | PHX1 |
| `--provider` | `-p` | Cloud provider | civo |
| `--nodes` | `-n` | Node sizes (comma-separated) | g4s.kube.small |
| `--cluster-type` | `-t` | Kubernetes type | k3s |
| `--cluster` | `-c` | Cluster name (for run command) | current/prompt |

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
  ingress:
    enabled: true
    loadBalancer: true
```

### Kubeconfig Management

Hyve automatically syncs kubeconfigs from active clusters after every reconciliation and stores them encrypted in SQLite. This provides secure, repository-specific access to your Kubernetes clusters.

#### Automatic Sync

```bash
# Kubeconfigs are automatically synced after reconciliation
./hyve reconcile

# Manual sync if needed
./hyve kubeconfig sync
```

#### List Stored Kubeconfigs

```bash
# List all kubeconfigs for current repository
./hyve kubeconfig list
```

#### Get Kubeconfig

```bash
# Display kubeconfig (for use with kubectl)
./hyve kubeconfig get production

# Save to ~/.kube/config-production
./hyve kubeconfig get production --save

# Save to specific file
./hyve kubeconfig get production -o ./my-cluster.yaml

# Use with kubectl directly
export KUBECONFIG=$(./hyve kubeconfig get production -o /tmp/kubeconfig)
kubectl get nodes

# Or pipe directly
./hyve kubeconfig get production | kubectl --kubeconfig=/dev/stdin get nodes
```

#### Set Kubeconfig for Terminal Session

##### Method 1: Shell Integration (Recommended - No eval needed!)

```bash
# One-time setup: Install shell integration
./hyve install

# After installation, restart your terminal or run:
source ~/.bashrc  # or ~/.zshrc

# Now you can directly switch clusters without eval commands:
hyve-use production    # Instantly switches to production cluster
hyve-use development   # Instantly switches to development cluster
hyve-status           # Shows current cluster
hyve-unset            # Reverts to default kubeconfig
hyve-list             # Lists available clusters
```

##### Method 2: Manual eval (when shell integration isn't available)

```bash
# Convenience command
eval $(./hyve use production)

# With kubeconfig subcommand
eval $(./hyve kubeconfig use production --eval)

# Traditional approach
./hyve kubeconfig use production
# Then copy and execute the provided export command
```

##### Method 3: Custom shell function

```bash
# Add to ~/.bashrc or ~/.zshrc
hyve-use() {
    eval $(./hyve use "$1")
}

# Usage:
hyve-use production
```

After switching clusters:
```bash
# Now kubectl uses the selected cluster by default
kubectl get nodes
kubectl get pods

# To revert back to your original kubeconfig
unset KUBECONFIG  # or hyve-unset if using shell integration
```

### Command Execution with Kubeconfig

The `hyve run` command provides a clean way to execute commands with a specific cluster's kubeconfig without modifying your terminal environment.

#### Basic Usage

```bash
# Run kubectl with specific cluster
./hyve run --cluster production kubectl get nodes

# Run any command with cluster context
./hyve run --cluster staging kubectl get pods -A

# Interactive mode (prompts for cluster selection)
./hyve run kubectl get services

# Complex commands with shell
./hyve run --cluster production sh -c "kubectl get pods | grep nginx"

# Run non-kubectl commands that use kubeconfig
./hyve run --cluster dev helm list
```

#### Automatic Cluster Selection

```bash
# If only one cluster exists, it's automatically selected
./hyve run kubectl get namespaces

# If multiple clusters exist, you'll see a list and need to specify --cluster
./hyve run kubectl get nodes
# Output: Available clusters:
#   1. production
#   2. staging
# Please specify a cluster with --cluster flag
```

#### Command Exit Codes

The `run` command preserves the exit code of the executed command:

```bash
# If kubectl fails, hyve run will exit with the same code
./hyve run --cluster production kubectl get invalid-resource
echo $?  # Will show kubectl's exit code
```

#### Kubeconfig Storage

```
~/.hyve/
├── repositories.db              # Repository configurations
├── credentials.db               # Global Git credentials (encrypted)
├── kubeconfigs.db              # Cluster kubeconfigs (encrypted per repository)
├── temp/                       # Temporary kubeconfig files for terminal sessions
│   ├── kubeconfig-production-cluster1
│   └── kubeconfig-development-test-app
└── repositories/               # Centralized repository storage
    ├── production/
    └── development/
```

#### Security Features

- **Repository Isolation**: Each repository has its own set of kubeconfigs
- **AES-GCM Encryption**: All kubeconfigs stored with strong encryption
- **Automatic Cleanup**: Orphaned kubeconfigs removed when clusters are deleted
- **Key Derivation**: Encryption keys derived from system info + repository name

### Local Environment Configuration

```bash
# .env file in Hyve CLI directory
CIVO_TOKEN=your_civo_api_token_here

# Environment variable for Git authentication (optional)
export HYVE_GIT_TOKEN=your_personal_access_token
```

### Repository Configuration Storage

Hyve stores repository, credential, and kubeconfig data in SQLite databases:

```
~/.hyve/
├── repositories.db              # SQLite database with repository configs
├── credentials.db               # SQLite database with encrypted global credentials
├── kubeconfigs.db              # SQLite database with encrypted cluster kubeconfigs
├── temp/                       # Temporary kubeconfig files for terminal sessions
├── repositories/               # Centralized repository storage directory
└── config.yaml                 # Legacy config (unused in current version)
```

## Security Considerations

- **Secret Protection**: CIVO_TOKEN is only accessible to workflows
- **Encrypted Credential Storage**: Git passwords stored using AES-GCM encryption in local SQLite database
- **Encrypted Kubeconfig Storage**: Cluster kubeconfigs stored with AES-GCM encryption, isolated per repository
- **Global Authentication**: Single set of credentials used across all repositories for simplicity
- **Environment Token Fallback**: HYVE_GIT_TOKEN provides fallback authentication when no global credentials stored
- **Repository Isolation**: Kubeconfigs are isolated per repository for security boundaries
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
   ./hyve cluster add dev-app --region PHX1 --nodes g4s.kube.medium,g4s.kube.medium
   
   # Update global credentials if needed
   ./hyve git credentials --password new-dev-token
   ```

3. **Production Environment**:
   ```bash
   # Switch to production
   ./hyve git use production
   
   # Add production clusters (automatic reconciliation)
   ./hyve cluster add prod-app --region PHX1 --nodes g4s.kube.large,g4s.kube.large,g4s.kube.large
   ```

4. **Scaling Operations**:
   ```bash
   # Scale production app cluster (automatic reconciliation)
   ./hyve cluster modify prod-app --nodes g4s.kube.large,g4s.kube.large,g4s.kube.large,g4s.kube.large,g4s.kube.large
   ```

### Multi-Region Setup

```bash
# Set up production repository
./hyve git use production

# Clusters in different regions (automatic reconciliation for each)
./hyve cluster add app-nyc --region NYC1 --nodes g4s.kube.small,g4s.kube.small,g4s.kube.small
./hyve cluster add app-fra --region FRA1 --nodes g4s.kube.small,g4s.kube.small

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