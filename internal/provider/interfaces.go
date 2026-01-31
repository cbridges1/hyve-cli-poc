package provider

import (
	"context"
	"time"

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

// ClusterProvider interface defines the operations a cloud provider must implement
type ClusterProvider interface {
	// Cluster operations
	ListClusters(ctx context.Context) ([]*Cluster, error)
	GetCluster(ctx context.Context, clusterID string) (*Cluster, error)
	FindClusterByName(ctx context.Context, name string) (*Cluster, error)
	CreateCluster(ctx context.Context, config *ClusterConfig) (*Cluster, error)
	UpdateCluster(ctx context.Context, clusterID string, config *ClusterUpdateConfig) (*Cluster, error)
	DeleteCluster(ctx context.Context, clusterID string) error
	WaitForClusterReady(ctx context.Context, clusterID string) error
	GetClusterInfo(ctx context.Context, name string) (*ClusterInfo, error)
}

// FirewallProvider interface defines firewall operations
type FirewallProvider interface {
	ListFirewalls(ctx context.Context) ([]*Firewall, error)
	CreateFirewall(ctx context.Context, config *FirewallConfig) (*Firewall, error)
	DeleteFirewall(ctx context.Context, firewallID string) error
	FindFirewallByName(ctx context.Context, name string) (*Firewall, error)
}

// IngressProvider interface defines ingress operations
type IngressProvider interface {
	ListLoadBalancers(ctx context.Context) ([]*LoadBalancer, error)
	DeployIngressController(ctx context.Context, clusterID string, spec types.IngressSpec) (*LoadBalancer, error)
	RemoveIngressController(ctx context.Context, clusterID string) error
	GetLoadBalancerIP(ctx context.Context, clusterID string) (string, error)
}

// Provider combines all provider interfaces
type Provider interface {
	ClusterProvider
	FirewallProvider
	IngressProvider

	// Provider metadata
	Name() string
	Region() string
}
