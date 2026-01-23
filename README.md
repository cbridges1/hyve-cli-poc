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

3. **Configure API token**:
   ```bash
   # Recommended: Store encrypted token in database
   ./hyve config set-token civo
   # Enter your Civo API token when prompted

   # Alternative: Create .env file
   echo "CIVO_TOKEN=your_civo_api_token_here" > .env

   # Alternative: Set environment variable
   export CIVO_TOKEN=your_civo_api_token_here

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

#### API Token Management

Hyve can securely store your cloud provider API tokens in an encrypted database, eliminating the need for `.env` files or environment variables.

```bash
# Store Civo API token (recommended)
./hyve config set-token civo
# Enter token when prompted (input will be hidden)

# Or provide token directly
./hyve config set-token civo --token YOUR_TOKEN_HERE

# View stored token
./hyve config get-token civo

# List all stored provider tokens
./hyve config list-tokens

# Remove stored token
./hyve config clear-token civo
```

**Token Priority:**
1. Database (encrypted storage via `hyve config set-token`)
2. Environment variable (`CIVO_TOKEN`)
3. `.env` file

**Security:**
- Tokens are encrypted using AES-GCM before storage
- Stored in `~/.hyve/credentials.db`
- Portable across machines (no hostname dependency)
- Same encryption as Git credentials

### Cluster Management

#### CLI Architecture

The CLI follows a command-subcommand structure:
- **`hyve git`**: Git repository management
- **`hyve config`**: Configuration management (API tokens, settings)
  - **`set-token`**: Store encrypted provider API token
  - **`get-token`**: Retrieve stored API token
  - **`list-tokens`**: List providers with stored tokens
  - **`clear-token`**: Remove stored API token
- **`hyve reconcile`**: Manual reconciliation of all clusters in current repository
- **`hyve cluster`**: Cluster operations (with automatic reconciliation)
  - **`add`**: Create cluster + reconcile
  - **`modify`**: Update cluster + reconcile
  - **`delete`**: Remove cluster + reconcile
- **`hyve kubeconfig`**: Kubeconfig management (automatically synced after reconcile)
  - **`sync`**: Manually sync kubeconfigs from active clusters
  - **`list`**: List all stored kubeconfigs for current repository
  - **`get`**: Retrieve and display/save kubeconfig for specific cluster
  - **`merge`**: Merge cluster context into local ~/.kube/config
  - **`remove`**: Remove cluster context from local ~/.kube/config
- **`hyve run`**: Execute commands with specific cluster kubeconfig
- **`hyve workflow`**: Manage and execute workflows for automated task execution

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

# Merge cluster context into local ~/.kube/config
./hyve kubeconfig merge production

# Switch to merged cluster context
kubectl config use-context production

# Remove cluster context from local ~/.kube/config
./hyve kubeconfig remove production

# Run kubectl commands with specific cluster (without modifying local kubeconfig)
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
| `hyve config set-token [provider]` | Store encrypted API token in database | No |
| `hyve config get-token [provider]` | Retrieve stored API token | No |
| `hyve config list-tokens` | List providers with stored tokens | No |
| `hyve config clear-token [provider]` | Remove stored API token | No |
| `hyve reconcile` | Deploy all clusters in current repository | Manual |
| `hyve cluster add [name]` | Create cluster configuration | Automatic |
| `hyve cluster modify [name]` | Update cluster configuration | Automatic |
| `hyve cluster delete [name]` | Remove cluster configuration | Automatic |
| `hyve kubeconfig sync` | Sync kubeconfigs from all active clusters | No |
| `hyve kubeconfig list` | List all stored kubeconfigs | No |
| `hyve kubeconfig get [name]` | Get kubeconfig for specific cluster | No |
| `hyve kubeconfig merge [name]` | Merge cluster context into ~/.kube/config | No |
| `hyve kubeconfig remove [name]` | Remove cluster context from ~/.kube/config | No |
| `hyve run [cmd] [args...]` | Execute command with cluster kubeconfig | No |
| `hyve workflow create [name]` | Create a new workflow definition | No |
| `hyve workflow list` | List all available workflows | No |
| `hyve workflow run [name]` | Execute a workflow | No |
| `hyve workflow delete [name]` | Delete a workflow definition | No |

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

#### Merge Cluster Context into Local Kubeconfig

The recommended way to use Hyve clusters is to merge their context into your local `~/.kube/config` file. This is simple, maintainable, and works across all platforms.

```bash
# Merge cluster context into ~/.kube/config
./hyve kubeconfig merge production

# Switch to the merged cluster context
kubectl config use-context production

# Verify you're using the correct context
kubectl config current-context

# Use kubectl normally
kubectl get nodes
kubectl get pods

# Switch back to another context
kubectl config use-context minikube

# List all available contexts
kubectl config get-contexts
```

#### Remove Cluster Context from Local Kubeconfig

When you no longer need a cluster context in your local kubeconfig:

```bash
# Remove cluster context from ~/.kube/config
./hyve kubeconfig remove production

# This removes the context, cluster, and user entries
# Your other contexts remain intact
```

#### Alternative: Temporary Kubeconfig Access

If you don't want to modify your local kubeconfig, use `hyve run`:

```bash
# Run commands with cluster context (no local kubeconfig modification)
./hyve run --cluster production kubectl get nodes
./hyve run --cluster staging kubectl get pods -A

# Or use temporary KUBECONFIG environment variable
export KUBECONFIG=$(./hyve kubeconfig get production -o /tmp/production-kubeconfig)
kubectl get nodes
unset KUBECONFIG
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

### Workflow Management

Hyve includes a powerful workflow system for automating deployment pipelines and operational tasks. Workflows are defined in YAML files stored in the `workflows/` directory of your repository and can execute commands both locally and on specific Kubernetes clusters.

#### Workflow Features

- **YAML-based definitions**: Simple, version-controlled workflow definitions
- **Dependency management**: Jobs can depend on other jobs for execution order
- **Multi-cluster support**: Run workflows on specific clusters or locally
- **Environment variables**: Support for workflow, job, and step-level variables
- **Pre-defined actions**: Built-in actions for common Kubernetes operations
- **Detailed logging**: Comprehensive execution logs and step outputs
- **Variable substitution**: Dynamic variable expansion in commands and scripts
- **Requirements validation**: Automatic verification of CLI tools and secrets before execution

#### Workflow Requirements

Workflows can specify requirements that are validated before execution. This ensures all necessary tools and secrets are available, preventing runtime failures.

**Tool Requirements**: Validate CLI tools are installed and optionally check minimum versions
```yaml
spec:
  requirements:
    tools:
      - name: kubectl
        version: "1.28"
        description: Kubernetes CLI for cluster operations
      - name: helm
        version: "3.12"
        description: Helm package manager
      - name: docker
        description: Docker CLI (version check optional)
```

**Secret Requirements**: Validate secrets are available from database or environment
```yaml
spec:
  requirements:
    secrets:
      - name: DOCKER_TOKEN
        provider: docker
        required: true
        description: Docker Hub authentication token
      - name: GITHUB_TOKEN
        provider: github
        required: false
        description: GitHub token (optional)
```

**How It Works**:
1. **Tool Validation**: Checks if tools exist in PATH and validates version if specified
2. **Secret Loading**: Automatically loads secrets from credentials database into environment
3. **Environment Fallback**: Checks environment variables if secret not in database
4. **Helpful Errors**: Provides clear instructions when requirements not met

**Secret Priority**:
1. Environment variable (if already set)
2. Credentials database via provider name (`hyve config set-token <provider>`)

**Example Error Message**:
```
Requirements validation failed:
  - Required tool 'helm' not found in PATH (Helm package manager)
  - Required secret 'DOCKER_TOKEN' not found (Docker Hub authentication token)
    Set via: hyve config set-token docker OR export DOCKER_TOKEN=your-secret
```

#### Creating Workflows

```bash
# Create a workflow from template
./hyve workflow create --template my-deployment --description "Application deployment pipeline"

# Create from existing YAML file
./hyve workflow create --file ./my-workflow.yaml

# List all workflows
./hyve workflow list

# Show workflow details
./hyve workflow show my-deployment
```

#### Workflow Structure

```yaml
apiVersion: v1
kind: Workflow
metadata:
  name: deployment-pipeline
  description: Complete deployment pipeline
  labels:
    environment: production
    team: platform
spec:
  env:
    APP_NAME: my-application
    NAMESPACE: default
  jobs:
    - name: pre-checks
      description: Pre-deployment validation
      steps:
        - name: check-cluster-health
          command: kubectl get nodes
        - name: verify-namespace
          command: kubectl get namespace ${NAMESPACE}

    - name: deploy-app
      description: Deploy the application
      dependsOn: ["pre-checks"]
      cluster: production
      steps:
        - name: apply-manifests
          action: kubectl-apply
          with:
            file: k8s/deployment.yaml
        - name: wait-for-rollout
          command: kubectl rollout status deployment/${APP_NAME} -n ${NAMESPACE}

    - name: post-deploy-tests
      description: Post-deployment testing
      dependsOn: ["deploy-app"]
      steps:
        - name: health-check
          script: |
            echo "Running health checks..."
            kubectl exec deployment/${APP_NAME} -- curl -f http://localhost:8080/health
        - name: smoke-tests
          command: kubectl get pods -n ${NAMESPACE} -l app=${APP_NAME}
```

#### Running Workflows

```bash
# Run workflow locally (no cluster context)
./hyve workflow run my-workflow

# Run workflow on specific cluster
./hyve workflow run my-workflow --cluster production

# Run with detailed output
./hyve workflow run my-workflow --output

# Run without logs
./hyve workflow run my-workflow --logs=false
```

#### Workflow Components

**Jobs**: Independent units of work that can run in parallel or sequence
- Support job dependencies with `dependsOn`
- Can target specific clusters with `cluster`
- Support conditional execution with `if`

**Steps**: Individual tasks within a job
- **Commands**: Single shell commands
- **Scripts**: Multi-line shell scripts
- **Actions**: Pre-defined operations (kubectl-apply, kubectl-delete)

**Environment Variables**: Available at workflow, job, and step levels
- Automatic variables: `WORKFLOW_NAME`, `WORKFLOW_CLUSTER`, `KUBECONFIG`
- Custom variables with `${VARIABLE}` substitution

**Pre-defined Actions**:
- `kubectl-apply`: Apply Kubernetes manifests
- `kubectl-delete`: Delete Kubernetes resources

#### Workflow Management

```bash
# Validate workflow syntax
./hyve workflow validate my-workflow

# Delete workflow
./hyve workflow delete my-workflow

# Force delete without confirmation
./hyve workflow delete my-workflow --force
```

#### Example: Simple CI/CD Pipeline

```yaml
apiVersion: v1
kind: Workflow
metadata:
  name: cicd-pipeline
  description: Simple CI/CD pipeline
spec:
  env:
    PROJECT_NAME: my-app
    BUILD_VERSION: "1.0.0"
  jobs:
    - name: build
      description: Build and test
      steps:
        - name: compile
          command: echo "Building ${PROJECT_NAME} v${BUILD_VERSION}"
        - name: test
          script: |
            echo "Running tests..."
            # Add your test commands here
            echo "✅ Tests passed"

    - name: deploy-staging
      description: Deploy to staging
      dependsOn: ["build"]
      cluster: staging
      steps:
        - name: deploy
          action: kubectl-apply
          with:
            file: k8s/staging/
        - name: verify
          command: kubectl rollout status deployment/${PROJECT_NAME}

    - name: deploy-production
      description: Deploy to production
      dependsOn: ["deploy-staging"]
      cluster: production
      steps:
        - name: deploy
          action: kubectl-apply
          with:
            file: k8s/production/
        - name: verify
          command: kubectl rollout status deployment/${PROJECT_NAME}
```

#### Example: Docker Build with Requirements

```yaml
apiVersion: v1
kind: Workflow
metadata:
  name: docker-build
  description: Build and push Docker images with validation
spec:
  requirements:
    tools:
      - name: docker
        version: "20.10"
        description: Docker CLI for building and pushing images
      - name: git
        description: Git CLI for source control operations
    secrets:
      - name: DOCKER_TOKEN
        provider: docker
        required: true
        description: Docker Hub authentication token
      - name: GITHUB_TOKEN
        provider: github
        required: false
        description: GitHub token for private repos (optional)

  env:
    IMAGE_NAME: my-application
    IMAGE_TAG: latest
    DOCKER_REGISTRY: docker.io

  jobs:
    - name: validate-environment
      description: Validate build environment
      steps:
        - name: check-docker
          command: docker --version
        - name: verify-auth
          script: |
            if [ -z "$DOCKER_TOKEN" ]; then
              echo "❌ DOCKER_TOKEN not available"
              exit 1
            fi
            echo "✅ Docker authentication configured"

    - name: build-image
      description: Build Docker image
      dependsOn: ["validate-environment"]
      steps:
        - name: docker-build
          script: |
            docker build -t ${DOCKER_REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG} .
            echo "✅ Build complete"

    - name: push-image
      description: Push to registry
      dependsOn: ["build-image"]
      steps:
        - name: docker-login
          script: |
            echo "$DOCKER_TOKEN" | docker login -u myuser --password-stdin
        - name: docker-push
          command: docker push ${DOCKER_REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}
```

**Usage**:
```bash
# Set required token
./hyve config set-token docker
# Enter your Docker Hub token when prompted

# Run the workflow
./hyve workflow run docker-build

# Output shows:
# ✅ All requirements validated successfully
# ✅ Docker authentication configured
# ✅ Build complete
```

#### Kubeconfig Storage

```
~/.hyve/
├── repositories.db              # Repository configurations
├── credentials.db               # Global Git credentials (encrypted)
├── kubeconfigs.db              # Cluster kubeconfigs (encrypted per repository)
├── temp/                       # Temporary kubeconfig files (used by hyve run)
│   └── kubeconfig-workflow-*
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
├── temp/                       # Temporary kubeconfig files (used by hyve run and workflows)
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

### Kubeconfig Workflow Example

```bash
# Add clusters to your repository
./hyve cluster add staging --region PHX1 --nodes g4s.kube.medium
./hyve cluster add production --region NYC1 --nodes g4s.kube.large,g4s.kube.large,g4s.kube.large

# Merge both clusters into your local kubeconfig
./hyve kubeconfig merge staging
./hyve kubeconfig merge production

# Now you can easily switch between clusters
kubectl config use-context staging
kubectl get nodes

kubectl config use-context production
kubectl get nodes

# View all available contexts
kubectl config get-contexts

# When you're done with a cluster, remove it
./hyve kubeconfig remove staging

# Or use hyve run without modifying local kubeconfig
./hyve run --cluster production kubectl get pods -A
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes and test locally
4. Submit a pull request (triggers dry-run validation)
5. Merge to main (triggers production deployment)

## Testing

Hyve includes comprehensive test coverage for all core functionality. The project uses Go's built-in testing framework.

### Running Tests

#### Basic Commands

```bash
# Run all tests
go test ./...

# Run all tests with verbose output
go test ./... -v

# Run all tests and show coverage
go test ./... -cover
```

#### Run Tests for Specific Packages

```bash
# Run tests for credentials package
go test ./internal/credentials -v

# Run tests for kubeconfig package
go test ./internal/kubeconfig -v

# Run tests for repository package
go test ./internal/repository -v

# Run tests for cluster package
go test ./internal/cluster -v
```

#### Run Individual Tests

```bash
# Run a specific test by name
go test ./internal/credentials -run TestStoreAndGetCredentials

# Run all tests matching a pattern
go test ./internal/repository -run TestAdd

# Run a specific subtest
go test ./internal/cluster -run TestShouldManage/hyve_prefix
```

### Coverage Reports

```bash
# Show coverage percentage for all packages
go test ./... -cover

# Generate detailed coverage report
go test ./... -coverprofile=coverage.out

# View coverage in browser
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Show coverage by function
go tool cover -func=coverage.out

# Coverage for specific package
go test ./internal/credentials -coverprofile=creds_coverage.out
go tool cover -html=creds_coverage.out
```

### Benchmarks

```bash
# Run all benchmarks
go test ./internal/credentials -bench=.

# Run specific benchmark
go test ./internal/credentials -bench=BenchmarkEncryption

# Run benchmarks with memory allocation stats
go test ./internal/credentials -bench=. -benchmem

# Run benchmarks multiple times for accuracy
go test ./internal/credentials -bench=. -benchtime=10s -count=5
```

### Advanced Testing

```bash
# Run tests with race detector
go test ./... -race

# Disable test caching (force re-run)
go test ./... -count=1

# Clear test cache
go clean -testcache

# Run tests in parallel
go test ./... -parallel 4

# Run with specific timeout
go test ./... -timeout 30s
```

### Test Coverage

The project currently has **56 tests** covering:

| Package | Tests | Coverage | Description |
|---------|-------|----------|-------------|
| `internal/credentials` | 11 + 2 benchmarks | ~92% | Git credentials & API token storage |
| `internal/kubeconfig` | 9 | ~89% | Kubeconfig merge/remove operations |
| `internal/repository` | 17 | ~90% | Repository management |
| `internal/cluster` | 16 | ~86% | Cluster lifecycle operations |

**Tested Features:**
- ✅ Credentials management (Git + API tokens)
- ✅ AES-GCM encryption/decryption
- ✅ Kubeconfig merge/remove operations
- ✅ Repository CRUD operations
- ✅ Cluster lifecycle (create, update, delete, wait)
- ✅ Orphaned cluster detection
- ✅ Database persistence
- ✅ Error handling

### CI/CD Integration

```bash
# CI-friendly output with coverage
go test ./... -v -race -coverprofile=coverage.out -covermode=atomic

# Check coverage threshold (example: 80%)
go test ./... -cover | grep "coverage:" | awk '{if ($2 < 80) exit 1}'
```

### Development Workflow

```bash
# Quick check during development
go test ./internal/credentials -v -count=1

# Full test suite before commit
go test ./... -cover -race

# Generate coverage report for review
go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out
```

### Test Structure

All tests follow Go best practices:
- Use `t.TempDir()` for clean test isolation
- Clear, descriptive test names following `TestFunction_Scenario` convention
- Table-driven tests where appropriate
- Comprehensive assertions with helpful error messages
- Proper error handling verification
- No external dependencies (use mocks and temporary databases)
- Benchmarks for performance-critical operations

### Example Test Output

```bash
$ go test ./... -cover
ok      civo-cluster-deploy/internal/cluster        0.213s  coverage: 85.7% of statements
ok      civo-cluster-deploy/internal/credentials    0.324s  coverage: 92.3% of statements
ok      civo-cluster-deploy/internal/kubeconfig     0.531s  coverage: 88.9% of statements
ok      civo-cluster-deploy/internal/repository     0.366s  coverage: 90.1% of statements
```

## License

[Add your license here]

## Support

- **Issues**: Create issues in the GitHub repository
- **Documentation**: This README and inline help (`--help`)
- **Logs**: Check workflow artifacts for detailed deployment logs