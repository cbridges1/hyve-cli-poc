package gcp

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	container "google.golang.org/api/container/v1"
	"google.golang.org/api/option"

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

// Provider implements the provider interfaces for GCP
type Provider struct {
	containerService *container.Service
	projectID        string
	region           string
}

// NewProvider creates a new GCP provider
func NewProvider(credentialsJSON, projectID, region string) (*Provider, error) {
	ctx := context.Background()

	var svc *container.Service
	var err error

	if credentialsJSON != "" {
		svc, err = container.NewService(ctx, option.WithCredentialsJSON([]byte(credentialsJSON)))
	} else {
		// Use default credentials (ADC)
		svc, err = container.NewService(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create GCP container service: %w", err)
	}

	return &Provider{
		containerService: svc,
		projectID:        projectID,
		region:           region,
	}, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "gcp"
}

// Region returns the provider region
func (p *Provider) Region() string {
	return p.region
}

// clusterPath returns the full path for a cluster
func (p *Provider) clusterPath(clusterName string) string {
	return fmt.Sprintf("projects/%s/locations/%s/clusters/%s", p.projectID, p.region, clusterName)
}

// parentPath returns the parent path for listing clusters
func (p *Provider) parentPath() string {
	return fmt.Sprintf("projects/%s/locations/%s", p.projectID, p.region)
}

// ListClusters lists all clusters
func (p *Provider) ListClusters(ctx context.Context) ([]*Cluster, error) {
	resp, err := p.containerService.Projects.Locations.Clusters.List(p.parentPath()).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list GKE clusters: %w", err)
	}

	var clusters []*Cluster
	for _, c := range resp.Clusters {
		clusters = append(clusters, p.convertCluster(c))
	}

	return clusters, nil
}

// GetCluster gets a cluster by ID (name in GKE)
func (p *Provider) GetCluster(ctx context.Context, clusterID string) (*Cluster, error) {
	cluster, err := p.containerService.Projects.Locations.Clusters.Get(p.clusterPath(clusterID)).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get GKE cluster: %w", err)
	}

	return p.convertCluster(cluster), nil
}

// FindClusterByName finds a cluster by name
func (p *Provider) FindClusterByName(ctx context.Context, name string) (*Cluster, error) {
	cluster, err := p.containerService.Projects.Locations.Clusters.Get(p.clusterPath(name)).Context(ctx).Do()
	if err != nil {
		if strings.Contains(err.Error(), "notFound") || strings.Contains(err.Error(), "404") {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find GKE cluster: %w", err)
	}

	return p.convertCluster(cluster), nil
}

// CreateCluster creates a new cluster
func (p *Provider) CreateCluster(ctx context.Context, config *ClusterConfig) (*Cluster, error) {
	log.Printf("Creating GKE cluster %s in region %s", config.Name, p.region)

	// Determine machine type from nodes config
	machineType := "e2-medium"
	nodeCount := int64(len(config.Nodes))
	if nodeCount == 0 {
		nodeCount = 1
	}
	if len(config.Nodes) > 0 {
		machineType = config.Nodes[0]
	}

	createReq := &container.CreateClusterRequest{
		Cluster: &container.Cluster{
			Name:             config.Name,
			InitialNodeCount: nodeCount,
			NodeConfig: &container.NodeConfig{
				MachineType: machineType,
			},
		},
	}

	op, err := p.containerService.Projects.Locations.Clusters.Create(p.parentPath(), createReq).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create GKE cluster: %w", err)
	}

	log.Printf("GKE cluster creation started, operation: %s", op.Name)

	return &Cluster{
		ID:     config.Name,
		Name:   config.Name,
		Status: "PROVISIONING",
	}, nil
}

// UpdateCluster updates an existing cluster
func (p *Provider) UpdateCluster(ctx context.Context, clusterID string, config *ClusterUpdateConfig) (*Cluster, error) {
	// GKE cluster updates are complex - for now just return the current cluster
	// Real implementation would use SetNodePoolSize or UpdateCluster
	return p.GetCluster(ctx, clusterID)
}

// DeleteCluster deletes a cluster
func (p *Provider) DeleteCluster(ctx context.Context, clusterID string) error {
	_, err := p.containerService.Projects.Locations.Clusters.Delete(p.clusterPath(clusterID)).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to delete GKE cluster: %w", err)
	}
	return nil
}

// WaitForClusterReady waits for cluster to be ready
func (p *Provider) WaitForClusterReady(ctx context.Context, clusterID string) error {
	for {
		cluster, err := p.containerService.Projects.Locations.Clusters.Get(p.clusterPath(clusterID)).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to get cluster status: %w", err)
		}

		log.Printf("GKE cluster status: %s, waiting...", cluster.Status)

		if cluster.Status == "RUNNING" {
			break
		}

		if cluster.Status == "ERROR" || cluster.Status == "DEGRADED" {
			return fmt.Errorf("cluster creation failed with status: %s", cluster.Status)
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
	cluster, err := p.containerService.Projects.Locations.Clusters.Get(p.clusterPath(name)).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get GKE cluster info: %w", err)
	}

	return &ClusterInfo{
		Name:       cluster.Name,
		IPAddress:  cluster.Endpoint,
		AccessPort: "443",
		Status:     cluster.Status,
		ID:         cluster.Name,
	}, nil
}

// ListFirewalls lists all firewalls (not directly supported in GKE context)
func (p *Provider) ListFirewalls(ctx context.Context) ([]*Firewall, error) {
	// GKE manages firewall rules automatically
	return []*Firewall{}, nil
}

// CreateFirewall creates a firewall (GKE manages this automatically)
func (p *Provider) CreateFirewall(ctx context.Context, config *FirewallConfig) (*Firewall, error) {
	// GKE creates firewall rules automatically for clusters
	return &Firewall{
		ID:    config.Name,
		Name:  config.Name,
		Rules: config.Rules,
	}, nil
}

// DeleteFirewall deletes a firewall
func (p *Provider) DeleteFirewall(ctx context.Context, firewallID string) error {
	// GKE manages firewall rules automatically
	return nil
}

// FindFirewallByName finds a firewall by name
func (p *Provider) FindFirewallByName(ctx context.Context, name string) (*Firewall, error) {
	// GKE manages firewall rules automatically
	return nil, nil
}

// ListLoadBalancers lists all load balancers
func (p *Provider) ListLoadBalancers(ctx context.Context) ([]*LoadBalancer, error) {
	// Load balancers are managed by Kubernetes services in GKE
	return []*LoadBalancer{}, nil
}

// DeployIngressController deploys ingress controller
func (p *Provider) DeployIngressController(ctx context.Context, clusterID string, spec types.IngressSpec) (*LoadBalancer, error) {
	if !spec.LoadBalancer {
		return nil, nil
	}
	// GKE has built-in ingress controller
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

// convertCluster converts a GKE cluster to provider cluster
func (p *Provider) convertCluster(gkeCluster *container.Cluster) *Cluster {
	return &Cluster{
		ID:        gkeCluster.Name,
		Name:      gkeCluster.Name,
		Status:    gkeCluster.Status,
		MasterIP:  gkeCluster.Endpoint,
		CreatedAt: time.Now(), // GKE doesn't expose creation time in the same way
	}
}
