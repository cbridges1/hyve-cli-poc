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

	"hyve/internal/types"
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
	RoleARN     string   // IAM role ARN for the EKS cluster
	NodeRoleARN string   // IAM role ARN for the EKS node group
	VPCID       string   // VPC ID where the cluster will be created
	SubnetIDs   []string // Subnet IDs for the cluster (if empty, will be discovered from VPC)
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

// validAWSRegions contains common AWS regions for validation
var validAWSRegions = map[string]bool{
	"us-east-1":      true,
	"us-east-2":      true,
	"us-west-1":      true,
	"us-west-2":      true,
	"af-south-1":     true,
	"ap-east-1":      true,
	"ap-south-1":     true,
	"ap-south-2":     true,
	"ap-southeast-1": true,
	"ap-southeast-2": true,
	"ap-southeast-3": true,
	"ap-southeast-4": true,
	"ap-northeast-1": true,
	"ap-northeast-2": true,
	"ap-northeast-3": true,
	"ca-central-1":   true,
	"eu-central-1":   true,
	"eu-central-2":   true,
	"eu-west-1":      true,
	"eu-west-2":      true,
	"eu-west-3":      true,
	"eu-south-1":     true,
	"eu-south-2":     true,
	"eu-north-1":     true,
	"me-south-1":     true,
	"me-central-1":   true,
	"sa-east-1":      true,
}

// NewProvider creates a new AWS provider.
// When accessKeyID and secretAccessKey are non-empty, static credentials are used (sessionToken
// is optional and may be empty). Otherwise the AWS SDK default credential chain is used.
func NewProvider(accessKeyID, secretAccessKey, sessionToken, region string) (*Provider, error) {
	ctx := context.Background()

	// Validate and normalize region
	if region == "" {
		region = "us-east-1" // Default region
		log.Printf("No AWS region specified, using default: %s", region)
	}

	// Check if the region looks like a valid AWS region
	if !validAWSRegions[region] {
		// Check if it looks like a non-AWS region (e.g., Civo regions like PHX1)
		if !strings.Contains(region, "-") {
			return nil, fmt.Errorf("invalid AWS region '%s'. AWS regions use format like 'us-east-1', 'eu-west-1', etc. "+
				"The provided region appears to be for a different provider (e.g., Civo uses regions like PHX1)", region)
		}
		log.Printf("Warning: Region '%s' not in known AWS regions list, proceeding anyway", region)
	}

	var opts []func(*config.LoadOptions) error
	opts = append(opts, config.WithRegion(region))

	if accessKeyID != "" && secretAccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, sessionToken)))
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
	log.Printf("Looking for EKS cluster '%s' in region %s", name, p.region)

	resp, err := p.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: &name})
	if err != nil {
		// Only return nil for actual "not found" errors
		if isClusterNotFoundError(err) {
			log.Printf("EKS cluster '%s' not found in region %s", name, p.region)
			return nil, nil
		}
		// Return the actual error for other issues (auth, permissions, etc.)
		log.Printf("Error looking up EKS cluster '%s': %v", name, err)
		return nil, fmt.Errorf("failed to describe EKS cluster '%s': %w", name, err)
	}

	log.Printf("Found EKS cluster '%s' in region %s (status: %s)", name, p.region, resp.Cluster.Status)
	return p.convertCluster(resp.Cluster), nil
}

// CreateCluster creates a new cluster with a node group
func (p *Provider) CreateCluster(ctx context.Context, clusterConfig *ClusterConfig) (*Cluster, error) {
	log.Printf("Creating EKS cluster %s in region %s", clusterConfig.Name, p.region)

	// Validate required configuration
	if clusterConfig.RoleARN == "" {
		return nil, fmt.Errorf("EKS cluster creation requires a role ARN. Use --eks-role-name flag")
	}
	if clusterConfig.VPCID == "" {
		return nil, fmt.Errorf("EKS cluster creation requires a VPC ID. Use --vpc-name flag")
	}
	if clusterConfig.NodeRoleARN == "" {
		return nil, fmt.Errorf("EKS cluster creation requires a node role ARN. Use --node-role-name flag")
	}

	// Track resources created for cleanup on failure
	var createdSubnetIDs []string
	var securityGroupID string

	// Cleanup function for failure cases
	cleanup := func() {
		for _, subnetID := range createdSubnetIDs {
			log.Printf("Cleaning up subnet %s", subnetID)
			_ = p.deleteSubnet(ctx, subnetID)
		}
		if securityGroupID != "" {
			log.Printf("Cleaning up security group %s", securityGroupID)
			_ = p.deleteSecurityGroup(ctx, securityGroupID)
		}
	}

	// Get existing subnets from VPC
	subnetIDs := clusterConfig.SubnetIDs
	if len(subnetIDs) == 0 {
		var err error
		subnetIDs, err = p.getVPCSubnets(ctx, clusterConfig.VPCID)
		if err != nil {
			return nil, fmt.Errorf("failed to get subnets from VPC: %w", err)
		}
		log.Printf("Found %d existing subnets in VPC %s", len(subnetIDs), clusterConfig.VPCID)
	}

	// Check if we have at least 2 subnets in different AZs
	if len(subnetIDs) < 2 {
		log.Printf("EKS requires at least 2 subnets in different availability zones, creating subnets...")

		// Get VPC CIDR to calculate subnet CIDRs
		vpcCIDR, err := p.getVPCCIDR(ctx, clusterConfig.VPCID)
		if err != nil {
			return nil, fmt.Errorf("failed to get VPC CIDR: %w", err)
		}

		// Get available availability zones
		azs, err := p.getAvailabilityZones(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get availability zones: %w", err)
		}
		if len(azs) < 2 {
			return nil, fmt.Errorf("need at least 2 availability zones, found %d", len(azs))
		}

		// Create subnets in at least 2 different AZs
		createdSubnetIDs, err = p.createClusterSubnets(ctx, clusterConfig.VPCID, clusterConfig.Name, vpcCIDR, azs[:2])
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("failed to create subnets: %w", err)
		}
		subnetIDs = createdSubnetIDs
		log.Printf("Created %d subnets for cluster %s", len(createdSubnetIDs), clusterConfig.Name)
	}

	// Create a security group for the cluster
	var err error
	securityGroupID, err = p.createClusterSecurityGroup(ctx, clusterConfig.VPCID, clusterConfig.Name)
	if err != nil {
		cleanup()
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
		cleanup()
		return nil, fmt.Errorf("failed to create EKS cluster: %w", err)
	}

	log.Printf("EKS cluster creation started: %s", *resp.Cluster.Name)

	// Wait for cluster to be active before creating node group
	log.Printf("Waiting for EKS cluster %s to become active...", clusterConfig.Name)
	if err := p.WaitForClusterReady(ctx, clusterConfig.Name); err != nil {
		// Don't cleanup cluster resources on wait failure - cluster may still be creating
		return nil, fmt.Errorf("failed waiting for cluster to be ready: %w", err)
	}

	// Create node group with the specified nodes
	log.Printf("Creating node group for cluster %s...", clusterConfig.Name)
	if err := p.createNodeGroup(ctx, clusterConfig.Name, clusterConfig.NodeRoleARN, subnetIDs, clusterConfig.Nodes); err != nil {
		log.Printf("Warning: Failed to create node group: %v", err)
		// Return the cluster even if node group creation fails - cluster is still usable
	} else {
		log.Printf("Node group creation started for cluster %s", clusterConfig.Name)
	}

	// Refresh cluster info
	cluster, err := p.GetCluster(ctx, clusterConfig.Name)
	if err != nil {
		return p.convertCluster(resp.Cluster), nil
	}

	return cluster, nil
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

// getVPCCIDR gets the CIDR block for a VPC
func (p *Provider) getVPCCIDR(ctx context.Context, vpcID string) (string, error) {
	resp, err := p.ec2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		VpcIds: []string{vpcID},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe VPC: %w", err)
	}

	if len(resp.Vpcs) == 0 {
		return "", fmt.Errorf("VPC %s not found", vpcID)
	}

	if resp.Vpcs[0].CidrBlock == nil {
		return "", fmt.Errorf("VPC %s has no CIDR block", vpcID)
	}

	return *resp.Vpcs[0].CidrBlock, nil
}

// getAvailabilityZones gets available availability zones in the region
func (p *Provider) getAvailabilityZones(ctx context.Context) ([]string, error) {
	resp, err := p.ec2Client.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{
		Filters: []ec2types.Filter{
			{Name: aws.String("state"), Values: []string{"available"}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe availability zones: %w", err)
	}

	var azs []string
	for _, az := range resp.AvailabilityZones {
		if az.ZoneName != nil {
			azs = append(azs, *az.ZoneName)
		}
	}

	return azs, nil
}

// createClusterSubnets creates subnets for the EKS cluster in different AZs
func (p *Provider) createClusterSubnets(ctx context.Context, vpcID, clusterName, vpcCIDR string, azs []string) ([]string, error) {
	// Parse the VPC CIDR to generate subnet CIDRs
	// For a /16 VPC, we'll create /24 subnets
	// For example: 10.0.0.0/16 -> 10.0.1.0/24, 10.0.2.0/24
	subnetCIDRs, err := generateSubnetCIDRs(vpcCIDR, len(azs))
	if err != nil {
		return nil, fmt.Errorf("failed to generate subnet CIDRs: %w", err)
	}

	var createdSubnetIDs []string

	for i, az := range azs {
		if i >= len(subnetCIDRs) {
			break
		}

		subnetName := fmt.Sprintf("hyve-eks-%s-subnet-%d", clusterName, i+1)
		log.Printf("Creating subnet %s in %s with CIDR %s", subnetName, az, subnetCIDRs[i])

		createResp, err := p.ec2Client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
			VpcId:            aws.String(vpcID),
			CidrBlock:        aws.String(subnetCIDRs[i]),
			AvailabilityZone: aws.String(az),
			TagSpecifications: []ec2types.TagSpecification{
				{
					ResourceType: ec2types.ResourceTypeSubnet,
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String(subnetName)},
						{Key: aws.String("CreatedBy"), Value: aws.String("hyve")},
						{Key: aws.String("EKSCluster"), Value: aws.String(clusterName)},
					},
				},
			},
		})
		if err != nil {
			// Clean up already created subnets on failure
			for _, subnetID := range createdSubnetIDs {
				_ = p.deleteSubnet(ctx, subnetID)
			}
			return nil, fmt.Errorf("failed to create subnet in %s: %w", az, err)
		}

		subnetID := *createResp.Subnet.SubnetId
		createdSubnetIDs = append(createdSubnetIDs, subnetID)
		log.Printf("Created subnet %s (%s) in %s", subnetName, subnetID, az)

		// Enable auto-assign public IP for the subnet (required for EKS nodes to access internet)
		_, err = p.ec2Client.ModifySubnetAttribute(ctx, &ec2.ModifySubnetAttributeInput{
			SubnetId:            aws.String(subnetID),
			MapPublicIpOnLaunch: &ec2types.AttributeBooleanValue{Value: aws.Bool(true)},
		})
		if err != nil {
			log.Printf("Warning: Failed to enable auto-assign public IP for subnet %s: %v", subnetID, err)
		}
	}

	return createdSubnetIDs, nil
}

// generateSubnetCIDRs generates subnet CIDRs from a VPC CIDR
func generateSubnetCIDRs(vpcCIDR string, count int) ([]string, error) {
	// Parse the VPC CIDR (e.g., "10.0.0.0/16")
	parts := strings.Split(vpcCIDR, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid CIDR format: %s", vpcCIDR)
	}

	ipParts := strings.Split(parts[0], ".")
	if len(ipParts) != 4 {
		return nil, fmt.Errorf("invalid IP format: %s", parts[0])
	}

	// Generate /24 subnets starting from .1.0, .2.0, etc.
	var cidrs []string
	for i := 1; i <= count; i++ {
		cidr := fmt.Sprintf("%s.%s.%d.0/24", ipParts[0], ipParts[1], i)
		cidrs = append(cidrs, cidr)
	}

	return cidrs, nil
}

// deleteSubnet deletes a subnet
func (p *Provider) deleteSubnet(ctx context.Context, subnetID string) error {
	_, err := p.ec2Client.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{
		SubnetId: aws.String(subnetID),
	})
	return err
}

// createClusterSecurityGroup creates a security group for the EKS cluster or returns existing one
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
		// Check if security group already exists
		if isSecurityGroupDuplicateError(err) {
			log.Printf("Security group %s already exists, looking it up", sgName)
			existingSgID, lookupErr := p.findSecurityGroupByName(ctx, vpcID, sgName)
			if lookupErr != nil {
				return "", fmt.Errorf("security group exists but failed to look up: %w", lookupErr)
			}
			log.Printf("Found existing security group %s", existingSgID)
			return existingSgID, nil
		}
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

// isSecurityGroupDuplicateError checks if the error indicates a duplicate security group
func isSecurityGroupDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	// Check for AWS API error code InvalidGroup.Duplicate
	errStr := err.Error()
	return strings.Contains(errStr, "InvalidGroup.Duplicate")
}

// findSecurityGroupByName finds a security group by name in a VPC
func (p *Provider) findSecurityGroupByName(ctx context.Context, vpcID, sgName string) (string, error) {
	resp, err := p.ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []ec2types.Filter{
			{Name: aws.String("vpc-id"), Values: []string{vpcID}},
			{Name: aws.String("group-name"), Values: []string{sgName}},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe security groups: %w", err)
	}

	if len(resp.SecurityGroups) == 0 {
		return "", fmt.Errorf("security group %s not found in VPC %s", sgName, vpcID)
	}

	return *resp.SecurityGroups[0].GroupId, nil
}

// createNodeGroup creates a managed node group for an EKS cluster
func (p *Provider) createNodeGroup(ctx context.Context, clusterName, nodeRoleARN string, subnetIDs, nodes []string) error {
	nodeGroupName := fmt.Sprintf("%s-nodes", clusterName)

	// Determine instance type and count from nodes config
	instanceType := "t3.medium" // Default instance type
	desiredSize := int32(2)     // Default node count

	if len(nodes) > 0 {
		// First node entry is the instance type
		instanceType = nodes[0]
		// Use the number of node entries as the desired count (minimum 1)
		desiredSize = int32(len(nodes))
		if desiredSize < 1 {
			desiredSize = 1
		}
	}

	log.Printf("Creating node group %s with instance type %s and %d nodes", nodeGroupName, instanceType, desiredSize)

	createInput := &eks.CreateNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(nodeGroupName),
		NodeRole:      aws.String(nodeRoleARN),
		Subnets:       subnetIDs,
		ScalingConfig: &ekstypes.NodegroupScalingConfig{
			DesiredSize: aws.Int32(desiredSize),
			MinSize:     aws.Int32(1),
			MaxSize:     aws.Int32(desiredSize + 2), // Allow some scaling headroom
		},
		InstanceTypes: []string{instanceType},
		Tags: map[string]string{
			"CreatedBy":  "hyve",
			"EKSCluster": clusterName,
		},
	}

	_, err := p.eksClient.CreateNodegroup(ctx, createInput)
	if err != nil {
		return fmt.Errorf("failed to create node group: %w", err)
	}

	return nil
}

// deleteNodeGroups deletes all node groups for a cluster
func (p *Provider) deleteNodeGroups(ctx context.Context, clusterName string) error {
	// List all node groups for the cluster
	listResp, err := p.eksClient.ListNodegroups(ctx, &eks.ListNodegroupsInput{
		ClusterName: aws.String(clusterName),
	})
	if err != nil {
		return fmt.Errorf("failed to list node groups: %w", err)
	}

	if len(listResp.Nodegroups) == 0 {
		log.Printf("No node groups found for cluster %s", clusterName)
		return nil
	}

	// Delete each node group
	for _, nodeGroupName := range listResp.Nodegroups {
		log.Printf("Deleting node group %s from cluster %s", nodeGroupName, clusterName)
		_, err := p.eksClient.DeleteNodegroup(ctx, &eks.DeleteNodegroupInput{
			ClusterName:   aws.String(clusterName),
			NodegroupName: aws.String(nodeGroupName),
		})
		if err != nil {
			log.Printf("Warning: Failed to delete node group %s: %v", nodeGroupName, err)
			continue
		}
	}

	// Wait for all node groups to be deleted
	log.Printf("Waiting for node groups to be deleted...")
	for _, nodeGroupName := range listResp.Nodegroups {
		if err := p.waitForNodeGroupDeleted(ctx, clusterName, nodeGroupName); err != nil {
			log.Printf("Warning: Error waiting for node group %s deletion: %v", nodeGroupName, err)
		}
	}

	return nil
}

// waitForNodeGroupDeleted waits for a node group to be fully deleted
func (p *Provider) waitForNodeGroupDeleted(ctx context.Context, clusterName, nodeGroupName string) error {
	for {
		_, err := p.eksClient.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
			ClusterName:   aws.String(clusterName),
			NodegroupName: aws.String(nodeGroupName),
		})
		if err != nil {
			// Check if it's a not found error (node group deleted)
			if strings.Contains(err.Error(), "ResourceNotFoundException") ||
				strings.Contains(err.Error(), "not found") {
				log.Printf("Node group %s has been deleted", nodeGroupName)
				return nil
			}
			return fmt.Errorf("failed to check node group status: %w", err)
		}

		log.Printf("Node group %s still deleting, waiting...", nodeGroupName)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(30 * time.Second):
		}
	}
}

// UpdateCluster updates an existing cluster
func (p *Provider) UpdateCluster(ctx context.Context, clusterID string, config *ClusterUpdateConfig) (*Cluster, error) {
	// EKS cluster updates are limited - return current cluster
	return p.GetCluster(ctx, clusterID)
}

// DeleteCluster deletes a cluster and cleans up associated resources
func (p *Provider) DeleteCluster(ctx context.Context, clusterID string) error {
	log.Printf("Deleting EKS cluster %s and cleaning up resources...", clusterID)

	// Find resources created by Hyve for this cluster before deletion
	securityGroupIDs, err := p.findClusterSecurityGroups(ctx, clusterID)
	if err != nil {
		log.Printf("Warning: Failed to find security groups for cluster %s: %v", clusterID, err)
	}

	subnetIDs, err := p.findClusterSubnets(ctx, clusterID)
	if err != nil {
		log.Printf("Warning: Failed to find subnets for cluster %s: %v", clusterID, err)
	}

	// Delete node groups first - EKS requires this before cluster deletion
	log.Printf("Deleting node groups for cluster %s...", clusterID)
	if err := p.deleteNodeGroups(ctx, clusterID); err != nil {
		log.Printf("Warning: Failed to delete node groups: %v", err)
		// Continue anyway - cluster deletion may still work
	}

	// Delete the EKS cluster
	_, err = p.eksClient.DeleteCluster(ctx, &eks.DeleteClusterInput{Name: &clusterID})
	if err != nil {
		return fmt.Errorf("failed to delete EKS cluster: %w", err)
	}

	// Wait for cluster to be deleted before cleaning up resources
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

	// Clean up subnets created by Hyve
	for _, subnetID := range subnetIDs {
		log.Printf("Deleting subnet %s created for cluster %s", subnetID, clusterID)
		if err := p.deleteSubnet(ctx, subnetID); err != nil {
			log.Printf("Warning: Failed to delete subnet %s: %v", subnetID, err)
		} else {
			log.Printf("Successfully deleted subnet %s", subnetID)
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

// findClusterSubnets finds subnets created by Hyve for a cluster
func (p *Provider) findClusterSubnets(ctx context.Context, clusterName string) ([]string, error) {
	// Find subnets tagged with EKSCluster: clusterName and CreatedBy: hyve
	resp, err := p.ec2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []ec2types.Filter{
			{Name: aws.String("tag:EKSCluster"), Values: []string{clusterName}},
			{Name: aws.String("tag:CreatedBy"), Values: []string{"hyve"}},
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

		log.Printf("EKS cluster %s still deleting, waiting...", clusterID)

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
	errStr := err.Error()
	// Check for AWS ResourceNotFoundException
	// Also check for specific EKS "cluster not found" messages
	return strings.Contains(errStr, "ResourceNotFoundException") ||
		strings.Contains(errStr, "No cluster found") ||
		(strings.Contains(errStr, "cluster") && strings.Contains(errStr, "not found"))
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

	// Generate kubeconfig for EKS cluster
	kubeconfig := ""
	if cluster.CertificateAuthority != nil && cluster.CertificateAuthority.Data != nil {
		kubeconfig = p.generateEKSKubeconfig(name, endpoint, *cluster.CertificateAuthority.Data)
	}

	return &ClusterInfo{
		Name:       *cluster.Name,
		IPAddress:  endpoint,
		AccessPort: "443",
		Kubeconfig: kubeconfig,
		Status:     string(cluster.Status),
		ID:         *cluster.Name,
	}, nil
}

// generateEKSKubeconfig generates a kubeconfig for an EKS cluster
func (p *Provider) generateEKSKubeconfig(clusterName, endpoint, caData string) string {
	// Generate kubeconfig that uses aws eks get-token for authentication
	kubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: %s
    certificate-authority-data: %s
  name: %s
contexts:
- context:
    cluster: %s
    user: %s
  name: %s
current-context: %s
users:
- name: %s
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args:
        - eks
        - get-token
        - --cluster-name
        - %s
        - --region
        - %s
`, endpoint, caData, clusterName, clusterName, clusterName, clusterName, clusterName, clusterName, clusterName, p.region)

	return kubeconfig
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
