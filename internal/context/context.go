package context

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProviderContext represents the current context for a provider
type ProviderContext struct {
	Account string `yaml:"account,omitempty"` // Current account/organization name (alias)
}

// Context represents the current context for all providers
type Context struct {
	AWS   ProviderContext `yaml:"aws,omitempty"`
	GCP   ProviderContext `yaml:"gcp,omitempty"`
	Azure ProviderContext `yaml:"azure,omitempty"`
	Civo  ProviderContext `yaml:"civo,omitempty"`
}

// Manager handles context operations
type Manager struct {
	contextPath string
	context     *Context
}

var hyveHomeOverride string

// SetHyveHome overrides the Hyve home directory used by context managers.
// Must be called before any NewManager() call (e.g. from a PersistentPreRun hook).
func SetHyveHome(dir string) {
	hyveHomeOverride = dir
}

// NewManager creates a new context manager
func NewManager() (*Manager, error) {
	var contextPath string
	if hyveHomeOverride != "" {
		contextPath = filepath.Join(hyveHomeOverride, "context.yaml")
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "."
		}
		contextPath = filepath.Join(homeDir, ".hyve", "context.yaml")
	}

	mgr := &Manager{
		contextPath: contextPath,
		context:     &Context{},
	}

	if err := mgr.Load(); err != nil {
		return nil, err
	}

	return mgr, nil
}

// Load loads the context from file
func (m *Manager) Load() error {
	if _, err := os.Stat(m.contextPath); os.IsNotExist(err) {
		// Context file doesn't exist, use empty context
		m.context = &Context{}
		return nil
	}

	data, err := os.ReadFile(m.contextPath)
	if err != nil {
		return fmt.Errorf("failed to read context file: %w", err)
	}

	if err := yaml.Unmarshal(data, m.context); err != nil {
		return fmt.Errorf("failed to parse context file: %w", err)
	}

	return nil
}

// Save saves the context to file
func (m *Manager) Save() error {
	// Create config directory if it doesn't exist
	configDir := filepath.Dir(m.contextPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(m.context)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	if err := os.WriteFile(m.contextPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write context file: %w", err)
	}

	return nil
}

// GetAWSAccount returns the current AWS account name
func (m *Manager) GetAWSAccount() string {
	return m.context.AWS.Account
}

// SetAWSAccount sets the current AWS account
func (m *Manager) SetAWSAccount(account string) error {
	m.context.AWS.Account = account
	return m.Save()
}

// GetGCPProject returns the current GCP project name
func (m *Manager) GetGCPProject() string {
	return m.context.GCP.Account
}

// SetGCPProject sets the current GCP project
func (m *Manager) SetGCPProject(project string) error {
	m.context.GCP.Account = project
	return m.Save()
}

// GetAzureSubscription returns the current Azure subscription name
func (m *Manager) GetAzureSubscription() string {
	return m.context.Azure.Account
}

// SetAzureSubscription sets the current Azure subscription
func (m *Manager) SetAzureSubscription(subscription string) error {
	m.context.Azure.Account = subscription
	return m.Save()
}

// GetCivoOrganization returns the current Civo organization name
func (m *Manager) GetCivoOrganization() string {
	return m.context.Civo.Account
}

// SetCivoOrganization sets the current Civo organization
func (m *Manager) SetCivoOrganization(organization string) error {
	m.context.Civo.Account = organization
	return m.Save()
}

// GetCurrentAccount returns the current account for a provider
func (m *Manager) GetCurrentAccount(provider string) string {
	switch provider {
	case "aws":
		return m.GetAWSAccount()
	case "gcp":
		return m.GetGCPProject()
	case "azure":
		return m.GetAzureSubscription()
	case "civo":
		return m.GetCivoOrganization()
	default:
		return ""
	}
}

// SetCurrentAccount sets the current account for a provider
func (m *Manager) SetCurrentAccount(provider, account string) error {
	switch provider {
	case "aws":
		return m.SetAWSAccount(account)
	case "gcp":
		return m.SetGCPProject(account)
	case "azure":
		return m.SetAzureSubscription(account)
	case "civo":
		return m.SetCivoOrganization(account)
	default:
		return fmt.Errorf("unknown provider: %s", provider)
	}
}

// GetContext returns the full context
func (m *Manager) GetContext() *Context {
	return m.context
}

// Clear clears all context
func (m *Manager) Clear() error {
	m.context = &Context{}
	return m.Save()
}

// ClearProvider clears the context for a specific provider
func (m *Manager) ClearProvider(provider string) error {
	switch provider {
	case "aws":
		m.context.AWS = ProviderContext{}
	case "gcp":
		m.context.GCP = ProviderContext{}
	case "azure":
		m.context.Azure = ProviderContext{}
	case "civo":
		m.context.Civo = ProviderContext{}
	default:
		return fmt.Errorf("unknown provider: %s", provider)
	}
	return m.Save()
}

// ProviderSwitchCommands returns CLI commands to switch to a specific account
type ProviderSwitchCommands struct {
	CheckCommand  string // Command to check current account
	SwitchCommand string // Command to switch account
}

// GetSwitchCommands returns the CLI commands to verify and switch accounts for a provider
func GetSwitchCommands(provider string, accountID string) ProviderSwitchCommands {
	switch provider {
	case "aws":
		return ProviderSwitchCommands{
			CheckCommand:  "aws sts get-caller-identity --query Account --output text",
			SwitchCommand: fmt.Sprintf("# Set AWS credentials for account %s:\nexport AWS_PROFILE=<profile-name>\n# Or use: aws configure --profile <profile-name>", accountID),
		}
	case "gcp":
		return ProviderSwitchCommands{
			CheckCommand:  "gcloud config get-value project",
			SwitchCommand: fmt.Sprintf("gcloud config set project %s", accountID),
		}
	case "azure":
		return ProviderSwitchCommands{
			CheckCommand:  "az account show --query id --output tsv",
			SwitchCommand: fmt.Sprintf("az account set --subscription %s", accountID),
		}
	case "civo":
		return ProviderSwitchCommands{
			CheckCommand:  "civo apikey current",
			SwitchCommand: fmt.Sprintf("civo apikey use <api-key-name>\n# Or use: hyve config civo use %s", accountID),
		}
	default:
		return ProviderSwitchCommands{}
	}
}
