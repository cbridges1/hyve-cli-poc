package config

import (
	"os"
	"path/filepath"
	"testing"
)

func setupConfigTest(t *testing.T) (*Manager, string, func()) {
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Override home directory for testing
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)

	manager := NewManager()

	cleanup := func() {
		os.Setenv("HOME", origHome)
		os.RemoveAll(tmpDir)
	}

	return manager, tmpDir, cleanup
}

func TestNewManager(t *testing.T) {
	manager, tmpDir, cleanup := setupConfigTest(t)
	defer cleanup()

	if manager == nil {
		t.Fatal("Expected manager to be created, got nil")
	}

	expectedPath := filepath.Join(tmpDir, ".hyve", "config.yaml")
	if manager.configPath != expectedPath {
		t.Errorf("Expected config path %s, got %s", expectedPath, manager.configPath)
	}

	if manager.config == nil {
		t.Error("Expected config to be initialized")
	}
}

func TestLoadConfig_NotExists(t *testing.T) {
	manager, _, cleanup := setupConfigTest(t)
	defer cleanup()

	err := manager.LoadConfig()
	if err != nil {
		t.Fatalf("Expected no error when loading non-existent config, got: %v", err)
	}

	// Should use default values
	if manager.config.Git.LocalPath != ".hyve-state" {
		t.Errorf("Expected default local path '.hyve-state', got %s", manager.config.Git.LocalPath)
	}
}

func TestLoadConfig_EnvironmentVariable(t *testing.T) {
	manager, _, cleanup := setupConfigTest(t)
	defer cleanup()

	// Create a config file first
	gitConfig := GitConfig{
		RepoURL:   "https://github.com/test/repo.git",
		LocalPath: "/tmp/test-repo",
		Username:  "testuser",
	}
	manager.SetGitConfig(gitConfig)

	// Save config
	if err := manager.SaveConfig(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Set environment variable
	testToken := "test-token-from-env"
	os.Setenv("HYVE_GIT_TOKEN", testToken)
	defer os.Unsetenv("HYVE_GIT_TOKEN")

	// Create new manager and load
	manager2 := NewManager()
	err := manager2.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if manager2.config.Git.Token != testToken {
		t.Errorf("Expected token from environment %s, got %s", testToken, manager2.config.Git.Token)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	manager, _, cleanup := setupConfigTest(t)
	defer cleanup()

	// Set configuration
	gitConfig := GitConfig{
		RepoURL:   "https://github.com/test/repo.git",
		LocalPath: "/tmp/test-repo",
		Username:  "testuser",
		Token:     "test-token", // Should not be saved to file
	}
	manager.SetGitConfig(gitConfig)

	// Save configuration
	err := manager.SaveConfig()
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Create new manager and load
	manager2 := NewManager()
	err = manager2.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify values (except token which shouldn't be saved)
	if manager2.config.Git.RepoURL != gitConfig.RepoURL {
		t.Errorf("Expected RepoURL %s, got %s", gitConfig.RepoURL, manager2.config.Git.RepoURL)
	}

	if manager2.config.Git.LocalPath != gitConfig.LocalPath {
		t.Errorf("Expected LocalPath %s, got %s", gitConfig.LocalPath, manager2.config.Git.LocalPath)
	}

	if manager2.config.Git.Username != gitConfig.Username {
		t.Errorf("Expected Username %s, got %s", gitConfig.Username, manager2.config.Git.Username)
	}

	// Token should not be saved to file
	if manager2.config.Git.Token != "" {
		t.Error("Expected token to not be saved to file")
	}
}

func TestGetGitConfig(t *testing.T) {
	manager, _, cleanup := setupConfigTest(t)
	defer cleanup()

	gitConfig := GitConfig{
		RepoURL:   "https://github.com/test/repo.git",
		LocalPath: "/tmp/test-repo",
		Username:  "testuser",
		Token:     "test-token",
	}
	manager.SetGitConfig(gitConfig)

	retrieved := manager.GetGitConfig()

	if retrieved.RepoURL != gitConfig.RepoURL {
		t.Errorf("Expected RepoURL %s, got %s", gitConfig.RepoURL, retrieved.RepoURL)
	}

	if retrieved.LocalPath != gitConfig.LocalPath {
		t.Errorf("Expected LocalPath %s, got %s", gitConfig.LocalPath, retrieved.LocalPath)
	}

	if retrieved.Username != gitConfig.Username {
		t.Errorf("Expected Username %s, got %s", gitConfig.Username, retrieved.Username)
	}

	if retrieved.Token != gitConfig.Token {
		t.Errorf("Expected Token %s, got %s", gitConfig.Token, retrieved.Token)
	}
}

func TestSetGitConfig(t *testing.T) {
	manager, _, cleanup := setupConfigTest(t)
	defer cleanup()

	gitConfig := GitConfig{
		RepoURL:   "https://github.com/test/new-repo.git",
		LocalPath: "/tmp/new-repo",
		Username:  "newuser",
		Token:     "new-token",
	}

	manager.SetGitConfig(gitConfig)

	if manager.config.Git.RepoURL != gitConfig.RepoURL {
		t.Errorf("Expected RepoURL %s, got %s", gitConfig.RepoURL, manager.config.Git.RepoURL)
	}

	if manager.config.Git.LocalPath != gitConfig.LocalPath {
		t.Errorf("Expected LocalPath %s, got %s", gitConfig.LocalPath, manager.config.Git.LocalPath)
	}

	if manager.config.Git.Username != gitConfig.Username {
		t.Errorf("Expected Username %s, got %s", gitConfig.Username, manager.config.Git.Username)
	}

	if manager.config.Git.Token != gitConfig.Token {
		t.Errorf("Expected Token %s, got %s", gitConfig.Token, manager.config.Git.Token)
	}
}

func TestIsGitConfigured_True(t *testing.T) {
	manager, _, cleanup := setupConfigTest(t)
	defer cleanup()

	gitConfig := GitConfig{
		RepoURL: "https://github.com/test/repo.git",
	}
	manager.SetGitConfig(gitConfig)

	if !manager.IsGitConfigured() {
		t.Error("Expected IsGitConfigured to return true")
	}
}

func TestIsGitConfigured_False(t *testing.T) {
	manager, _, cleanup := setupConfigTest(t)
	defer cleanup()

	// Empty RepoURL
	gitConfig := GitConfig{
		RepoURL: "",
	}
	manager.SetGitConfig(gitConfig)

	if manager.IsGitConfigured() {
		t.Error("Expected IsGitConfigured to return false")
	}
}

func TestGetCivoToken_EnvironmentVariable(t *testing.T) {
	manager, _, cleanup := setupConfigTest(t)
	defer cleanup()

	testToken := "test-civo-token"
	os.Setenv("CIVO_TOKEN", testToken)
	defer os.Unsetenv("CIVO_TOKEN")

	token := manager.GetCivoToken()
	if token != testToken {
		t.Errorf("Expected token %s, got %s", testToken, token)
	}
}

func TestGetCivoToken_NoToken(t *testing.T) {
	manager, _, cleanup := setupConfigTest(t)
	defer cleanup()

	// Ensure no token in environment
	os.Unsetenv("CIVO_TOKEN")

	token := manager.GetCivoToken()
	if token != "" {
		t.Errorf("Expected empty token, got %s", token)
	}
}

func TestSaveConfig_CreatesDirectory(t *testing.T) {
	manager, tmpDir, cleanup := setupConfigTest(t)
	defer cleanup()

	// Ensure directory doesn't exist
	configDir := filepath.Join(tmpDir, ".hyve")
	os.RemoveAll(configDir)

	gitConfig := GitConfig{
		RepoURL:  "https://github.com/test/repo.git",
		Username: "testuser",
	}
	manager.SetGitConfig(gitConfig)

	err := manager.SaveConfig()
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Error("Expected config directory to be created")
	}
}

func TestSaveConfig_FilePermissions(t *testing.T) {
	manager, _, cleanup := setupConfigTest(t)
	defer cleanup()

	gitConfig := GitConfig{
		RepoURL:  "https://github.com/test/repo.git",
		Username: "testuser",
	}
	manager.SetGitConfig(gitConfig)

	err := manager.SaveConfig()
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Check file permissions
	info, err := os.Stat(manager.configPath)
	if err != nil {
		t.Fatalf("Failed to stat config file: %v", err)
	}

	expectedPerms := os.FileMode(0600)
	if info.Mode().Perm() != expectedPerms {
		t.Errorf("Expected file permissions %v, got %v", expectedPerms, info.Mode().Perm())
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	manager, _, cleanup := setupConfigTest(t)
	defer cleanup()

	// Create config directory
	configDir := filepath.Dir(manager.configPath)
	os.MkdirAll(configDir, 0755)

	// Write invalid YAML
	invalidYAML := "this is not valid: yaml: content"
	os.WriteFile(manager.configPath, []byte(invalidYAML), 0644)

	err := manager.LoadConfig()
	if err == nil {
		t.Error("Expected error when loading invalid YAML, got nil")
	}
}

func TestConfigRoundTrip(t *testing.T) {
	manager, _, cleanup := setupConfigTest(t)
	defer cleanup()

	// Set original config
	original := GitConfig{
		RepoURL:   "https://github.com/original/repo.git",
		LocalPath: "/tmp/original-path",
		Username:  "originaluser",
	}
	manager.SetGitConfig(original)

	// Save
	if err := manager.SaveConfig(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Create new manager and load
	manager2 := NewManager()
	if err := manager2.LoadConfig(); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	retrieved := manager2.GetGitConfig()

	// Verify all fields match (except token)
	if retrieved.RepoURL != original.RepoURL {
		t.Errorf("RepoURL mismatch: expected %s, got %s", original.RepoURL, retrieved.RepoURL)
	}

	if retrieved.LocalPath != original.LocalPath {
		t.Errorf("LocalPath mismatch: expected %s, got %s", original.LocalPath, retrieved.LocalPath)
	}

	if retrieved.Username != original.Username {
		t.Errorf("Username mismatch: expected %s, got %s", original.Username, retrieved.Username)
	}
}
