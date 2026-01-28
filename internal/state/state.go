package state

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"civo-cluster-deploy/internal/git"
	"civo-cluster-deploy/internal/types"
)

// Manager handles state file operations using Git repositories
type Manager struct {
	stateDir   string
	gitManager git.GitBackend
}

// NewManager creates a new state manager with Git repository support
func NewManager(gitRepoURL, localPath, username, token string) (*Manager, error) {
	backendType := git.GetBackendType()
	gitMgr, err := git.NewBackend(gitRepoURL, localPath, username, token, backendType)
	if err != nil {
		return nil, fmt.Errorf("failed to create git backend: %w", err)
	}

	return &Manager{
		stateDir:   gitMgr.GetStateDir(),
		gitManager: gitMgr,
	}, nil
}

// InitializeGitRepo initializes or clones the Git repository
func (m *Manager) InitializeGitRepo(ctx context.Context) error {
	return m.gitManager.InitializeRepo(ctx)
}

// SyncWithRemote pulls latest changes from the remote repository
func (m *Manager) SyncWithRemote(ctx context.Context) error {
	return m.gitManager.Pull(ctx)
}

// CommitAndPush commits changes and pushes to remote repository
func (m *Manager) CommitAndPush(ctx context.Context, message string) error {
	if err := m.gitManager.Commit(ctx, message); err != nil {
		return err
	}

	return m.gitManager.Push(ctx)
}

// LoadClusterDefinitions loads all cluster definitions from YAML files
func (m *Manager) LoadClusterDefinitions() ([]types.ClusterDefinition, error) {
	var clusters []types.ClusterDefinition

	err := filepath.WalkDir(m.stateDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		var cluster types.ClusterDefinition
		if err := yaml.Unmarshal(data, &cluster); err != nil {
			return fmt.Errorf("failed to unmarshal YAML file %s: %w", path, err)
		}

		clusters = append(clusters, cluster)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return clusters, nil
}

// ValidateClusterDefinitions validates cluster definitions
func (m *Manager) ValidateClusterDefinitions(clusters []types.ClusterDefinition) error {
	// Basic validation can be added here if needed in the future
	return nil
}

// OrderClusters returns clusters in their original order
func (m *Manager) OrderClusters(clusters []types.ClusterDefinition) []types.ClusterDefinition {
	return clusters
}
