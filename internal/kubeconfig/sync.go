package kubeconfig

import (
	"context"
	"fmt"
	"log"

	"hyve/internal/provider"
	"hyve/internal/types"
)

// Syncer handles synchronization of kubeconfigs from clusters
type Syncer struct {
	manager  *Manager
	provider provider.Provider
}

// NewSyncer creates a new kubeconfig syncer
func NewSyncer(manager *Manager, provider provider.Provider) *Syncer {
	return &Syncer{
		manager:  manager,
		provider: provider,
	}
}

// SyncKubeconfigs retrieves and stores kubeconfigs for all active clusters
func (s *Syncer) SyncKubeconfigs(ctx context.Context, clusterDefinitions []types.ClusterDefinition) error {
	log.Println("Syncing kubeconfigs for active clusters...")

	var activeClusterNames []string
	successCount := 0

	// Retrieve and store kubeconfigs for each cluster
	for _, clusterDef := range clusterDefinitions {
		activeClusterNames = append(activeClusterNames, clusterDef.Metadata.Name)

		// Check if cluster exists and is active
		cluster, err := s.provider.FindClusterByName(ctx, clusterDef.Metadata.Name)
		if err != nil {
			log.Printf("Failed to find cluster %s: %v", clusterDef.Metadata.Name, err)
			continue
		}

		if cluster == nil {
			log.Printf("Cluster %s not found, skipping kubeconfig sync", clusterDef.Metadata.Name)
			continue
		}

		if cluster.Status != "ACTIVE" {
			log.Printf("Cluster %s is not active (status: %s), skipping kubeconfig sync",
				clusterDef.Metadata.Name, cluster.Status)
			continue
		}

		// Get cluster info with kubeconfig
		clusterInfo, err := s.provider.GetClusterInfo(ctx, clusterDef.Metadata.Name)
		if err != nil {
			log.Printf("Failed to get cluster info for %s: %v", clusterDef.Metadata.Name, err)
			continue
		}

		if clusterInfo.Kubeconfig == "" {
			log.Printf("No kubeconfig available for cluster %s", clusterDef.Metadata.Name)
			continue
		}

		// Store the kubeconfig
		_, err = s.manager.StoreKubeconfig(clusterDef.Metadata.Name, clusterInfo.Kubeconfig)
		if err != nil {
			log.Printf("Failed to store kubeconfig for %s: %v", clusterDef.Metadata.Name, err)
			continue
		}

		log.Printf("✅ Synced kubeconfig for cluster %s", clusterDef.Metadata.Name)
		successCount++
	}

	// Clean up orphaned kubeconfigs (those without corresponding cluster definitions)
	err := s.manager.CleanupOrphanedKubeconfigs(activeClusterNames)
	if err != nil {
		log.Printf("⚠️  Failed to cleanup orphaned kubeconfigs: %v", err)
	}

	log.Printf("Kubeconfig sync completed: %d/%d clusters synced successfully",
		successCount, len(clusterDefinitions))

	return nil
}

// SyncSingleKubeconfig syncs kubeconfig for a single cluster
func (s *Syncer) SyncSingleKubeconfig(ctx context.Context, clusterName string) error {
	log.Printf("Syncing kubeconfig for cluster %s...", clusterName)

	// Check if cluster exists and is active
	cluster, err := s.provider.FindClusterByName(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to find cluster %s: %w", clusterName, err)
	}

	if cluster == nil {
		return fmt.Errorf("cluster %s not found", clusterName)
	}

	if cluster.Status != "ACTIVE" {
		return fmt.Errorf("cluster %s is not active (status: %s)", clusterName, cluster.Status)
	}

	// Get cluster info with kubeconfig
	clusterInfo, err := s.provider.GetClusterInfo(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to get cluster info for %s: %w", clusterName, err)
	}

	if clusterInfo.Kubeconfig == "" {
		return fmt.Errorf("no kubeconfig available for cluster %s", clusterName)
	}

	// Store the kubeconfig
	_, err = s.manager.StoreKubeconfig(clusterName, clusterInfo.Kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to store kubeconfig for %s: %w", clusterName, err)
	}

	log.Printf("✅ Synced kubeconfig for cluster %s", clusterName)
	return nil
}

// GetKubeconfigContent retrieves and decrypts kubeconfig for a cluster
func (s *Syncer) GetKubeconfigContent(clusterName string) (string, error) {
	kc, err := s.manager.GetKubeconfig(clusterName)
	if err != nil {
		return "", fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	if kc == nil {
		return "", fmt.Errorf("kubeconfig not found for cluster %s", clusterName)
	}

	config, err := kc.GetConfig()
	if err != nil {
		return "", fmt.Errorf("failed to decrypt kubeconfig: %w", err)
	}

	return config, nil
}

// ListStoredKubeconfigs lists all stored kubeconfigs for the current repository
func (s *Syncer) ListStoredKubeconfigs() ([]*Kubeconfig, error) {
	return s.manager.ListKubeconfigs()
}
