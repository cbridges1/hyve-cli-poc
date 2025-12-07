package state

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"civo-cluster-deploy/internal/cluster"
	"civo-cluster-deploy/internal/types"
)

// Manager handles state file operations
type Manager struct {
	stateDir string
}

// NewManager creates a new state manager
func NewManager(stateDir string) *Manager {
	return &Manager{
		stateDir: stateDir,
	}
}

// LoadClusterDefinitions loads all cluster definitions from YAML files
func (m *Manager) LoadClusterDefinitions() ([]types.ClusterDefinition, error) {
	var clusters []types.ClusterDefinition

	err := filepath.WalkDir(m.stateDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		var cluster types.ClusterDefinition
		if err := yaml.Unmarshal(data, &cluster); err != nil {
			return fmt.Errorf("failed to unmarshal YAML file %s: %w", path, err)
		}

		clusters = append(clusters, cluster)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return clusters, nil
}

// ValidateClusterDefinitions validates cluster definitions and ensures master cluster requirements
func (m *Manager) ValidateClusterDefinitions(clusters []types.ClusterDefinition) error {
	masterClusters := make([]types.ClusterDefinition, 0)
	regularClusters := make([]types.ClusterDefinition, 0)

	// Separate master and regular clusters
	for _, cluster := range clusters {
		if cluster.Spec.MasterCluster {
			masterClusters = append(masterClusters, cluster)
		} else {
			regularClusters = append(regularClusters, cluster)
		}
	}

	// Validate that there is exactly one master cluster if any clusters are defined
	if len(clusters) > 0 {
		if len(masterClusters) == 0 {
			return fmt.Errorf("no master cluster defined - at least one cluster must have masterCluster: true")
		}
		if len(masterClusters) > 1 {
			masterNames := make([]string, len(masterClusters))
			for i, master := range masterClusters {
				masterNames[i] = master.Metadata.Name
			}
			return fmt.Errorf("multiple master clusters defined: %v - only one cluster can have masterCluster: true", masterNames)
		}
	}

	return nil
}

// OrderClusters orders clusters so that master clusters come first
func (m *Manager) OrderClusters(clusters []types.ClusterDefinition) []types.ClusterDefinition {
	masterClusters := make([]types.ClusterDefinition, 0)
	regularClusters := make([]types.ClusterDefinition, 0)

	// Separate master and regular clusters
	for _, cluster := range clusters {
		if cluster.Spec.MasterCluster {
			masterClusters = append(masterClusters, cluster)
		} else {
			regularClusters = append(regularClusters, cluster)
		}
	}

	// Return master clusters first, then regular clusters
	orderedClusters := make([]types.ClusterDefinition, 0, len(clusters))
	orderedClusters = append(orderedClusters, masterClusters...)
	orderedClusters = append(orderedClusters, regularClusters...)

	return orderedClusters
}

// FindMasterCluster finds the master cluster in a list of clusters
func (m *Manager) FindMasterCluster(clusters []types.ClusterDefinition) *types.ClusterDefinition {
	for _, cluster := range clusters {
		if cluster.Spec.MasterCluster {
			return &cluster
		}
	}
	return nil
}

// IsMasterClusterReady checks if the master cluster exists and is in active state by querying the API
func (m *Manager) IsMasterClusterReady(clusters []types.ClusterDefinition, clusterMgr *cluster.Manager) bool {
	masterCluster := m.FindMasterCluster(clusters)
	if masterCluster == nil {
		return false
	}

	// Check if master cluster exists and is active via API
	actualCluster, err := clusterMgr.FindByName(masterCluster.Metadata.Name)
	if err != nil || actualCluster == nil {
		return false
	}

	return actualCluster.Status == "ACTIVE"
}
