package reconcile

import (
	"context"
	"fmt"
	"log"

	"civo-cluster-deploy/internal/cluster"
	"civo-cluster-deploy/internal/ingress"
	"civo-cluster-deploy/internal/provider"
	"civo-cluster-deploy/internal/state"
	"civo-cluster-deploy/internal/types"
)

// Reconciler handles the reconciliation of clusters using provider abstraction
type Reconciler struct {
	providerFactory *provider.Factory
	stateMgr        *state.Manager
	apiKey          string
}

// NewReconciler creates a new reconciler
func NewReconciler(apiKey string, stateMgr *state.Manager) *Reconciler {
	return &Reconciler{
		providerFactory: provider.NewFactory(),
		stateMgr:        stateMgr,
		apiKey:          apiKey,
	}
}

// ReconcileAll reconciles all clusters across all regions
func (r *Reconciler) ReconcileAll(ctx context.Context, clusterDefs []types.ClusterDefinition) error {
	regionClusters := make(map[string][]types.ClusterDefinition)
	for _, clusterDef := range clusterDefs {
		region := clusterDef.Metadata.Region
		regionClusters[region] = append(regionClusters[region], clusterDef)
	}

	if len(clusterDefs) == 0 {
		log.Println("No cluster definitions found in state/clusters directory")
		r.cleanupAllRegions(ctx, clusterDefs)
		return nil
	}

	for region, clusters := range regionClusters {
		err := r.reconcileRegion(ctx, region, clusters)
		if err != nil {
			log.Printf("Failed to reconcile region %s: %v", region, err)
		}
	}

	log.Println("Exporting cluster information...")
	r.exportAllClusterInfo(ctx, clusterDefs)
	return nil
}

// reconcileRegion reconciles clusters in a specific region
func (r *Reconciler) reconcileRegion(ctx context.Context, region string, clusters []types.ClusterDefinition) error {
	log.Printf("Processing region: %s", region)

	// Get provider name from first cluster (assuming all clusters in region use same provider)
	providerName := "civo" // default
	if len(clusters) > 0 {
		providerName = clusters[0].Spec.Provider
	}

	// Create provider for this region
	prov, err := r.providerFactory.CreateProvider(providerName, r.apiKey, region)
	if err != nil {
		return fmt.Errorf("failed to create provider for region %s: %w", region, err)
	}

	// Initialize managers for this region
	clusterMgr := cluster.NewManager(prov)
	ingressMgr := ingress.NewManager(prov)

	// Reconcile all desired clusters
	for _, clusterDef := range clusters {
		err := r.reconcileCluster(ctx, clusterMgr, ingressMgr, clusterDef)
		if err != nil {
			log.Printf("Failed to reconcile cluster %s: %v", clusterDef.Metadata.Name, err)
		}
	}

	// Then, cleanup orphaned resources
	err = r.cleanupOrphanedResources(ctx, clusterMgr, clusters)
	if err != nil {
		log.Printf("Failed to cleanup orphaned resources in region %s: %v", region, err)
	}

	return nil
}

// reconcileCluster handles the reconciliation of a single cluster
func (r *Reconciler) reconcileCluster(ctx context.Context, clusterMgr *cluster.Manager, ingressMgr *ingress.Manager, clusterDef types.ClusterDefinition) error {
	action := clusterMgr.DetermineAction(ctx, clusterDef)

	switch action {
	case types.ActionCreate:
		return r.createCluster(ctx, clusterMgr, ingressMgr, clusterDef)
	case types.ActionUpdate:
		return r.updateCluster(ctx, clusterMgr, ingressMgr, clusterDef)
	case types.ActionDelete:
		return r.deleteCluster(ctx, clusterMgr, ingressMgr, clusterDef)
	case types.ActionNone:
		log.Printf("Cluster %s is up to date, no action needed", clusterDef.Metadata.Name)
		return nil
	default:
		log.Printf("Unknown action for cluster %s: %v", clusterDef.Metadata.Name, action)
		return nil
	}
}

// createCluster creates a new cluster with optional ingress controller
func (r *Reconciler) createCluster(ctx context.Context, clusterMgr *cluster.Manager, ingressMgr *ingress.Manager, clusterDef types.ClusterDefinition) error {
	existingCluster, err := clusterMgr.FindByName(ctx, clusterDef.Metadata.Name)
	if err != nil {
		return err
	}

	if existingCluster != nil {
		log.Printf("Cluster %s already exists with ID %s, updating status and continuing",
			clusterDef.Metadata.Name, existingCluster.ID)
		log.Printf("Cluster %s already exists and is in %s state", clusterDef.Metadata.Name, existingCluster.Status)
		return nil
	}

	log.Printf("Creating cluster with definition: %+v", clusterDef)
	createdCluster, err := clusterMgr.Create(ctx, clusterDef)
	if err != nil {
		return err
	}

	err = clusterMgr.WaitForReady(ctx, createdCluster.ID)
	if err != nil {
		return err
	}
	log.Println("Cluster is ready!")

	var loadBalancerIP string
	if clusterDef.Spec.Ingress.Enabled {
		lb, err := ingressMgr.DeployIngressController(ctx, createdCluster.ID, clusterDef.Spec.Ingress)
		if err != nil {
			log.Printf("Failed to deploy ingress controller: %v", err)
		} else if lb != nil {
			loadBalancerIP = lb.PublicIP
			log.Printf("Ingress controller deployed with load balancer IP: %s", loadBalancerIP)
		}
	}

	log.Printf("Cluster %s created successfully!", clusterDef.Metadata.Name)
	return nil
}

// updateCluster updates an existing cluster
func (r *Reconciler) updateCluster(ctx context.Context, clusterMgr *cluster.Manager, ingressMgr *ingress.Manager, clusterDef types.ClusterDefinition) error {
	return clusterMgr.Update(ctx, clusterDef)
}

// deleteCluster deletes a cluster and its associated resources
func (r *Reconciler) deleteCluster(ctx context.Context, clusterMgr *cluster.Manager, ingressMgr *ingress.Manager, clusterDef types.ClusterDefinition) error {
	existingCluster, err := clusterMgr.FindByName(ctx, clusterDef.Metadata.Name)
	if err != nil {
		return err
	}

	if existingCluster == nil {
		log.Printf("Cluster %s not found, nothing to delete", clusterDef.Metadata.Name)
		return nil
	}

	log.Printf("Deleting cluster %s with ID %s", clusterDef.Metadata.Name, existingCluster.ID)

	if clusterDef.Spec.Ingress.Enabled {
		err := ingressMgr.RemoveIngressController(ctx, existingCluster.ID)
		if err != nil {
			log.Printf("Failed to remove ingress controller: %v", err)
		}
	}

	err = clusterMgr.Delete(ctx, existingCluster.ID)
	if err != nil {
		return err
	}

	log.Printf("Cluster %s deleted successfully!", clusterDef.Metadata.Name)
	return nil
}

// cleanupOrphanedResources removes clusters that are no longer defined
func (r *Reconciler) cleanupOrphanedResources(ctx context.Context, clusterMgr *cluster.Manager, clusters []types.ClusterDefinition) error {
	orphanedClusters, err := clusterMgr.FindOrphaned(ctx, clusters)
	if err != nil {
		return err
	}

	return clusterMgr.CleanupOrphaned(ctx, orphanedClusters)
}

// cleanupAllRegions handles cleanup when no clusters are defined
func (r *Reconciler) cleanupAllRegions(ctx context.Context, clusterDefs []types.ClusterDefinition) {
	defaultRegion := "PHX1"
	prov, err := r.providerFactory.CreateProvider("civo", r.apiKey, defaultRegion)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	clusterMgr := cluster.NewManager(prov)

	err = r.cleanupOrphanedResources(ctx, clusterMgr, clusterDefs)
	if err != nil {
		log.Printf("Failed to cleanup orphaned resources: %v", err)
	}
}

// exportAllClusterInfo exports information for all active clusters
func (r *Reconciler) exportAllClusterInfo(ctx context.Context, clusterDefs []types.ClusterDefinition) {
	for _, clusterDef := range clusterDefs {
		err := r.exportClusterInfo(ctx, clusterDef)
		if err != nil {
			log.Printf("Failed to export info for cluster %s: %v", clusterDef.Metadata.Name, err)
		}
	}
}

// exportClusterInfo exports cluster information
func (r *Reconciler) exportClusterInfo(ctx context.Context, clusterDef types.ClusterDefinition) error {
	prov, err := r.providerFactory.CreateProvider(clusterDef.Spec.Provider, r.apiKey, clusterDef.Metadata.Region)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	clusterMgr := cluster.NewManager(prov)
	return exportClusterInfoToEnv(ctx, clusterMgr, clusterDef.Metadata.Name)
}
