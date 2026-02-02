package provider

import (
	"fmt"
	"strings"

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
func (f *Factory) CreateProvider(providerName, apiKey, region string) (Provider, error) {
	switch strings.ToLower(providerName) {
	case "civo":
		civoProvider, err := civo.NewProvider(apiKey, region)
		if err != nil {
			return nil, err
		}
		return &ProviderAdapter{civo: civoProvider}, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
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
