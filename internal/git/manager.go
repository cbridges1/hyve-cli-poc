package git

import (
	"fmt"
	"os"
	"os/exec"

	"civo-cluster-deploy/internal/config"
)

// NewBackend creates a new Git backend based on configuration
// By default, it uses system git for easier onboarding
func NewBackend(repoURL, localPath, username, token string, backendType BackendType) (GitBackend, error) {
	// Default to system git if not specified
	if backendType == "" {
		backendType = BackendSystem
	}

	switch backendType {
	case BackendSystem:
		// Check if git is available
		if err := checkSystemGit(); err != nil {
			return nil, fmt.Errorf("system git not available: %w\nConsider setting GIT_BACKEND=builtin to use built-in git library", err)
		}
		return NewSystemBackend(repoURL, localPath, username, token), nil

	case BackendBuiltIn:
		return NewBuiltInBackend(repoURL, localPath, username, token), nil

	default:
		return nil, fmt.Errorf("unknown git backend type: %s", backendType)
	}
}

// GetBackendType returns the configured git backend type
// Priority: 1) Database config 2) Environment variable 3) Default to system git
func GetBackendType() BackendType {
	// Load from config manager (which checks config file, then env var, then defaults to "system")
	configMgr := config.NewManager()
	if err := configMgr.LoadConfig(); err == nil {
		backend := configMgr.GetGitBackend()
		return GetBackendTypeFromConfig(backend)
	}

	// Fallback to environment variable if config fails
	backend := os.Getenv("GIT_BACKEND")
	if backend == "" {
		backend = "system" // Default to system git
	}

	return GetBackendTypeFromConfig(backend)
}

// GetBackendTypeFromConfig returns the backend type from config manager
func GetBackendTypeFromConfig(configBackend string) BackendType {
	switch configBackend {
	case "system":
		return BackendSystem
	case "builtin":
		return BackendBuiltIn
	default:
		return BackendSystem
	}
}

// checkSystemGit verifies that system git is available
func checkSystemGit() error {
	cmd := exec.Command("git", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git command not found in PATH: %w", err)
	}
	return nil
}
