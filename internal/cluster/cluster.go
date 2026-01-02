package cluster

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/civo/civogo"

	"civo-cluster-deploy/internal/types"
)

// Manager handles cluster operations
type Manager struct {
	client *civogo.Client
}

// NewManager creates a new cluster manager
func NewManager(client *civogo.Client) *Manager {
	return &Manager{
		client: client,
	}
}

// DetermineAction decides what action to take based on desired vs actual state
func (m *Manager) DetermineAction(ctx context.Context, desired types.ClusterDefinition) types.ReconcileAction {
	// First check if cluster exists by name (more reliable than stored ID)
	cluster, err := m.FindByName(desired.Metadata.Name)
	if err != nil {
		log.Printf("Error checking for existing cluster %s: %v", desired.Metadata.Name, err)
		return types.ActionCreate
	}

	if cluster == nil {
		// Cluster doesn't exist, create it
		log.Printf("Cluster %s not found, will create", desired.Metadata.Name)
		return types.ActionCreate
	}

	// Log that we found an existing cluster
	log.Printf("Found existing cluster %s with ID %s", desired.Metadata.Name, cluster.ID)

	if cluster.Status == "ACTIVE" {
		if m.needsUpdate(cluster, desired) {
			log.Printf("Cluster %s configuration differs from desired state, will update", desired.Metadata.Name)
			return types.ActionUpdate
		}
		log.Printf("Cluster %s is up to date", desired.Metadata.Name)
		return types.ActionNone
	}

	if cluster.Status == "ERROR" {
		log.Printf("Cluster %s is in error state, will recreate", desired.Metadata.Name)
		return types.ActionCreate
	}

	if cluster.Status == "BUILDING" || cluster.Status == "SCALING" {
		log.Printf("Cluster %s is in %s state, will wait", desired.Metadata.Name, cluster.Status)
		return types.ActionNone
	}

	log.Printf("Cluster %s is in unknown state %s, will check again", desired.Metadata.Name, cluster.Status)
	return types.ActionNone
}

// needsUpdate compares actual cluster with desired configuration
func (m *Manager) needsUpdate(actual *civogo.KubernetesCluster, desired types.ClusterDefinition) bool {
	configDiffers := false

	desiredNodeCount := len(desired.Spec.Nodes)
	if actual.NumTargetNode != desiredNodeCount {
		log.Printf("Node count differs: actual=%d, desired=%d", actual.NumTargetNode, desiredNodeCount)
		configDiffers = true
	}

	// For now, we'll use the first node size as the target size
	// In the future, we might want to support mixed node sizes per cluster
	var desiredSize string
	if len(desired.Spec.Nodes) > 0 {
		desiredSize = desired.Spec.Nodes[0]
	}
	if actual.TargetNodeSize != desiredSize {
		log.Printf("Node size differs: actual=%s, desired=%s", actual.TargetNodeSize, desiredSize)
		configDiffers = true
	}

	// Note: Kubernetes version (cluster type) usually cannot be updated in place
	// This would typically require a cluster recreation
	if actual.KubernetesVersion != desired.Spec.ClusterType {
		log.Printf("Kubernetes version differs: actual=%s, desired=%s (may require recreation)",
			actual.KubernetesVersion, desired.Spec.ClusterType)
		// For now, we'll log this but not trigger an update as it's complex
	}

	return configDiffers
}

// FindByName finds a cluster by name
func (m *Manager) FindByName(name string) (*civogo.KubernetesCluster, error) {
	clusters, err := m.client.ListKubernetesClusters()
	if err != nil {
		return nil, err
	}

	for _, cluster := range clusters.Items {
		if cluster.Name == name {
			return &cluster, nil
		}
	}

	return nil, nil // Not found, but no error
}

// Create creates a new cluster
func (m *Manager) Create(ctx context.Context, clusterDef types.ClusterDefinition, firewallID string) (*civogo.KubernetesCluster, error) {
	log.Printf("Creating cluster %s in region %s", clusterDef.Metadata.Name, clusterDef.Metadata.Region)

	// Calculate node count and size from nodes array
	nodeCount := len(clusterDef.Spec.Nodes)
	var nodeSize string
	if nodeCount > 0 {
		nodeSize = clusterDef.Spec.Nodes[0] // Use first node size as cluster size
		// Log if multiple different sizes are specified (not currently supported by Civo)
		if nodeCount > 1 {
			firstSize := clusterDef.Spec.Nodes[0]
			for i, size := range clusterDef.Spec.Nodes {
				if size != firstSize {
					log.Printf("Warning: Node %d has different size %s than first node %s. Using %s for all nodes.",
						i, size, firstSize, firstSize)
					break
				}
			}
		}
	} else {
		return nil, fmt.Errorf("no nodes specified for cluster %s", clusterDef.Metadata.Name)
	}

	clusterConfig := &civogo.KubernetesClusterConfig{
		Name:            clusterDef.Metadata.Name,
		NumTargetNodes:  nodeCount,
		TargetNodesSize: nodeSize,
		ClusterType:     clusterDef.Spec.ClusterType,
		FirewallID:      firewallID,
	}

	cluster, err := m.client.NewKubernetesClusters(clusterConfig)
	if err != nil {
		// If cluster creation fails, check if it was created by another process
		existingCluster, findErr := m.FindByName(clusterDef.Metadata.Name)
		if findErr == nil && existingCluster != nil {
			log.Printf("Cluster %s was created by another process, using existing cluster", clusterDef.Metadata.Name)
			cluster = existingCluster
		} else {
			return nil, fmt.Errorf("failed to create cluster: %w", err)
		}
	} else {
		log.Printf("Cluster creation started with ID: %s", cluster.ID)
	}

	return cluster, nil
}

// Update updates an existing cluster
func (m *Manager) Update(ctx context.Context, clusterDef types.ClusterDefinition) error {
	// Find the existing cluster
	existingCluster, err := m.FindByName(clusterDef.Metadata.Name)
	if err != nil {
		return fmt.Errorf("failed to find existing cluster: %w", err)
	}

	if existingCluster == nil {
		return fmt.Errorf("cluster %s not found for update", clusterDef.Metadata.Name)
	}

	log.Printf("Updating cluster %s (ID: %s)", clusterDef.Metadata.Name, existingCluster.ID)

	// Check what needs to be updated
	updateNeeded := false
	updateConfig := &civogo.KubernetesClusterConfig{
		Name:            existingCluster.Name,          // Keep existing name
		NumTargetNodes:  existingCluster.NumTargetNode, // Start with current values
		TargetNodesSize: existingCluster.TargetNodeSize,
		ClusterType:     existingCluster.KubernetesVersion,
		FirewallID:      existingCluster.FirewallID,
	}

	// Calculate desired node count and size from nodes array
	desiredNodeCount := len(clusterDef.Spec.Nodes)
	var desiredSize string
	if desiredNodeCount > 0 {
		desiredSize = clusterDef.Spec.Nodes[0]
	}

	// Update node count if different
	if existingCluster.NumTargetNode != desiredNodeCount {
		log.Printf("Updating node count from %d to %d", existingCluster.NumTargetNode, desiredNodeCount)
		updateConfig.NumTargetNodes = desiredNodeCount
		updateNeeded = true
	}

	// Update node size if different
	if existingCluster.TargetNodeSize != desiredSize {
		log.Printf("Updating node size from %s to %s", existingCluster.TargetNodeSize, desiredSize)
		updateConfig.TargetNodesSize = desiredSize
		updateNeeded = true
	}

	if !updateNeeded {
		log.Printf("No updates needed for cluster %s", clusterDef.Metadata.Name)
		return nil
	}

	// Perform the update
	_, err = m.client.UpdateKubernetesCluster(existingCluster.ID, updateConfig)
	if err != nil {
		return fmt.Errorf("failed to update cluster: %w", err)
	}

	log.Printf("Update initiated for cluster %s, waiting for completion", clusterDef.Metadata.Name)

	// Wait for the cluster to be ready after update
	err = m.WaitForReady(ctx, existingCluster.ID)
	if err != nil {
		return fmt.Errorf("failed while waiting for cluster update: %w", err)
	}

	log.Printf("Cluster %s updated successfully!", clusterDef.Metadata.Name)
	return nil
}

// Delete deletes a cluster
func (m *Manager) Delete(ctx context.Context, clusterID string) error {
	_, err := m.client.DeleteKubernetesCluster(clusterID)
	return err
}

// WaitForReady waits for a cluster to become ready
func (m *Manager) WaitForReady(ctx context.Context, clusterID string) error {
	for {
		cluster, err := m.client.GetKubernetesCluster(clusterID)
		if err != nil {
			return err
		}

		if cluster.Status == "ACTIVE" {
			return nil
		}

		if cluster.Status == "ERROR" {
			return fmt.Errorf("cluster creation failed")
		}

		log.Printf("Cluster status: %s, waiting...", cluster.Status)
		time.Sleep(30 * time.Second)
	}
}

// FindOrphaned finds clusters that exist but are not in the desired list
func (m *Manager) FindOrphaned(desiredClusters []types.ClusterDefinition) ([]*civogo.KubernetesCluster, error) {
	// Get all existing clusters
	allClusters, err := m.client.ListKubernetesClusters()
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	// Create a map of desired cluster names for quick lookup
	desiredNames := make(map[string]bool)
	for _, cluster := range desiredClusters {
		desiredNames[cluster.Metadata.Name] = true
	}

	// Find clusters that exist but are not in our desired state
	var orphaned []*civogo.KubernetesCluster
	for _, cluster := range allClusters.Items {
		// Only consider clusters that might have been created by this tool
		if !desiredNames[cluster.Name] {
			// Check if this cluster was likely created by our tool
			if m.ShouldManage(cluster) {
				orphaned = append(orphaned, &cluster)
			}
		}
	}

	return orphaned, nil
}

// ShouldManage determines if a cluster should be managed by this tool
func (m *Manager) ShouldManage(cluster civogo.KubernetesCluster) bool {
	// Add logic to determine if this cluster should be managed by our tool
	// This function determines which clusters should be considered for deletion
	// when they're not found in our YAML definitions

	// Option 1: Check if cluster has a firewall (indicating it might be managed by us)
	// This is conservative but safer
	if cluster.FirewallID != "" {
		return true
	}

	// For safety, we won't delete clusters that don't have clear indicators
	// they were created by our tool. You can make this more aggressive if needed.
	return false
}

// GetClusterInfo returns detailed information about a cluster for GitHub Actions export
func (m *Manager) GetClusterInfo(ctx context.Context, clusterName string) (*ClusterInfo, error) {
	cluster, err := m.FindByName(clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to find cluster %s: %w", clusterName, err)
	}

	if cluster == nil {
		return nil, fmt.Errorf("cluster %s not found", clusterName)
	}

	// Get kubeconfig
	kubeconfig, err := m.client.GetKubernetesClusterKubeconfig(cluster.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig for cluster %s: %w", clusterName, err)
	}

	// Get cluster details
	clusterDetails, err := m.client.GetKubernetesCluster(cluster.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster details for %s: %w", clusterName, err)
	}

	info := &ClusterInfo{
		Name:       cluster.Name,
		IPAddress:  clusterDetails.MasterIP,
		AccessPort: "6443", // Standard Kubernetes API port
		Kubeconfig: kubeconfig,
		Status:     cluster.Status,
		ID:         cluster.ID,
	}

	return info, nil
}

// ClusterInfo holds cluster information for export
type ClusterInfo struct {
	Name       string
	IPAddress  string
	AccessPort string
	Kubeconfig string
	Status     string
	ID         string
}

// CleanupOrphaned deletes orphaned clusters
func (m *Manager) CleanupOrphaned(ctx context.Context, orphanedClusters []*civogo.KubernetesCluster) error {
	if len(orphanedClusters) == 0 {
		log.Println("No orphaned clusters found")
		return nil
	}

	log.Printf("Found %d orphaned clusters that will be deleted", len(orphanedClusters))

	for _, cluster := range orphanedClusters {
		log.Printf("Deleting orphaned cluster: %s (ID: %s)", cluster.Name, cluster.ID)

		// Delete the cluster
		err := m.Delete(ctx, cluster.ID)
		if err != nil {
			log.Printf("Failed to delete orphaned cluster %s: %v", cluster.Name, err)
			continue
		}

		// Delete associated firewall if it exists
		if cluster.FirewallID != "" {
			_, err = m.client.DeleteFirewall(cluster.FirewallID)
			if err != nil {
				log.Printf("Failed to delete firewall %s for cluster %s: %v",
					cluster.FirewallID, cluster.Name, err)
			} else {
				log.Printf("Deleted firewall %s for cluster %s", cluster.FirewallID, cluster.Name)
			}
		}

		log.Printf("Successfully deleted orphaned cluster: %s", cluster.Name)
	}

	return nil
}
