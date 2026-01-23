# Workflow Requirements Feature

## Overview

Hyve workflows now support a `requirements` field that validates prerequisites before workflow execution. This ensures all necessary CLI tools and secrets are available, preventing runtime failures and providing clear error messages.

## Features

### Tool Requirements
- **Automatic validation**: Check if CLI tools are available in PATH
- **Version checking**: Optionally validate minimum version requirements
- **Helpful errors**: Clear messages when tools are missing or wrong version

### Secret Requirements
- **Database integration**: Automatically load secrets from credentials database
- **Environment fallback**: Check environment variables if not in database
- **Optional secrets**: Mark secrets as required or optional
- **Multiple sources**: Support secrets from different providers (docker, github, datadog, etc.)

## Implementation Details

### New Types (`internal/workflow/types.go`)

```go
type WorkflowRequirements struct {
    Tools   []ToolRequirement   // CLI tools that must be available
    Secrets []SecretRequirement // Secrets that must be configured
}

type ToolRequirement struct {
    Name        string // Tool name (e.g., "kubectl", "helm", "docker")
    Version     string // Minimum version (optional)
    Description string // Human-readable description
}

type SecretRequirement struct {
    Name        string // Environment variable name
    Provider    string // Provider name for database lookup
    Required    bool   // Whether this secret is mandatory
    Description string // Human-readable description
}
```

### Validation Logic (`internal/workflow/requirements.go`)

The `RequirementValidator` provides:
- `ValidateRequirements()`: Validates all tools and secrets
- `validateTool()`: Checks tool availability and version
- `validateSecret()`: Checks secret in environment or database
- `LoadSecretsIntoEnvironment()`: Loads secrets from database into environment

### Workflow Executor Integration (`internal/workflow/executor.go`)

Requirements validation happens before workflow execution:
1. Create requirement validator
2. Validate all requirements (tools + secrets)
3. Load secrets into environment variables
4. Continue with workflow execution or fail with helpful errors

## Usage Examples

### Basic Tool Requirements

```yaml
apiVersion: v1
kind: Workflow
metadata:
  name: my-workflow
spec:
  requirements:
    tools:
      - name: kubectl
        version: "1.28"
        description: Kubernetes CLI for cluster operations
      - name: helm
        version: "3.12"
        description: Helm package manager
```

### Basic Secret Requirements

```yaml
apiVersion: v1
kind: Workflow
metadata:
  name: docker-build
spec:
  requirements:
    secrets:
      - name: DOCKER_TOKEN
        provider: docker
        required: true
        description: Docker Hub authentication token
```

### Complete Example

```yaml
apiVersion: v1
kind: Workflow
metadata:
  name: k8s-deploy
  description: Deploy to Kubernetes with validation
spec:
  requirements:
    tools:
      - name: kubectl
        version: "1.28"
        description: Kubernetes CLI
      - name: helm
        version: "3.12"
        description: Helm package manager
    secrets:
      - name: REGISTRY_TOKEN
        provider: registry
        required: true
        description: Container registry token
      - name: DATADOG_API_KEY
        provider: datadog
        required: false
        description: Monitoring API key (optional)

  env:
    APP_NAME: my-app
    NAMESPACE: default

  jobs:
    - name: deploy
      steps:
        - name: helm-deploy
          command: helm upgrade --install ${APP_NAME} ./charts
```

## Setting Up Secrets

### Via Database (Recommended)

```bash
# Store secret in encrypted database
./hyve config set-token docker
# Enter token when prompted

# Or provide directly
./hyve config set-token github --token ghp_xxxxxxxxxxxx

# List stored tokens
./hyve config list-tokens

# View specific token
./hyve config get-token docker
```

### Via Environment Variable

```bash
# Set environment variable
export DOCKER_TOKEN=my-docker-token

# Run workflow (uses environment variable)
./hyve workflow run docker-build
```

## Secret Loading Priority

When a workflow requires a secret:
1. **Environment Variable**: If `$SECRET_NAME` is already set, use it
2. **Credentials Database**: If `provider` specified, load from database
3. **Error**: If required and not found, fail with helpful message

## Error Messages

### Missing Tool

```
Requirements validation failed:
  - Required tool 'helm' not found in PATH (Helm package manager)
```

### Version Mismatch

```
Requirements validation failed:
  - tool 'kubectl' version mismatch: found 1.26.0, requires 1.28
```

### Missing Secret

```
Requirements validation failed:
  - Required secret 'DOCKER_TOKEN' not found (Docker Hub authentication token)
    Set via: hyve config set-token docker OR export DOCKER_TOKEN=your-secret
```

### Multiple Errors

```
Requirements validation failed:
  - Required tool 'helm' not found in PATH (Helm package manager)
  - Required secret 'DOCKER_TOKEN' not found (Docker Hub authentication token)
    Set via: hyve config set-token docker OR export DOCKER_TOKEN=your-secret
  - Required secret 'GITHUB_TOKEN' not found (GitHub API token)
    Set via: hyve config set-token github OR export GITHUB_TOKEN=your-secret
```

## Successful Validation

When all requirements are met:

```
[INFO] Starting workflow 'docker-build'
[INFO] Validating workflow requirements...
[INFO] ✅ All requirements validated successfully
[INFO][validate-environment] Starting job 'validate-environment'
...
```

## Testing

### Test Missing Requirements

```bash
# Run workflow without setting secrets
./hyve workflow run docker-build
# Error: Required secret 'DOCKER_TOKEN' not found

# Run workflow without tool in PATH
# Temporarily rename docker
sudo mv /usr/local/bin/docker /usr/local/bin/docker.bak
./hyve workflow run docker-build
# Error: Required tool 'docker' not found in PATH
```

### Test With Requirements Met

```bash
# Set required secret
./hyve config set-token docker

# Run workflow successfully
./hyve workflow run docker-build
# ✅ All requirements validated successfully
```

## Best Practices

### 1. Be Specific with Versions

```yaml
requirements:
  tools:
    - name: kubectl
      version: "1.28"  # Specify minimum version
```

### 2. Add Helpful Descriptions

```yaml
requirements:
  secrets:
    - name: DOCKER_TOKEN
      provider: docker
      required: true
      description: Get token from https://hub.docker.com/settings/security
```

### 3. Mark Optional Secrets Appropriately

```yaml
requirements:
  secrets:
    - name: DATADOG_API_KEY
      provider: datadog
      required: false  # Won't fail if missing
      description: Monitoring is optional
```

### 4. Group Related Requirements

```yaml
requirements:
  tools:
    # Container tools
    - name: docker
      version: "20.10"
    - name: buildx

    # Kubernetes tools
    - name: kubectl
      version: "1.28"
    - name: helm
      version: "3.12"
```

## Implementation Files

- `internal/workflow/types.go` - Type definitions
- `internal/workflow/requirements.go` - Validation logic
- `internal/workflow/executor.go` - Integration with workflow execution
- Example workflows:
  - `.hyve/repositories/test/workflows/docker-build.yaml`
  - `.hyve/repositories/test/workflows/k8s-deploy.yaml`

## Future Enhancements

Potential improvements:
- Support for custom version comparison logic
- Integration with OS package managers (apt, brew, etc.)
- Auto-installation of missing tools (optional)
- Secret rotation and expiration warnings
- Support for secret encryption at rest
- Validation caching to avoid repeated checks
