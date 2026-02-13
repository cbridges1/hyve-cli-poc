package provider

import (
	"context"
	"log"

	"civo-cluster-deploy/internal/provider/aws"
	"civo-cluster-deploy/internal/provider/azure"
	"civo-cluster-deploy/internal/provider/civo"
	"civo-cluster-deploy/internal/provider/gcp"
	"civo-cluster-deploy/internal/types"
)

// ProviderAdapter adapts provider implementations to the generic provider interface
type ProviderAdapter struct {
	civo  *civo.Provider
	gcp   *gcp.Provider
	aws   *aws.Provider
	azure *azure.Provider
}

// Name returns the provider name
func (a *ProviderAdapter) Name() string {
	if a.aws != nil {
		return a.aws.Name()
	}
	if a.azure != nil {
		return a.azure.Name()
	}
	if a.gcp != nil {
		return a.gcp.Name()
	}
	return a.civo.Name()
}

// Region returns the provider region
func (a *ProviderAdapter) Region() string {
	if a.aws != nil {
		return a.aws.Region()
	}
	if a.azure != nil {
		return a.azure.Region()
	}
	if a.gcp != nil {
		return a.gcp.Region()
	}
	return a.civo.Region()
}

// ListClusters lists all clusters
func (a *ProviderAdapter) ListClusters(ctx context.Context) ([]*Cluster, error) {
	if a.aws != nil {
		awsClusters, err := a.aws.ListClusters(ctx)
		if err != nil {
			return nil, err
		}
		var clusters []*Cluster
		for _, c := range awsClusters {
			clusters = append(clusters, convertAWSCluster(c))
		}
		return clusters, nil
	}
	if a.azure != nil {
		azureClusters, err := a.azure.ListClusters(ctx)
		if err != nil {
			return nil, err
		}
		var clusters []*Cluster
		for _, c := range azureClusters {
			clusters = append(clusters, convertAzureCluster(c))
		}
		return clusters, nil
	}
	if a.gcp != nil {
		gcpClusters, err := a.gcp.ListClusters(ctx)
		if err != nil {
			return nil, err
		}
		var clusters []*Cluster
		for _, c := range gcpClusters {
			clusters = append(clusters, convertGCPCluster(c))
		}
		return clusters, nil
	}

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
	if a.aws != nil {
		awsCluster, err := a.aws.GetCluster(ctx, clusterID)
		if err != nil {
			return nil, err
		}
		return convertAWSCluster(awsCluster), nil
	}
	if a.azure != nil {
		azureCluster, err := a.azure.GetCluster(ctx, clusterID)
		if err != nil {
			return nil, err
		}
		return convertAzureCluster(azureCluster), nil
	}
	if a.gcp != nil {
		gcpCluster, err := a.gcp.GetCluster(ctx, clusterID)
		if err != nil {
			return nil, err
		}
		return convertGCPCluster(gcpCluster), nil
	}

	civoCluster, err := a.civo.GetCluster(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	return convertCivoCluster(civoCluster), nil
}

// FindClusterByName finds a cluster by name
func (a *ProviderAdapter) FindClusterByName(ctx context.Context, name string) (*Cluster, error) {
	if a.aws != nil {
		awsCluster, err := a.aws.FindClusterByName(ctx, name)
		if err != nil {
			return nil, err
		}
		if awsCluster == nil {
			return nil, nil
		}
		return convertAWSCluster(awsCluster), nil
	}
	if a.azure != nil {
		azureCluster, err := a.azure.FindClusterByName(ctx, name)
		if err != nil {
			return nil, err
		}
		if azureCluster == nil {
			return nil, nil
		}
		return convertAzureCluster(azureCluster), nil
	}
	if a.gcp != nil {
		gcpCluster, err := a.gcp.FindClusterByName(ctx, name)
		if err != nil {
			return nil, err
		}
		if gcpCluster == nil {
			return nil, nil
		}
		return convertGCPCluster(gcpCluster), nil
	}

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
	if a.aws != nil {
		awsConfig := &aws.ClusterConfig{
			Name:         config.Name,
			Region:       config.Region,
			Nodes:        config.Nodes,
			ClusterType:  config.ClusterType,
			FirewallID:   config.FirewallID,
			Applications: config.Applications,
			// EKS-specific configuration
			RoleARN:     config.AWSRoleARN,
			NodeRoleARN: config.AWSNodeRoleARN,
			VPCID:       config.AWSVPCID,
			SubnetIDs:   config.AWSSubnetIDs,
		}
		log.Printf("Creating AWS cluster with configuration: %+v", awsConfig)
		awsCluster, err := a.aws.CreateCluster(ctx, awsConfig)
		if err != nil {
			return nil, err
		}
		return convertAWSCluster(awsCluster), nil
	}
	if a.azure != nil {
		azureConfig := &azure.ClusterConfig{
			Name:         config.Name,
			Region:       config.Region,
			Nodes:        config.Nodes,
			ClusterType:  config.ClusterType,
			FirewallID:   config.FirewallID,
			Applications: config.Applications,
		}
		log.Printf("Creating Azure cluster with configuration: %+v", azureConfig)
		azureCluster, err := a.azure.CreateCluster(ctx, azureConfig)
		if err != nil {
			return nil, err
		}
		return convertAzureCluster(azureCluster), nil
	}
	if a.gcp != nil {
		gcpConfig := &gcp.ClusterConfig{
			Name:         config.Name,
			Region:       config.Region,
			Nodes:        config.Nodes,
			ClusterType:  config.ClusterType,
			FirewallID:   config.FirewallID,
			Applications: config.Applications,
		}
		log.Printf("Creating GCP cluster with configuration: %+v", gcpConfig)
		gcpCluster, err := a.gcp.CreateCluster(ctx, gcpConfig)
		if err != nil {
			return nil, err
		}
		return convertGCPCluster(gcpCluster), nil
	}

	civoConfig := &civo.ClusterConfig{
		Name:         config.Name,
		Region:       config.Region,
		Nodes:        config.Nodes,
		ClusterType:  config.ClusterType,
		FirewallID:   config.FirewallID,
		Applications: config.Applications,
	}

	log.Printf("Creating cluster with configuration: %+v", civoConfig)
	civoCluster, err := a.civo.CreateCluster(ctx, civoConfig)
	if err != nil {
		return nil, err
	}

	return convertCivoCluster(civoCluster), nil
}

// UpdateCluster updates an existing cluster
func (a *ProviderAdapter) UpdateCluster(ctx context.Context, clusterID string, config *ClusterUpdateConfig) (*Cluster, error) {
	if a.aws != nil {
		awsConfig := &aws.ClusterUpdateConfig{
			Name:  config.Name,
			Nodes: config.Nodes,
		}
		awsCluster, err := a.aws.UpdateCluster(ctx, clusterID, awsConfig)
		if err != nil {
			return nil, err
		}
		return convertAWSCluster(awsCluster), nil
	}
	if a.azure != nil {
		azureConfig := &azure.ClusterUpdateConfig{
			Name:  config.Name,
			Nodes: config.Nodes,
		}
		azureCluster, err := a.azure.UpdateCluster(ctx, clusterID, azureConfig)
		if err != nil {
			return nil, err
		}
		return convertAzureCluster(azureCluster), nil
	}
	if a.gcp != nil {
		gcpConfig := &gcp.ClusterUpdateConfig{
			Name:  config.Name,
			Nodes: config.Nodes,
		}
		gcpCluster, err := a.gcp.UpdateCluster(ctx, clusterID, gcpConfig)
		if err != nil {
			return nil, err
		}
		return convertGCPCluster(gcpCluster), nil
	}

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
	if a.aws != nil {
		return a.aws.DeleteCluster(ctx, clusterID)
	}
	if a.azure != nil {
		return a.azure.DeleteCluster(ctx, clusterID)
	}
	if a.gcp != nil {
		return a.gcp.DeleteCluster(ctx, clusterID)
	}
	return a.civo.DeleteCluster(ctx, clusterID)
}

// WaitForClusterReady waits for cluster to be ready
func (a *ProviderAdapter) WaitForClusterReady(ctx context.Context, clusterID string) error {
	if a.aws != nil {
		return a.aws.WaitForClusterReady(ctx, clusterID)
	}
	if a.azure != nil {
		return a.azure.WaitForClusterReady(ctx, clusterID)
	}
	if a.gcp != nil {
		return a.gcp.WaitForClusterReady(ctx, clusterID)
	}
	return a.civo.WaitForClusterReady(ctx, clusterID)
}

// GetClusterInfo gets cluster information for export
func (a *ProviderAdapter) GetClusterInfo(ctx context.Context, name string) (*ClusterInfo, error) {
	if a.aws != nil {
		awsInfo, err := a.aws.GetClusterInfo(ctx, name)
		if err != nil {
			return nil, err
		}
		return &ClusterInfo{
			Name:       awsInfo.Name,
			IPAddress:  awsInfo.IPAddress,
			AccessPort: awsInfo.AccessPort,
			Kubeconfig: awsInfo.Kubeconfig,
			Status:     awsInfo.Status,
			ID:         awsInfo.ID,
		}, nil
	}
	if a.azure != nil {
		azureInfo, err := a.azure.GetClusterInfo(ctx, name)
		if err != nil {
			return nil, err
		}
		return &ClusterInfo{
			Name:       azureInfo.Name,
			IPAddress:  azureInfo.IPAddress,
			AccessPort: azureInfo.AccessPort,
			Kubeconfig: azureInfo.Kubeconfig,
			Status:     azureInfo.Status,
			ID:         azureInfo.ID,
		}, nil
	}
	if a.gcp != nil {
		gcpInfo, err := a.gcp.GetClusterInfo(ctx, name)
		if err != nil {
			return nil, err
		}
		return &ClusterInfo{
			Name:       gcpInfo.Name,
			IPAddress:  gcpInfo.IPAddress,
			AccessPort: gcpInfo.AccessPort,
			Kubeconfig: gcpInfo.Kubeconfig,
			Status:     gcpInfo.Status,
			ID:         gcpInfo.ID,
		}, nil
	}

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
	if a.aws != nil {
		awsFirewalls, err := a.aws.ListFirewalls(ctx)
		if err != nil {
			return nil, err
		}
		var firewalls []*Firewall
		for _, f := range awsFirewalls {
			firewalls = append(firewalls, convertAWSFirewall(f))
		}
		return firewalls, nil
	}
	if a.azure != nil {
		azureFirewalls, err := a.azure.ListFirewalls(ctx)
		if err != nil {
			return nil, err
		}
		var firewalls []*Firewall
		for _, f := range azureFirewalls {
			firewalls = append(firewalls, convertAzureFirewall(f))
		}
		return firewalls, nil
	}
	if a.gcp != nil {
		gcpFirewalls, err := a.gcp.ListFirewalls(ctx)
		if err != nil {
			return nil, err
		}
		var firewalls []*Firewall
		for _, f := range gcpFirewalls {
			firewalls = append(firewalls, convertGCPFirewall(f))
		}
		return firewalls, nil
	}

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
	if a.aws != nil {
		awsConfig := &aws.FirewallConfig{
			Name:  config.Name,
			Rules: convertFirewallRulesToAWS(config.Rules),
		}
		awsFirewall, err := a.aws.CreateFirewall(ctx, awsConfig)
		if err != nil {
			return nil, err
		}
		return convertAWSFirewall(awsFirewall), nil
	}
	if a.azure != nil {
		azureConfig := &azure.FirewallConfig{
			Name:  config.Name,
			Rules: convertFirewallRulesToAzure(config.Rules),
		}
		azureFirewall, err := a.azure.CreateFirewall(ctx, azureConfig)
		if err != nil {
			return nil, err
		}
		return convertAzureFirewall(azureFirewall), nil
	}
	if a.gcp != nil {
		gcpConfig := &gcp.FirewallConfig{
			Name:  config.Name,
			Rules: convertFirewallRulesToGCP(config.Rules),
		}
		gcpFirewall, err := a.gcp.CreateFirewall(ctx, gcpConfig)
		if err != nil {
			return nil, err
		}
		return convertGCPFirewall(gcpFirewall), nil
	}

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
	if a.aws != nil {
		return a.aws.DeleteFirewall(ctx, firewallID)
	}
	if a.azure != nil {
		return a.azure.DeleteFirewall(ctx, firewallID)
	}
	if a.gcp != nil {
		return a.gcp.DeleteFirewall(ctx, firewallID)
	}
	return a.civo.DeleteFirewall(ctx, firewallID)
}

// FindFirewallByName finds a firewall by name
func (a *ProviderAdapter) FindFirewallByName(ctx context.Context, name string) (*Firewall, error) {
	if a.aws != nil {
		awsFirewall, err := a.aws.FindFirewallByName(ctx, name)
		if err != nil {
			return nil, err
		}
		if awsFirewall == nil {
			return nil, nil
		}
		return convertAWSFirewall(awsFirewall), nil
	}
	if a.azure != nil {
		azureFirewall, err := a.azure.FindFirewallByName(ctx, name)
		if err != nil {
			return nil, err
		}
		if azureFirewall == nil {
			return nil, nil
		}
		return convertAzureFirewall(azureFirewall), nil
	}
	if a.gcp != nil {
		gcpFirewall, err := a.gcp.FindFirewallByName(ctx, name)
		if err != nil {
			return nil, err
		}
		if gcpFirewall == nil {
			return nil, nil
		}
		return convertGCPFirewall(gcpFirewall), nil
	}

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
	if a.aws != nil {
		awsLBs, err := a.aws.ListLoadBalancers(ctx)
		if err != nil {
			return nil, err
		}
		var lbs []*LoadBalancer
		for _, lb := range awsLBs {
			lbs = append(lbs, &LoadBalancer{
				ID:        lb.ID,
				Name:      lb.Name,
				PublicIP:  lb.PublicIP,
				ClusterID: lb.ClusterID,
			})
		}
		return lbs, nil
	}
	if a.azure != nil {
		azureLBs, err := a.azure.ListLoadBalancers(ctx)
		if err != nil {
			return nil, err
		}
		var lbs []*LoadBalancer
		for _, lb := range azureLBs {
			lbs = append(lbs, &LoadBalancer{
				ID:        lb.ID,
				Name:      lb.Name,
				PublicIP:  lb.PublicIP,
				ClusterID: lb.ClusterID,
			})
		}
		return lbs, nil
	}
	if a.gcp != nil {
		gcpLBs, err := a.gcp.ListLoadBalancers(ctx)
		if err != nil {
			return nil, err
		}
		var lbs []*LoadBalancer
		for _, lb := range gcpLBs {
			lbs = append(lbs, &LoadBalancer{
				ID:        lb.ID,
				Name:      lb.Name,
				PublicIP:  lb.PublicIP,
				ClusterID: lb.ClusterID,
			})
		}
		return lbs, nil
	}

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
	if a.aws != nil {
		awsLB, err := a.aws.DeployIngressController(ctx, clusterID, spec)
		if err != nil {
			return nil, err
		}
		if awsLB == nil {
			return nil, nil
		}
		return &LoadBalancer{
			ID:        awsLB.ID,
			Name:      awsLB.Name,
			PublicIP:  awsLB.PublicIP,
			ClusterID: awsLB.ClusterID,
		}, nil
	}
	if a.azure != nil {
		azureLB, err := a.azure.DeployIngressController(ctx, clusterID, spec)
		if err != nil {
			return nil, err
		}
		if azureLB == nil {
			return nil, nil
		}
		return &LoadBalancer{
			ID:        azureLB.ID,
			Name:      azureLB.Name,
			PublicIP:  azureLB.PublicIP,
			ClusterID: azureLB.ClusterID,
		}, nil
	}
	if a.gcp != nil {
		gcpLB, err := a.gcp.DeployIngressController(ctx, clusterID, spec)
		if err != nil {
			return nil, err
		}
		if gcpLB == nil {
			return nil, nil
		}
		return &LoadBalancer{
			ID:        gcpLB.ID,
			Name:      gcpLB.Name,
			PublicIP:  gcpLB.PublicIP,
			ClusterID: gcpLB.ClusterID,
		}, nil
	}

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
	if a.aws != nil {
		return a.aws.RemoveIngressController(ctx, clusterID)
	}
	if a.azure != nil {
		return a.azure.RemoveIngressController(ctx, clusterID)
	}
	if a.gcp != nil {
		return a.gcp.RemoveIngressController(ctx, clusterID)
	}
	return a.civo.RemoveIngressController(ctx, clusterID)
}

// GetLoadBalancerIP gets load balancer IP for cluster
func (a *ProviderAdapter) GetLoadBalancerIP(ctx context.Context, clusterID string) (string, error) {
	if a.aws != nil {
		return a.aws.GetLoadBalancerIP(ctx, clusterID)
	}
	if a.azure != nil {
		return a.azure.GetLoadBalancerIP(ctx, clusterID)
	}
	if a.gcp != nil {
		return a.gcp.GetLoadBalancerIP(ctx, clusterID)
	}
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

// GCP conversion functions

func convertGCPCluster(c *gcp.Cluster) *Cluster {
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

func convertGCPFirewall(f *gcp.Firewall) *Firewall {
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

func convertFirewallRulesToGCP(rules []FirewallRule) []gcp.FirewallRule {
	var gcpRules []gcp.FirewallRule
	for _, rule := range rules {
		gcpRules = append(gcpRules, gcp.FirewallRule{
			Protocol:  rule.Protocol,
			StartPort: rule.StartPort,
			EndPort:   rule.EndPort,
			Cidr:      rule.Cidr,
			Direction: rule.Direction,
		})
	}

	return gcpRules
}

// AWS conversion functions

func convertAWSCluster(c *aws.Cluster) *Cluster {
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

func convertAWSFirewall(f *aws.Firewall) *Firewall {
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

func convertFirewallRulesToAWS(rules []FirewallRule) []aws.FirewallRule {
	var awsRules []aws.FirewallRule
	for _, rule := range rules {
		awsRules = append(awsRules, aws.FirewallRule{
			Protocol:  rule.Protocol,
			StartPort: rule.StartPort,
			EndPort:   rule.EndPort,
			Cidr:      rule.Cidr,
			Direction: rule.Direction,
		})
	}

	return awsRules
}

// Azure conversion functions

func convertAzureCluster(c *azure.Cluster) *Cluster {
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

func convertAzureFirewall(f *azure.Firewall) *Firewall {
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

func convertFirewallRulesToAzure(rules []FirewallRule) []azure.FirewallRule {
	var azureRules []azure.FirewallRule
	for _, rule := range rules {
		azureRules = append(azureRules, azure.FirewallRule{
			Protocol:  rule.Protocol,
			StartPort: rule.StartPort,
			EndPort:   rule.EndPort,
			Cidr:      rule.Cidr,
			Direction: rule.Direction,
		})
	}

	return azureRules
}
