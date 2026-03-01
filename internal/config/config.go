package config

import (
	"hyve/internal/context"
	"hyve/internal/credentials"
)

// Manager handles configuration
type Manager struct{}

// NewManager creates a new config manager
func NewManager() *Manager {
	return &Manager{}
}

// GetCivoToken loads the Civo API token from the local database, keyed by the
// current Civo organization set in context. Returns an empty string when no
// token is stored or no organization is active.
func (m *Manager) GetCivoToken() string {
	credsMgr, err := credentials.NewManager()
	if err != nil {
		return ""
	}
	defer credsMgr.Close()

	ctxMgr, err := context.NewManager()
	if err != nil {
		return ""
	}

	orgName := ctxMgr.GetCivoOrganization()
	if orgName == "" {
		return ""
	}

	token, err := credsMgr.GetCivoToken(orgName)
	if err != nil {
		return ""
	}
	return token
}
