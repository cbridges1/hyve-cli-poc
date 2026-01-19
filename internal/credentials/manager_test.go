package credentials

import (
	"path/filepath"
	"testing"
)

// TestStoreAndGetCredentials tests storing and retrieving Git credentials
func TestStoreAndGetCredentials(t *testing.T) {
	// Create a temporary directory for test database
	tempDir := t.TempDir()

	// Create test manager
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_credentials.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Test data
	testUsername := "testuser"
	testPassword := "testpassword123"

	// Store credentials
	creds, err := mgr.StoreCredentials(testUsername, testPassword)
	if err != nil {
		t.Fatalf("Failed to store credentials: %v", err)
	}

	// Verify credentials were stored
	if creds.Username != testUsername {
		t.Errorf("Expected username %s, got %s", testUsername, creds.Username)
	}

	// Retrieve credentials
	retrieved, err := mgr.GetCredentials()
	if err != nil {
		t.Fatalf("Failed to get credentials: %v", err)
	}

	// Verify username matches
	if retrieved.Username != testUsername {
		t.Errorf("Expected username %s, got %s", testUsername, retrieved.Username)
	}

	// Verify password can be decrypted
	decryptedPassword, err := retrieved.GetPassword()
	if err != nil {
		t.Fatalf("Failed to decrypt password: %v", err)
	}

	if decryptedPassword != testPassword {
		t.Errorf("Expected password %s, got %s", testPassword, decryptedPassword)
	}
}

// TestUpdateCredentials tests updating existing credentials
func TestUpdateCredentials(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_credentials.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Store initial credentials
	_, err := mgr.StoreCredentials("user1", "pass1")
	if err != nil {
		t.Fatalf("Failed to store initial credentials: %v", err)
	}

	// Update credentials
	_, err = mgr.StoreCredentials("user2", "pass2")
	if err != nil {
		t.Fatalf("Failed to update credentials: %v", err)
	}

	// Verify updated credentials
	retrieved, err := mgr.GetCredentials()
	if err != nil {
		t.Fatalf("Failed to get credentials: %v", err)
	}

	if retrieved.Username != "user2" {
		t.Errorf("Expected username user2, got %s", retrieved.Username)
	}

	password, err := retrieved.GetPassword()
	if err != nil {
		t.Fatalf("Failed to decrypt password: %v", err)
	}

	if password != "pass2" {
		t.Errorf("Expected password pass2, got %s", password)
	}
}

// TestClearCredentials tests clearing stored credentials
func TestClearCredentials(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_credentials.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Store credentials
	_, err := mgr.StoreCredentials("testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to store credentials: %v", err)
	}

	// Verify credentials exist
	hasCredsBefore, err := mgr.HasCredentials()
	if err != nil {
		t.Fatalf("Failed to check credentials: %v", err)
	}
	if !hasCredsBefore {
		t.Error("Expected credentials to exist")
	}

	// Clear credentials
	if err := mgr.ClearCredentials(); err != nil {
		t.Fatalf("Failed to clear credentials: %v", err)
	}

	// Verify credentials are gone
	hasCredsAfter, err := mgr.HasCredentials()
	if err != nil {
		t.Fatalf("Failed to check credentials: %v", err)
	}
	if hasCredsAfter {
		t.Error("Expected credentials to be cleared")
	}
}

// TestStoreAndGetAPIToken tests API token storage and retrieval
func TestStoreAndGetAPIToken(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_credentials.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	testCases := []struct {
		provider string
		token    string
	}{
		{"civo", "test-civo-token-123"},
		{"aws", "test-aws-token-456"},
		{"gcp", "test-gcp-token-789"},
	}

	// Store tokens for different providers
	for _, tc := range testCases {
		err := mgr.StoreAPIToken(tc.provider, tc.token)
		if err != nil {
			t.Fatalf("Failed to store token for %s: %v", tc.provider, err)
		}
	}

	// Retrieve and verify each token
	for _, tc := range testCases {
		retrievedToken, err := mgr.GetAPIToken(tc.provider)
		if err != nil {
			t.Fatalf("Failed to get token for %s: %v", tc.provider, err)
		}

		if retrievedToken != tc.token {
			t.Errorf("Provider %s: expected token %s, got %s", tc.provider, tc.token, retrievedToken)
		}
	}
}

// TestUpdateAPIToken tests updating an existing API token
func TestUpdateAPIToken(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_credentials.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	provider := "civo"

	// Store initial token
	err := mgr.StoreAPIToken(provider, "old-token")
	if err != nil {
		t.Fatalf("Failed to store initial token: %v", err)
	}

	// Update token
	err = mgr.StoreAPIToken(provider, "new-token")
	if err != nil {
		t.Fatalf("Failed to update token: %v", err)
	}

	// Verify updated token
	retrievedToken, err := mgr.GetAPIToken(provider)
	if err != nil {
		t.Fatalf("Failed to get token: %v", err)
	}

	if retrievedToken != "new-token" {
		t.Errorf("Expected token new-token, got %s", retrievedToken)
	}
}

// TestClearAPIToken tests removing an API token
func TestClearAPIToken(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_credentials.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	provider := "civo"

	// Store token
	err := mgr.StoreAPIToken(provider, "test-token")
	if err != nil {
		t.Fatalf("Failed to store token: %v", err)
	}

	// Verify token exists
	hasToken, err := mgr.HasAPIToken(provider)
	if err != nil {
		t.Fatalf("Failed to check token: %v", err)
	}
	if !hasToken {
		t.Error("Expected token to exist")
	}

	// Clear token
	err = mgr.ClearAPIToken(provider)
	if err != nil {
		t.Fatalf("Failed to clear token: %v", err)
	}

	// Verify token is gone
	hasTokenAfter, err := mgr.HasAPIToken(provider)
	if err != nil {
		t.Fatalf("Failed to check token after clear: %v", err)
	}
	if hasTokenAfter {
		t.Error("Expected token to be cleared")
	}
}

// TestListAPITokens tests listing all stored tokens
func TestListAPITokens(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_credentials.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Store tokens for multiple providers
	providers := []string{"civo", "aws", "gcp"}
	for _, provider := range providers {
		err := mgr.StoreAPIToken(provider, "token-for-"+provider)
		if err != nil {
			t.Fatalf("Failed to store token for %s: %v", provider, err)
		}
	}

	// List tokens
	list, err := mgr.ListAPITokens()
	if err != nil {
		t.Fatalf("Failed to list tokens: %v", err)
	}

	// Verify count
	if len(list) != len(providers) {
		t.Errorf("Expected %d providers, got %d", len(providers), len(list))
	}

	// Verify all providers are in the list
	providerMap := make(map[string]bool)
	for _, p := range list {
		providerMap[p] = true
	}

	for _, expected := range providers {
		if !providerMap[expected] {
			t.Errorf("Expected provider %s not found in list", expected)
		}
	}
}

// TestEncryptionDecryption tests that encryption/decryption works correctly
func TestEncryptionDecryption(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_credentials.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	testPasswords := []string{
		"simple",
		"with spaces and special chars!@#$%",
		"unicode: 你好世界 🚀",
		"very-long-password-" + string(make([]byte, 1000)),
	}

	for _, password := range testPasswords {
		// Encrypt
		encrypted, err := mgr.encryptPassword(password)
		if err != nil {
			t.Fatalf("Failed to encrypt password: %v", err)
		}

		// Verify encrypted is different from original
		if encrypted == password {
			t.Error("Encrypted password should be different from original")
		}

		// Decrypt
		decrypted, err := mgr.decryptPassword(encrypted)
		if err != nil {
			t.Fatalf("Failed to decrypt password: %v", err)
		}

		// Verify decrypted matches original
		if decrypted != password {
			t.Errorf("Decrypted password doesn't match original")
		}
	}
}

// TestEmptyValues tests handling of empty values
func TestEmptyValues(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_credentials.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

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

	// Test empty API token provider
	err = mgr.StoreAPIToken("", "token")
	if err == nil {
		t.Error("Expected error for empty provider")
	}

	// Test empty API token
	err = mgr.StoreAPIToken("civo", "")
	if err == nil {
		t.Error("Expected error for empty token")
	}
}

// TestGetNonExistentToken tests retrieving a token that doesn't exist
func TestGetNonExistentToken(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_credentials.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Try to get non-existent token
	token, err := mgr.GetAPIToken("nonexistent")
	if err != nil {
		t.Fatalf("Should not error on non-existent token: %v", err)
	}

	if token != "" {
		t.Errorf("Expected empty token, got %s", token)
	}

	// Verify HasAPIToken returns false
	hasToken, err := mgr.HasAPIToken("nonexistent")
	if err != nil {
		t.Fatalf("Failed to check for token: %v", err)
	}

	if hasToken {
		t.Error("Expected HasAPIToken to return false for non-existent token")
	}
}

// TestDatabasePersistence tests that data persists across manager instances
func TestDatabasePersistence(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_credentials.db")

	// First manager - store data
	mgr1 := &Manager{dbPath: dbPath}
	if err := mgr1.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	err := mgr1.StoreAPIToken("civo", "persistent-token")
	if err != nil {
		t.Fatalf("Failed to store token: %v", err)
	}
	mgr1.Close()

	// Second manager - verify data persists
	mgr2 := &Manager{dbPath: dbPath}
	if err := mgr2.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize second database: %v", err)
	}
	defer mgr2.Close()

	token, err := mgr2.GetAPIToken("civo")
	if err != nil {
		t.Fatalf("Failed to get token from second manager: %v", err)
	}

	if token != "persistent-token" {
		t.Errorf("Expected token persistent-token, got %s", token)
	}
}

// BenchmarkEncryption benchmarks the encryption operation
func BenchmarkEncryption(b *testing.B) {
	tempDir := b.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "bench_credentials.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		b.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	password := "test-password-for-benchmarking"

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
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "bench_credentials.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		b.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	password := "test-password-for-benchmarking"
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
