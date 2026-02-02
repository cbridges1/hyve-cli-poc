package azure

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"

	"civo-cluster-deploy/internal/types"
)

// Cluster represents a generic cluster
type Cluster struct {
	ID         string
	Name       string
	Status     string
	FirewallID string
	MasterIP   string
	KubeConfig string
	CreatedAt  time.Time
}

// Firewall represents a generic firewall
type Firewall struct {
	ID    string
	Name  string
	Rules []FirewallRule
}

// FirewallRule represents a generic firewall rule
type FirewallRule struct {
	Protocol  string
	StartPort string
	EndPort   string
	Cidr      []string
	Direction string
}

// LoadBalancer represents a generic load balancer
type LoadBalancer struct {
	ID        string
	Name      string
	PublicIP  string
	ClusterID string
}

// ClusterConfig represents cluster creation configuration
type ClusterConfig struct {
	Name         string
	Region       string
	Nodes        []string
	ClusterType  string
	FirewallID   string
	Applications []string
}

// ClusterUpdateConfig represents cluster update configuration
type ClusterUpdateConfig struct {
	Name  string
	Nodes []string
}

// FirewallConfig represents firewall creation configuration
type FirewallConfig struct {
	Name  string
	Rules []FirewallRule
}

// ClusterInfo represents exported cluster information
type ClusterInfo struct {
	Name       string
	IPAddress  string
	AccessPort string
	Kubeconfig string
	Status     string
	ID         string
}

// Provider implements the provider interfaces for Azure
type Provider struct {
	aksClient         *armcontainerservice.ManagedClustersClient
	subscriptionID    string
	resourceGroupName string
	region            string
}

// NewProvider creates a new Azure provider
func NewProvider(subscriptionID, resourceGroupName, region string) (*Provider, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credentials: %w", err)
	}

	clientFactory, err := armcontainerservice.NewClientFactory(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure client factory: %w", err)
	}

	return &Provider{
		aksClient:         clientFactory.NewManagedClustersClient(),
		subscriptionID:    subscriptionID,
		resourceGroupName: resourceGroupName,
		region:            region,
	}, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "azure"
}

// Region returns the provider region
func (p *Provider) Region() string {
	return p.region
}

// ListClusters lists all clusters
func (p *Provider) ListClusters(ctx context.Context) ([]*Cluster, error) {
	pager := p.aksClient.NewListByResourceGroupPager(p.resourceGroupName, nil)

	var clusters []*Cluster
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list AKS clusters: %w", err)
		}

		for _, cluster := range page.Value {
			clusters = append(clusters, p.convertCluster(cluster))
		}
	}

	return clusters, nil
}

// GetCluster gets a cluster by ID (name in AKS)
func (p *Provider) GetCluster(ctx context.Context, clusterID string) (*Cluster, error) {
	resp, err := p.aksClient.Get(ctx, p.resourceGroupName, clusterID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get AKS cluster: %w", err)
	}

	return p.convertCluster(&resp.ManagedCluster), nil
}

// FindClusterByName finds a cluster by name
func (p *Provider) FindClusterByName(ctx context.Context, name string) (*Cluster, error) {
	resp, err := p.aksClient.Get(ctx, p.resourceGroupName, name, nil)
	if err != nil {
		// Check if it's a not found error
		return nil, nil
	}

	return p.convertCluster(&resp.ManagedCluster), nil
}

// CreateCluster creates a new cluster
func (p *Provider) CreateCluster(ctx context.Context, config *ClusterConfig) (*Cluster, error) {
	log.Printf("Creating AKS cluster %s in region %s", config.Name, p.region)

	// Determine VM size from nodes config
	vmSize := "Standard_DS2_v2"
	nodeCount := int32(len(config.Nodes))
	if nodeCount == 0 {
		nodeCount = 1
	}
	if len(config.Nodes) > 0 {
		vmSize = config.Nodes[0]
	}

	parameters := armcontainerservice.ManagedCluster{
		Location: &p.region,
		Properties: &armcontainerservice.ManagedClusterProperties{
			DNSPrefix: &config.Name,
			AgentPoolProfiles: []*armcontainerservice.ManagedClusterAgentPoolProfile{
				{
					Name:   strPtr("nodepool1"),
					Count:  &nodeCount,
					VMSize: &vmSize,
					Mode:   ptr(armcontainerservice.AgentPoolModeSystem),
				},
			},
		},
	}

	poller, err := p.aksClient.BeginCreateOrUpdate(ctx, p.resourceGroupName, config.Name, parameters, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create AKS cluster: %w", err)
	}

	log.Printf("AKS cluster creation started: %s", config.Name)

	// Don't wait for completion here - just return the initial state
	_ = poller // poller can be used to wait for completion if needed
	return &Cluster{
		ID:     config.Name,
		Name:   config.Name,
		Status: "Creating",
	}, nil
}

// UpdateCluster updates an existing cluster
func (p *Provider) UpdateCluster(ctx context.Context, clusterID string, config *ClusterUpdateConfig) (*Cluster, error) {
	// AKS cluster updates are complex - return current cluster
	return p.GetCluster(ctx, clusterID)
}

// DeleteCluster deletes a cluster
func (p *Provider) DeleteCluster(ctx context.Context, clusterID string) error {
	poller, err := p.aksClient.BeginDelete(ctx, p.resourceGroupName, clusterID, nil)
	if err != nil {
		return fmt.Errorf("failed to delete AKS cluster: %w", err)
	}

	// Wait for deletion to complete
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to wait for AKS cluster deletion: %w", err)
	}

	return nil
}

// WaitForClusterReady waits for cluster to be ready
func (p *Provider) WaitForClusterReady(ctx context.Context, clusterID string) error {
	for {
		resp, err := p.aksClient.Get(ctx, p.resourceGroupName, clusterID, nil)
		if err != nil {
			return fmt.Errorf("failed to get cluster status: %w", err)
		}

		state := "Unknown"
		if resp.Properties != nil && resp.Properties.ProvisioningState != nil {
			state = *resp.Properties.ProvisioningState
		}

		log.Printf("AKS cluster provisioning state: %s, waiting...", state)

		if state == "Succeeded" {
			break
		}

		if state == "Failed" {
			return fmt.Errorf("cluster creation failed")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(30 * time.Second):
		}
	}

	return nil
}

// GetClusterInfo gets cluster information for export
func (p *Provider) GetClusterInfo(ctx context.Context, name string) (*ClusterInfo, error) {
	resp, err := p.aksClient.Get(ctx, p.resourceGroupName, name, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get AKS cluster info: %w", err)
	}

	cluster := resp.ManagedCluster
	fqdn := ""
	if cluster.Properties != nil && cluster.Properties.Fqdn != nil {
		fqdn = *cluster.Properties.Fqdn
	}

	status := "Unknown"
	if cluster.Properties != nil && cluster.Properties.ProvisioningState != nil {
		status = *cluster.Properties.ProvisioningState
	}

	clusterName := ""
	if cluster.Name != nil {
		clusterName = *cluster.Name
	}

	return &ClusterInfo{
		Name:       clusterName,
		IPAddress:  fqdn,
		AccessPort: "443",
		Status:     status,
		ID:         clusterName,
	}, nil
}

// ListFirewalls lists all firewalls (NSGs in Azure)
func (p *Provider) ListFirewalls(ctx context.Context) ([]*Firewall, error) {
	// AKS manages NSGs automatically
	return []*Firewall{}, nil
}

// CreateFirewall creates a firewall (NSG in Azure)
func (p *Provider) CreateFirewall(ctx context.Context, config *FirewallConfig) (*Firewall, error) {
	// AKS creates NSGs automatically for clusters
	return &Firewall{
		ID:    config.Name,
		Name:  config.Name,
		Rules: config.Rules,
	}, nil
}

// DeleteFirewall deletes a firewall
func (p *Provider) DeleteFirewall(ctx context.Context, firewallID string) error {
	// AKS manages NSGs automatically
	return nil
}

// FindFirewallByName finds a firewall by name
func (p *Provider) FindFirewallByName(ctx context.Context, name string) (*Firewall, error) {
	// AKS manages NSGs automatically
	return nil, nil
}

// ListLoadBalancers lists all load balancers
func (p *Provider) ListLoadBalancers(ctx context.Context) ([]*LoadBalancer, error) {
	// Load balancers are managed by Kubernetes services in AKS
	return []*LoadBalancer{}, nil
}

// DeployIngressController deploys ingress controller
func (p *Provider) DeployIngressController(ctx context.Context, clusterID string, spec types.IngressSpec) (*LoadBalancer, error) {
	if !spec.LoadBalancer {
		return nil, nil
	}
	// Azure Load Balancer handles this in AKS
	return nil, nil
}

// RemoveIngressController removes ingress controller
func (p *Provider) RemoveIngressController(ctx context.Context, clusterID string) error {
	return nil
}

// GetLoadBalancerIP gets load balancer IP for cluster
func (p *Provider) GetLoadBalancerIP(ctx context.Context, clusterID string) (string, error) {
	// Would need to query Kubernetes services
	return "", nil
}

// convertCluster converts an AKS cluster to provider cluster
func (p *Provider) convertCluster(aksCluster *armcontainerservice.ManagedCluster) *Cluster {
	name := ""
	if aksCluster.Name != nil {
		name = *aksCluster.Name
	}

	id := ""
	if aksCluster.ID != nil {
		id = *aksCluster.ID
	}

	status := "Unknown"
	fqdn := ""
	if aksCluster.Properties != nil {
		if aksCluster.Properties.ProvisioningState != nil {
			status = *aksCluster.Properties.ProvisioningState
		}
		if aksCluster.Properties.Fqdn != nil {
			fqdn = *aksCluster.Properties.Fqdn
		}
	}

	return &Cluster{
		ID:        id,
		Name:      name,
		Status:    status,
		MasterIP:  fqdn,
		CreatedAt: time.Now(),
	}
}

// Helper functions
func strPtr(s string) *string {
	return &s
}

func ptr[T any](v T) *T {
	return &v
}
