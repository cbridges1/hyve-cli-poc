package aws

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

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

// Provider implements the provider interfaces for AWS
type Provider struct {
	eksClient *eks.Client
	region    string
}

// NewProvider creates a new AWS provider
func NewProvider(accessKeyID, secretAccessKey, region string) (*Provider, error) {
	ctx := context.Background()

	var opts []func(*config.LoadOptions) error
	opts = append(opts, config.WithRegion(region))

	if accessKeyID != "" && secretAccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	eksClient := eks.NewFromConfig(cfg)

	return &Provider{
		eksClient: eksClient,
		region:    region,
	}, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "aws"
}

// Region returns the provider region
func (p *Provider) Region() string {
	return p.region
}

// ListClusters lists all clusters
func (p *Provider) ListClusters(ctx context.Context) ([]*Cluster, error) {
	resp, err := p.eksClient.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list EKS clusters: %w", err)
	}

	var clusters []*Cluster
	for _, name := range resp.Clusters {
		cluster, err := p.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: &name})
		if err != nil {
			continue
		}
		clusters = append(clusters, p.convertCluster(cluster.Cluster))
	}

	return clusters, nil
}

// GetCluster gets a cluster by ID (name in EKS)
func (p *Provider) GetCluster(ctx context.Context, clusterID string) (*Cluster, error) {
	resp, err := p.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: &clusterID})
	if err != nil {
		return nil, fmt.Errorf("failed to get EKS cluster: %w", err)
	}

	return p.convertCluster(resp.Cluster), nil
}

// FindClusterByName finds a cluster by name
func (p *Provider) FindClusterByName(ctx context.Context, name string) (*Cluster, error) {
	resp, err := p.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: &name})
	if err != nil {
		// Check if it's a not found error
		return nil, nil
	}

	return p.convertCluster(resp.Cluster), nil
}

// CreateCluster creates a new cluster
func (p *Provider) CreateCluster(ctx context.Context, config *ClusterConfig) (*Cluster, error) {
	log.Printf("Creating EKS cluster %s in region %s", config.Name, p.region)

	// Note: EKS cluster creation requires additional parameters like roleArn and subnets
	// This is a simplified version - real implementation needs VPC/subnet configuration
	createInput := &eks.CreateClusterInput{
		Name: &config.Name,
		// RoleArn and ResourcesVpcConfig would need to be provided
	}

	resp, err := p.eksClient.CreateCluster(ctx, createInput)
	if err != nil {
		return nil, fmt.Errorf("failed to create EKS cluster: %w", err)
	}

	log.Printf("EKS cluster creation started: %s", *resp.Cluster.Name)

	return p.convertCluster(resp.Cluster), nil
}

// UpdateCluster updates an existing cluster
func (p *Provider) UpdateCluster(ctx context.Context, clusterID string, config *ClusterUpdateConfig) (*Cluster, error) {
	// EKS cluster updates are limited - return current cluster
	return p.GetCluster(ctx, clusterID)
}

// DeleteCluster deletes a cluster
func (p *Provider) DeleteCluster(ctx context.Context, clusterID string) error {
	_, err := p.eksClient.DeleteCluster(ctx, &eks.DeleteClusterInput{Name: &clusterID})
	if err != nil {
		return fmt.Errorf("failed to delete EKS cluster: %w", err)
	}
	return nil
}

// WaitForClusterReady waits for cluster to be ready
func (p *Provider) WaitForClusterReady(ctx context.Context, clusterID string) error {
	for {
		resp, err := p.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: &clusterID})
		if err != nil {
			return fmt.Errorf("failed to get cluster status: %w", err)
		}

		status := resp.Cluster.Status
		log.Printf("EKS cluster status: %s, waiting...", status)

		if status == ekstypes.ClusterStatusActive {
			break
		}

		if status == ekstypes.ClusterStatusFailed {
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
	resp, err := p.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: &name})
	if err != nil {
		return nil, fmt.Errorf("failed to get EKS cluster info: %w", err)
	}

	cluster := resp.Cluster
	endpoint := ""
	if cluster.Endpoint != nil {
		endpoint = *cluster.Endpoint
	}

	return &ClusterInfo{
		Name:       *cluster.Name,
		IPAddress:  endpoint,
		AccessPort: "443",
		Status:     string(cluster.Status),
		ID:         *cluster.Name,
	}, nil
}

// ListFirewalls lists all firewalls (security groups in AWS)
func (p *Provider) ListFirewalls(ctx context.Context) ([]*Firewall, error) {
	// EKS manages security groups automatically
	return []*Firewall{}, nil
}

// CreateFirewall creates a firewall (security group in AWS)
func (p *Provider) CreateFirewall(ctx context.Context, config *FirewallConfig) (*Firewall, error) {
	// EKS creates security groups automatically for clusters
	return &Firewall{
		ID:    config.Name,
		Name:  config.Name,
		Rules: config.Rules,
	}, nil
}

// DeleteFirewall deletes a firewall
func (p *Provider) DeleteFirewall(ctx context.Context, firewallID string) error {
	// EKS manages security groups automatically
	return nil
}

// FindFirewallByName finds a firewall by name
func (p *Provider) FindFirewallByName(ctx context.Context, name string) (*Firewall, error) {
	// EKS manages security groups automatically
	return nil, nil
}

// ListLoadBalancers lists all load balancers
func (p *Provider) ListLoadBalancers(ctx context.Context) ([]*LoadBalancer, error) {
	// Load balancers are managed by Kubernetes services in EKS
	return []*LoadBalancer{}, nil
}

// DeployIngressController deploys ingress controller
func (p *Provider) DeployIngressController(ctx context.Context, clusterID string, spec types.IngressSpec) (*LoadBalancer, error) {
	if !spec.LoadBalancer {
		return nil, nil
	}
	// AWS Load Balancer Controller handles this in EKS
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

// convertCluster converts an EKS cluster to provider cluster
func (p *Provider) convertCluster(eksCluster *ekstypes.Cluster) *Cluster {
	name := ""
	if eksCluster.Name != nil {
		name = *eksCluster.Name
	}

	endpoint := ""
	if eksCluster.Endpoint != nil {
		endpoint = *eksCluster.Endpoint
	}

	var createdAt time.Time
	if eksCluster.CreatedAt != nil {
		createdAt = *eksCluster.CreatedAt
	}

	return &Cluster{
		ID:        name,
		Name:      name,
		Status:    string(eksCluster.Status),
		MasterIP:  endpoint,
		CreatedAt: createdAt,
	}
}
