package config

import (
	"log"

	"github.com/spf13/viper"
)

// Manager handles configuration loading
type Manager struct{}

// NewManager creates a new config manager
func NewManager() *Manager {
	return &Manager{}
}

// GetCivoToken loads the Civo API token from configuration
func (m *Manager) GetCivoToken() string {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: Could not read .env file: %v", err)
	}

	return viper.GetString("CIVO_TOKEN")
}

// GetCloudflareToken loads the Cloudflare API token from configuration
func (m *Manager) GetCloudflareToken() string {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: Could not read .env file: %v", err)
	}

	return viper.GetString("CLOUDFLARE_API_TOKEN")
}

// GetDomain loads the domain from configuration
func (m *Manager) GetDomain() string {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: Could not read .env file: %v", err)
	}

	return viper.GetString("DOMAIN")
}
