# Civo Cluster Deploy

A declarative Kubernetes cluster management tool for Civo Cloud that supports both local CLI usage and automated GitHub Actions deployment.

## Overview

This application provides multiple ways to manage Civo Kubernetes clusters:
- **Local CLI**: Run directly on your machine for development and testing
- **GitHub Actions**: Automated deployment triggered by repository changes
- **GitHub CLI**: Trigger deployments remotely using the `gh` command

## Features

- **Declarative Configuration**: Define clusters using YAML files
- **Master Cluster Dependencies**: Ensures master clusters are ready before deploying workers
- **Idempotent Operations**: Safe to run multiple times without side effects
- **Firewall Management**: Automatic creation and cleanup of cluster firewalls
- **CLI and Web UI**: Multiple interfaces for cluster management
- **GitOps Ready**: Full GitHub Actions integration with automated deployments

## Prerequisites

### For All Usage Methods
- **Civo API Token**: Get your API token from [Civo Dashboard](https://dashboard.civo.com/) → Settings → Security → API Keys

### For Local Usage
- **Go 1.21+**: [Install Go](https://golang.org/doc/install)
- **Git**: For cloning and managing the repository

### For GitHub Actions
- **Repository Secrets**: Configure `CIVO_TOKEN` as a repository secret
- **Repository Permissions**: Workflow has `contents: write` permission for configuration updates

### For GitHub CLI
- **GitHub CLI**: [Install gh CLI](https://cli.github.com/)
- **Authentication**: `gh auth login` to authenticate with GitHub

## Installation

### Local Installation

1. **Clone the repository**:
   ```bash
   git clone <repository-url>
   cd portainer-master-cluster
   ```

2. **Build the application**:
   ```bash
   go build -o civo-cluster-deploy .
   ```

3. **Configure environment**:
   ```bash
   # Create .env file with your Civo API token
   echo "CIVO_TOKEN=your_civo_api_token_here" > .env
   ```

### GitHub Actions Setup

1. **Add repository secret**:
   - Go to repository Settings → Secrets and variables → Actions
   - Create new secret: `CIVO_TOKEN` with your Civo API token

2. **The workflow is ready to use** - it's already configured in `.github/workflows/deploy-clusters.yml`

## Usage

### Method 1: Local CLI

#### Basic Commands

```bash
# Show help
./civo-cluster-deploy --help

# Run reconciliation (deploy all clusters defined in state/clusters/)
./civo-cluster-deploy

# Add a new cluster
./civo-cluster-deploy -action=add -cluster-name=production -region=PHX1 -node-count=3 -size=g4s.kube.large

# Add a master cluster
./civo-cluster-deploy -action=add -cluster-name=master -region=PHX1 -node-count=1 -size=g4s.kube.small -master-cluster=true

# Modify existing cluster
./civo-cluster-deploy -action=modify -cluster-name=production -node-count=5

# Delete a cluster configuration
./civo-cluster-deploy -action=delete -cluster-name=production
```

#### CLI Parameters

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `-action` | Action to perform: add, modify, delete | | When managing clusters |
| `-cluster-name` | Name of the cluster | | When using -action |
| `-region` | Civo region | PHX1 | No |
| `-node-count` | Number of nodes | 1 | No |
| `-size` | Node size | g4s.kube.small | No |
| `-cluster-type` | Kubernetes type | k3s | No |
| `-master-cluster` | Is master cluster | false | No |

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

## Configuration Files

### Cluster YAML Structure

Cluster configurations are stored in `state/clusters/` directory:

```yaml
# state/clusters/production.yaml
apiVersion: v1
kind: Cluster
metadata:
  name: production
  region: PHX1
spec:
  nodeCount: 3
  size: g4s.kube.large
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
```

### Environment Configuration

Create a `.env` file in the project root:

```bash
# .env
CIVO_TOKEN=your_civo_api_token_here
```

## Deployment Modes

### 1. Reconciliation Mode (Default)
- Reads all YAML files from `state/clusters/`
- Compares with actual Civo cluster state
- Creates, updates, or deletes clusters as needed
- Ensures master cluster is ready before deploying workers

### 2. CLI Mode
- Uses command-line parameters to create/modify/delete cluster YAML files
- Operates on local filesystem only
- Requires separate reconciliation run to deploy

### 3. GitHub Actions Mode
- **Automatic**: Triggered by push to main branch
- **Manual**: Triggered via GitHub web UI or CLI
- **With Parameters**: Creates YAML files, commits them, then deploys

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
./civo-cluster-deploy 2>&1 | tee deployment.log

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

### Complete Workflow Example

1. **Local Development**:
   ```bash
   # Create and test locally
   ./civo-cluster-deploy -action=add -cluster-name=staging -region=PHX1 -node-count=2 -size=g4s.kube.medium
   ./civo-cluster-deploy  # Deploy to test
   ```

2. **Commit to Repository**:
   ```bash
   git add state/clusters/staging.yaml
   git commit -m "Add staging cluster"
   git push origin main  # Triggers automatic deployment
   ```

3. **Scale via GitHub CLI**:
   ```bash
   gh workflow run "Deploy Civo Clusters" \
     -f action=modify \
     -f cluster-name=staging \
     -f node-count=4
   ```

4. **Monitor Deployment**:
   ```bash
   gh run watch
   ```

### Multi-Region Setup

```bash
# Master cluster in PHX1
./civo-cluster-deploy -action=add -cluster-name=master-phx -region=PHX1 -master-cluster=true

# Worker clusters in different regions  
./civo-cluster-deploy -action=add -cluster-name=worker-nyc -region=NYC1 -node-count=3
./civo-cluster-deploy -action=add -cluster-name=worker-fra -region=FRA1 -node-count=2

# Deploy all
./civo-cluster-deploy
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