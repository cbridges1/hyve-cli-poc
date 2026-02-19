package provider

import (
	"fmt"
	"os"
	"strings"

	"hyve/internal/context"
	"hyve/internal/credentials"
	"hyve/internal/provider/aws"
	"hyve/internal/provider/azure"
	"hyve/internal/provider/civo"
	"hyve/internal/provider/gcp"
)

// Factory creates provider instances
type Factory struct{}

// NewFactory creates a new provider factory
func NewFactory() *Factory {
	return &Factory{}
}

// CreateProvider creates a provider based on the provider name
// For Civo, the apiKey parameter is used directly (or loaded from credentials store).
// For AWS, GCP, and Azure, authentication uses the native CLI credentials:
//   - AWS: Uses AWS CLI credentials (~/.aws/credentials) or environment variables
//   - GCP: Uses gcloud CLI credentials (Application Default Credentials)
//   - Azure: Uses Azure CLI credentials (az login)
func (f *Factory) CreateProvider(providerName, apiKey, region string) (Provider, error) {
	switch strings.ToLower(providerName) {
	case "civo":
		// For Civo, use provided apiKey or load from credentials store
		token := apiKey
		if token == "" {
			// Try environment variable first
			token = os.Getenv("CIVO_TOKEN")
		}
		if token == "" {
			// Load token from secrets store using the current civo organization name
			credsMgr, err := credentials.NewManager()
			if err == nil {
				defer credsMgr.Close()
				orgName := getCivoOrgFromContext()
				if orgName != "" {
					token, _ = credsMgr.GetCivoToken(orgName)
				}
			}
		}
		if token == "" {
			return nil, fmt.Errorf("Civo API token not found. Please run 'hyve config use civo <org-name>' then 'hyve config civo set-token', or set CIVO_TOKEN environment variable")
		}
		civoProvider, err := civo.NewProvider(token, region)
		if err != nil {
			return nil, err
		}
		return &ProviderAdapter{civo: civoProvider}, nil

	case "aws":
		// AWS uses native CLI authentication via AWS SDK's default credential chain
		// This automatically checks: environment variables, ~/.aws/credentials, IAM roles, etc.
		// No credentials need to be stored in Hyve - use 'aws configure' to set up
		awsProvider, err := aws.NewProvider("", "", region)
		if err != nil {
			return nil, fmt.Errorf("AWS authentication failed. Please run 'aws configure' or set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables: %w", err)
		}
		return &ProviderAdapter{aws: awsProvider}, nil

	case "gcp":
		// GCP uses Application Default Credentials (ADC)
		// This automatically checks: GOOGLE_APPLICATION_CREDENTIALS, gcloud auth, metadata server
		// No credentials need to be stored in Hyve - use 'gcloud auth application-default login' to set up
		projectID := os.Getenv("GCP_PROJECT_ID")
		if projectID == "" {
			projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
		}
		if projectID == "" {
			return nil, fmt.Errorf("GCP project ID not found. Please set GCP_PROJECT_ID or GOOGLE_CLOUD_PROJECT environment variable")
		}
		gcpProvider, err := gcp.NewProvider("", projectID, region)
		if err != nil {
			return nil, fmt.Errorf("GCP authentication failed. Please run 'gcloud auth application-default login': %w", err)
		}
		return &ProviderAdapter{gcp: gcpProvider}, nil

	case "azure":
		// Azure uses DefaultAzureCredential which checks:
		// Environment variables, managed identity, Azure CLI, Azure PowerShell, etc.
		// No credentials need to be stored in Hyve - use 'az login' to set up
		subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
		if subscriptionID == "" {
			return nil, fmt.Errorf("Azure subscription ID not found. Please set AZURE_SUBSCRIPTION_ID environment variable")
		}
		resourceGroup := os.Getenv("AZURE_RESOURCE_GROUP")
		if resourceGroup == "" {
			return nil, fmt.Errorf("Azure resource group not found. Please set AZURE_RESOURCE_GROUP environment variable")
		}
		azureProvider, err := azure.NewProvider(subscriptionID, resourceGroup, region)
		if err != nil {
			return nil, fmt.Errorf("Azure authentication failed. Please run 'az login': %w", err)
		}
		return &ProviderAdapter{azure: azureProvider}, nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s. Valid providers are: civo, aws, gcp, azure", providerName)
	}
}

// CreateProviderWithOptions creates a provider with additional options
// For Civo, credentials are required in opts.APIKey or stored in credentials
// For AWS/GCP/Azure, native CLI authentication is used (options are for environment overrides only)
func (f *Factory) CreateProviderWithOptions(providerName string, opts ProviderOptions) (Provider, error) {
	switch strings.ToLower(providerName) {
	case "civo":
		token := opts.APIKey
		if token == "" {
			// Load token from secrets store using the current civo organization name
			credsMgr, err := credentials.NewManager()
			if err == nil {
				defer credsMgr.Close()
				orgName := getCivoOrgFromContext()
				if orgName != "" {
					token, _ = credsMgr.GetCivoToken(orgName)
				}
			}
		}
		if token == "" {
			token = os.Getenv("CIVO_TOKEN")
		}
		if token == "" {
			return nil, fmt.Errorf("Civo API token not found")
		}
		civoProvider, err := civo.NewProvider(token, opts.Region)
		if err != nil {
			return nil, err
		}
		return &ProviderAdapter{civo: civoProvider}, nil

	case "gcp":
		// GCP uses ADC - ProjectID can be passed or read from environment
		projectID := opts.ProjectID
		if projectID == "" {
			projectID = os.Getenv("GCP_PROJECT_ID")
		}
		if projectID == "" {
			projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
		}
		gcpProvider, err := gcp.NewProvider("", projectID, opts.Region)
		if err != nil {
			return nil, err
		}
		return &ProviderAdapter{gcp: gcpProvider}, nil

	case "aws":
		// AWS uses default credential chain - no explicit credentials needed
		awsProvider, err := aws.NewProvider("", "", opts.Region)
		if err != nil {
			return nil, err
		}
		return &ProviderAdapter{aws: awsProvider}, nil

	case "azure":
		// Azure uses DefaultAzureCredential - subscription/resource group from opts or env
		subscriptionID := opts.AzureSubscriptionID
		if subscriptionID == "" {
			subscriptionID = os.Getenv("AZURE_SUBSCRIPTION_ID")
		}
		resourceGroup := opts.AzureResourceGroup
		if resourceGroup == "" {
			resourceGroup = os.Getenv("AZURE_RESOURCE_GROUP")
		}
		azureProvider, err := azure.NewProvider(subscriptionID, resourceGroup, opts.Region)
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

	// Civo - requires API token stored in Hyve or environment
	APIKey string // Direct API key (optional, can load from credentials store)

	// GCP - uses gcloud CLI authentication (Application Default Credentials)
	ProjectID string // GCP project ID (can also be set via GCP_PROJECT_ID env var)

	// Azure - uses Azure CLI authentication (az login)
	AzureSubscriptionID string // Azure subscription ID (can also be set via AZURE_SUBSCRIPTION_ID env var)
	AzureResourceGroup  string // Azure resource group (can also be set via AZURE_RESOURCE_GROUP env var)

	// Note: AWS uses AWS CLI authentication automatically - no options needed
}

// GetSupportedProviders returns list of supported providers
func (f *Factory) GetSupportedProviders() []string {
	return []string{"civo", "gcp", "aws", "azure"}
}

// getCivoOrgFromContext reads the current Civo organization name from context
func getCivoOrgFromContext() string {
	ctxMgr, err := context.NewManager()
	if err != nil {
		return ""
	}
	return ctxMgr.GetCivoOrganization()
}
