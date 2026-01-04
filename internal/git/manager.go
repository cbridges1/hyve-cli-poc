package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// Manager handles Git repository operations for state management
type Manager struct {
	repoURL   string
	localPath string
	username  string
	token     string
	repo      *git.Repository
}

// NewManager creates a new Git repository manager
func NewManager(repoURL, localPath, username, token string) *Manager {
	return &Manager{
		repoURL:   repoURL,
		localPath: localPath,
		username:  username,
		token:     token,
	}
}

// Clone clones the repository to the local path
func (m *Manager) Clone(ctx context.Context) error {
	if _, err := os.Stat(m.localPath); err == nil {
		// Directory exists, try to open as existing repo
		repo, err := git.PlainOpen(m.localPath)
		if err == nil {
			m.repo = repo
			return nil
		}
		// If opening fails, remove and re-clone
		os.RemoveAll(m.localPath)
	}

	// Create parent directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(m.localPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}

	var auth *http.BasicAuth
	if m.username != "" && m.token != "" {
		auth = &http.BasicAuth{
			Username: m.username,
			Password: m.token,
		}
	}

	repo, err := git.PlainClone(m.localPath, false, &git.CloneOptions{
		URL:  m.repoURL,
		Auth: auth,
	})
	if err != nil {
		// If repository is empty, initialize a new one and add remote
		if err.Error() == "remote repository is empty" {
			return m.createNewRepo(context.Background())
		}
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	m.repo = repo
	return nil
}

// Pull pulls the latest changes from the remote repository
func (m *Manager) Pull(ctx context.Context) error {
	if m.repo == nil {
		return fmt.Errorf("repository not initialized")
	}

	workTree, err := m.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	var auth *http.BasicAuth
	if m.username != "" && m.token != "" {
		auth = &http.BasicAuth{
			Username: m.username,
			Password: m.token,
		}
	}

	err = workTree.Pull(&git.PullOptions{
		RemoteName: "origin",
		Auth:       auth,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate && err.Error() != "remote repository is empty" {
		return fmt.Errorf("failed to pull changes: %w", err)
	}

	return nil
}

// Commit commits changes to the repository
func (m *Manager) Commit(ctx context.Context, message string) error {
	if m.repo == nil {
		return fmt.Errorf("repository not initialized")
	}

	workTree, err := m.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Add all changes
	_, err = workTree.Add(".")
	if err != nil {
		return fmt.Errorf("failed to add changes: %w", err)
	}

	// Commit changes
	_, err = workTree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Hyve CLI",
			Email: "cli@hyve.local",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	return nil
}

// Push pushes changes to the remote repository
func (m *Manager) Push(ctx context.Context) error {
	if m.repo == nil {
		return fmt.Errorf("repository not initialized")
	}

	var auth *http.BasicAuth
	if m.username != "" && m.token != "" {
		auth = &http.BasicAuth{
			Username: m.username,
			Password: m.token,
		}
	}

	err := m.repo.Push(&git.PushOptions{
		RemoteName: "origin",
		Auth:       auth,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to push changes: %w", err)
	}

	return nil
}

// GetStateDir returns the path to the clusters directory within the repository
func (m *Manager) GetStateDir() string {
	return filepath.Join(m.localPath, "clusters")
}

// EnsureStateDir ensures the clusters directory exists in the repository
func (m *Manager) EnsureStateDir() error {
	stateDir := m.GetStateDir()
	return os.MkdirAll(stateDir, 0755)
}

// InitializeRepo initializes a new repository if it doesn't exist
func (m *Manager) InitializeRepo(ctx context.Context) error {
	// Try to clone first
	err := m.Clone(ctx)
	if err == nil {
		return m.EnsureStateDir()
	}

	// If clone fails, create a new repository
	return m.createNewRepo(ctx)
}

// createNewRepo creates a new local repository
func (m *Manager) createNewRepo(ctx context.Context) error {
	if err := os.MkdirAll(m.localPath, 0755); err != nil {
		return fmt.Errorf("failed to create local path: %w", err)
	}

	repo, err := git.PlainInit(m.localPath, false)
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}

	m.repo = repo

	// Add remote origin if URL is provided
	if m.repoURL != "" {
		_, err = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{m.repoURL},
		})
		if err != nil {
			return fmt.Errorf("failed to add remote origin: %w", err)
		}
	}

	// Ensure state directory exists
	if err := m.EnsureStateDir(); err != nil {
		return err
	}

	// Create initial commit with README
	readmePath := filepath.Join(m.localPath, "README.md")
	readmeContent := `# Hyve State Repository

This repository contains cluster state definitions for Hyve.
All cluster YAML files are stored in the clusters/ directory.
`
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		return fmt.Errorf("failed to create README: %w", err)
	}

	// Create initial commit
	return m.Commit(ctx, "Initialize Hyve state repository")
}
