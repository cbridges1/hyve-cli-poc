package reconcile

import (
	"context"
	"fmt"
	"log"

	"hyve/internal/cluster"
	"hyve/internal/ingress"
	"hyve/internal/kubeconfig"
	"hyve/internal/provider"
	"hyve/internal/providerconfig"
	"hyve/internal/repository"
	"hyve/internal/state"
	"hyve/internal/types"
	"hyve/internal/workflow"
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

	log.Println("Syncing kubeconfigs...")
	err := r.syncKubeconfigs(ctx, clusterDefs)
	if err != nil {
		log.Printf("Failed to sync kubeconfigs: %v", err)
	}

	return nil
}

// reconcileRegion reconciles clusters in a specific region
func (r *Reconciler) reconcileRegion(ctx context.Context, region string, clusters []types.ClusterDefinition) error {
	log.Printf("Processing region: %s", region)

	// Reconcile all desired clusters
	for _, clusterDef := range clusters {
		// Create provider with appropriate options for each cluster
		prov, err := r.createProviderForCluster(clusterDef)
		if err != nil {
			log.Printf("Failed to create provider for cluster %s: %v", clusterDef.Metadata.Name, err)
			continue
		}

		clusterMgr := cluster.NewManager(prov)
		ingressMgr := ingress.NewManager(prov)

		err = r.reconcileCluster(ctx, clusterMgr, ingressMgr, clusterDef)
		if err != nil {
			log.Printf("Failed to reconcile cluster %s: %v", clusterDef.Metadata.Name, err)
		}
	}

	// For cleanup, use a default provider (assuming Civo for backward compatibility)
	// TODO: This should be improved to handle multi-provider cleanup
	if len(clusters) > 0 {
		prov, err := r.createProviderForCluster(clusters[0])
		if err != nil {
			log.Printf("Failed to create provider for cleanup in region %s: %v", region, err)
			return nil
		}
		clusterMgr := cluster.NewManager(prov)
		err = r.cleanupOrphanedResources(ctx, clusterMgr, clusters)
		if err != nil {
			log.Printf("Failed to cleanup orphaned resources in region %s: %v", region, err)
		}
	}

	return nil
}

// createProviderForCluster creates a provider with the appropriate options for a cluster
func (r *Reconciler) createProviderForCluster(clusterDef types.ClusterDefinition) (provider.Provider, error) {
	providerName := clusterDef.Spec.Provider
	if providerName == "" {
		providerName = "civo" // default
	}

	opts := provider.ProviderOptions{
		Region: clusterDef.Metadata.Region,
		APIKey: r.apiKey,
	}

	// Handle GCP-specific configuration
	if providerName == "gcp" {
		// Use stored project ID if available, otherwise resolve from alias
		if clusterDef.Spec.GCPProjectID != "" {
			opts.ProjectID = clusterDef.Spec.GCPProjectID
			log.Printf("Using GCP project ID '%s' for cluster %s",
				clusterDef.Spec.GCPProjectID, clusterDef.Metadata.Name)
		} else if clusterDef.Spec.GCPProject != "" {
			// Fall back to resolving alias (for backward compatibility)
			projectID, err := r.resolveGCPProjectID(clusterDef.Spec.GCPProject)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve GCP project '%s': %w", clusterDef.Spec.GCPProject, err)
			}
			opts.ProjectID = projectID
			log.Printf("Using GCP project '%s' (ID: %s) for cluster %s",
				clusterDef.Spec.GCPProject, projectID, clusterDef.Metadata.Name)
		}
	}

	return r.providerFactory.CreateProviderWithOptions(providerName, opts)
}

// resolveGCPProjectID resolves a GCP project alias to its project ID
func (r *Reconciler) resolveGCPProjectID(projectAlias string) (string, error) {
	// Get current repository
	repoMgr, err := repository.NewManager()
	if err != nil {
		return "", fmt.Errorf("failed to create repository manager: %w", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		return "", fmt.Errorf("failed to get current repository: %w", err)
	}

	pcMgr := providerconfig.NewManager(currentRepo.LocalPath)
	return pcMgr.GetGCPProjectID(projectAlias)
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

	// Run onCreated workflows if defined
	if len(clusterDef.Spec.Workflows.OnCreated) > 0 {
		log.Printf("🔄 Running onCreated workflows for cluster %s...", clusterDef.Metadata.Name)
		r.runWorkflows(ctx, clusterDef.Spec.Workflows.OnCreated, clusterDef.Metadata.Name)
	}

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

	// Run onDestroy workflows before deletion if defined
	if len(clusterDef.Spec.Workflows.OnDestroy) > 0 {
		log.Printf("🔄 Running onDestroy workflows for cluster %s...", clusterDef.Metadata.Name)
		r.runWorkflows(ctx, clusterDef.Spec.Workflows.OnDestroy, clusterDef.Metadata.Name)
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
	// Create a default cluster definition for Civo cleanup
	// TODO: This should be improved to handle multi-provider cleanup
	defaultCluster := types.ClusterDefinition{
		Metadata: types.ClusterMetadata{Region: "PHX1"},
		Spec:     types.ClusterSpec{Provider: "civo"},
	}

	prov, err := r.createProviderForCluster(defaultCluster)
	if err != nil {
		log.Printf("Failed to create provider for cleanup: %v", err)
		return
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
	prov, err := r.createProviderForCluster(clusterDef)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	clusterMgr := cluster.NewManager(prov)
	return exportClusterInfoToEnv(ctx, clusterMgr, clusterDef.Metadata.Name)
}

// syncKubeconfigs syncs kubeconfigs for all active clusters
func (r *Reconciler) syncKubeconfigs(ctx context.Context, clusterDefs []types.ClusterDefinition) error {
	// Get current repository name
	repoMgr, err := repository.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create repository manager: %w", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		return fmt.Errorf("failed to get current repository: %w", err)
	}

	// Create kubeconfig manager
	kubeconfigMgr, err := kubeconfig.NewManager(currentRepo.Name)
	if err != nil {
		return fmt.Errorf("failed to create kubeconfig manager: %w", err)
	}
	defer kubeconfigMgr.Close()

	// Group clusters by region to minimize provider creation
	regionClusters := make(map[string][]types.ClusterDefinition)
	for _, clusterDef := range clusterDefs {
		region := clusterDef.Metadata.Region
		regionClusters[region] = append(regionClusters[region], clusterDef)
	}

	// Sync kubeconfigs for each region
	for region, clusters := range regionClusters {
		// Sync kubeconfig for each cluster individually to handle different project IDs
		for _, clusterDef := range clusters {
			prov, err := r.createProviderForCluster(clusterDef)
			if err != nil {
				log.Printf("Failed to create provider for cluster %s in region %s: %v",
					clusterDef.Metadata.Name, region, err)
				continue
			}

			// Create syncer and sync kubeconfig for this cluster
			syncer := kubeconfig.NewSyncer(kubeconfigMgr, prov)
			err = syncer.SyncKubeconfigs(ctx, []types.ClusterDefinition{clusterDef})
			if err != nil {
				log.Printf("Failed to sync kubeconfig for cluster %s in region %s: %v",
					clusterDef.Metadata.Name, region, err)
				continue
			}
		}
	}

	return nil
}

// runWorkflows executes a list of workflows for a cluster
func (r *Reconciler) runWorkflows(ctx context.Context, workflowNames []string, clusterName string) {
	if len(workflowNames) == 0 {
		return
	}

	// Get local path from current repository
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Printf("⚠️  Failed to create repository manager: %v", err)
		return
	}
	defer repoMgr.Close()
	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		log.Printf("⚠️  Failed to get current repository: %v", err)
		return
	}

	// Create workflow manager
	workflowMgr, err := workflow.NewManager(currentRepo.LocalPath)
	if err != nil {
		log.Printf("⚠️  Failed to create workflow manager: %v", err)
		return
	}

	// Create workflow executor
	executor, err := workflow.NewExecutor(workflowMgr, clusterName)
	if err != nil {
		log.Printf("⚠️  Failed to create workflow executor: %v", err)
		return
	}
	defer executor.Close()

	// Run each workflow
	for _, workflowName := range workflowNames {
		log.Printf("▶️  Running workflow '%s' for cluster '%s'...", workflowName, clusterName)

		execution, err := executor.RunWorkflow(ctx, workflowName, clusterName)
		if err != nil {
			log.Printf("⚠️  Workflow '%s' failed: %v", workflowName, err)
			continue
		}

		if execution.Status == workflow.StatusCompleted {
			log.Printf("✅ Workflow '%s' completed successfully", workflowName)
		} else {
			log.Printf("⚠️  Workflow '%s' finished with status: %s", workflowName, execution.Status)
		}
	}
}
