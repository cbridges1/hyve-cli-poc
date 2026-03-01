package ingress

import (
	"context"

	"hyve/internal/provider"
	"hyve/internal/types"
)

// Manager handles ingress operations using a generic provider
type Manager struct {
	provider provider.IngressProvider
}

// NewManager creates a new ingress manager
func NewManager(p provider.Provider) *Manager {
	return &Manager{
		provider: p,
	}
}

// DeployIngressController deploys an ingress controller
func (m *Manager) DeployIngressController(ctx context.Context, clusterID string, spec types.IngressSpec) (*provider.LoadBalancer, error) {
	return m.provider.DeployIngressController(ctx, clusterID, spec)
}

// RemoveIngressController removes an ingress controller
func (m *Manager) RemoveIngressController(ctx context.Context, clusterID string) error {
	return m.provider.RemoveIngressController(ctx, clusterID)
}

// GetLoadBalancerIP gets the load balancer IP for a cluster
func (m *Manager) GetLoadBalancerIP(ctx context.Context, clusterID string) (string, error) {
	return m.provider.GetLoadBalancerIP(ctx, clusterID)
}
