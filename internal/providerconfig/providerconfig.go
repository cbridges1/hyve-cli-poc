package providerconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProviderConfigDir is the directory name for provider configurations
const ProviderConfigDir = "provider-configs"

// ========== GCP Types ==========

// GCPProject represents a named GCP project
type GCPProject struct {
	Name            string `yaml:"name"`
	ProjectID       string `yaml:"project_id"`
	CredentialsJSON string `yaml:"credentials_json,omitempty"`
}

// GCPConfig represents GCP-specific configuration
type GCPConfig struct {
	Projects []GCPProject `yaml:"projects"`
}

// ========== AWS Types ==========

// AWSEKSRole represents a named EKS IAM role
type AWSEKSRole struct {
	Name    string `yaml:"name"`
	RoleARN string `yaml:"role_arn"`
}

// AWSNodeRole represents a named EKS node IAM role
type AWSNodeRole struct {
	Name    string `yaml:"name"`
	RoleARN string `yaml:"role_arn"`
}

// AWSVPC represents a named VPC
type AWSVPC struct {
	Name  string `yaml:"name"`
	VPCID string `yaml:"vpc_id"`
}

// AWSAccount represents a named AWS account with all its resources
type AWSAccount struct {
	Name            string        `yaml:"name"`
	AccountID       string        `yaml:"account_id"`
	AccessKeyID     string        `yaml:"access_key_id,omitempty"`
	SecretAccessKey string        `yaml:"secret_access_key,omitempty"`
	SessionToken    string        `yaml:"session_token,omitempty"`
	Regions         []string      `yaml:"regions,omitempty"`
	VPCs            []AWSVPC      `yaml:"vpcs,omitempty"`
	EKSRoles        []AWSEKSRole  `yaml:"eks_roles,omitempty"`
	NodeRoles       []AWSNodeRole `yaml:"node_roles,omitempty"`
}

// AWSConfig represents AWS-specific configuration
type AWSConfig struct {
	Accounts []AWSAccount `yaml:"accounts,omitempty"`
}

// ========== Azure Types ==========

// AzureResourceGroup represents a named resource group
type AzureResourceGroup struct {
	Name     string `yaml:"name"`
	Location string `yaml:"location,omitempty"`
}

// AzureSubscription represents a named Azure subscription
type AzureSubscription struct {
	Name           string               `yaml:"name"`
	SubscriptionID string               `yaml:"subscription_id"`
	TenantID       string               `yaml:"tenant_id,omitempty"`
	ClientID       string               `yaml:"client_id,omitempty"`
	ClientSecret   string               `yaml:"client_secret,omitempty"`
	ResourceGroups []AzureResourceGroup `yaml:"resource_groups,omitempty"`
}

// AzureConfig represents Azure-specific configuration
type AzureConfig struct {
	Subscriptions []AzureSubscription `yaml:"subscriptions,omitempty"`
}

// ========== Civo Types ==========

// CivoNetwork represents a named Civo network
type CivoNetwork struct {
	Name      string `yaml:"name"`
	NetworkID string `yaml:"network_id"`
}

// CivoOrganization represents a named Civo organization/account
type CivoOrganization struct {
	Name     string        `yaml:"name"`
	OrgID    string        `yaml:"org_id"`
	Token    string        `yaml:"token,omitempty"`
	Regions  []string      `yaml:"regions,omitempty"`
	Networks []CivoNetwork `yaml:"networks,omitempty"`
}

// CivoConfig represents Civo-specific configuration
type CivoConfig struct {
	Organizations []CivoOrganization `yaml:"organizations,omitempty"`
}

// ========== Manager ==========

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

// ConfigExists checks if a provider config file exists
func (m *Manager) ConfigExists(provider string) bool {
	configPath := m.getConfigPath(provider)
	_, err := os.Stat(configPath)
	return err == nil
}

// resolveCredential resolves a credential field value.
// If the value is wrapped in ${...} it is treated as an environment variable reference
// and the named variable's value is returned. Otherwise the literal value is returned as-is.
func resolveCredential(v string) string {
	if strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
		return os.Getenv(strings.TrimSpace(v[2 : len(v)-1]))
	}
	return v
}

// GetCivoToken returns the API token for a Civo organization.
// The token field may be a literal value or an env var reference (${VAR_NAME}).
func (m *Manager) GetCivoToken(orgName string) (string, error) {
	config, err := m.LoadCivoConfig()
	if err != nil {
		return "", err
	}
	for _, o := range config.Organizations {
		if o.Name == orgName {
			return resolveCredential(o.Token), nil
		}
	}
	return "", fmt.Errorf("Civo organization '%s' not found", orgName)
}

// GetGCPCredentialsJSON returns the service account credentials JSON for a GCP project.
// The credentials_json field may be a literal value or an env var reference (${VAR_NAME}).
func (m *Manager) GetGCPCredentialsJSON(projectName string) (string, error) {
	config, err := m.LoadGCPConfig()
	if err != nil {
		return "", err
	}
	for _, p := range config.Projects {
		if p.Name == projectName {
			return resolveCredential(p.CredentialsJSON), nil
		}
	}
	return "", fmt.Errorf("GCP project '%s' not found", projectName)
}

// GetAWSCredentials returns the credentials for an AWS account.
// Each credential field may be a literal value or an env var reference (${VAR_NAME}).
func (m *Manager) GetAWSCredentials(accountName string) (accessKeyID, secretAccessKey, sessionToken string, err error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return
	}
	for _, a := range config.Accounts {
		if a.Name == accountName {
			return resolveCredential(a.AccessKeyID), resolveCredential(a.SecretAccessKey), resolveCredential(a.SessionToken), nil
		}
	}
	err = fmt.Errorf("AWS account '%s' not found", accountName)
	return
}

// GetAzureCredentials returns the service principal credentials for an Azure subscription.
// Each credential field may be a literal value or an env var reference (${VAR_NAME}).
func (m *Manager) GetAzureCredentials(subscriptionName string) (tenantID, clientID, clientSecret string, err error) {
	config, err := m.LoadAzureConfig()
	if err != nil {
		return
	}
	for _, s := range config.Subscriptions {
		if s.Name == subscriptionName {
			return resolveCredential(s.TenantID), resolveCredential(s.ClientID), resolveCredential(s.ClientSecret), nil
		}
	}
	err = fmt.Errorf("Azure subscription '%s' not found", subscriptionName)
	return
}

// ========== GCP Functions ==========

// LoadGCPConfig loads the GCP configuration from the repository
func (m *Manager) LoadGCPConfig() (*GCPConfig, error) {
	configPath := m.getConfigPath("gcp")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
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

	for i, p := range config.Projects {
		if p.Name == name {
			config.Projects[i].ProjectID = projectID
			return m.SaveGCPConfig(config)
		}
	}

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

// ========== AWS Functions ==========

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

// findAWSAccount finds an account by name and returns its index
func (m *Manager) findAWSAccount(config *AWSConfig, name string) int {
	for i, a := range config.Accounts {
		if a.Name == name {
			return i
		}
	}
	return -1
}

// AddAWSAccount adds a named account to the AWS configuration
func (m *Manager) AddAWSAccount(name, accountID string) error {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return err
	}

	idx := m.findAWSAccount(config, name)
	if idx >= 0 {
		config.Accounts[idx].AccountID = accountID
		return m.SaveAWSConfig(config)
	}

	config.Accounts = append(config.Accounts, AWSAccount{
		Name:      name,
		AccountID: accountID,
	})

	return m.SaveAWSConfig(config)
}

// RemoveAWSAccount removes an account by name from the AWS configuration
func (m *Manager) RemoveAWSAccount(name string) error {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return err
	}

	filtered := []AWSAccount{}
	found := false
	for _, a := range config.Accounts {
		if a.Name != name {
			filtered = append(filtered, a)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("account '%s' not found", name)
	}

	config.Accounts = filtered
	return m.SaveAWSConfig(config)
}

// GetAWSAccountID returns the account ID for a given name/alias
func (m *Manager) GetAWSAccountID(name string) (string, error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return "", err
	}

	for _, a := range config.Accounts {
		if a.Name == name {
			return a.AccountID, nil
		}
	}

	return "", fmt.Errorf("AWS account '%s' not found in repository configuration", name)
}

// GetAWSAccount returns the full account config for a given name
func (m *Manager) GetAWSAccount(name string) (*AWSAccount, error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return nil, err
	}

	for _, a := range config.Accounts {
		if a.Name == name {
			return &a, nil
		}
	}

	return nil, fmt.Errorf("AWS account '%s' not found in repository configuration", name)
}

// ListAWSAccounts returns all configured AWS accounts
func (m *Manager) ListAWSAccounts() ([]AWSAccount, error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return nil, err
	}

	return config.Accounts, nil
}

// HasAWSAccount checks if an account with the given name exists
func (m *Manager) HasAWSAccount(name string) (bool, error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return false, err
	}

	for _, a := range config.Accounts {
		if a.Name == name {
			return true, nil
		}
	}

	return false, nil
}

// AddAWSEKSRole adds a named EKS role to an account
func (m *Manager) AddAWSEKSRole(accountName, roleName, roleARN string) error {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return err
	}

	idx := m.findAWSAccount(config, accountName)
	if idx < 0 {
		return fmt.Errorf("AWS account '%s' not found", accountName)
	}

	// Check if role already exists
	for i, r := range config.Accounts[idx].EKSRoles {
		if r.Name == roleName {
			config.Accounts[idx].EKSRoles[i].RoleARN = roleARN
			return m.SaveAWSConfig(config)
		}
	}

	config.Accounts[idx].EKSRoles = append(config.Accounts[idx].EKSRoles, AWSEKSRole{
		Name:    roleName,
		RoleARN: roleARN,
	})

	return m.SaveAWSConfig(config)
}

// RemoveAWSEKSRole removes an EKS role by name from an account
func (m *Manager) RemoveAWSEKSRole(accountName, roleName string) error {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return err
	}

	idx := m.findAWSAccount(config, accountName)
	if idx < 0 {
		return fmt.Errorf("AWS account '%s' not found", accountName)
	}

	filtered := []AWSEKSRole{}
	found := false
	for _, r := range config.Accounts[idx].EKSRoles {
		if r.Name != roleName {
			filtered = append(filtered, r)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("EKS role '%s' not found in account '%s'", roleName, accountName)
	}

	config.Accounts[idx].EKSRoles = filtered
	return m.SaveAWSConfig(config)
}

// GetAWSEKSRoleARN returns the role ARN for a given role name in an account
func (m *Manager) GetAWSEKSRoleARN(accountName, roleName string) (string, error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return "", err
	}

	idx := m.findAWSAccount(config, accountName)
	if idx < 0 {
		return "", fmt.Errorf("AWS account '%s' not found", accountName)
	}

	for _, r := range config.Accounts[idx].EKSRoles {
		if r.Name == roleName {
			return r.RoleARN, nil
		}
	}

	return "", fmt.Errorf("EKS role '%s' not found in account '%s'", roleName, accountName)
}

// ListAWSEKSRoles returns all configured EKS roles for an account
func (m *Manager) ListAWSEKSRoles(accountName string) ([]AWSEKSRole, error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return nil, err
	}

	idx := m.findAWSAccount(config, accountName)
	if idx < 0 {
		return nil, fmt.Errorf("AWS account '%s' not found", accountName)
	}

	return config.Accounts[idx].EKSRoles, nil
}

// HasAWSEKSRole checks if an EKS role with the given name exists in an account
func (m *Manager) HasAWSEKSRole(accountName, roleName string) (bool, error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return false, err
	}

	idx := m.findAWSAccount(config, accountName)
	if idx < 0 {
		return false, fmt.Errorf("AWS account '%s' not found", accountName)
	}

	for _, r := range config.Accounts[idx].EKSRoles {
		if r.Name == roleName {
			return true, nil
		}
	}

	return false, nil
}

// AddAWSNodeRole adds a named node role to an account
func (m *Manager) AddAWSNodeRole(accountName, roleName, roleARN string) error {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return err
	}

	idx := m.findAWSAccount(config, accountName)
	if idx < 0 {
		return fmt.Errorf("AWS account '%s' not found", accountName)
	}

	for i, r := range config.Accounts[idx].NodeRoles {
		if r.Name == roleName {
			config.Accounts[idx].NodeRoles[i].RoleARN = roleARN
			return m.SaveAWSConfig(config)
		}
	}

	config.Accounts[idx].NodeRoles = append(config.Accounts[idx].NodeRoles, AWSNodeRole{
		Name:    roleName,
		RoleARN: roleARN,
	})

	return m.SaveAWSConfig(config)
}

// RemoveAWSNodeRole removes a node role by name from an account
func (m *Manager) RemoveAWSNodeRole(accountName, roleName string) error {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return err
	}

	idx := m.findAWSAccount(config, accountName)
	if idx < 0 {
		return fmt.Errorf("AWS account '%s' not found", accountName)
	}

	filtered := []AWSNodeRole{}
	found := false
	for _, r := range config.Accounts[idx].NodeRoles {
		if r.Name != roleName {
			filtered = append(filtered, r)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("node role '%s' not found in account '%s'", roleName, accountName)
	}

	config.Accounts[idx].NodeRoles = filtered
	return m.SaveAWSConfig(config)
}

// GetAWSNodeRoleARN returns the role ARN for a given node role name in an account
func (m *Manager) GetAWSNodeRoleARN(accountName, roleName string) (string, error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return "", err
	}

	idx := m.findAWSAccount(config, accountName)
	if idx < 0 {
		return "", fmt.Errorf("AWS account '%s' not found", accountName)
	}

	for _, r := range config.Accounts[idx].NodeRoles {
		if r.Name == roleName {
			return r.RoleARN, nil
		}
	}

	return "", fmt.Errorf("node role '%s' not found in account '%s'", roleName, accountName)
}

// ListAWSNodeRoles returns all configured node roles for an account
func (m *Manager) ListAWSNodeRoles(accountName string) ([]AWSNodeRole, error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return nil, err
	}

	idx := m.findAWSAccount(config, accountName)
	if idx < 0 {
		return nil, fmt.Errorf("AWS account '%s' not found", accountName)
	}

	return config.Accounts[idx].NodeRoles, nil
}

// HasAWSNodeRole checks if a node role with the given name exists in an account
func (m *Manager) HasAWSNodeRole(accountName, roleName string) (bool, error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return false, err
	}

	idx := m.findAWSAccount(config, accountName)
	if idx < 0 {
		return false, fmt.Errorf("AWS account '%s' not found", accountName)
	}

	for _, r := range config.Accounts[idx].NodeRoles {
		if r.Name == roleName {
			return true, nil
		}
	}

	return false, nil
}

// AddAWSVPC adds a named VPC to an account
func (m *Manager) AddAWSVPC(accountName, vpcName, vpcID string) error {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return err
	}

	idx := m.findAWSAccount(config, accountName)
	if idx < 0 {
		return fmt.Errorf("AWS account '%s' not found", accountName)
	}

	for i, v := range config.Accounts[idx].VPCs {
		if v.Name == vpcName {
			config.Accounts[idx].VPCs[i].VPCID = vpcID
			return m.SaveAWSConfig(config)
		}
	}

	config.Accounts[idx].VPCs = append(config.Accounts[idx].VPCs, AWSVPC{
		Name:  vpcName,
		VPCID: vpcID,
	})

	return m.SaveAWSConfig(config)
}

// RemoveAWSVPC removes a VPC by name from an account
func (m *Manager) RemoveAWSVPC(accountName, vpcName string) error {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return err
	}

	idx := m.findAWSAccount(config, accountName)
	if idx < 0 {
		return fmt.Errorf("AWS account '%s' not found", accountName)
	}

	filtered := []AWSVPC{}
	found := false
	for _, v := range config.Accounts[idx].VPCs {
		if v.Name != vpcName {
			filtered = append(filtered, v)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("VPC '%s' not found in account '%s'", vpcName, accountName)
	}

	config.Accounts[idx].VPCs = filtered
	return m.SaveAWSConfig(config)
}

// GetAWSVPCID returns the VPC ID for a given VPC name in an account
func (m *Manager) GetAWSVPCID(accountName, vpcName string) (string, error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return "", err
	}

	idx := m.findAWSAccount(config, accountName)
	if idx < 0 {
		return "", fmt.Errorf("AWS account '%s' not found", accountName)
	}

	for _, v := range config.Accounts[idx].VPCs {
		if v.Name == vpcName {
			return v.VPCID, nil
		}
	}

	return "", fmt.Errorf("VPC '%s' not found in account '%s'", vpcName, accountName)
}

// ListAWSVPCs returns all configured VPCs for an account
func (m *Manager) ListAWSVPCs(accountName string) ([]AWSVPC, error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return nil, err
	}

	idx := m.findAWSAccount(config, accountName)
	if idx < 0 {
		return nil, fmt.Errorf("AWS account '%s' not found", accountName)
	}

	return config.Accounts[idx].VPCs, nil
}

// HasAWSVPC checks if a VPC with the given name exists in an account
func (m *Manager) HasAWSVPC(accountName, vpcName string) (bool, error) {
	config, err := m.LoadAWSConfig()
	if err != nil {
		return false, err
	}

	idx := m.findAWSAccount(config, accountName)
	if idx < 0 {
		return false, fmt.Errorf("AWS account '%s' not found", accountName)
	}

	for _, v := range config.Accounts[idx].VPCs {
		if v.Name == vpcName {
			return true, nil
		}
	}

	return false, nil
}

// ========== Azure Functions ==========

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

// AddAzureSubscription adds a named subscription to the Azure configuration
func (m *Manager) AddAzureSubscription(name, subscriptionID string) error {
	config, err := m.LoadAzureConfig()
	if err != nil {
		return err
	}

	for i, s := range config.Subscriptions {
		if s.Name == name {
			config.Subscriptions[i].SubscriptionID = subscriptionID
			return m.SaveAzureConfig(config)
		}
	}

	config.Subscriptions = append(config.Subscriptions, AzureSubscription{
		Name:           name,
		SubscriptionID: subscriptionID,
	})

	return m.SaveAzureConfig(config)
}

// RemoveAzureSubscription removes a subscription by name
func (m *Manager) RemoveAzureSubscription(name string) error {
	config, err := m.LoadAzureConfig()
	if err != nil {
		return err
	}

	filtered := []AzureSubscription{}
	found := false
	for _, s := range config.Subscriptions {
		if s.Name != name {
			filtered = append(filtered, s)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("subscription '%s' not found", name)
	}

	config.Subscriptions = filtered
	return m.SaveAzureConfig(config)
}

// GetAzureSubscriptionID returns the subscription ID for a given name
func (m *Manager) GetAzureSubscriptionID(name string) (string, error) {
	config, err := m.LoadAzureConfig()
	if err != nil {
		return "", err
	}

	for _, s := range config.Subscriptions {
		if s.Name == name {
			return s.SubscriptionID, nil
		}
	}

	return "", fmt.Errorf("Azure subscription '%s' not found", name)
}

// ListAzureSubscriptions returns all configured subscriptions
func (m *Manager) ListAzureSubscriptions() ([]AzureSubscription, error) {
	config, err := m.LoadAzureConfig()
	if err != nil {
		return nil, err
	}

	return config.Subscriptions, nil
}

// HasAzureSubscription checks if a subscription with the given name exists
func (m *Manager) HasAzureSubscription(name string) (bool, error) {
	config, err := m.LoadAzureConfig()
	if err != nil {
		return false, err
	}

	for _, s := range config.Subscriptions {
		if s.Name == name {
			return true, nil
		}
	}

	return false, nil
}

// AddAzureResourceGroup adds a resource group to a named subscription
func (m *Manager) AddAzureResourceGroup(subscriptionName, rgName, location string) error {
	config, err := m.LoadAzureConfig()
	if err != nil {
		return err
	}

	for i, s := range config.Subscriptions {
		if s.Name != subscriptionName {
			continue
		}
		for j, rg := range s.ResourceGroups {
			if rg.Name == rgName {
				config.Subscriptions[i].ResourceGroups[j].Location = location
				return m.SaveAzureConfig(config)
			}
		}
		config.Subscriptions[i].ResourceGroups = append(config.Subscriptions[i].ResourceGroups, AzureResourceGroup{
			Name:     rgName,
			Location: location,
		})
		return m.SaveAzureConfig(config)
	}

	return fmt.Errorf("subscription '%s' not found", subscriptionName)
}

// ListAzureResourceGroups returns all resource groups for a named subscription
func (m *Manager) ListAzureResourceGroups(subscriptionName string) ([]AzureResourceGroup, error) {
	config, err := m.LoadAzureConfig()
	if err != nil {
		return nil, err
	}

	for _, s := range config.Subscriptions {
		if s.Name == subscriptionName {
			return s.ResourceGroups, nil
		}
	}

	return nil, fmt.Errorf("subscription '%s' not found", subscriptionName)
}

// RemoveAzureResourceGroup removes a resource group from a named subscription
func (m *Manager) RemoveAzureResourceGroup(subscriptionName, rgName string) error {
	config, err := m.LoadAzureConfig()
	if err != nil {
		return err
	}

	for i, s := range config.Subscriptions {
		if s.Name != subscriptionName {
			continue
		}
		filtered := []AzureResourceGroup{}
		found := false
		for _, rg := range s.ResourceGroups {
			if rg.Name != rgName {
				filtered = append(filtered, rg)
			} else {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("resource group '%s' not found in subscription '%s'", rgName, subscriptionName)
		}
		config.Subscriptions[i].ResourceGroups = filtered
		return m.SaveAzureConfig(config)
	}

	return fmt.Errorf("subscription '%s' not found", subscriptionName)
}

// ========== Civo Functions ==========

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

// AddCivoOrganization adds a named organization to the Civo configuration
func (m *Manager) AddCivoOrganization(name, orgID string) error {
	config, err := m.LoadCivoConfig()
	if err != nil {
		return err
	}

	for i, o := range config.Organizations {
		if o.Name == name {
			config.Organizations[i].OrgID = orgID
			return m.SaveCivoConfig(config)
		}
	}

	config.Organizations = append(config.Organizations, CivoOrganization{
		Name:  name,
		OrgID: orgID,
	})

	return m.SaveCivoConfig(config)
}

// RemoveCivoOrganization removes an organization by name
func (m *Manager) RemoveCivoOrganization(name string) error {
	config, err := m.LoadCivoConfig()
	if err != nil {
		return err
	}

	filtered := []CivoOrganization{}
	found := false
	for _, o := range config.Organizations {
		if o.Name != name {
			filtered = append(filtered, o)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("organization '%s' not found", name)
	}

	config.Organizations = filtered
	return m.SaveCivoConfig(config)
}

// GetCivoOrgID returns the org ID for a given name
func (m *Manager) GetCivoOrgID(name string) (string, error) {
	config, err := m.LoadCivoConfig()
	if err != nil {
		return "", err
	}

	for _, o := range config.Organizations {
		if o.Name == name {
			return o.OrgID, nil
		}
	}

	return "", fmt.Errorf("Civo organization '%s' not found", name)
}

// GetCivoOrganization returns the full organization config for a given name
func (m *Manager) GetCivoOrganization(name string) (*CivoOrganization, error) {
	config, err := m.LoadCivoConfig()
	if err != nil {
		return nil, err
	}

	for _, o := range config.Organizations {
		if o.Name == name {
			return &o, nil
		}
	}

	return nil, fmt.Errorf("Civo organization '%s' not found", name)
}

// ListCivoOrganizations returns all configured organizations
func (m *Manager) ListCivoOrganizations() ([]CivoOrganization, error) {
	config, err := m.LoadCivoConfig()
	if err != nil {
		return nil, err
	}

	return config.Organizations, nil
}

// HasCivoOrganization checks if an organization with the given name exists
func (m *Manager) HasCivoOrganization(name string) (bool, error) {
	config, err := m.LoadCivoConfig()
	if err != nil {
		return false, err
	}

	for _, o := range config.Organizations {
		if o.Name == name {
			return true, nil
		}
	}

	return false, nil
}

// AddCivoNetwork adds a named network to an organization
func (m *Manager) AddCivoNetwork(orgName, networkName, networkID string) error {
	config, err := m.LoadCivoConfig()
	if err != nil {
		return err
	}

	for i, o := range config.Organizations {
		if o.Name == orgName {
			for j, n := range o.Networks {
				if n.Name == networkName {
					config.Organizations[i].Networks[j].NetworkID = networkID
					return m.SaveCivoConfig(config)
				}
			}
			config.Organizations[i].Networks = append(config.Organizations[i].Networks, CivoNetwork{
				Name:      networkName,
				NetworkID: networkID,
			})
			return m.SaveCivoConfig(config)
		}
	}

	return fmt.Errorf("Civo organization '%s' not found", orgName)
}

// RemoveCivoNetwork removes a network by name from an organization
func (m *Manager) RemoveCivoNetwork(orgName, networkName string) error {
	config, err := m.LoadCivoConfig()
	if err != nil {
		return err
	}

	for i, o := range config.Organizations {
		if o.Name == orgName {
			filtered := []CivoNetwork{}
			found := false
			for _, n := range o.Networks {
				if n.Name != networkName {
					filtered = append(filtered, n)
				} else {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("network '%s' not found in organization '%s'", networkName, orgName)
			}
			config.Organizations[i].Networks = filtered
			return m.SaveCivoConfig(config)
		}
	}

	return fmt.Errorf("Civo organization '%s' not found", orgName)
}

// GetCivoNetworkID returns the network ID for a given network name in an organization
func (m *Manager) GetCivoNetworkID(orgName, networkName string) (string, error) {
	config, err := m.LoadCivoConfig()
	if err != nil {
		return "", err
	}

	for _, o := range config.Organizations {
		if o.Name == orgName {
			for _, n := range o.Networks {
				if n.Name == networkName {
					return n.NetworkID, nil
				}
			}
			return "", fmt.Errorf("network '%s' not found in organization '%s'", networkName, orgName)
		}
	}

	return "", fmt.Errorf("Civo organization '%s' not found", orgName)
}

// ListCivoNetworks returns all configured networks for an organization
func (m *Manager) ListCivoNetworks(orgName string) ([]CivoNetwork, error) {
	config, err := m.LoadCivoConfig()
	if err != nil {
		return nil, err
	}

	for _, o := range config.Organizations {
		if o.Name == orgName {
			return o.Networks, nil
		}
	}

	return nil, fmt.Errorf("Civo organization '%s' not found", orgName)
}
