package cluster

import (
	"context"
	"errors"
	"testing"
	"time"

	"civo-cluster-deploy/internal/provider"
	"civo-cluster-deploy/internal/types"
)

// mockProvider implements the provider.Provider interface for testing
type mockProvider struct {
	clusters     map[string]*provider.Cluster
	shouldError  bool
	errorMessage string
	clusterInfos map[string]*provider.ClusterInfo
}

func newMockProvider() *mockProvider {
	return &mockProvider{
		clusters:     make(map[string]*provider.Cluster),
		clusterInfos: make(map[string]*provider.ClusterInfo),
	}
}

// ClusterProvider methods
func (m *mockProvider) ListClusters(ctx context.Context) ([]*provider.Cluster, error) {
	if m.shouldError {
		return nil, errors.New(m.errorMessage)
	}

	clusters := make([]*provider.Cluster, 0, len(m.clusters))
	for _, cluster := range m.clusters {
		clusters = append(clusters, cluster)
	}
	return clusters, nil
}

func (m *mockProvider) GetCluster(ctx context.Context, clusterID string) (*provider.Cluster, error) {
	if m.shouldError {
		return nil, errors.New(m.errorMessage)
	}

	for _, cluster := range m.clusters {
		if cluster.ID == clusterID {
			return cluster, nil
		}
	}
	return nil, errors.New("cluster not found")
}

func (m *mockProvider) FindClusterByName(ctx context.Context, name string) (*provider.Cluster, error) {
	if m.shouldError {
		return nil, errors.New(m.errorMessage)
	}

	cluster, exists := m.clusters[name]
	if !exists {
		return nil, nil
	}
	return cluster, nil
}

func (m *mockProvider) CreateCluster(ctx context.Context, config *provider.ClusterConfig) (*provider.Cluster, error) {
	if m.shouldError {
		return nil, errors.New(m.errorMessage)
	}

	cluster := &provider.Cluster{
		ID:        "cluster-" + config.Name,
		Name:      config.Name,
		Status:    "BUILDING",
		CreatedAt: time.Now(),
	}
	m.clusters[config.Name] = cluster
	return cluster, nil
}

func (m *mockProvider) UpdateCluster(ctx context.Context, clusterID string, config *provider.ClusterUpdateConfig) (*provider.Cluster, error) {
	if m.shouldError {
		return nil, errors.New(m.errorMessage)
	}

	for _, cluster := range m.clusters {
		if cluster.ID == clusterID {
			cluster.Status = "UPDATING"
			return cluster, nil
		}
	}
	return nil, errors.New("cluster not found")
}

func (m *mockProvider) DeleteCluster(ctx context.Context, clusterID string) error {
	if m.shouldError {
		return errors.New(m.errorMessage)
	}

	for name, cluster := range m.clusters {
		if cluster.ID == clusterID {
			delete(m.clusters, name)
			return nil
		}
	}
	return errors.New("cluster not found")
}

func (m *mockProvider) WaitForClusterReady(ctx context.Context, clusterID string) error {
	if m.shouldError {
		return errors.New(m.errorMessage)
	}

	for _, cluster := range m.clusters {
		if cluster.ID == clusterID {
			cluster.Status = "ACTIVE"
			return nil
		}
	}
	return errors.New("cluster not found")
}

func (m *mockProvider) GetClusterInfo(ctx context.Context, name string) (*provider.ClusterInfo, error) {
	if m.shouldError {
		return nil, errors.New(m.errorMessage)
	}

	info, exists := m.clusterInfos[name]
	if !exists {
		return nil, errors.New("cluster info not found")
	}
	return info, nil
}

// FirewallProvider methods (not used in cluster manager but required by interface)
func (m *mockProvider) ListFirewalls(ctx context.Context) ([]*provider.Firewall, error) {
	return nil, nil
}

func (m *mockProvider) CreateFirewall(ctx context.Context, config *provider.FirewallConfig) (*provider.Firewall, error) {
	return nil, nil
}

func (m *mockProvider) DeleteFirewall(ctx context.Context, firewallID string) error {
	return nil
}

func (m *mockProvider) FindFirewallByName(ctx context.Context, name string) (*provider.Firewall, error) {
	return nil, nil
}

// IngressProvider methods (not used in cluster manager but required by interface)
func (m *mockProvider) ListLoadBalancers(ctx context.Context) ([]*provider.LoadBalancer, error) {
	return nil, nil
}

func (m *mockProvider) DeployIngressController(ctx context.Context, clusterID string, spec types.IngressSpec) (*provider.LoadBalancer, error) {
	return nil, nil
}

func (m *mockProvider) RemoveIngressController(ctx context.Context, clusterID string) error {
	return nil
}

func (m *mockProvider) GetLoadBalancerIP(ctx context.Context, clusterID string) (string, error) {
	return "", nil
}

// Provider metadata
func (m *mockProvider) Name() string {
	return "mock"
}

func (m *mockProvider) Region() string {
	return "mock-region"
}

// TestDetermineAction_Create tests that DetermineAction returns ActionCreate for new cluster
func TestDetermineAction_Create(t *testing.T) {
	mockProv := newMockProvider()
	mgr := NewManager(mockProv)

	desired := types.ClusterDefinition{
		Metadata: types.ClusterMetadata{
			Name:   "new-cluster",
			Region: "mock-region",
		},
	}

	action := mgr.DetermineAction(context.Background(), desired)
	if action != types.ActionCreate {
		t.Errorf("Expected ActionCreate, got %v", action)
	}
}

// TestDetermineAction_None tests that DetermineAction returns ActionNone for active cluster
func TestDetermineAction_None(t *testing.T) {
	mockProv := newMockProvider()
	mockProv.clusters["existing-cluster"] = &provider.Cluster{
		ID:     "cluster-1",
		Name:   "existing-cluster",
		Status: "ACTIVE",
	}

	mgr := NewManager(mockProv)

	desired := types.ClusterDefinition{
		Metadata: types.ClusterMetadata{
			Name:   "existing-cluster",
			Region: "mock-region",
		},
	}

	action := mgr.DetermineAction(context.Background(), desired)
	if action != types.ActionNone {
		t.Errorf("Expected ActionNone, got %v", action)
	}
}

// TestDetermineAction_CreateForFailedCluster tests recreation for failed cluster
func TestDetermineAction_CreateForFailedCluster(t *testing.T) {
	mockProv := newMockProvider()
	mockProv.clusters["failed-cluster"] = &provider.Cluster{
		ID:     "cluster-1",
		Name:   "failed-cluster",
		Status: "FAILED",
	}

	mgr := NewManager(mockProv)

	desired := types.ClusterDefinition{
		Metadata: types.ClusterMetadata{
			Name:   "failed-cluster",
			Region: "mock-region",
		},
	}

	action := mgr.DetermineAction(context.Background(), desired)
	if action != types.ActionCreate {
		t.Errorf("Expected ActionCreate for failed cluster, got %v", action)
	}
}

// TestFindByName tests finding a cluster by name
func TestFindByName(t *testing.T) {
	mockProv := newMockProvider()
	mockProv.clusters["test-cluster"] = &provider.Cluster{
		ID:     "cluster-1",
		Name:   "test-cluster",
		Status: "ACTIVE",
	}

	mgr := NewManager(mockProv)

	cluster, err := mgr.FindByName(context.Background(), "test-cluster")
	if err != nil {
		t.Fatalf("Failed to find cluster: %v", err)
	}

	if cluster.Name != "test-cluster" {
		t.Errorf("Expected cluster name test-cluster, got %s", cluster.Name)
	}
}

// TestFindByName_NotFound tests finding a non-existent cluster
func TestFindByName_NotFound(t *testing.T) {
	mockProv := newMockProvider()
	mgr := NewManager(mockProv)

	cluster, err := mgr.FindByName(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if cluster != nil {
		t.Error("Expected nil cluster for non-existent name")
	}
}

// TestCreate tests creating a new cluster
func TestCreate(t *testing.T) {
	mockProv := newMockProvider()
	mgr := NewManager(mockProv)

	clusterDef := types.ClusterDefinition{
		Metadata: types.ClusterMetadata{
			Name:   "new-cluster",
			Region: "mock-region",
		},
		Spec: types.ClusterSpec{
			Nodes:       []string{"g4s.kube.small:2"},
			ClusterType: "k3s",
		},
	}

	cluster, err := mgr.Create(context.Background(), clusterDef)
	if err != nil {
		t.Fatalf("Failed to create cluster: %v", err)
	}

	if cluster.Name != "new-cluster" {
		t.Errorf("Expected cluster name new-cluster, got %s", cluster.Name)
	}

	if cluster.Status != "BUILDING" {
		t.Errorf("Expected cluster status BUILDING, got %s", cluster.Status)
	}
}

// TestUpdate tests updating an existing cluster
func TestUpdate(t *testing.T) {
	mockProv := newMockProvider()
	mockProv.clusters["existing-cluster"] = &provider.Cluster{
		ID:     "cluster-1",
		Name:   "existing-cluster",
		Status: "ACTIVE",
	}

	mgr := NewManager(mockProv)

	clusterDef := types.ClusterDefinition{
		Metadata: types.ClusterMetadata{
			Name:   "existing-cluster",
			Region: "mock-region",
		},
		Spec: types.ClusterSpec{
			Nodes: []string{"g4s.kube.small:3"},
		},
	}

	err := mgr.Update(context.Background(), clusterDef)
	if err != nil {
		t.Fatalf("Failed to update cluster: %v", err)
	}
}

// TestUpdate_NotFound tests updating a non-existent cluster
func TestUpdate_NotFound(t *testing.T) {
	mockProv := newMockProvider()
	mgr := NewManager(mockProv)

	clusterDef := types.ClusterDefinition{
		Metadata: types.ClusterMetadata{
			Name:   "nonexistent",
			Region: "mock-region",
		},
	}

	err := mgr.Update(context.Background(), clusterDef)
	if err == nil {
		t.Error("Expected error when updating non-existent cluster")
	}
}

// TestDelete tests deleting a cluster
func TestDelete(t *testing.T) {
	mockProv := newMockProvider()
	mockProv.clusters["test-cluster"] = &provider.Cluster{
		ID:     "cluster-1",
		Name:   "test-cluster",
		Status: "ACTIVE",
	}

	mgr := NewManager(mockProv)

	err := mgr.Delete(context.Background(), "cluster-1")
	if err != nil {
		t.Fatalf("Failed to delete cluster: %v", err)
	}

	// Verify cluster is gone
	if _, exists := mockProv.clusters["test-cluster"]; exists {
		t.Error("Cluster should have been deleted")
	}
}

// TestWaitForReady tests waiting for cluster to be ready
func TestWaitForReady(t *testing.T) {
	mockProv := newMockProvider()
	mockProv.clusters["test-cluster"] = &provider.Cluster{
		ID:     "cluster-1",
		Name:   "test-cluster",
		Status: "BUILDING",
	}

	mgr := NewManager(mockProv)

	err := mgr.WaitForReady(context.Background(), "cluster-1")
	if err != nil {
		t.Fatalf("Failed to wait for cluster: %v", err)
	}

	// Verify cluster is now ACTIVE
	cluster := mockProv.clusters["test-cluster"]
	if cluster.Status != "ACTIVE" {
		t.Errorf("Expected cluster status ACTIVE, got %s", cluster.Status)
	}
}

// TestFindOrphaned tests finding orphaned clusters
func TestFindOrphaned(t *testing.T) {
	mockProv := newMockProvider()
	mockProv.clusters["hyve-managed"] = &provider.Cluster{
		ID:     "cluster-1",
		Name:   "hyve-managed",
		Status: "ACTIVE",
	}
	mockProv.clusters["hyve-orphaned"] = &provider.Cluster{
		ID:     "cluster-2",
		Name:   "hyve-orphaned",
		Status: "ACTIVE",
	}
	mockProv.clusters["unmanaged-cluster"] = &provider.Cluster{
		ID:     "cluster-3",
		Name:   "unmanaged-cluster",
		Status: "ACTIVE",
	}

	mgr := NewManager(mockProv)

	desiredClusters := []types.ClusterDefinition{
		{
			Metadata: types.ClusterMetadata{
				Name: "hyve-managed",
			},
		},
	}

	orphaned, err := mgr.FindOrphaned(context.Background(), desiredClusters)
	if err != nil {
		t.Fatalf("Failed to find orphaned clusters: %v", err)
	}

	// Should find hyve-orphaned but not unmanaged-cluster
	if len(orphaned) != 1 {
		t.Errorf("Expected 1 orphaned cluster, got %d", len(orphaned))
	}

	if len(orphaned) > 0 && orphaned[0].Name != "hyve-orphaned" {
		t.Errorf("Expected orphaned cluster hyve-orphaned, got %s", orphaned[0].Name)
	}
}

// TestShouldManage tests the ShouldManage logic
func TestShouldManage(t *testing.T) {
	mgr := NewManager(newMockProvider())

	testCases := []struct {
		name     string
		cluster  provider.Cluster
		expected bool
	}{
		{
			name:     "hyve prefix",
			cluster:  provider.Cluster{Name: "hyve-test-cluster"},
			expected: true,
		},
		{
			name:     "civo-deploy prefix",
			cluster:  provider.Cluster{Name: "civo-deploy-test"},
			expected: true,
		},
		{
			name:     "unmanaged cluster",
			cluster:  provider.Cluster{Name: "my-custom-cluster"},
			expected: false,
		},
		{
			name:     "empty name",
			cluster:  provider.Cluster{Name: ""},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := mgr.ShouldManage(tc.cluster)
			if result != tc.expected {
				t.Errorf("Expected ShouldManage to return %v for %s, got %v", tc.expected, tc.cluster.Name, result)
			}
		})
	}
}

// TestGetClusterInfo tests getting cluster information
func TestGetClusterInfo(t *testing.T) {
	mockProv := newMockProvider()
	mockProv.clusterInfos["test-cluster"] = &provider.ClusterInfo{
		Name:       "test-cluster",
		IPAddress:  "1.2.3.4",
		AccessPort: "6443",
		Status:     "ACTIVE",
		ID:         "cluster-1",
	}

	mgr := NewManager(mockProv)

	info, err := mgr.GetClusterInfo(context.Background(), "test-cluster")
	if err != nil {
		t.Fatalf("Failed to get cluster info: %v", err)
	}

	if info.Name != "test-cluster" {
		t.Errorf("Expected name test-cluster, got %s", info.Name)
	}
	if info.IPAddress != "1.2.3.4" {
		t.Errorf("Expected IP 1.2.3.4, got %s", info.IPAddress)
	}
}

// TestCleanupOrphaned tests cleaning up orphaned clusters
func TestCleanupOrphaned(t *testing.T) {
	mockProv := newMockProvider()
	mockProv.clusters["orphan1"] = &provider.Cluster{
		ID:     "cluster-1",
		Name:   "orphan1",
		Status: "ACTIVE",
	}
	mockProv.clusters["orphan2"] = &provider.Cluster{
		ID:     "cluster-2",
		Name:   "orphan2",
		Status: "ACTIVE",
	}

	mgr := NewManager(mockProv)

	orphaned := []*provider.Cluster{
		mockProv.clusters["orphan1"],
		mockProv.clusters["orphan2"],
	}

	err := mgr.CleanupOrphaned(context.Background(), orphaned)
	if err != nil {
		t.Fatalf("Failed to cleanup orphaned clusters: %v", err)
	}

	// Verify clusters were deleted
	if len(mockProv.clusters) != 0 {
		t.Errorf("Expected all orphaned clusters to be deleted, %d remaining", len(mockProv.clusters))
	}
}

// TestErrorHandling tests error handling in various operations
func TestErrorHandling(t *testing.T) {
	mockProv := newMockProvider()
	mockProv.shouldError = true
	mockProv.errorMessage = "mock error"

	mgr := NewManager(mockProv)

	// Test Create error
	clusterDef := types.ClusterDefinition{
		Metadata: types.ClusterMetadata{
			Name:   "test-cluster",
			Region: "mock-region",
		},
	}

	_, err := mgr.Create(context.Background(), clusterDef)
	if err == nil {
		t.Error("Expected error from Create")
	}

	// Test Delete error
	err = mgr.Delete(context.Background(), "cluster-1")
	if err == nil {
		t.Error("Expected error from Delete")
	}

	// Test WaitForReady error
	err = mgr.WaitForReady(context.Background(), "cluster-1")
	if err == nil {
		t.Error("Expected error from WaitForReady")
	}

	// Test GetClusterInfo error
	_, err = mgr.GetClusterInfo(context.Background(), "test-cluster")
	if err == nil {
		t.Error("Expected error from GetClusterInfo")
	}
}
