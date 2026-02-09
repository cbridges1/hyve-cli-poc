package providerconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProviderConfigDir is the directory name for provider configurations
const ProviderConfigDir = "provider-configs"

// GCPProject represents a named GCP project
type GCPProject struct {
	Name      string `yaml:"name"`
	ProjectID string `yaml:"project_id"`
}

// GCPConfig represents GCP-specific configuration
type GCPConfig struct {
	Projects []GCPProject `yaml:"projects"`
}

// AWSEKSRole represents a named EKS IAM role
type AWSEKSRole struct {
	Name    string `yaml:"name"`
	RoleARN string `yaml:"role_arn"`
}

// AWSVPC represents a named VPC
type AWSVPC struct {
	Name  string `yaml:"name"`
	VPCID string `yaml:"vpc_id"`
}

// AWSConfig represents AWS-specific configuration
type AWSConfig struct {
	AccountIDs []string     `yaml:"account_ids,omitempty"`
	Regions    []string     `yaml:"regions,omitempty"`
	EKSRoles   []AWSEKSRole `yaml:"eks_roles,omitempty"`
	VPCs       []AWSVPC     `yaml:"vpcs,omitempty"`
}

// AzureConfig represents Azure-specific configuration
type AzureConfig struct {
	SubscriptionIDs []string `yaml:"subscription_ids,omitempty"`
	ResourceGroups  []string `yaml:"resource_groups,omitempty"`
}

// CivoConfig represents Civo-specific configuration
type CivoConfig struct {
	Regions []string `yaml:"regions,omitempty"`
}

// Manager handles provider configuration operations
type Manager struct {
	repoPath string
}

// NewManager creates a new provider config manager for a repository
func NewManager(repoPath string) *Manager {
	return &Manager{
		repoPath: repoPath,
	}
}

// getConfigDir returns the provider-configs directory path
func (m *Manager) getConfigDir() string {
	return filepath.Join(m.repoPath, ProviderConfigDir)
}

// getConfigPath returns the path to a provider's config file
func (m *Manager) getConfigPath(provider string) string {
	return filepath.Join(m.getConfigDir(), fmt.Sprintf("%s.yaml", provider))
}

// ensureConfigDir creates the provider-configs directory if it doesn't exist
func (m *Manager) ensureConfigDir() error {
	configDir := m.getConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create provider-configs directory: %w", err)
	}
	return nil
}

// LoadGCPConfig loads the GCP configuration from the repository
func (m *Manager) LoadGCPConfig() (*GCPConfig, error) {
	configPath := m.getConfigPath("gcp")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty config if file doesn't exist
			return &GCPConfig{Projects: []GCPProject{}}, nil
		}
		return nil, fmt.Errorf("failed to read GCP config: %w", err)
	}

	var config GCPConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse GCP config: %w", err)
	}

	return &config, nil
}

// SaveGCPConfig saves the GCP configuration to the repository
func (m *Manager) SaveGCPConfig(config *GCPConfig) error {
	if err := m.ensureConfigDir(); err != nil {
		return err
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal GCP config: %w", err)
	}

	configPath := m.getConfigPath("gcp")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write GCP config: %w", err)
	}

	return nil
}

// AddGCPProject adds a named project to the GCP configuration
func (m *Manager) AddGCPProject(name, projectID string) error {
	config, err := m.LoadGCPConfig()
	if err != nil {
		return err
	}

	// Check if name already exists
	for i, p := range config.Projects {
		if p.Name == name {
			// Update existing project
			config.Projects[i].ProjectID = projectID
			return m.SaveGCPConfig(config)
		}
	}

	// Add new project
	config.Projects = append(config.Projects, GCPProject{
		Name:      name,
		ProjectID: projectID,
	})

	return m.SaveGCPConfig(config)
}

// RemoveGCPProject removes a project by name from the GCP configuration
func (m *Manager) RemoveGCPProject(name string) error {
	config, err := m.LoadGCPConfig()
	if err != nil {
		return err
	}

	// Filter out the project
	filtered := []GCPProject{}
	found := false
	for _, p := range config.Projects {
		if p.Name != name {
			filtered = append(filtered, p)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("project '%s' not found", name)
	}

	config.Projects = filtered
	return m.SaveGCPConfig(config)
}

// GetGCPProjectID returns the project ID for a given name/alias
func (m *Manager) GetGCPProjectID(name string) (string, error) {
	config, err := m.LoadGCPConfig()
	if err != nil {
		return "", err
	}

	for _, p := range config.Projects {
		if p.Name == name {
			return p.ProjectID, nil
		}
	}

	return "", fmt.Errorf("GCP project '%s' not found in repository configuration", name)
}

// ListGCPProjects returns all configured GCP projects
func (m *Manager) ListGCPProjects() ([]GCPProject, error) {
	config, err := m.LoadGCPConfig()
	if err != nil {
		return nil, err
	}

	return config.Projects, nil
}

// HasGCPProject checks if a project with the given name exists
func (m *Manager) HasGCPProject(name string) (bool, error) {
	config, err := m.LoadGCPConfig()
	if err != nil {
		return false, err
	}

	for _, p := range config.Projects {
		if p.Name == name {
			return true, nil
		}
	}

	return false, nil
}

// LoadAWSConfig loads the AWS configuration from the repository
func (m *Manager) LoadAWSConfig() (*AWSConfig, error) {
	configPath := m.getConfigPath("aws")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &AWSConfig{}, nil
		}
		return nil, fmt.Errorf("failed to read AWS config: %w", err)
	}

	var config AWSConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse AWS config: %w", err)
	}

	return &config, nil
}

// SaveAWSConfig saves the AWS configuration to the repository
func (m *Manager) SaveAWSConfig(config *AWSConfig) error {
	if err := m.ensureConfigDir(); err != nil {
		return err
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal AWS config: %w", err)
	}

	configPath := m.getConfigPath("aws")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write AWS config: %w", err)
	}

	return nil
}

// AddAWSEKSRole adds a named EKS role to the AWS configuration
func (m *Manager) AddAWSEKSRole(name, roleARN string) error {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return err
	}

	// Check if name already exists
	for i, r := range config.EKSRoles {
		if r.Name == name {
			// Update existing role
			config.EKSRoles[i].RoleARN = roleARN
			return m.SaveAWSConfig(config)
		}
	}

	// Add new role
	config.EKSRoles = append(config.EKSRoles, AWSEKSRole{
		Name:    name,
		RoleARN: roleARN,
	})

	return m.SaveAWSConfig(config)
}

// RemoveAWSEKSRole removes an EKS role by name from the AWS configuration
func (m *Manager) RemoveAWSEKSRole(name string) error {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return err
	}

	filtered := []AWSEKSRole{}
	found := false
	for _, r := range config.EKSRoles {
		if r.Name != name {
			filtered = append(filtered, r)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("EKS role '%s' not found", name)
	}

	config.EKSRoles = filtered
	return m.SaveAWSConfig(config)
}

// GetAWSEKSRoleARN returns the role ARN for a given name
func (m *Manager) GetAWSEKSRoleARN(name string) (string, error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return "", err
	}

	for _, r := range config.EKSRoles {
		if r.Name == name {
			return r.RoleARN, nil
		}
	}

	return "", fmt.Errorf("EKS role '%s' not found in repository configuration", name)
}

// ListAWSEKSRoles returns all configured EKS roles
func (m *Manager) ListAWSEKSRoles() ([]AWSEKSRole, error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return nil, err
	}

	return config.EKSRoles, nil
}

// HasAWSEKSRole checks if an EKS role with the given name exists
func (m *Manager) HasAWSEKSRole(name string) (bool, error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return false, err
	}

	for _, r := range config.EKSRoles {
		if r.Name == name {
			return true, nil
		}
	}

	return false, nil
}

// AddAWSVPC adds a named VPC to the AWS configuration
func (m *Manager) AddAWSVPC(name, vpcID string) error {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return err
	}

	// Check if name already exists
	for i, v := range config.VPCs {
		if v.Name == name {
			// Update existing VPC
			config.VPCs[i].VPCID = vpcID
			return m.SaveAWSConfig(config)
		}
	}

	// Add new VPC
	config.VPCs = append(config.VPCs, AWSVPC{
		Name:  name,
		VPCID: vpcID,
	})

	return m.SaveAWSConfig(config)
}

// RemoveAWSVPC removes a VPC by name from the AWS configuration
func (m *Manager) RemoveAWSVPC(name string) error {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return err
	}

	filtered := []AWSVPC{}
	found := false
	for _, v := range config.VPCs {
		if v.Name != name {
			filtered = append(filtered, v)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("VPC '%s' not found", name)
	}

	config.VPCs = filtered
	return m.SaveAWSConfig(config)
}

// GetAWSVPCID returns the VPC ID for a given name
func (m *Manager) GetAWSVPCID(name string) (string, error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return "", err
	}

	for _, v := range config.VPCs {
		if v.Name == name {
			return v.VPCID, nil
		}
	}

	return "", fmt.Errorf("VPC '%s' not found in repository configuration", name)
}

// ListAWSVPCs returns all configured VPCs
func (m *Manager) ListAWSVPCs() ([]AWSVPC, error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return nil, err
	}

	return config.VPCs, nil
}

// HasAWSVPC checks if a VPC with the given name exists
func (m *Manager) HasAWSVPC(name string) (bool, error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return false, err
	}

	for _, v := range config.VPCs {
		if v.Name == name {
			return true, nil
		}
	}

	return false, nil
}

// LoadAzureConfig loads the Azure configuration from the repository
func (m *Manager) LoadAzureConfig() (*AzureConfig, error) {
	configPath := m.getConfigPath("azure")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &AzureConfig{}, nil
		}
		return nil, fmt.Errorf("failed to read Azure config: %w", err)
	}

	var config AzureConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Azure config: %w", err)
	}

	return &config, nil
}

// SaveAzureConfig saves the Azure configuration to the repository
func (m *Manager) SaveAzureConfig(config *AzureConfig) error {
	if err := m.ensureConfigDir(); err != nil {
		return err
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal Azure config: %w", err)
	}

	configPath := m.getConfigPath("azure")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write Azure config: %w", err)
	}

	return nil
}

// LoadCivoConfig loads the Civo configuration from the repository
func (m *Manager) LoadCivoConfig() (*CivoConfig, error) {
	configPath := m.getConfigPath("civo")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &CivoConfig{}, nil
		}
		return nil, fmt.Errorf("failed to read Civo config: %w", err)
	}

	var config CivoConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Civo config: %w", err)
	}

	return &config, nil
}

// SaveCivoConfig saves the Civo configuration to the repository
func (m *Manager) SaveCivoConfig(config *CivoConfig) error {
	if err := m.ensureConfigDir(); err != nil {
		return err
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal Civo config: %w", err)
	}

	configPath := m.getConfigPath("civo")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write Civo config: %w", err)
	}

	return nil
}

// ConfigExists checks if a provider config file exists
func (m *Manager) ConfigExists(provider string) bool {
	configPath := m.getConfigPath(provider)
	_, err := os.Stat(configPath)
	return err == nil
}
