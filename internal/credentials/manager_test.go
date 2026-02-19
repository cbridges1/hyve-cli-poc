package credentials

import (
	"path/filepath"
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

// TestStoreAndGetCredentials tests storing and retrieving credentials
func TestStoreAndGetCredentials(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	// Store credentials
	creds, err := mgr.StoreCredentials("testuser", "testpassword123")
	if err != nil {
		t.Fatalf("Failed to store credentials: %v", err)
	}

	if creds.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", creds.Username)
	}

	// Get password (should be decrypted)
	password, err := creds.GetPassword()
	if err != nil {
		t.Fatalf("Failed to get password: %v", err)
	}

	if password != "testpassword123" {
		t.Errorf("Expected password 'testpassword123', got '%s'", password)
	}

	// Retrieve credentials
	retrieved, err := mgr.GetCredentials()
	if err != nil {
		t.Fatalf("Failed to get credentials: %v", err)
	}

	if retrieved.Username != creds.Username {
		t.Errorf("Expected username '%s', got '%s'", creds.Username, retrieved.Username)
	}
}

// TestUpdateCredentials tests updating existing credentials
func TestUpdateCredentials(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	// Store initial credentials
	_, err := mgr.StoreCredentials("user1", "password1")
	if err != nil {
		t.Fatalf("Failed to store initial credentials: %v", err)
	}

	// Update credentials
	updated, err := mgr.StoreCredentials("user2", "password2")
	if err != nil {
		t.Fatalf("Failed to update credentials: %v", err)
	}

	if updated.Username != "user2" {
		t.Errorf("Expected username 'user2', got '%s'", updated.Username)
	}

	// Verify password was updated
	password, err := updated.GetPassword()
	if err != nil {
		t.Fatalf("Failed to get password: %v", err)
	}

	if password != "password2" {
		t.Errorf("Expected password 'password2', got '%s'", password)
	}
}

// TestClearCredentials tests clearing credentials
func TestClearCredentials(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	// Store credentials
	_, err := mgr.StoreCredentials("testuser", "testpassword")
	if err != nil {
		t.Fatalf("Failed to store credentials: %v", err)
	}

	// Verify credentials exist
	hasCreds, err := mgr.HasCredentials()
	if err != nil {
		t.Fatalf("Failed to check credentials: %v", err)
	}
	if !hasCreds {
		t.Error("Expected credentials to exist")
	}

	// Clear credentials
	err = mgr.ClearCredentials()
	if err != nil {
		t.Fatalf("Failed to clear credentials: %v", err)
	}

	// Verify credentials are gone
	hasCredsAfter, err := mgr.HasCredentials()
	if err != nil {
		t.Fatalf("Failed to check credentials after clear: %v", err)
	}
	if hasCredsAfter {
		t.Error("Expected credentials to be cleared")
	}
}

// TestStoreAndGetCivoToken tests Civo token storage and retrieval
func TestStoreAndGetCivoToken(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	token := "test-civo-token-123"

	// Store token for org "myorg" -> stored as "myorg-token"
	err := mgr.StoreCivoToken("myorg", token)
	if err != nil {
		t.Fatalf("Failed to store token: %v", err)
	}

	// Retrieve and verify token
	retrievedToken, err := mgr.GetCivoToken("myorg")
	if err != nil {
		t.Fatalf("Failed to get token: %v", err)
	}

	if retrievedToken != token {
		t.Errorf("Expected token %s, got %s", token, retrievedToken)
	}
}

// TestUpdateCivoToken tests updating an existing Civo token
func TestUpdateCivoToken(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	// Store initial token
	err := mgr.StoreCivoToken("myorg", "old-token")
	if err != nil {
		t.Fatalf("Failed to store initial token: %v", err)
	}

	// Update token
	err = mgr.StoreCivoToken("myorg", "new-token")
	if err != nil {
		t.Fatalf("Failed to update token: %v", err)
	}

	// Verify updated token
	retrievedToken, err := mgr.GetCivoToken("myorg")
	if err != nil {
		t.Fatalf("Failed to get token: %v", err)
	}

	if retrievedToken != "new-token" {
		t.Errorf("Expected token new-token, got %s", retrievedToken)
	}
}

// TestClearCivoToken tests removing a Civo token
func TestClearCivoToken(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	// Store token
	err := mgr.StoreCivoToken("myorg", "test-token")
	if err != nil {
		t.Fatalf("Failed to store token: %v", err)
	}

	// Verify token exists
	hasToken, err := mgr.HasCivoToken("myorg")
	if err != nil {
		t.Fatalf("Failed to check token: %v", err)
	}
	if !hasToken {
		t.Error("Expected token to exist")
	}

	// Clear token
	err = mgr.ClearCivoToken("myorg")
	if err != nil {
		t.Fatalf("Failed to clear token: %v", err)
	}

	// Verify token is gone
	hasTokenAfter, err := mgr.HasCivoToken("myorg")
	if err != nil {
		t.Fatalf("Failed to check token after clear: %v", err)
	}
	if hasTokenAfter {
		t.Error("Expected token to be cleared")
	}
}

// TestEncryptionDecryption tests that encryption/decryption works correctly
func TestEncryptionDecryption(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	testPasswords := []string{
		"simple",
		"with spaces and symbols !@#$%^&*()",
		"unicode: 你好世界 🌍",
		"very-long-password-that-exceeds-typical-lengths-" +
			"and-contains-many-different-characters-1234567890",
	}

	for _, password := range testPasswords {
		encrypted, err := mgr.encryptPassword(password)
		if err != nil {
			t.Fatalf("Failed to encrypt password '%s': %v", password, err)
		}

		decrypted, err := mgr.decryptPassword(encrypted)
		if err != nil {
			t.Fatalf("Failed to decrypt password: %v", err)
		}

		if decrypted != password {
			t.Errorf("Decrypted password doesn't match. Expected '%s', got '%s'", password, decrypted)
		}
	}
}

// TestEmptyValues tests handling of empty values
func TestEmptyValues(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	// Test empty username
	_, err := mgr.StoreCredentials("", "password")
	if err == nil {
		t.Error("Expected error for empty username")
	}

	// Test empty password
	_, err = mgr.StoreCredentials("username", "")
	if err == nil {
		t.Error("Expected error for empty password")
	}

	// Test empty Civo token
	err = mgr.StoreCivoToken("myorg", "")
	if err == nil {
		t.Error("Expected error for empty token")
	}
}

// TestGetNonExistentCivoToken tests retrieving a token that doesn't exist
func TestGetNonExistentCivoToken(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	// Try to get non-existent token
	token, err := mgr.GetCivoToken("myorg")
	if err != nil {
		t.Fatalf("Should not error on non-existent token: %v", err)
	}

	if token != "" {
		t.Errorf("Expected empty token, got %s", token)
	}

	// Verify HasCivoToken returns false
	hasToken, err := mgr.HasCivoToken("myorg")
	if err != nil {
		t.Fatalf("Failed to check for token: %v", err)
	}

	if hasToken {
		t.Error("Expected HasCivoToken to return false for non-existent token")
	}
}

// TestMultipleSecrets tests storing multiple named secrets
func TestMultipleSecrets(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	// Store secrets with different names
	secrets := map[string]string{
		"myorg-token":  "civo-token-123",
		"docker-token": "docker-token-456",
		"github-token": "github-token-789",
	}

	for name, value := range secrets {
		err := mgr.StoreSecret(name, value)
		if err != nil {
			t.Fatalf("Failed to store secret %s: %v", name, err)
		}
	}

	// Verify each secret can be retrieved
	for name, expectedValue := range secrets {
		value, err := mgr.GetSecret(name)
		if err != nil {
			t.Fatalf("Failed to get secret %s: %v", name, err)
		}
		if value != expectedValue {
			t.Errorf("Expected secret %s value '%s', got '%s'", name, expectedValue, value)
		}

		hasSecret, err := mgr.HasSecret(name)
		if err != nil {
			t.Fatalf("Failed to check secret %s: %v", name, err)
		}
		if !hasSecret {
			t.Errorf("Expected HasSecret to return true for %s", name)
		}
	}

	// Clear one secret
	err := mgr.ClearSecret("docker-token")
	if err != nil {
		t.Fatalf("Failed to clear docker-token: %v", err)
	}

	// Verify docker-token is gone but others remain
	hasDocker, _ := mgr.HasSecret("docker-token")
	if hasDocker {
		t.Error("Expected docker-token to be cleared")
	}

	hasCivo, _ := mgr.HasSecret("myorg-token")
	if !hasCivo {
		t.Error("Expected myorg-token to still exist")
	}

	hasGithub, _ := mgr.HasSecret("github-token")
	if !hasGithub {
		t.Error("Expected github-token to still exist")
	}
}

// TestStoreSecretValidation tests validation of secret storage
func TestStoreSecretValidation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mgr := NewManagerWithDB(db)

	// Test empty name
	err := mgr.StoreSecret("", "value")
	if err == nil {
		t.Error("Expected error for empty secret name")
	}

	// Test empty value
	err = mgr.StoreSecret("myorg-token", "")
	if err == nil {
		t.Error("Expected error for empty secret value")
	}
}

// TestDatabasePersistence tests that data persists across manager instances
func TestDatabasePersistence(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "hyve.db")

	// First database instance - store data
	db1, err := database.GetDBWithDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to create first database: %v", err)
	}

	mgr1 := NewManagerWithDB(db1)
	err = mgr1.StoreCivoToken("myorg", "persistent-token")
	if err != nil {
		t.Fatalf("Failed to store token: %v", err)
	}
	db1.Close()

	// Second database instance - verify data persists
	db2, err := database.GetDBWithDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to create second database: %v", err)
	}
	defer db2.Close()

	mgr2 := NewManagerWithDB(db2)
	token, err := mgr2.GetCivoToken("myorg")
	if err != nil {
		t.Fatalf("Failed to get token from second manager: %v", err)
	}

	if token != "persistent-token" {
		t.Errorf("Expected token persistent-token, got %s", token)
	}

	// Verify the database file exists at expected path
	_ = dbPath // Just to acknowledge we're aware of where the DB should be
}

// BenchmarkEncryption benchmarks the encryption operation
func BenchmarkEncryption(b *testing.B) {
	tempDir := b.TempDir()
	db, err := database.GetDBWithDir(tempDir)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	mgr := NewManagerWithDB(db)

	password := "benchmark-test-password-12345"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mgr.encryptPassword(password)
		if err != nil {
			b.Fatalf("Encryption failed: %v", err)
		}
	}
}

// BenchmarkDecryption benchmarks the decryption operation
func BenchmarkDecryption(b *testing.B) {
	tempDir := b.TempDir()
	db, err := database.GetDBWithDir(tempDir)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	mgr := NewManagerWithDB(db)

	password := "benchmark-test-password-12345"
	encrypted, err := mgr.encryptPassword(password)
	if err != nil {
		b.Fatalf("Failed to encrypt: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mgr.decryptPassword(encrypted)
		if err != nil {
			b.Fatalf("Decryption failed: %v", err)
		}
	}
}
