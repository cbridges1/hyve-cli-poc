package repository

import (
	"path/filepath"
	"testing"
)

// TestAddRepository tests adding a new repository
func TestAddRepository(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_repositories.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Add first repository
	repo, err := mgr.AddRepository("test-repo", "https://github.com/test/repo.git", "/path/to/repo", "testuser")
	if err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	// Verify repository was added
	if repo.Name != "test-repo" {
		t.Errorf("Expected name test-repo, got %s", repo.Name)
	}
	if repo.RepoURL != "https://github.com/test/repo.git" {
		t.Errorf("Expected repo URL https://github.com/test/repo.git, got %s", repo.RepoURL)
	}
	if repo.LocalPath != "/path/to/repo" {
		t.Errorf("Expected local path /path/to/repo, got %s", repo.LocalPath)
	}
	if repo.Username != "testuser" {
		t.Errorf("Expected username testuser, got %s", repo.Username)
	}

	// First repository should be marked as current
	if !repo.IsCurrent {
		t.Error("First repository should be marked as current")
	}
}

// TestAddDuplicateRepository tests that adding a duplicate repository fails
func TestAddDuplicateRepository(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_repositories.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Add first repository
	_, err := mgr.AddRepository("test-repo", "https://github.com/test/repo.git", "/path/to/repo", "testuser")
	if err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	// Try to add duplicate
	_, err = mgr.AddRepository("test-repo", "https://github.com/test/repo2.git", "/path/to/repo2", "testuser2")
	if err == nil {
		t.Error("Expected error when adding duplicate repository")
	}
}

// TestUpdateRepository tests updating a repository
func TestUpdateRepository(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_repositories.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Add repository
	_, err := mgr.AddRepository("test-repo", "https://github.com/test/repo.git", "/path/to/repo", "testuser")
	if err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	// Update repository
	updated, err := mgr.UpdateRepository("test-repo", "https://github.com/test/updated.git", "/new/path", "newuser")
	if err != nil {
		t.Fatalf("Failed to update repository: %v", err)
	}

	// Verify updates
	if updated.RepoURL != "https://github.com/test/updated.git" {
		t.Errorf("Expected updated repo URL, got %s", updated.RepoURL)
	}
	if updated.LocalPath != "/new/path" {
		t.Errorf("Expected updated local path, got %s", updated.LocalPath)
	}
	if updated.Username != "newuser" {
		t.Errorf("Expected updated username, got %s", updated.Username)
	}
}

// TestUpdateNonExistentRepository tests updating a repository that doesn't exist
func TestUpdateNonExistentRepository(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_repositories.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Try to update non-existent repository
	_, err := mgr.UpdateRepository("nonexistent", "https://github.com/test/repo.git", "/path", "user")
	if err == nil {
		t.Error("Expected error when updating non-existent repository")
	}
}

// TestDeleteRepository tests deleting a repository
func TestDeleteRepository(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_repositories.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Add repository
	_, err := mgr.AddRepository("test-repo", "https://github.com/test/repo.git", "/path/to/repo", "testuser")
	if err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	// Delete repository
	err = mgr.DeleteRepository("test-repo")
	if err != nil {
		t.Fatalf("Failed to delete repository: %v", err)
	}

	// Verify repository is gone
	_, err = mgr.GetRepositoryByName("test-repo")
	if err == nil {
		t.Error("Expected error when getting deleted repository")
	}
}

// TestDeleteNonExistentRepository tests deleting a repository that doesn't exist
func TestDeleteNonExistentRepository(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_repositories.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Try to delete non-existent repository
	err := mgr.DeleteRepository("nonexistent")
	if err == nil {
		t.Error("Expected error when deleting non-existent repository")
	}
}

// TestListRepositories tests listing all repositories
func TestListRepositories(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_repositories.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Add multiple repositories
	repos := []struct {
		name      string
		repoURL   string
		localPath string
		username  string
	}{
		{"repo1", "https://github.com/test/repo1.git", "/path/to/repo1", "user1"},
		{"repo2", "https://github.com/test/repo2.git", "/path/to/repo2", "user2"},
		{"repo3", "https://github.com/test/repo3.git", "/path/to/repo3", "user3"},
	}

	for _, r := range repos {
		_, err := mgr.AddRepository(r.name, r.repoURL, r.localPath, r.username)
		if err != nil {
			t.Fatalf("Failed to add repository %s: %v", r.name, err)
		}
	}

	// List repositories
	list, err := mgr.ListRepositories()
	if err != nil {
		t.Fatalf("Failed to list repositories: %v", err)
	}

	// Verify count
	if len(list) != len(repos) {
		t.Errorf("Expected %d repositories, got %d", len(repos), len(list))
	}

	// Verify all repositories are in the list
	repoMap := make(map[string]bool)
	for _, repo := range list {
		repoMap[repo.Name] = true
	}

	for _, expected := range repos {
		if !repoMap[expected.name] {
			t.Errorf("Expected repository %s not found in list", expected.name)
		}
	}
}

// TestGetRepositoryByName tests retrieving a repository by name
func TestGetRepositoryByName(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_repositories.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Add repository
	_, err := mgr.AddRepository("test-repo", "https://github.com/test/repo.git", "/path/to/repo", "testuser")
	if err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	// Get repository by name
	repo, err := mgr.GetRepositoryByName("test-repo")
	if err != nil {
		t.Fatalf("Failed to get repository by name: %v", err)
	}

	if repo.Name != "test-repo" {
		t.Errorf("Expected name test-repo, got %s", repo.Name)
	}
}

// TestGetNonExistentRepositoryByName tests getting a repository that doesn't exist
func TestGetNonExistentRepositoryByName(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_repositories.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Try to get non-existent repository
	_, err := mgr.GetRepositoryByName("nonexistent")
	if err == nil {
		t.Error("Expected error when getting non-existent repository")
	}
}

// TestGetRepositoryByID tests retrieving a repository by ID
func TestGetRepositoryByID(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_repositories.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Add repository
	added, err := mgr.AddRepository("test-repo", "https://github.com/test/repo.git", "/path/to/repo", "testuser")
	if err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	// Get repository by ID
	repo, err := mgr.GetRepositoryByID(added.ID)
	if err != nil {
		t.Fatalf("Failed to get repository by ID: %v", err)
	}

	if repo.ID != added.ID {
		t.Errorf("Expected ID %d, got %d", added.ID, repo.ID)
	}
	if repo.Name != "test-repo" {
		t.Errorf("Expected name test-repo, got %s", repo.Name)
	}
}

// TestGetCurrentRepository tests getting the current repository
func TestGetCurrentRepository(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_repositories.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Add first repository (should be marked as current)
	_, err := mgr.AddRepository("test-repo", "https://github.com/test/repo.git", "/path/to/repo", "testuser")
	if err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	// Get current repository
	current, err := mgr.GetCurrentRepository()
	if err != nil {
		t.Fatalf("Failed to get current repository: %v", err)
	}

	if current.Name != "test-repo" {
		t.Errorf("Expected current repository name test-repo, got %s", current.Name)
	}
	if !current.IsCurrent {
		t.Error("Repository should be marked as current")
	}
}

// TestSetCurrentRepository tests setting a repository as current
func TestSetCurrentRepository(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_repositories.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Add two repositories
	_, err := mgr.AddRepository("repo1", "https://github.com/test/repo1.git", "/path/to/repo1", "user1")
	if err != nil {
		t.Fatalf("Failed to add repo1: %v", err)
	}

	_, err = mgr.AddRepository("repo2", "https://github.com/test/repo2.git", "/path/to/repo2", "user2")
	if err != nil {
		t.Fatalf("Failed to add repo2: %v", err)
	}

	// Set repo2 as current
	err = mgr.SetCurrentRepository("repo2")
	if err != nil {
		t.Fatalf("Failed to set current repository: %v", err)
	}

	// Verify repo2 is current
	current, err := mgr.GetCurrentRepository()
	if err != nil {
		t.Fatalf("Failed to get current repository: %v", err)
	}

	if current.Name != "repo2" {
		t.Errorf("Expected current repository repo2, got %s", current.Name)
	}

	// Verify repo1 is not current
	repo1, err := mgr.GetRepositoryByName("repo1")
	if err != nil {
		t.Fatalf("Failed to get repo1: %v", err)
	}

	if repo1.IsCurrent {
		t.Error("repo1 should not be current")
	}
}

// TestSetNonExistentRepositoryAsCurrent tests setting a non-existent repository as current
func TestSetNonExistentRepositoryAsCurrent(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_repositories.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Try to set non-existent repository as current
	err := mgr.SetCurrentRepository("nonexistent")
	if err == nil {
		t.Error("Expected error when setting non-existent repository as current")
	}
}

// TestHasRepositories tests checking if repositories exist
func TestHasRepositories(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_repositories.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Initially should have no repositories
	hasRepos, err := mgr.HasRepositories()
	if err != nil {
		t.Fatalf("Failed to check for repositories: %v", err)
	}
	if hasRepos {
		t.Error("Expected no repositories initially")
	}

	// Add a repository
	_, err = mgr.AddRepository("test-repo", "https://github.com/test/repo.git", "/path/to/repo", "testuser")
	if err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	// Should now have repositories
	hasRepos, err = mgr.HasRepositories()
	if err != nil {
		t.Fatalf("Failed to check for repositories: %v", err)
	}
	if !hasRepos {
		t.Error("Expected to have repositories")
	}
}

// TestOnlyOneCurrentRepository tests that only one repository can be current at a time
func TestOnlyOneCurrentRepository(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_repositories.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Add three repositories
	_, err := mgr.AddRepository("repo1", "https://github.com/test/repo1.git", "/path/to/repo1", "user1")
	if err != nil {
		t.Fatalf("Failed to add repo1: %v", err)
	}

	_, err = mgr.AddRepository("repo2", "https://github.com/test/repo2.git", "/path/to/repo2", "user2")
	if err != nil {
		t.Fatalf("Failed to add repo2: %v", err)
	}

	_, err = mgr.AddRepository("repo3", "https://github.com/test/repo3.git", "/path/to/repo3", "user3")
	if err != nil {
		t.Fatalf("Failed to add repo3: %v", err)
	}

	// Set repo2 as current
	err = mgr.SetCurrentRepository("repo2")
	if err != nil {
		t.Fatalf("Failed to set current repository: %v", err)
	}

	// List all repositories and count how many are current
	repos, err := mgr.ListRepositories()
	if err != nil {
		t.Fatalf("Failed to list repositories: %v", err)
	}

	currentCount := 0
	for _, repo := range repos {
		if repo.IsCurrent {
			currentCount++
			if repo.Name != "repo2" {
				t.Errorf("Expected repo2 to be current, but %s is current", repo.Name)
			}
		}
	}

	if currentCount != 1 {
		t.Errorf("Expected exactly 1 current repository, got %d", currentCount)
	}
}

// TestDeleteCurrentRepository tests that deleting the current repository switches to another
func TestDeleteCurrentRepository(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_repositories.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Add two repositories
	_, err := mgr.AddRepository("repo1", "https://github.com/test/repo1.git", "/path/to/repo1", "user1")
	if err != nil {
		t.Fatalf("Failed to add repo1: %v", err)
	}

	_, err = mgr.AddRepository("repo2", "https://github.com/test/repo2.git", "/path/to/repo2", "user2")
	if err != nil {
		t.Fatalf("Failed to add repo2: %v", err)
	}

	// Verify repo1 is current (first one added)
	current, err := mgr.GetCurrentRepository()
	if err != nil {
		t.Fatalf("Failed to get current repository: %v", err)
	}
	if current.Name != "repo1" {
		t.Errorf("Expected repo1 to be current initially, got %s", current.Name)
	}

	// Delete current repository (repo1)
	err = mgr.DeleteRepository("repo1")
	if err != nil {
		t.Fatalf("Failed to delete current repository: %v", err)
	}

	// Verify repo2 is now current
	current, err = mgr.GetCurrentRepository()
	if err != nil {
		t.Fatalf("Failed to get current repository after deletion: %v", err)
	}
	if current.Name != "repo2" {
		t.Errorf("Expected repo2 to be current after deletion, got %s", current.Name)
	}
}

// TestDatabasePersistence tests that data persists across manager instances
func TestDatabasePersistence(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_repositories.db")

	// First manager - add repository
	mgr1 := &Manager{dbPath: dbPath}
	if err := mgr1.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	_, err := mgr1.AddRepository("test-repo", "https://github.com/test/repo.git", "/path/to/repo", "testuser")
	if err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}
	mgr1.Close()

	// Second manager - verify data persists
	mgr2 := &Manager{dbPath: dbPath}
	if err := mgr2.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize second database: %v", err)
	}
	defer mgr2.Close()

	repo, err := mgr2.GetRepositoryByName("test-repo")
	if err != nil {
		t.Fatalf("Failed to get repository from second manager: %v", err)
	}

	if repo.Name != "test-repo" {
		t.Errorf("Expected repository test-repo, got %s", repo.Name)
	}
}
