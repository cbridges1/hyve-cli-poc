package provider

import (
	"context"

	"civo-cluster-deploy/internal/provider/civo"
	"civo-cluster-deploy/internal/types"
)

// ProviderAdapter adapts the Civo provider to the generic provider interface
type ProviderAdapter struct {
	civo *civo.Provider
}

// Name returns the provider name
func (a *ProviderAdapter) Name() string {
	return a.civo.Name()
}

// Region returns the provider region
func (a *ProviderAdapter) Region() string {
	return a.civo.Region()
}

// ListClusters lists all clusters
func (a *ProviderAdapter) ListClusters(ctx context.Context) ([]*Cluster, error) {
	civoClusters, err := a.civo.ListClusters(ctx)
	if err != nil {
		return nil, err
	}

	var clusters []*Cluster
	for _, c := range civoClusters {
		clusters = append(clusters, convertCivoCluster(c))
	}

	return clusters, nil
}

// GetCluster gets a cluster by ID
func (a *ProviderAdapter) GetCluster(ctx context.Context, clusterID string) (*Cluster, error) {
	civoCluster, err := a.civo.GetCluster(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	return convertCivoCluster(civoCluster), nil
}

// FindClusterByName finds a cluster by name
func (a *ProviderAdapter) FindClusterByName(ctx context.Context, name string) (*Cluster, error) {
	civoCluster, err := a.civo.FindClusterByName(ctx, name)
	if err != nil {
		return nil, err
	}

	if civoCluster == nil {
		return nil, nil
	}

	return convertCivoCluster(civoCluster), nil
}

// CreateCluster creates a new cluster
func (a *ProviderAdapter) CreateCluster(ctx context.Context, config *ClusterConfig) (*Cluster, error) {
	civoConfig := &civo.ClusterConfig{
		Name:         config.Name,
		Region:       config.Region,
		Nodes:        config.Nodes,
		ClusterType:  config.ClusterType,
		FirewallID:   config.FirewallID,
		Applications: config.Applications,
	}

	civoCluster, err := a.civo.CreateCluster(ctx, civoConfig)
	if err != nil {
		return nil, err
	}

	return convertCivoCluster(civoCluster), nil
}

// UpdateCluster updates an existing cluster
func (a *ProviderAdapter) UpdateCluster(ctx context.Context, clusterID string, config *ClusterUpdateConfig) (*Cluster, error) {
	civoConfig := &civo.ClusterUpdateConfig{
		Name:  config.Name,
		Nodes: config.Nodes,
	}

	civoCluster, err := a.civo.UpdateCluster(ctx, clusterID, civoConfig)
	if err != nil {
		return nil, err
	}

	return convertCivoCluster(civoCluster), nil
}

// DeleteCluster deletes a cluster
func (a *ProviderAdapter) DeleteCluster(ctx context.Context, clusterID string) error {
	return a.civo.DeleteCluster(ctx, clusterID)
}

// WaitForClusterReady waits for cluster to be ready
func (a *ProviderAdapter) WaitForClusterReady(ctx context.Context, clusterID string) error {
	return a.civo.WaitForClusterReady(ctx, clusterID)
}

// GetClusterInfo gets cluster information for export
func (a *ProviderAdapter) GetClusterInfo(ctx context.Context, name string) (*ClusterInfo, error) {
	civoInfo, err := a.civo.GetClusterInfo(ctx, name)
	if err != nil {
		return nil, err
	}

	return &ClusterInfo{
		Name:       civoInfo.Name,
		IPAddress:  civoInfo.IPAddress,
		AccessPort: civoInfo.AccessPort,
		Kubeconfig: civoInfo.Kubeconfig,
		Status:     civoInfo.Status,
		ID:         civoInfo.ID,
	}, nil
}

// ListFirewalls lists all firewalls
func (a *ProviderAdapter) ListFirewalls(ctx context.Context) ([]*Firewall, error) {
	civoFirewalls, err := a.civo.ListFirewalls(ctx)
	if err != nil {
		return nil, err
	}

	var firewalls []*Firewall
	for _, f := range civoFirewalls {
		firewalls = append(firewalls, convertCivoFirewall(f))
	}

	return firewalls, nil
}

// CreateFirewall creates a firewall
func (a *ProviderAdapter) CreateFirewall(ctx context.Context, config *FirewallConfig) (*Firewall, error) {
	civoConfig := &civo.FirewallConfig{
		Name:  config.Name,
		Rules: convertFirewallRules(config.Rules),
	}

	civoFirewall, err := a.civo.CreateFirewall(ctx, civoConfig)
	if err != nil {
		return nil, err
	}

	return convertCivoFirewall(civoFirewall), nil
}

// DeleteFirewall deletes a firewall
func (a *ProviderAdapter) DeleteFirewall(ctx context.Context, firewallID string) error {
	return a.civo.DeleteFirewall(ctx, firewallID)
}

// FindFirewallByName finds a firewall by name
func (a *ProviderAdapter) FindFirewallByName(ctx context.Context, name string) (*Firewall, error) {
	civoFirewall, err := a.civo.FindFirewallByName(ctx, name)
	if err != nil {
		return nil, err
	}

	if civoFirewall == nil {
		return nil, nil
	}

	return convertCivoFirewall(civoFirewall), nil
}

// ListLoadBalancers lists all load balancers
func (a *ProviderAdapter) ListLoadBalancers(ctx context.Context) ([]*LoadBalancer, error) {
	civoLBs, err := a.civo.ListLoadBalancers(ctx)
	if err != nil {
		return nil, err
	}

	var lbs []*LoadBalancer
	for _, lb := range civoLBs {
		lbs = append(lbs, &LoadBalancer{
			ID:        lb.ID,
			Name:      lb.Name,
			PublicIP:  lb.PublicIP,
			ClusterID: lb.ClusterID,
		})
	}

	return lbs, nil
}

// DeployIngressController deploys ingress controller
func (a *ProviderAdapter) DeployIngressController(ctx context.Context, clusterID string, spec types.IngressSpec) (*LoadBalancer, error) {
	civoLB, err := a.civo.DeployIngressController(ctx, clusterID, spec)
	if err != nil {
		return nil, err
	}

	if civoLB == nil {
		return nil, nil
	}

	return &LoadBalancer{
		ID:        civoLB.ID,
		Name:      civoLB.Name,
		PublicIP:  civoLB.PublicIP,
		ClusterID: civoLB.ClusterID,
	}, nil
}

// RemoveIngressController removes ingress controller
func (a *ProviderAdapter) RemoveIngressController(ctx context.Context, clusterID string) error {
	return a.civo.RemoveIngressController(ctx, clusterID)
}

// GetLoadBalancerIP gets load balancer IP for cluster
func (a *ProviderAdapter) GetLoadBalancerIP(ctx context.Context, clusterID string) (string, error) {
	return a.civo.GetLoadBalancerIP(ctx, clusterID)
}

// Helper functions for conversion

func convertCivoCluster(c *civo.Cluster) *Cluster {
	return &Cluster{
		ID:         c.ID,
		Name:       c.Name,
		Status:     c.Status,
		FirewallID: c.FirewallID,
		MasterIP:   c.MasterIP,
		KubeConfig: c.KubeConfig,
		CreatedAt:  c.CreatedAt,
	}
}

func convertCivoFirewall(f *civo.Firewall) *Firewall {
	var rules []FirewallRule
	for _, rule := range f.Rules {
		rules = append(rules, FirewallRule{
			Protocol:  rule.Protocol,
			StartPort: rule.StartPort,
			EndPort:   rule.EndPort,
			Cidr:      rule.Cidr,
			Direction: rule.Direction,
		})
	}

	return &Firewall{
		ID:    f.ID,
		Name:  f.Name,
		Rules: rules,
	}
}

func convertFirewallRules(rules []FirewallRule) []civo.FirewallRule {
	var civoRules []civo.FirewallRule
	for _, rule := range rules {
		civoRules = append(civoRules, civo.FirewallRule{
			Protocol:  rule.Protocol,
			StartPort: rule.StartPort,
			EndPort:   rule.EndPort,
			Cidr:      rule.Cidr,
			Direction: rule.Direction,
		})
	}

	return civoRules
}
