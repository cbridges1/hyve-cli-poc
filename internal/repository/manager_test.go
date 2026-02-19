package repository

import (
	"testing"

	"hyve/internal/database"
)

func setupTestDB(t *testing.T) (*database.DB, func()) {
	tempDir := t.TempDir()
	db, err := database.GetDBWithDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

func TestAddRepository(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	repo, err := mgr.AddRepository("test-repo", "https://github.com/test/test.git", "/tmp/test", "testuser")
	if err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	if repo.Name != "test-repo" {
		t.Errorf("Expected name 'test-repo', got '%s'", repo.Name)
	}
	if repo.RepoURL != "https://github.com/test/test.git" {
		t.Errorf("Expected repo URL 'https://github.com/test/test.git', got '%s'", repo.RepoURL)
	}
	if repo.LocalPath != "/tmp/test" {
		t.Errorf("Expected local path '/tmp/test', got '%s'", repo.LocalPath)
	}
	if repo.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", repo.Username)
	}

	// First repository should be set as current
	if !repo.IsCurrent {
		t.Error("Expected first repository to be set as current")
	}
}

func TestAddDuplicateRepository(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	_, err := mgr.AddRepository("test-repo", "https://github.com/test/test.git", "/tmp/test", "testuser")
	if err != nil {
		t.Fatalf("Failed to add first repository: %v", err)
	}

	_, err = mgr.AddRepository("test-repo", "https://github.com/test/test2.git", "/tmp/test2", "testuser2")
	if err == nil {
		t.Error("Expected error when adding duplicate repository name")
	}
}

func TestGetRepositoryByName(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	_, err := mgr.AddRepository("test-repo", "https://github.com/test/test.git", "/tmp/test", "testuser")
	if err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	repo, err := mgr.GetRepositoryByName("test-repo")
	if err != nil {
		t.Fatalf("Failed to get repository: %v", err)
	}

	if repo.Name != "test-repo" {
		t.Errorf("Expected name 'test-repo', got '%s'", repo.Name)
	}
}

func TestGetRepositoryByNameNotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	_, err := mgr.GetRepositoryByName("nonexistent")
	if err == nil {
		t.Error("Expected error when getting nonexistent repository")
	}
}

func TestUpdateRepository(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	_, err := mgr.AddRepository("test-repo", "https://github.com/test/test.git", "/tmp/test", "testuser")
	if err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	updated, err := mgr.UpdateRepository("test-repo", "https://github.com/test/updated.git", "/tmp/updated", "updateduser")
	if err != nil {
		t.Fatalf("Failed to update repository: %v", err)
	}

	if updated.RepoURL != "https://github.com/test/updated.git" {
		t.Errorf("Expected updated repo URL, got '%s'", updated.RepoURL)
	}
	if updated.LocalPath != "/tmp/updated" {
		t.Errorf("Expected updated local path, got '%s'", updated.LocalPath)
	}
	if updated.Username != "updateduser" {
		t.Errorf("Expected updated username, got '%s'", updated.Username)
	}
}

func TestDeleteRepository(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	_, err := mgr.AddRepository("test-repo", "https://github.com/test/test.git", "/tmp/test", "testuser")
	if err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	err = mgr.DeleteRepository("test-repo")
	if err != nil {
		t.Fatalf("Failed to delete repository: %v", err)
	}

	_, err = mgr.GetRepositoryByName("test-repo")
	if err == nil {
		t.Error("Expected error when getting deleted repository")
	}
}

func TestListRepositories(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	_, err := mgr.AddRepository("repo1", "https://github.com/test/repo1.git", "/tmp/repo1", "user1")
	if err != nil {
		t.Fatalf("Failed to add repository 1: %v", err)
	}
	_, err = mgr.AddRepository("repo2", "https://github.com/test/repo2.git", "/tmp/repo2", "user2")
	if err != nil {
		t.Fatalf("Failed to add repository 2: %v", err)
	}

	repos, err := mgr.ListRepositories()
	if err != nil {
		t.Fatalf("Failed to list repositories: %v", err)
	}

	if len(repos) != 2 {
		t.Errorf("Expected 2 repositories, got %d", len(repos))
	}
}

func TestSetCurrentRepository(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	_, err := mgr.AddRepository("repo1", "https://github.com/test/repo1.git", "/tmp/repo1", "user1")
	if err != nil {
		t.Fatalf("Failed to add repository 1: %v", err)
	}
	_, err = mgr.AddRepository("repo2", "https://github.com/test/repo2.git", "/tmp/repo2", "user2")
	if err != nil {
		t.Fatalf("Failed to add repository 2: %v", err)
	}

	err = mgr.SetCurrentRepository("repo2")
	if err != nil {
		t.Fatalf("Failed to set current repository: %v", err)
	}

	current, err := mgr.GetCurrentRepository()
	if err != nil {
		t.Fatalf("Failed to get current repository: %v", err)
	}

	if current.Name != "repo2" {
		t.Errorf("Expected current repository 'repo2', got '%s'", current.Name)
	}
}

func TestGetCurrentRepository(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	_, err := mgr.AddRepository("test-repo", "https://github.com/test/test.git", "/tmp/test", "testuser")
	if err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	current, err := mgr.GetCurrentRepository()
	if err != nil {
		t.Fatalf("Failed to get current repository: %v", err)
	}

	if current.Name != "test-repo" {
		t.Errorf("Expected current repository 'test-repo', got '%s'", current.Name)
	}
}

func TestGetCurrentRepositoryNone(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	_, err := mgr.GetCurrentRepository()
	if err == nil {
		t.Error("Expected error when no current repository is set")
	}
}

func TestHasRepositories(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	has, err := mgr.HasRepositories()
	if err != nil {
		t.Fatalf("Failed to check for repositories: %v", err)
	}
	if has {
		t.Error("Expected no repositories initially")
	}

	_, err = mgr.AddRepository("test-repo", "https://github.com/test/test.git", "/tmp/test", "testuser")
	if err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	has, err = mgr.HasRepositories()
	if err != nil {
		t.Fatalf("Failed to check for repositories: %v", err)
	}
	if !has {
		t.Error("Expected repositories after adding one")
	}
}

func TestDeleteCurrentRepository(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	_, err := mgr.AddRepository("repo1", "https://github.com/test/repo1.git", "/tmp/repo1", "user1")
	if err != nil {
		t.Fatalf("Failed to add repository 1: %v", err)
	}
	_, err = mgr.AddRepository("repo2", "https://github.com/test/repo2.git", "/tmp/repo2", "user2")
	if err != nil {
		t.Fatalf("Failed to add repository 2: %v", err)
	}

	// repo1 should be current (first added)
	err = mgr.DeleteRepository("repo1")
	if err != nil {
		t.Fatalf("Failed to delete repository: %v", err)
	}

	// repo2 should now be current
	current, err := mgr.GetCurrentRepository()
	if err != nil {
		t.Fatalf("Failed to get current repository: %v", err)
	}

	if current.Name != "repo2" {
		t.Errorf("Expected repo2 to be current after deleting repo1, got '%s'", current.Name)
	}
}

func TestGetRepositoryByID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	repo, err := mgr.AddRepository("test-repo", "https://github.com/test/test.git", "/tmp/test", "testuser")
	if err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	retrieved, err := mgr.GetRepositoryByID(repo.ID)
	if err != nil {
		t.Fatalf("Failed to get repository by ID: %v", err)
	}

	if retrieved.Name != repo.Name {
		t.Errorf("Expected name '%s', got '%s'", repo.Name, retrieved.Name)
	}
}

func TestRepositoryTimestamps(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	repo, err := mgr.AddRepository("test-repo", "https://github.com/test/test.git", "/tmp/test", "testuser")
	if err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	if repo.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
	if repo.UpdatedAt.IsZero() {
		t.Error("Expected UpdatedAt to be set")
	}
}

func TestDatabasePersistence(t *testing.T) {
	tempDir := t.TempDir()

	// First database instance - store data
	db1, err := database.GetDBWithDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to create first database: %v", err)
	}

	mgr1 := NewManagerWithDB(db1)
	_, err = mgr1.AddRepository("persistent-repo", "https://github.com/test/test.git", "/tmp/test", "testuser")
	if err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}
	db1.Close()

	// Second database instance - verify data persists
	db2, err := database.GetDBWithDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to create second database: %v", err)
	}
	defer db2.Close()

	mgr2 := NewManagerWithDB(db2)
	repo, err := mgr2.GetRepositoryByName("persistent-repo")
	if err != nil {
		t.Fatalf("Failed to get repository: %v", err)
	}

	if repo.Name != "persistent-repo" {
		t.Errorf("Expected repository name 'persistent-repo', got '%s'", repo.Name)
	}
}
