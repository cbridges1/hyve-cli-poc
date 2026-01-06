package kubeconfig

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
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Kubeconfig represents stored kubeconfig data
type Kubeconfig struct {
	ID              int       `json:"id"`
	ClusterName     string    `json:"cluster_name"`
	RepositoryName  string    `json:"repository_name"`
	EncryptedConfig string    `json:"-"` // Not serialized for security
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	manager         *Manager  `json:"-"` // Reference to manager for decryption
}

// GetConfig returns the decrypted kubeconfig for this cluster
func (k *Kubeconfig) GetConfig() (string, error) {
	if k.manager == nil {
		return "", fmt.Errorf("kubeconfig manager not available for config decryption")
	}
	return k.manager.decryptConfig(k.EncryptedConfig)
}

// Manager handles kubeconfig storage using SQLite with encryption
type Manager struct {
	dbPath         string
	db             *sql.DB
	repositoryName string
}

// NewManager creates a new kubeconfig manager for a specific repository
func NewManager(repositoryName string) (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	configDir := filepath.Join(homeDir, ".hyve")
	dbPath := filepath.Join(configDir, "kubeconfigs.db")

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	mgr := &Manager{
		dbPath:         dbPath,
		repositoryName: repositoryName,
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

	// Create kubeconfigs table
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS kubeconfigs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		cluster_name TEXT NOT NULL,
		repository_name TEXT NOT NULL,
		encrypted_config TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(cluster_name, repository_name)
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

// StoreKubeconfig stores or updates a kubeconfig for a cluster
func (m *Manager) StoreKubeconfig(clusterName, kubeconfig string) (*Kubeconfig, error) {
	if clusterName == "" || kubeconfig == "" {
		return nil, fmt.Errorf("cluster name and kubeconfig are required")
	}

	// Encrypt the kubeconfig
	encryptedConfig, err := m.encryptConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt kubeconfig: %w", err)
	}

	// Check if kubeconfig already exists
	existing, _ := m.GetKubeconfig(clusterName)
	if existing != nil {
		// Update existing kubeconfig
		updateSQL := `
		UPDATE kubeconfigs 
		SET encrypted_config = ?, updated_at = CURRENT_TIMESTAMP
		WHERE cluster_name = ? AND repository_name = ?
		`
		_, err := m.db.Exec(updateSQL, encryptedConfig, clusterName, m.repositoryName)
		if err != nil {
			return nil, fmt.Errorf("failed to update kubeconfig: %w", err)
		}
	} else {
		// Insert new kubeconfig
		insertSQL := `
		INSERT INTO kubeconfigs (cluster_name, repository_name, encrypted_config)
		VALUES (?, ?, ?)
		`
		_, err := m.db.Exec(insertSQL, clusterName, m.repositoryName, encryptedConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to insert kubeconfig: %w", err)
		}
	}

	return m.GetKubeconfig(clusterName)
}

// GetKubeconfig retrieves a kubeconfig for a specific cluster
func (m *Manager) GetKubeconfig(clusterName string) (*Kubeconfig, error) {
	selectSQL := `
	SELECT id, cluster_name, repository_name, encrypted_config, created_at, updated_at
	FROM kubeconfigs
	WHERE cluster_name = ? AND repository_name = ?
	`

	kc := &Kubeconfig{}
	var createdAt, updatedAt string

	err := m.db.QueryRow(selectSQL, clusterName, m.repositoryName).Scan(&kc.ID, &kc.ClusterName,
		&kc.RepositoryName, &kc.EncryptedConfig, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No kubeconfig stored
		}
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	// Parse timestamps
	if kc.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt); err != nil {
		kc.CreatedAt = time.Now()
	}
	if kc.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt); err != nil {
		kc.UpdatedAt = time.Now()
	}

	// Set manager reference for config decryption
	kc.manager = m
	return kc, nil
}

// ListKubeconfigs lists all kubeconfigs for the current repository
func (m *Manager) ListKubeconfigs() ([]*Kubeconfig, error) {
	selectSQL := `
	SELECT id, cluster_name, repository_name, encrypted_config, created_at, updated_at
	FROM kubeconfigs
	WHERE repository_name = ?
	ORDER BY cluster_name
	`

	rows, err := m.db.Query(selectSQL, m.repositoryName)
	if err != nil {
		return nil, fmt.Errorf("failed to list kubeconfigs: %w", err)
	}
	defer rows.Close()

	var kubeconfigs []*Kubeconfig
	for rows.Next() {
		kc := &Kubeconfig{}
		var createdAt, updatedAt string

		err := rows.Scan(&kc.ID, &kc.ClusterName, &kc.RepositoryName,
			&kc.EncryptedConfig, &createdAt, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan kubeconfig: %w", err)
		}

		// Parse timestamps
		if kc.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt); err != nil {
			kc.CreatedAt = time.Now()
		}
		if kc.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt); err != nil {
			kc.UpdatedAt = time.Now()
		}

		// Set manager reference for config decryption
		kc.manager = m
		kubeconfigs = append(kubeconfigs, kc)
	}

	return kubeconfigs, nil
}

// DeleteKubeconfig removes a kubeconfig for a specific cluster
func (m *Manager) DeleteKubeconfig(clusterName string) error {
	deleteSQL := `DELETE FROM kubeconfigs WHERE cluster_name = ? AND repository_name = ?`
	_, err := m.db.Exec(deleteSQL, clusterName, m.repositoryName)
	if err != nil {
		return fmt.Errorf("failed to delete kubeconfig: %w", err)
	}
	return nil
}

// CleanupOrphanedKubeconfigs removes kubeconfigs that don't have corresponding cluster definitions
func (m *Manager) CleanupOrphanedKubeconfigs(activeClusterNames []string) error {
	if len(activeClusterNames) == 0 {
		// If no active clusters, remove all kubeconfigs for this repository
		deleteSQL := `DELETE FROM kubeconfigs WHERE repository_name = ?`
		_, err := m.db.Exec(deleteSQL, m.repositoryName)
		if err != nil {
			return fmt.Errorf("failed to cleanup all kubeconfigs: %w", err)
		}
		return nil
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(activeClusterNames))
	args := make([]interface{}, len(activeClusterNames)+1)
	args[0] = m.repositoryName

	for i, name := range activeClusterNames {
		placeholders[i] = "?"
		args[i+1] = name
	}

	deleteSQL := fmt.Sprintf(`
		DELETE FROM kubeconfigs 
		WHERE repository_name = ? 
		AND cluster_name NOT IN (%s)
	`, "?"+strings.Join(placeholders, ",?"))

	_, err := m.db.Exec(deleteSQL, args...)
	if err != nil {
		return fmt.Errorf("failed to cleanup orphaned kubeconfigs: %w", err)
	}

	return nil
}

// getEncryptionKey generates a deterministic encryption key based on system info
func (m *Manager) getEncryptionKey() []byte {
	// Use a combination of database path and repository name for key derivation
	hostname, _ := os.Hostname()
	keyMaterial := fmt.Sprintf("%s:%s:%s", m.dbPath, m.repositoryName, hostname)
	hash := sha256.Sum256([]byte(keyMaterial))
	return hash[:]
}

// encryptConfig encrypts a kubeconfig using AES-GCM
func (m *Manager) encryptConfig(kubeconfig string) (string, error) {
	if kubeconfig == "" {
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

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(kubeconfig), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptConfig decrypts a kubeconfig using AES-GCM
func (m *Manager) decryptConfig(encryptedConfig string) (string, error) {
	if encryptedConfig == "" {
		return "", nil
	}

	key := m.getEncryptionKey()

	ciphertext, err := base64.StdEncoding.DecodeString(encryptedConfig)
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted kubeconfig: %w", err)
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
		return "", fmt.Errorf("failed to decrypt kubeconfig: %w", err)
	}

	return string(plaintext), nil
}
