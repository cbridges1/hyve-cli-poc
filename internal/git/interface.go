package git

import "context"

// BranchInfo holds information about a Git branch
type BranchInfo struct {
	Name      string
	IsCurrent bool
	Hash      string
}

// GitBackend defines the interface for Git operations using system git
type GitBackend interface {
	// Clone clones the repository to the local path
	Clone(ctx context.Context) error

	// Pull pulls the latest changes from the remote repository
	Pull(ctx context.Context) error

	// Commit commits changes to the repository
	Commit(ctx context.Context, message string) error

	// Push pushes changes to the remote repository
	Push(ctx context.Context) error

	// GetStateDir returns the path to the clusters directory within the repository
	GetStateDir() string

	// EnsureStateDir ensures the clusters directory exists in the repository
	EnsureStateDir() error

	// InitializeRepo initializes a new repository if it doesn't exist
	InitializeRepo(ctx context.Context) error

	// ListBranches lists all branches in the repository
	ListBranches(ctx context.Context) ([]BranchInfo, error)

	// GetCurrentBranch returns the current branch name
	GetCurrentBranch(ctx context.Context) (string, error)

	// CreateBranch creates a new branch from the current HEAD
	CreateBranch(ctx context.Context, branchName string) error

	// SwitchBranch switches to a different branch (checkout)
	SwitchBranch(ctx context.Context, branchName string) error

	// DeleteBranch deletes a branch
	DeleteBranch(ctx context.Context, branchName string, force bool) error

	// PushBranch pushes a specific branch to remote
	PushBranch(ctx context.Context, branchName string) error

	// HasUncommittedChanges checks if there are uncommitted changes in the repository
	HasUncommittedChanges(ctx context.Context) (bool, error)

	// GetStatusSummary returns a summary of uncommitted changes
	GetStatusSummary(ctx context.Context) (string, error)
}

// BackendType represents the type of Git backend to use
type BackendType string

const (
	// BackendSystem uses system git command
	BackendSystem BackendType = "system"
)
