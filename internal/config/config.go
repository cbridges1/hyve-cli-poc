package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"hyve/internal/context"
	"hyve/internal/credentials"
)

// GitConfig represents Git repository configuration
type GitConfig struct {
	RepoURL   string `yaml:"repo_url"`
	LocalPath string `yaml:"local_path"`
	Username  string `yaml:"username"`
	Token     string `yaml:"token,omitempty"` // Not stored in file for security
}

// HyveConfig represents the main configuration structure
type HyveConfig struct {
	Git GitConfig `yaml:"git"`
}

// Manager handles configuration loading
type Manager struct {
	configPath string
	config     *HyveConfig
}

// NewManager creates a new config manager
func NewManager() *Manager {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	configPath := filepath.Join(homeDir, ".hyve", "config.yaml")

	return &Manager{
		configPath: configPath,
		config:     &HyveConfig{},
	}
}

// LoadConfig loads the configuration from file
func (m *Manager) LoadConfig() error {
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		// Config file doesn't exist, use defaults
		m.config = &HyveConfig{
			Git: GitConfig{
				LocalPath: ".hyve-state",
			},
		}
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, m.config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Load token from environment variable if not in config
	if m.config.Git.Token == "" {
		if token := os.Getenv("HYVE_GIT_TOKEN"); token != "" {
			m.config.Git.Token = token
		}
	}

	return nil
}

// SaveConfig saves the configuration to file
func (m *Manager) SaveConfig() error {
	// Create config directory if it doesn't exist
	configDir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Don't save token to file for security
	configToSave := *m.config
	configToSave.Git.Token = ""

	data, err := yaml.Marshal(&configToSave)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetGitConfig returns the Git configuration
func (m *Manager) GetGitConfig() GitConfig {
	return m.config.Git
}

// SetGitConfig sets the Git configuration
func (m *Manager) SetGitConfig(git GitConfig) {
	m.config.Git = git
}

// IsGitConfigured checks if Git is properly configured
func (m *Manager) IsGitConfigured() bool {
	return m.config.Git.RepoURL != ""
}

// GetCivoToken loads the Civo API token from configuration
// Priority: 1) Database (keyed by current org) 2) Environment variable 3) .env file
func (m *Manager) GetCivoToken() string {
	// First, try to get from database using the current org from context
	credsMgr, err := credentials.NewManager()
	if err == nil {
		defer credsMgr.Close()
		ctxMgr, ctxErr := context.NewManager()
		if ctxErr == nil {
			orgName := ctxMgr.GetCivoOrganization()
			if orgName != "" {
				token, err := credsMgr.GetCivoToken(orgName)
				if err == nil && token != "" {
					return token
				}
			}
		}
	}

	// Check environment variable
	if token := os.Getenv("CIVO_TOKEN"); token != "" {
		return token
	}

	// Also check .env file
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		if token := viper.GetString("CIVO_TOKEN"); token != "" {
			return token
		}
	}

	return ""
}
