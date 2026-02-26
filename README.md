<p align="center">
  <img src="images/banner.svg" alt="Hyve Banner" width="800">
</p>

# Hyve

A GitOps-first Kubernetes cluster management CLI. Define clusters as YAML, commit the change, and Hyve reconciles the desired state against your cloud provider — locally or through a CI/CD pipeline.

Supports **Civo, AWS (EKS), GCP (GKE), and Azure (AKS)** with multi-account credential routing, strict delete enforcement, automated post-deploy workflows, and encrypted kubeconfig storage.

[![Documentation](https://img.shields.io/badge/docs-hyve.mintlify.app-green)](https://hyve.mintlify.app)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Features

- **GitOps Native** - All cluster state managed through Git repositories
- **Multi-Repository** - Separate repos for dev/staging/prod environments
- **Automated Workflows** - Define deployment pipelines with requirements validation
- **Cluster Templates** - Reusable cluster patterns with automated workflows
- **Variable Substitution** - Full shell support with workflow and environment variables

## Documentation

Full documentation at **[hyve.mintlify.app](https://hyve.mintlify.app)** — CLI reference, guides, and provider configuration.

## Installation

Requires Go 1.21+ and Git in `PATH`.

**Using `go install`:**

```bash
go install github.com/cbridges1/hyve@latest
```

**From source:**

```bash
git clone https://github.com/cbridges1/hyve.git
cd hyve
go build -o hyve .
sudo mv hyve /usr/local/bin/
```

## Quick Start

```bash
# 1. Store Civo token (or use CIVO_TOKEN env var)
hyve config set-token civo

# 2. Add a state repository
hyve git add production --repo-url https://github.com/company/hyve-state.git

# 3. Create a cluster
hyve cluster add my-cluster --provider civo --region PHX1 --nodes g4s.kube.medium

# 4. Run a workflow
./hyve workflow run deploy-app --cluster my-cluster
```

## Development

[Task](https://taskfile.dev) is used to simplify common operations:

| Command | Description |
|---------|-------------|
| `task build` | Build the `hyve` binary |
| `task run -- [args]` | Build and run with arguments |
| `task dev -- [args]` | Run directly with `go run` |
| `task test` | Run all tests |
| `task test:verbose` | Run all tests with verbose output |
| `task test:race` | Run all tests with race detector |
| `task test:cover` | Run all tests with coverage report |
| `task test:report` | Run tests and generate JSON report |
| `task vet` | Run `go vet` |
| `task check` | Run vet and tests |
| `task tidy` | Tidy go modules |
| `task clean` | Remove binary and report artifacts |
