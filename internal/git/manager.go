package git

import (
	"fmt"
	"os"
	"os/exec"
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

// GetBackendType returns the configured git backend type from environment
// Defaults to system git
func GetBackendType() BackendType {
	backend := os.Getenv("GIT_BACKEND")
	if backend == "" {
		return BackendSystem // Default to system git
	}

	switch backend {
	case "system":
		return BackendSystem
	case "builtin":
		return BackendBuiltIn
	default:
		// Unknown backend, default to system
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
