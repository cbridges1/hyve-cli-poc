package config

import (
	"os"

	"github.com/spf13/viper"

	"hyve/internal/context"
	"hyve/internal/credentials"
)

// Manager handles configuration
type Manager struct{}

// NewManager creates a new config manager
func NewManager() *Manager {
	return &Manager{}
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
