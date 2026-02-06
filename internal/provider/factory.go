package provider

import (
	"fmt"
	"strings"

	"civo-cluster-deploy/internal/config"
	"civo-cluster-deploy/internal/provider/aws"
	"civo-cluster-deploy/internal/provider/azure"
	"civo-cluster-deploy/internal/provider/civo"
	"civo-cluster-deploy/internal/provider/gcp"
)

// Factory creates provider instances
type Factory struct{}

// NewFactory creates a new provider factory
func NewFactory() *Factory {
	return &Factory{}
}

// CreateProvider creates a provider based on the provider name
// For Civo, the apiKey parameter is used directly.
// For other providers, credentials are loaded from config/environment.
func (f *Factory) CreateProvider(providerName, apiKey, region string) (Provider, error) {
	configMgr := config.NewManager()

	switch strings.ToLower(providerName) {
	case "civo":
		civoProvider, err := civo.NewProvider(apiKey, region)
		if err != nil {
			return nil, err
		}
		return &ProviderAdapter{civo: civoProvider}, nil
	case "aws":
		accessKeyID, secretAccessKey := configMgr.GetAWSCredentials()
		if accessKeyID == "" || secretAccessKey == "" {
			return nil, fmt.Errorf("AWS credentials not found. Please run 'hyve config set-token aws-access-key' and 'hyve config set-token aws-secret-key' or set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables")
		}
		awsProvider, err := aws.NewProvider(accessKeyID, secretAccessKey, region)
		if err != nil {
			return nil, err
		}
		return &ProviderAdapter{aws: awsProvider}, nil
	case "gcp":
		projectID, credentialsJSON := configMgr.GetGCPCredentials()
		if projectID == "" {
			return nil, fmt.Errorf("GCP project ID not found. Please run 'hyve config set-token gcp-project' or set GCP_PROJECT_ID environment variable")
		}
		gcpProvider, err := gcp.NewProvider(credentialsJSON, projectID, region)
		if err != nil {
			return nil, err
		}
		return &ProviderAdapter{gcp: gcpProvider}, nil
	case "azure":
		subscriptionID, resourceGroup := configMgr.GetAzureCredentials()
		if subscriptionID == "" {
			return nil, fmt.Errorf("Azure subscription ID not found. Please run 'hyve config set-token azure-subscription' or set AZURE_SUBSCRIPTION_ID environment variable")
		}
		if resourceGroup == "" {
			return nil, fmt.Errorf("Azure resource group not found. Please run 'hyve config set-token azure-resource-group' or set AZURE_RESOURCE_GROUP environment variable")
		}
		azureProvider, err := azure.NewProvider(subscriptionID, resourceGroup, region)
		if err != nil {
			return nil, err
		}
		return &ProviderAdapter{azure: azureProvider}, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s. Valid providers are: civo, aws, gcp, azure", providerName)
	}
}

// CreateProviderWithOptions creates a provider with additional options
func (f *Factory) CreateProviderWithOptions(providerName string, opts ProviderOptions) (Provider, error) {
	switch strings.ToLower(providerName) {
	case "civo":
		civoProvider, err := civo.NewProvider(opts.APIKey, opts.Region)
		if err != nil {
			return nil, err
		}
		return &ProviderAdapter{civo: civoProvider}, nil
	case "gcp":
		gcpProvider, err := gcp.NewProvider(opts.CredentialsJSON, opts.ProjectID, opts.Region)
		if err != nil {
			return nil, err
		}
		return &ProviderAdapter{gcp: gcpProvider}, nil
	case "aws":
		awsProvider, err := aws.NewProvider(opts.AWSAccessKeyID, opts.AWSSecretAccessKey, opts.Region)
		if err != nil {
			return nil, err
		}
		return &ProviderAdapter{aws: awsProvider}, nil
	case "azure":
		azureProvider, err := azure.NewProvider(opts.AzureSubscriptionID, opts.AzureResourceGroup, opts.Region)
		if err != nil {
			return nil, err
		}
		return &ProviderAdapter{azure: azureProvider}, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}
}

// ProviderOptions contains configuration options for creating providers
type ProviderOptions struct {
	// Common
	Region string // For all providers

	// Civo
	APIKey string

	// GCP
	CredentialsJSON string // Optional, uses ADC if empty
	ProjectID       string

	// AWS
	AWSAccessKeyID     string
	AWSSecretAccessKey string

	// Azure
	AzureSubscriptionID string
	AzureResourceGroup  string
}

// GetSupportedProviders returns list of supported providers
func (f *Factory) GetSupportedProviders() []string {
	return []string{"civo", "gcp", "aws", "azure"}
}
