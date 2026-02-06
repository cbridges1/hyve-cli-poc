package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Credentials represents stored Git credentials
type Credentials struct {
	ID                int       `json:"id"`
	Username          string    `json:"username"`
	EncryptedPassword string    `json:"-"` // Not serialized for security
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	manager           *Manager  `json:"-"` // Reference to manager for decryption
}

// GetPassword returns the decrypted password for these credentials
func (c *Credentials) GetPassword() (string, error) {
	if c.manager == nil {
		return "", fmt.Errorf("credentials manager not available for password decryption")
	}
	return c.manager.decryptPassword(c.EncryptedPassword)
}

// Manager handles global Git credentials using SQLite
type Manager struct {
	dbPath string
	db     *sql.DB
}

// NewManager creates a new credentials manager
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	configDir := filepath.Join(homeDir, ".hyve")
	dbPath := filepath.Join(configDir, "credentials.db")

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	mgr := &Manager{
		dbPath: dbPath,
	}

	if err := mgr.initializeDB(); err != nil {
		return nil, err
	}

	return mgr, nil
}

// CivoAccount represents a stored Civo account
type CivoAccount struct {
	AccountID      string    `json:"account_id"`
	EncryptedToken string    `json:"-"` // Not serialized for security
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	manager        *Manager  `json:"-"` // Reference to manager for decryption
}

// GetToken returns the decrypted token for this account
func (c *CivoAccount) GetToken() (string, error) {
	if c.manager == nil {
		return "", fmt.Errorf("credentials manager not available for token decryption")
	}
	return c.manager.decryptPassword(c.EncryptedToken)
}

// initializeDB creates and initializes the SQLite database
func (m *Manager) initializeDB() error {
	db, err := sql.Open("sqlite3", m.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	m.db = db

	// Create credentials table (for Git credentials)
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS credentials (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL,
		encrypted_password TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("failed to create credentials table: %w", err)
	}

	// Create Civo accounts table with account_id as primary key
	createCivoAccountsSQL := `
	CREATE TABLE IF NOT EXISTS civo_accounts (
		account_id TEXT PRIMARY KEY,
		encrypted_token TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := db.Exec(createCivoAccountsSQL); err != nil {
		return fmt.Errorf("failed to create civo_accounts table: %w", err)
	}

	// Migrate from old api_tokens table if it exists
	if err := m.migrateFromAPITokens(); err != nil {
		// Log but don't fail - migration is best-effort
		fmt.Printf("Note: Could not migrate from api_tokens: %v\n", err)
	}

	return nil
}

// migrateFromAPITokens migrates data from the old api_tokens table to civo_accounts
func (m *Manager) migrateFromAPITokens() error {
	// Check if old table exists
	var tableName string
	err := m.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='api_tokens'").Scan(&tableName)
	if err != nil {
		// Table doesn't exist, nothing to migrate
		return nil
	}

	// Check if we have any Civo tokens in the old table
	var encryptedToken string
	err = m.db.QueryRow("SELECT encrypted_token FROM api_tokens WHERE provider = 'civo'").Scan(&encryptedToken)
	if err != nil {
		// No Civo token found, nothing to migrate
		return nil
	}

	// Insert into new table with default account_id
	insertSQL := `
	INSERT OR IGNORE INTO civo_accounts (account_id, encrypted_token)
	VALUES ('default', ?)
	`
	_, err = m.db.Exec(insertSQL, encryptedToken)
	if err != nil {
		return fmt.Errorf("failed to migrate Civo token: %w", err)
	}

	return nil
}

// Close closes the database connection
func (m *Manager) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// StoreCredentials stores or updates global Git credentials
func (m *Manager) StoreCredentials(username, password string) (*Credentials, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("username and password are required")
	}

	// Encrypt the password
	encryptedPassword, err := m.encryptPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt password: %w", err)
	}

	// Check if credentials already exist
	existing, _ := m.GetCredentials()
	if existing != nil {
		// Update existing credentials
		updateSQL := `
		UPDATE credentials 
		SET username = ?, encrypted_password = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
		`
		_, err := m.db.Exec(updateSQL, username, encryptedPassword, existing.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to update credentials: %w", err)
		}
	} else {
		// Insert new credentials
		insertSQL := `
		INSERT INTO credentials (username, encrypted_password)
		VALUES (?, ?)
		`
		_, err := m.db.Exec(insertSQL, username, encryptedPassword)
		if err != nil {
			return nil, fmt.Errorf("failed to insert credentials: %w", err)
		}
	}

	return m.GetCredentials()
}

// GetCredentials retrieves the stored Git credentials
func (m *Manager) GetCredentials() (*Credentials, error) {
	selectSQL := `
	SELECT id, username, encrypted_password, created_at, updated_at
	FROM credentials
	ORDER BY updated_at DESC
	LIMIT 1
	`

	creds := &Credentials{}
	var createdAt, updatedAt string

	err := m.db.QueryRow(selectSQL).Scan(&creds.ID, &creds.Username,
		&creds.EncryptedPassword, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No credentials stored
		}
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}

	// Parse timestamps
	if creds.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt); err != nil {
		creds.CreatedAt = time.Now()
	}
	if creds.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt); err != nil {
		creds.UpdatedAt = time.Now()
	}

	// Set manager reference for password decryption
	creds.manager = m
	return creds, nil
}

// HasCredentials checks if any credentials are stored
func (m *Manager) HasCredentials() (bool, error) {
	creds, err := m.GetCredentials()
	if err != nil {
		return false, err
	}
	return creds != nil, nil
}

// ClearCredentials removes all stored credentials
func (m *Manager) ClearCredentials() error {
	deleteSQL := `DELETE FROM credentials`
	_, err := m.db.Exec(deleteSQL)
	if err != nil {
		return fmt.Errorf("failed to clear credentials: %w", err)
	}
	return nil
}

// getEncryptionKey generates a deterministic encryption key based on database path
func (m *Manager) getEncryptionKey() []byte {
	// Use database path for key derivation
	// Note: Hostname is intentionally excluded to make the database portable across machines
	keyMaterial := m.dbPath
	hash := sha256.Sum256([]byte(keyMaterial))
	return hash[:]
}

// getEncryptionKeyWithHostname generates the old encryption key that included hostname
// This is used for migrating data encrypted with the old key format
func (m *Manager) getEncryptionKeyWithHostname(hostname string) []byte {
	keyMaterial := fmt.Sprintf("%s:%s", m.dbPath, hostname)
	hash := sha256.Sum256([]byte(keyMaterial))
	return hash[:]
}

// encryptPassword encrypts a password using AES-GCM
func (m *Manager) encryptPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}

	key := m.getEncryptionKey()

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(password), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptPassword decrypts a password using AES-GCM
func (m *Manager) decryptPassword(encryptedPassword string) (string, error) {
	if encryptedPassword == "" {
		return "", nil
	}

	key := m.getEncryptionKey()

	ciphertext, err := base64.StdEncoding.DecodeString(encryptedPassword)
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted password: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt password: %w", err)
	}

	return string(plaintext), nil
}

// decryptPasswordWithHostname decrypts a password using the old hostname-based key
func (m *Manager) decryptPasswordWithHostname(encryptedPassword string, hostname string) (string, error) {
	if encryptedPassword == "" {
		return "", nil
	}

	key := m.getEncryptionKeyWithHostname(hostname)

	ciphertext, err := base64.StdEncoding.DecodeString(encryptedPassword)
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted password: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt password with hostname: %w", err)
	}

	return string(plaintext), nil
}

// MigrateEncryption migrates credentials from hostname-based encryption to portable encryption
func (m *Manager) MigrateEncryption(oldHostname string) error {
	// Get current credentials
	creds, err := m.GetCredentials()
	if err != nil {
		return fmt.Errorf("no credentials found to migrate: %w", err)
	}

	// Decrypt with old hostname-based key
	plainPassword, err := m.decryptPasswordWithHostname(creds.EncryptedPassword, oldHostname)
	if err != nil {
		return fmt.Errorf("failed to decrypt password: %w", err)
	}

	// Re-encrypt with new portable key
	newEncrypted, err := m.encryptPassword(plainPassword)
	if err != nil {
		return fmt.Errorf("failed to re-encrypt password: %w", err)
	}

	// Update the database
	updateSQL := `
	UPDATE credentials
	SET encrypted_password = ?, updated_at = CURRENT_TIMESTAMP
	WHERE id = ?
	`
	_, err = m.db.Exec(updateSQL, newEncrypted, creds.ID)
	if err != nil {
		return fmt.Errorf("failed to update credentials: %w", err)
	}

	return nil
}

// StoreCivoToken stores or updates a Civo API token for an account
func (m *Manager) StoreCivoToken(accountID, token string) error {
	if accountID == "" || token == "" {
		return fmt.Errorf("account_id and token are required")
	}

	// Encrypt the token
	encryptedToken, err := m.encryptPassword(token)
	if err != nil {
		return fmt.Errorf("failed to encrypt token: %w", err)
	}

	// Use INSERT OR REPLACE to handle both insert and update
	upsertSQL := `
	INSERT INTO civo_accounts (account_id, encrypted_token, created_at, updated_at)
	VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	ON CONFLICT(account_id) DO UPDATE SET
		encrypted_token = excluded.encrypted_token,
		updated_at = CURRENT_TIMESTAMP
	`
	_, err = m.db.Exec(upsertSQL, accountID, encryptedToken)
	if err != nil {
		return fmt.Errorf("failed to store Civo token: %w", err)
	}

	return nil
}

// GetCivoToken retrieves and decrypts a Civo API token for an account
func (m *Manager) GetCivoToken(accountID string) (string, error) {
	selectSQL := `
	SELECT encrypted_token
	FROM civo_accounts
	WHERE account_id = ?
	`

	var encryptedToken string
	err := m.db.QueryRow(selectSQL, accountID).Scan(&encryptedToken)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // No token stored
		}
		return "", fmt.Errorf("failed to get Civo token: %w", err)
	}

	// Decrypt the token
	token, err := m.decryptPassword(encryptedToken)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt token: %w", err)
	}

	return token, nil
}

// GetCivoAccount retrieves a Civo account by account_id
func (m *Manager) GetCivoAccount(accountID string) (*CivoAccount, error) {
	selectSQL := `
	SELECT account_id, encrypted_token, created_at, updated_at
	FROM civo_accounts
	WHERE account_id = ?
	`

	account := &CivoAccount{}
	var createdAt, updatedAt string

	err := m.db.QueryRow(selectSQL, accountID).Scan(&account.AccountID, &account.EncryptedToken, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No account found
		}
		return nil, fmt.Errorf("failed to get Civo account: %w", err)
	}

	// Parse timestamps
	if account.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt); err != nil {
		account.CreatedAt = time.Now()
	}
	if account.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt); err != nil {
		account.UpdatedAt = time.Now()
	}

	// Set manager reference for token decryption
	account.manager = m
	return account, nil
}

// HasCivoToken checks if a token is stored for the given account
func (m *Manager) HasCivoToken(accountID string) (bool, error) {
	token, err := m.GetCivoToken(accountID)
	if err != nil {
		return false, err
	}
	return token != "", nil
}

// ClearCivoToken removes the stored Civo API token for an account
func (m *Manager) ClearCivoToken(accountID string) error {
	deleteSQL := `DELETE FROM civo_accounts WHERE account_id = ?`
	_, err := m.db.Exec(deleteSQL, accountID)
	if err != nil {
		return fmt.Errorf("failed to clear Civo token: %w", err)
	}
	return nil
}

// ListCivoAccounts returns a list of Civo account IDs that have tokens stored
func (m *Manager) ListCivoAccounts() ([]string, error) {
	selectSQL := `
	SELECT account_id
	FROM civo_accounts
	ORDER BY account_id
	`

	rows, err := m.db.Query(selectSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to list Civo accounts: %w", err)
	}
	defer rows.Close()

	var accounts []string
	for rows.Next() {
		var accountID string
		if err := rows.Scan(&accountID); err != nil {
			return nil, fmt.Errorf("failed to scan account_id: %w", err)
		}
		accounts = append(accounts, accountID)
	}

	return accounts, nil
}

// GetDefaultCivoToken returns the token for the "default" Civo account
// This is used for backward compatibility and environment variable fallback
func (m *Manager) GetDefaultCivoToken() (string, error) {
	return m.GetCivoToken("default")
}
