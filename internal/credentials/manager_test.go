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

// TestStoreAndGetCivoToken tests Civo token storage and retrieval
func TestStoreAndGetCivoToken(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_credentials.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	testCases := []struct {
		accountID string
		token     string
	}{
		{"default", "test-civo-token-123"},
		{"production", "test-prod-token-456"},
		{"staging", "test-staging-token-789"},
	}

	// Store tokens for different accounts
	for _, tc := range testCases {
		err := mgr.StoreCivoToken(tc.accountID, tc.token)
		if err != nil {
			t.Fatalf("Failed to store token for %s: %v", tc.accountID, err)
		}
	}

	// Retrieve and verify each token
	for _, tc := range testCases {
		retrievedToken, err := mgr.GetCivoToken(tc.accountID)
		if err != nil {
			t.Fatalf("Failed to get token for %s: %v", tc.accountID, err)
		}

		if retrievedToken != tc.token {
			t.Errorf("Account %s: expected token %s, got %s", tc.accountID, tc.token, retrievedToken)
		}
	}
}

// TestUpdateCivoToken tests updating an existing Civo token
func TestUpdateCivoToken(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_credentials.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	accountID := "default"

	// Store initial token
	err := mgr.StoreCivoToken(accountID, "old-token")
	if err != nil {
		t.Fatalf("Failed to store initial token: %v", err)
	}

	// Update token
	err = mgr.StoreCivoToken(accountID, "new-token")
	if err != nil {
		t.Fatalf("Failed to update token: %v", err)
	}

	// Verify updated token
	retrievedToken, err := mgr.GetCivoToken(accountID)
	if err != nil {
		t.Fatalf("Failed to get token: %v", err)
	}

	if retrievedToken != "new-token" {
		t.Errorf("Expected token new-token, got %s", retrievedToken)
	}
}

// TestClearCivoToken tests removing a Civo token
func TestClearCivoToken(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_credentials.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	accountID := "default"

	// Store token
	err := mgr.StoreCivoToken(accountID, "test-token")
	if err != nil {
		t.Fatalf("Failed to store token: %v", err)
	}

	// Verify token exists
	hasToken, err := mgr.HasCivoToken(accountID)
	if err != nil {
		t.Fatalf("Failed to check token: %v", err)
	}
	if !hasToken {
		t.Error("Expected token to exist")
	}

	// Clear token
	err = mgr.ClearCivoToken(accountID)
	if err != nil {
		t.Fatalf("Failed to clear token: %v", err)
	}

	// Verify token is gone
	hasTokenAfter, err := mgr.HasCivoToken(accountID)
	if err != nil {
		t.Fatalf("Failed to check token after clear: %v", err)
	}
	if hasTokenAfter {
		t.Error("Expected token to be cleared")
	}
}

// TestListCivoAccounts tests listing all stored Civo accounts
func TestListCivoAccounts(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_credentials.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Store tokens for multiple accounts
	accounts := []string{"default", "production", "staging"}
	for _, account := range accounts {
		err := mgr.StoreCivoToken(account, "token-for-"+account)
		if err != nil {
			t.Fatalf("Failed to store token for %s: %v", account, err)
		}
	}

	// List accounts
	list, err := mgr.ListCivoAccounts()
	if err != nil {
		t.Fatalf("Failed to list accounts: %v", err)
	}

	// Verify count
	if len(list) != len(accounts) {
		t.Errorf("Expected %d accounts, got %d", len(accounts), len(list))
	}

	// Verify all accounts are in the list
	accountMap := make(map[string]bool)
	for _, a := range list {
		accountMap[a] = true
	}

	for _, expected := range accounts {
		if !accountMap[expected] {
			t.Errorf("Expected account %s not found in list", expected)
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

	// Test empty Civo account_id
	err = mgr.StoreCivoToken("", "token")
	if err == nil {
		t.Error("Expected error for empty account_id")
	}

	// Test empty Civo token
	err = mgr.StoreCivoToken("default", "")
	if err == nil {
		t.Error("Expected error for empty token")
	}
}

// TestGetNonExistentCivoToken tests retrieving a token that doesn't exist
func TestGetNonExistentCivoToken(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		dbPath: filepath.Join(tempDir, "test_credentials.db"),
	}

	if err := mgr.initializeDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer mgr.Close()

	// Try to get non-existent token
	token, err := mgr.GetCivoToken("nonexistent")
	if err != nil {
		t.Fatalf("Should not error on non-existent token: %v", err)
	}

	if token != "" {
		t.Errorf("Expected empty token, got %s", token)
	}

	// Verify HasCivoToken returns false
	hasToken, err := mgr.HasCivoToken("nonexistent")
	if err != nil {
		t.Fatalf("Failed to check for token: %v", err)
	}

	if hasToken {
		t.Error("Expected HasCivoToken to return false for non-existent token")
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

	err := mgr1.StoreCivoToken("default", "persistent-token")
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

	token, err := mgr2.GetCivoToken("default")
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
