package provider

import (
	"fmt"
	"strings"

	"civo-cluster-deploy/internal/provider/civo"
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

// GetSupportedProviders returns list of supported providers
func (f *Factory) GetSupportedProviders() []string {
	return []string{"civo"}
}
