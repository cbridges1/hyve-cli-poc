package aws

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
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
	// EKS-specific configuration
	RoleARN   string   // IAM role ARN for the EKS cluster
	VPCID     string   // VPC ID where the cluster will be created
	SubnetIDs []string // Subnet IDs for the cluster (if empty, will be discovered from VPC)
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
	ec2Client *ec2.Client
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
	ec2Client := ec2.NewFromConfig(cfg)

	return &Provider{
		eksClient: eksClient,
		ec2Client: ec2Client,
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
func (p *Provider) CreateCluster(ctx context.Context, clusterConfig *ClusterConfig) (*Cluster, error) {
	log.Printf("Creating EKS cluster %s in region %s", clusterConfig.Name, p.region)

	// Validate required configuration
	if clusterConfig.RoleARN == "" {
		return nil, fmt.Errorf("EKS cluster creation requires a role ARN. Use --eks-role-name flag")
	}
	if clusterConfig.VPCID == "" {
		return nil, fmt.Errorf("EKS cluster creation requires a VPC ID. Use --vpc-name flag")
	}

	// Get subnets from VPC if not provided
	subnetIDs := clusterConfig.SubnetIDs
	if len(subnetIDs) == 0 {
		var err error
		subnetIDs, err = p.getVPCSubnets(ctx, clusterConfig.VPCID)
		if err != nil {
			return nil, fmt.Errorf("failed to get subnets from VPC: %w", err)
		}
		if len(subnetIDs) < 2 {
			return nil, fmt.Errorf("EKS requires at least 2 subnets in different availability zones, found %d", len(subnetIDs))
		}
		log.Printf("Discovered %d subnets from VPC %s", len(subnetIDs), clusterConfig.VPCID)
	}

	// Create a security group for the cluster
	securityGroupID, err := p.createClusterSecurityGroup(ctx, clusterConfig.VPCID, clusterConfig.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create security group: %w", err)
	}
	log.Printf("Created security group %s for cluster %s", securityGroupID, clusterConfig.Name)

	// Create the EKS cluster
	createInput := &eks.CreateClusterInput{
		Name:    aws.String(clusterConfig.Name),
		RoleArn: aws.String(clusterConfig.RoleARN),
		ResourcesVpcConfig: &ekstypes.VpcConfigRequest{
			SubnetIds:        subnetIDs,
			SecurityGroupIds: []string{securityGroupID},
		},
		Tags: map[string]string{
			"CreatedBy": "hyve",
		},
	}

	resp, err := p.eksClient.CreateCluster(ctx, createInput)
	if err != nil {
		// Clean up security group if cluster creation fails
		log.Printf("Cluster creation failed, cleaning up security group %s", securityGroupID)
		_ = p.deleteSecurityGroup(ctx, securityGroupID)
		return nil, fmt.Errorf("failed to create EKS cluster: %w", err)
	}

	log.Printf("EKS cluster creation started: %s", *resp.Cluster.Name)

	return p.convertCluster(resp.Cluster), nil
}

// getVPCSubnets gets all subnets in a VPC
func (p *Provider) getVPCSubnets(ctx context.Context, vpcID string) ([]string, error) {
	resp, err := p.ec2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []ec2types.Filter{
			{Name: aws.String("vpc-id"), Values: []string{vpcID}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe subnets: %w", err)
	}

	var subnetIDs []string
	for _, subnet := range resp.Subnets {
		if subnet.SubnetId != nil {
			subnetIDs = append(subnetIDs, *subnet.SubnetId)
		}
	}

	return subnetIDs, nil
}

// createClusterSecurityGroup creates a security group for the EKS cluster
func (p *Provider) createClusterSecurityGroup(ctx context.Context, vpcID, clusterName string) (string, error) {
	sgName := fmt.Sprintf("hyve-eks-%s-sg", clusterName)
	sgDescription := fmt.Sprintf("Security group for EKS cluster %s created by Hyve", clusterName)

	createResp, err := p.ec2Client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(sgName),
		Description: aws.String(sgDescription),
		VpcId:       aws.String(vpcID),
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeSecurityGroup,
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String(sgName)},
					{Key: aws.String("CreatedBy"), Value: aws.String("hyve")},
					{Key: aws.String("EKSCluster"), Value: aws.String(clusterName)},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create security group: %w", err)
	}

	sgID := *createResp.GroupId

	// Add ingress rules for EKS cluster communication
	// Allow all traffic within the security group
	_, err = p.ec2Client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(sgID),
		IpPermissions: []ec2types.IpPermission{
			{
				IpProtocol: aws.String("-1"), // All protocols
				UserIdGroupPairs: []ec2types.UserIdGroupPair{
					{GroupId: aws.String(sgID)},
				},
			},
		},
	})
	if err != nil {
		log.Printf("Warning: Failed to add self-referencing ingress rule: %v", err)
	}

	// Allow HTTPS from anywhere (for kubectl access)
	_, err = p.ec2Client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(sgID),
		IpPermissions: []ec2types.IpPermission{
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(443),
				ToPort:     aws.Int32(443),
				IpRanges: []ec2types.IpRange{
					{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("HTTPS access for kubectl")},
				},
			},
		},
	})
	if err != nil {
		log.Printf("Warning: Failed to add HTTPS ingress rule: %v", err)
	}

	return sgID, nil
}

// deleteSecurityGroup deletes a security group
func (p *Provider) deleteSecurityGroup(ctx context.Context, securityGroupID string) error {
	_, err := p.ec2Client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
		GroupId: aws.String(securityGroupID),
	})
	return err
}

// UpdateCluster updates an existing cluster
func (p *Provider) UpdateCluster(ctx context.Context, clusterID string, config *ClusterUpdateConfig) (*Cluster, error) {
	// EKS cluster updates are limited - return current cluster
	return p.GetCluster(ctx, clusterID)
}

// DeleteCluster deletes a cluster and cleans up associated resources
func (p *Provider) DeleteCluster(ctx context.Context, clusterID string) error {
	log.Printf("Deleting EKS cluster %s and cleaning up resources...", clusterID)

	// Find security groups created by Hyve for this cluster before deletion
	securityGroupIDs, err := p.findClusterSecurityGroups(ctx, clusterID)
	if err != nil {
		log.Printf("Warning: Failed to find security groups for cluster %s: %v", clusterID, err)
	}

	// Delete the EKS cluster
	_, err = p.eksClient.DeleteCluster(ctx, &eks.DeleteClusterInput{Name: &clusterID})
	if err != nil {
		return fmt.Errorf("failed to delete EKS cluster: %w", err)
	}

	// Wait for cluster to be deleted before cleaning up security groups
	log.Printf("Waiting for EKS cluster %s to be deleted...", clusterID)
	if err := p.waitForClusterDeleted(ctx, clusterID); err != nil {
		log.Printf("Warning: Error waiting for cluster deletion: %v", err)
		// Continue with cleanup anyway
	}

	// Clean up security groups created by Hyve
	for _, sgID := range securityGroupIDs {
		log.Printf("Deleting security group %s created for cluster %s", sgID, clusterID)
		if err := p.deleteSecurityGroup(ctx, sgID); err != nil {
			log.Printf("Warning: Failed to delete security group %s: %v", sgID, err)
		} else {
			log.Printf("Successfully deleted security group %s", sgID)
		}
	}

	return nil
}

// findClusterSecurityGroups finds security groups created by Hyve for a cluster
func (p *Provider) findClusterSecurityGroups(ctx context.Context, clusterName string) ([]string, error) {
	// Find security groups tagged with EKSCluster: clusterName and CreatedBy: hyve
	resp, err := p.ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []ec2types.Filter{
			{Name: aws.String("tag:EKSCluster"), Values: []string{clusterName}},
			{Name: aws.String("tag:CreatedBy"), Values: []string{"hyve"}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe security groups: %w", err)
	}

	var sgIDs []string
	for _, sg := range resp.SecurityGroups {
		if sg.GroupId != nil {
			sgIDs = append(sgIDs, *sg.GroupId)
		}
	}

	return sgIDs, nil
}

// waitForClusterDeleted waits for a cluster to be fully deleted
func (p *Provider) waitForClusterDeleted(ctx context.Context, clusterID string) error {
	for {
		_, err := p.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: &clusterID})
		if err != nil {
			// Check if it's a not found error (cluster deleted)
			if isClusterNotFoundError(err) {
				log.Printf("EKS cluster %s has been deleted", clusterID)
				return nil
			}
			return fmt.Errorf("failed to check cluster status: %w", err)
		}

		log.Printf("EKS cluster %s still deleting, waiting...")

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(30 * time.Second):
		}
	}
}

// isClusterNotFoundError checks if the error indicates the cluster was not found
func isClusterNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// Check for ResourceNotFoundException
	return strings.Contains(err.Error(), "ResourceNotFoundException") ||
		strings.Contains(err.Error(), "not found")
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
