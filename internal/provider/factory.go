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

// accountEnvVar looks up an environment variable using the naming pattern
// {accountName}-{provider}-{credential}, normalised to uppercase with underscores.
//
// Examples:
//   - ("main-account", "aws",   "access-key-id")    → MAIN_ACCOUNT_AWS_ACCESS_KEY_ID
//   - ("my-project",  "gcp",   "credentials-json")  → MY_PROJECT_GCP_CREDENTIALS_JSON
//   - ("prod",        "azure", "client-secret")      → PROD_AZURE_CLIENT_SECRET
//   - ("my-org",      "civo",  "token")              → MY_ORG_CIVO_TOKEN
//
// Returns an empty string when accountName is empty or the variable is not set.
func accountEnvVar(accountName, providerName, credential string) string {
	if accountName == "" {
		return ""
	}
	replacer := strings.NewReplacer("-", "_", " ", "_", ".", "_")
	key := strings.ToUpper(replacer.Replace(accountName)) +
		"_" + strings.ToUpper(replacer.Replace(providerName)) +
		"_" + strings.ToUpper(replacer.Replace(credential))
	return os.Getenv(key)
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
		awsProvider, err := aws.NewProvider("", "", "", region)
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
		azureProvider, err := azure.NewProvider(subscriptionID, resourceGroup, region, "", "", "")
		if err != nil {
			return nil, fmt.Errorf("Azure authentication failed. Please run 'az login': %w", err)
		}
		return &ProviderAdapter{azure: azureProvider}, nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s. Valid providers are: civo, aws, gcp, azure", providerName)
	}
}

// CreateProviderWithOptions creates a provider with additional options.
//
// When opts.AccountName is set, named environment variables are checked first using the
// pattern {ACCOUNT_NAME}_{PROVIDER}_{CREDENTIAL} (hyphens replaced with underscores, uppercase).
// This allows CI/CD pipelines to supply per-account credentials without changing code.
//
// GCP project IDs and Azure subscription IDs are resolved from the provider YAML config files
// (provider-configs/gcp.yaml, provider-configs/azure.yaml) by the caller before this function
// is invoked, so those values should already be present in opts.ProjectID / opts.AzureSubscriptionID.
//
// Examples with AccountName = "main-account":
//   - AWS:   MAIN_ACCOUNT_AWS_ACCESS_KEY_ID, MAIN_ACCOUNT_AWS_SECRET_ACCESS_KEY, MAIN_ACCOUNT_AWS_SESSION_TOKEN
//   - GCP:   MAIN_ACCOUNT_GCP_CREDENTIALS_JSON  (project ID comes from provider-configs/gcp.yaml)
//   - Azure: MAIN_ACCOUNT_AZURE_TENANT_ID, MAIN_ACCOUNT_AZURE_CLIENT_ID,
//     MAIN_ACCOUNT_AZURE_CLIENT_SECRET, MAIN_ACCOUNT_AZURE_RESOURCE_GROUP
//     (subscription ID comes from provider-configs/azure.yaml)
//   - Civo:  MAIN_ACCOUNT_CIVO_TOKEN
func (f *Factory) CreateProviderWithOptions(providerName string, opts ProviderOptions) (Provider, error) {
	switch strings.ToLower(providerName) {
	case "civo":
		token := opts.APIKey

		// Check named env var first (e.g. MY_ORG_CIVO_TOKEN)
		if token == "" {
			token = accountEnvVar(opts.AccountName, "civo", "token")
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
		// Named env var for credentials JSON; project ID is resolved from provider-configs/gcp.yaml
		// by the caller before reaching here and passed via opts.ProjectID.
		credentialsJSON := accountEnvVar(opts.AccountName, "gcp", "credentials-json")

		projectID := opts.ProjectID
		if projectID == "" {
			projectID = os.Getenv("GCP_PROJECT_ID")
		}
		if projectID == "" {
			projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
		}

		gcpProvider, err := gcp.NewProvider(credentialsJSON, projectID, opts.Region)
		if err != nil {
			return nil, err
		}
		return &ProviderAdapter{gcp: gcpProvider}, nil

	case "aws":
		// Named env vars take priority; fall back to AWS SDK default credential chain.
		accessKeyID := accountEnvVar(opts.AccountName, "aws", "access-key-id")
		secretAccessKey := accountEnvVar(opts.AccountName, "aws", "secret-access-key")
		sessionToken := accountEnvVar(opts.AccountName, "aws", "session-token")

		awsProvider, err := aws.NewProvider(accessKeyID, secretAccessKey, sessionToken, opts.Region)
		if err != nil {
			return nil, err
		}
		return &ProviderAdapter{aws: awsProvider}, nil

	case "azure":
		// Subscription ID is resolved from provider-configs/azure.yaml by the caller before
		// reaching here and passed via opts.AzureSubscriptionID. Named env vars supply
		// credentials (tenant/client/secret) and the resource group.
		subscriptionID := opts.AzureSubscriptionID
		if subscriptionID == "" {
			subscriptionID = os.Getenv("AZURE_SUBSCRIPTION_ID")
		}

		resourceGroup := opts.AzureResourceGroup
		if resourceGroup == "" {
			resourceGroup = accountEnvVar(opts.AccountName, "azure", "resource-group")
		}
		if resourceGroup == "" {
			resourceGroup = os.Getenv("AZURE_RESOURCE_GROUP")
		}

		tenantID := accountEnvVar(opts.AccountName, "azure", "tenant-id")
		clientID := accountEnvVar(opts.AccountName, "azure", "client-id")
		clientSecret := accountEnvVar(opts.AccountName, "azure", "client-secret")

		azureProvider, err := azure.NewProvider(subscriptionID, resourceGroup, opts.Region, tenantID, clientID, clientSecret)
		if err != nil {
			return nil, err
		}
		return &ProviderAdapter{azure: azureProvider}, nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}
}

// ProviderOptions contains configuration options for creating providers.
type ProviderOptions struct {
	// AccountName is the alias used for named environment variable lookups.
	// When set, the factory checks {ACCOUNT_NAME}_{PROVIDER}_{CREDENTIAL} env vars before
	// falling back to the standard credential chain.
	AccountName string

	// Common
	Region string // For all providers

	// Civo - requires API token stored in Hyve or environment
	APIKey string // Direct API key (optional, can load from credentials store)

	// GCP - uses gcloud CLI authentication (Application Default Credentials)
	ProjectID string // GCP project ID (can also be set via GCP_PROJECT_ID env var)

	// Azure - uses Azure CLI authentication (az login)
	AzureSubscriptionID string // Azure subscription ID (can also be set via AZURE_SUBSCRIPTION_ID env var)
	AzureResourceGroup  string // Azure resource group (can also be set via AZURE_RESOURCE_GROUP env var)

	// Note: AWS uses AWS CLI authentication automatically when no named env vars are found
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
