package repository

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

// Repository represents a Git repository configuration
type Repository struct {
	ID                int       `json:"id"`
	Name              string    `json:"name"`
	RepoURL           string    `json:"repo_url"`
	LocalPath         string    `json:"local_path"`
	Username          string    `json:"username"`
	Token             string    `json:"-"` // Not serialized for security (legacy)
	EncryptedPassword string    `json:"-"` // Not serialized for security
	IsCurrent         bool      `json:"is_current"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	manager           *Manager  `json:"-"` // Reference to manager for decryption
}

// GetPassword returns the decrypted password for this repository
func (r *Repository) GetPassword() (string, error) {
	if r.manager == nil {
		return "", fmt.Errorf("repository manager not available for password decryption")
	}
	return r.manager.decryptPassword(r.EncryptedPassword)
}

// Manager handles repository configurations using SQLite
type Manager struct {
	dbPath string
	db     *sql.DB
}

// NewManager creates a new repository manager
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	configDir := filepath.Join(homeDir, ".hyve")
	dbPath := filepath.Join(configDir, "repositories.db")

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

	// Create repositories table
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS repositories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		repo_url TEXT NOT NULL,
		local_path TEXT NOT NULL,
		username TEXT,
		encrypted_password TEXT,
		is_current BOOLEAN DEFAULT FALSE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Create index for faster lookups
	CREATE INDEX IF NOT EXISTS idx_repositories_name ON repositories(name);
	CREATE INDEX IF NOT EXISTS idx_repositories_current ON repositories(is_current);
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

// AddRepository adds a new repository configuration
func (m *Manager) AddRepository(name, repoURL, localPath, username, password string) (*Repository, error) {
	// Check if repository with this name already exists
	if exists, err := m.repositoryExists(name); err != nil {
		return nil, err
	} else if exists {
		return nil, fmt.Errorf("repository '%s' already exists", name)
	}

	// If this is the first repository, make it current
	isFirst, err := m.isFirstRepository()
	if err != nil {
		return nil, err
	}

	// If making this current, unset other current repositories
	if isFirst {
		if err := m.unsetCurrentRepository(); err != nil {
			return nil, err
		}
	}

	// Encrypt the password if provided
	encryptedPassword, err := m.encryptPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt password: %w", err)
	}

	insertSQL := `
	INSERT INTO repositories (name, repo_url, local_path, username, encrypted_password, is_current)
	VALUES (?, ?, ?, ?, ?, ?)
	`

	result, err := m.db.Exec(insertSQL, name, repoURL, localPath, username, encryptedPassword, isFirst)
	if err != nil {
		return nil, fmt.Errorf("failed to insert repository: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return m.GetRepositoryByID(int(id))
}

// UpdateRepository updates an existing repository configuration
func (m *Manager) UpdateRepository(name, repoURL, localPath, username, password string) (*Repository, error) {
	// Encrypt the password if provided
	encryptedPassword, err := m.encryptPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt password: %w", err)
	}

	updateSQL := `
	UPDATE repositories 
	SET repo_url = ?, local_path = ?, username = ?, encrypted_password = ?, updated_at = CURRENT_TIMESTAMP
	WHERE name = ?
	`

	result, err := m.db.Exec(updateSQL, repoURL, localPath, username, encryptedPassword, name)
	if err != nil {
		return nil, fmt.Errorf("failed to update repository: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, fmt.Errorf("repository '%s' not found", name)
	}

	return m.GetRepositoryByName(name)
}

// DeleteRepository removes a repository configuration
func (m *Manager) DeleteRepository(name string) error {
	// Check if this is the current repository
	current, err := m.GetCurrentRepository()
	if err == nil && current != nil && current.Name == name {
		// If deleting current repository, find another one to make current
		repos, err := m.ListRepositories()
		if err != nil {
			return err
		}

		for _, repo := range repos {
			if repo.Name != name {
				if err := m.SetCurrentRepository(repo.Name); err != nil {
					return fmt.Errorf("failed to set new current repository: %w", err)
				}
				break
			}
		}
	}

	deleteSQL := `DELETE FROM repositories WHERE name = ?`
	result, err := m.db.Exec(deleteSQL, name)
	if err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("repository '%s' not found", name)
	}

	return nil
}

// ListRepositories returns all repository configurations
func (m *Manager) ListRepositories() ([]*Repository, error) {
	selectSQL := `
	SELECT id, name, repo_url, local_path, username, encrypted_password, is_current, created_at, updated_at
	FROM repositories
	ORDER BY is_current DESC, name ASC
	`

	rows, err := m.db.Query(selectSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to query repositories: %w", err)
	}
	defer rows.Close()

	var repositories []*Repository
	for rows.Next() {
		repo := &Repository{}
		var createdAt, updatedAt string

		err := rows.Scan(&repo.ID, &repo.Name, &repo.RepoURL, &repo.LocalPath,
			&repo.Username, &repo.EncryptedPassword, &repo.IsCurrent, &createdAt, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan repository: %w", err)
		}

		// Parse timestamps
		if repo.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt); err != nil {
			repo.CreatedAt = time.Now()
		}
		if repo.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt); err != nil {
			repo.UpdatedAt = time.Now()
		}

		// Set manager reference for password decryption
		repo.manager = m
		repositories = append(repositories, repo)
	}

	return repositories, nil
}

// GetRepositoryByName returns a repository by name
func (m *Manager) GetRepositoryByName(name string) (*Repository, error) {
	selectSQL := `
	SELECT id, name, repo_url, local_path, username, encrypted_password, is_current, created_at, updated_at
	FROM repositories
	WHERE name = ?
	`

	repo := &Repository{}
	var createdAt, updatedAt string

	err := m.db.QueryRow(selectSQL, name).Scan(&repo.ID, &repo.Name, &repo.RepoURL,
		&repo.LocalPath, &repo.Username, &repo.EncryptedPassword, &repo.IsCurrent, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repository '%s' not found", name)
		}
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	// Parse timestamps
	if repo.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt); err != nil {
		repo.CreatedAt = time.Now()
	}
	if repo.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt); err != nil {
		repo.UpdatedAt = time.Now()
	}

	// Set manager reference for password decryption
	repo.manager = m
	return repo, nil
}

// GetRepositoryByID returns a repository by ID
func (m *Manager) GetRepositoryByID(id int) (*Repository, error) {
	selectSQL := `
	SELECT id, name, repo_url, local_path, username, encrypted_password, is_current, created_at, updated_at
	FROM repositories
	WHERE id = ?
	`

	repo := &Repository{}
	var createdAt, updatedAt string

	err := m.db.QueryRow(selectSQL, id).Scan(&repo.ID, &repo.Name, &repo.RepoURL,
		&repo.LocalPath, &repo.Username, &repo.EncryptedPassword, &repo.IsCurrent, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repository with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	// Parse timestamps
	if repo.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt); err != nil {
		repo.CreatedAt = time.Now()
	}
	if repo.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt); err != nil {
		repo.UpdatedAt = time.Now()
	}

	// Set manager reference for password decryption
	repo.manager = m
	return repo, nil
}

// GetCurrentRepository returns the currently selected repository
func (m *Manager) GetCurrentRepository() (*Repository, error) {
	selectSQL := `
	SELECT id, name, repo_url, local_path, username, encrypted_password, is_current, created_at, updated_at
	FROM repositories
	WHERE is_current = TRUE
	LIMIT 1
	`

	repo := &Repository{}
	var createdAt, updatedAt string

	err := m.db.QueryRow(selectSQL).Scan(&repo.ID, &repo.Name, &repo.RepoURL,
		&repo.LocalPath, &repo.Username, &repo.EncryptedPassword, &repo.IsCurrent, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no current repository configured")
		}
		return nil, fmt.Errorf("failed to get current repository: %w", err)
	}

	// Parse timestamps
	if repo.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt); err != nil {
		repo.CreatedAt = time.Now()
	}
	if repo.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt); err != nil {
		repo.UpdatedAt = time.Now()
	}

	// Set manager reference for password decryption
	repo.manager = m
	return repo, nil
}

// SetCurrentRepository sets a repository as the current one
func (m *Manager) SetCurrentRepository(name string) error {
	// First, unset all current repositories
	if err := m.unsetCurrentRepository(); err != nil {
		return err
	}

	// Set the specified repository as current
	updateSQL := `
	UPDATE repositories 
	SET is_current = TRUE, updated_at = CURRENT_TIMESTAMP
	WHERE name = ?
	`

	result, err := m.db.Exec(updateSQL, name)
	if err != nil {
		return fmt.Errorf("failed to set current repository: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("repository '%s' not found", name)
	}

	return nil
}

// HasRepositories checks if any repositories are configured
func (m *Manager) HasRepositories() (bool, error) {
	countSQL := `SELECT COUNT(*) FROM repositories`
	var count int
	err := m.db.QueryRow(countSQL).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to count repositories: %w", err)
	}
	return count > 0, nil
}

// repositoryExists checks if a repository with the given name exists
func (m *Manager) repositoryExists(name string) (bool, error) {
	countSQL := `SELECT COUNT(*) FROM repositories WHERE name = ?`
	var count int
	err := m.db.QueryRow(countSQL, name).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check repository existence: %w", err)
	}
	return count > 0, nil
}

// isFirstRepository checks if this would be the first repository
func (m *Manager) isFirstRepository() (bool, error) {
	hasRepos, err := m.HasRepositories()
	if err != nil {
		return false, err
	}
	return !hasRepos, nil
}

// unsetCurrentRepository unsets the current repository flag for all repositories
func (m *Manager) unsetCurrentRepository() error {
	updateSQL := `UPDATE repositories SET is_current = FALSE`
	_, err := m.db.Exec(updateSQL)
	if err != nil {
		return fmt.Errorf("failed to unset current repository: %w", err)
	}
	return nil
}

// getEncryptionKey generates a deterministic encryption key based on system info
// This is not the most secure approach but provides reasonable protection for local storage
func (m *Manager) getEncryptionKey() []byte {
	// Use a combination of database path and hostname for key derivation
	hostname, _ := os.Hostname()
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
