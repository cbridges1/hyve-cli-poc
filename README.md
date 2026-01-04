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
   ./hyve cluster add dev-app --region PHX1 --nodes g4s.kube.medium,g4s.kube.medium
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