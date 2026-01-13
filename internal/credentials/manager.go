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

// initializeDB creates and initializes the SQLite database
func (m *Manager) initializeDB() error {
	db, err := sql.Open("sqlite3", m.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	m.db = db

	// Create credentials table
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
		return fmt.Errorf("failed to create tables: %w", err)
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
