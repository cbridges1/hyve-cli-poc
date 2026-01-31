package kubeconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMergeKubeconfigs tests merging two kubeconfig YAML strings
func TestMergeKubeconfigs(t *testing.T) {
	existingConfig := `apiVersion: v1
kind: Config
current-context: existing-context
clusters:
- name: existing-cluster
  cluster:
    server: https://existing.example.com
contexts:
- name: existing-context
  context:
    cluster: existing-cluster
    user: existing-user
users:
- name: existing-user
  user:
    token: existing-token
`

	newConfig := `apiVersion: v1
kind: Config
clusters:
- name: new-cluster
  cluster:
    server: https://new.example.com
contexts:
- name: new-context
  context:
    cluster: new-cluster
    user: new-user
users:
- name: new-user
  user:
    token: new-token
`

	merged, err := MergeKubeconfigs(existingConfig, newConfig)
	if err != nil {
		t.Fatalf("Failed to merge kubeconfigs: %v", err)
	}

	// Verify merged config contains both clusters
	if !strings.Contains(merged, "existing-cluster") {
		t.Error("Merged config should contain existing-cluster")
	}
	if !strings.Contains(merged, "new-cluster") {
		t.Error("Merged config should contain new-cluster")
	}

	// Verify merged config contains both contexts
	if !strings.Contains(merged, "existing-context") {
		t.Error("Merged config should contain existing-context")
	}
	if !strings.Contains(merged, "new-context") {
		t.Error("Merged config should contain new-context")
	}

	// Verify merged config contains both users
	if !strings.Contains(merged, "existing-user") {
		t.Error("Merged config should contain existing-user")
	}
	if !strings.Contains(merged, "new-user") {
		t.Error("Merged config should contain new-user")
	}

	// Verify current-context is preserved
	if !strings.Contains(merged, "current-context: existing-context") {
		t.Error("Merged config should preserve current-context")
	}
}

// TestMergeKubeconfigsReplace tests that new configs replace old ones with same name
func TestMergeKubeconfigsReplace(t *testing.T) {
	existingConfig := `apiVersion: v1
kind: Config
clusters:
- name: my-cluster
  cluster:
    server: https://old.example.com
contexts:
- name: my-context
  context:
    cluster: my-cluster
    user: my-user
users:
- name: my-user
  user:
    token: old-token
`

	newConfig := `apiVersion: v1
kind: Config
clusters:
- name: my-cluster
  cluster:
    server: https://new.example.com
contexts:
- name: my-context
  context:
    cluster: my-cluster
    user: my-user
users:
- name: my-user
  user:
    token: new-token
`

	merged, err := MergeKubeconfigs(existingConfig, newConfig)
	if err != nil {
		t.Fatalf("Failed to merge kubeconfigs: %v", err)
	}

	// Verify new values replaced old values
	if strings.Contains(merged, "old.example.com") {
		t.Error("Merged config should not contain old server URL")
	}
	if !strings.Contains(merged, "new.example.com") {
		t.Error("Merged config should contain new server URL")
	}
	if strings.Contains(merged, "old-token") {
		t.Error("Merged config should not contain old token")
	}
	if !strings.Contains(merged, "new-token") {
		t.Error("Merged config should contain new token")
	}
}

// TestRemoveKubeconfigContext tests removing a context from kubeconfig
func TestRemoveKubeconfigContext(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config")

	originalConfig := `apiVersion: v1
kind: Config
current-context: context-to-remove
clusters:
- name: cluster-to-remove
  cluster:
    server: https://remove.example.com
- name: cluster-to-keep
  cluster:
    server: https://keep.example.com
contexts:
- name: context-to-remove
  context:
    cluster: cluster-to-remove
    user: user-to-remove
- name: context-to-keep
  context:
    cluster: cluster-to-keep
    user: user-to-keep
users:
- name: user-to-remove
  user:
    token: remove-token
- name: user-to-keep
  user:
    token: keep-token
`

	// Write original config
	err := os.WriteFile(configPath, []byte(originalConfig), 0600)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Remove context
	err = RemoveKubeconfigContext(originalConfig, "context-to-remove", configPath)
	if err != nil {
		t.Fatalf("Failed to remove context: %v", err)
	}

	// Read modified config
	modifiedData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read modified config: %v", err)
	}
	modified := string(modifiedData)

	// Verify removed items are gone
	if strings.Contains(modified, "context-to-remove") {
		t.Error("Modified config should not contain removed context")
	}
	if strings.Contains(modified, "cluster-to-remove") {
		t.Error("Modified config should not contain removed cluster")
	}
	if strings.Contains(modified, "user-to-remove") {
		t.Error("Modified config should not contain removed user")
	}

	// Verify kept items remain
	if !strings.Contains(modified, "context-to-keep") {
		t.Error("Modified config should contain kept context")
	}
	if !strings.Contains(modified, "cluster-to-keep") {
		t.Error("Modified config should contain kept cluster")
	}
	if !strings.Contains(modified, "user-to-keep") {
		t.Error("Modified config should contain kept user")
	}

	// Verify current-context is cleared
	if strings.Contains(modified, "current-context: context-to-remove") {
		t.Error("Current-context should be cleared when removing active context")
	}
}

// TestRemoveNonExistentContext tests removing a context that doesn't exist
func TestRemoveNonExistentContext(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config")

	originalConfig := `apiVersion: v1
kind: Config
clusters:
- name: my-cluster
  cluster:
    server: https://example.com
contexts:
- name: my-context
  context:
    cluster: my-cluster
    user: my-user
users:
- name: my-user
  user:
    token: my-token
`

	// Write original config
	err := os.WriteFile(configPath, []byte(originalConfig), 0600)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Try to remove non-existent context (should not error)
	err = RemoveKubeconfigContext(originalConfig, "non-existent", configPath)
	if err != nil {
		t.Fatalf("Should not error when removing non-existent context: %v", err)
	}

	// Verify config is unchanged
	modifiedData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read modified config: %v", err)
	}
	modified := string(modifiedData)

	// Verify all original items remain
	if !strings.Contains(modified, "my-cluster") {
		t.Error("Original cluster should remain")
	}
	if !strings.Contains(modified, "my-context") {
		t.Error("Original context should remain")
	}
	if !strings.Contains(modified, "my-user") {
		t.Error("Original user should remain")
	}
}

// TestMergeEmptyExistingConfig tests merging when existing config is empty
func TestMergeEmptyExistingConfig(t *testing.T) {
	emptyConfig := `apiVersion: v1
kind: Config
clusters: []
contexts: []
users: []
`

	newConfig := `apiVersion: v1
kind: Config
clusters:
- name: new-cluster
  cluster:
    server: https://new.example.com
contexts:
- name: new-context
  context:
    cluster: new-cluster
    user: new-user
users:
- name: new-user
  user:
    token: new-token
`

	merged, err := MergeKubeconfigs(emptyConfig, newConfig)
	if err != nil {
		t.Fatalf("Failed to merge kubeconfigs: %v", err)
	}

	// Verify new config items are in merged config
	if !strings.Contains(merged, "new-cluster") {
		t.Error("Merged config should contain new-cluster")
	}
	if !strings.Contains(merged, "new-context") {
		t.Error("Merged config should contain new-context")
	}
	if !strings.Contains(merged, "new-user") {
		t.Error("Merged config should contain new-user")
	}
}

// TestMergeInvalidYAML tests error handling for invalid YAML
func TestMergeInvalidYAML(t *testing.T) {
	validConfig := `apiVersion: v1
kind: Config
clusters: []
`

	invalidConfig := `this is not valid yaml: {[}`

	// Test invalid existing config
	_, err := MergeKubeconfigs(invalidConfig, validConfig)
	if err == nil {
		t.Error("Expected error when merging with invalid existing config")
	}

	// Test invalid new config
	_, err = MergeKubeconfigs(validConfig, invalidConfig)
	if err == nil {
		t.Error("Expected error when merging with invalid new config")
	}
}

// TestRemoveItemByName tests the removeItemByName helper function
func TestRemoveItemByName(t *testing.T) {
	items := []map[string]interface{}{
		{"name": "item1", "data": "value1"},
		{"name": "item2", "data": "value2"},
		{"name": "item3", "data": "value3"},
	}

	// Remove middle item
	result := removeItemByName(items, "item2")

	if len(result) != 2 {
		t.Errorf("Expected 2 items after removal, got %d", len(result))
	}

	// Verify correct items remain
	foundItem1 := false
	foundItem3 := false
	for _, item := range result {
		if item["name"] == "item1" {
			foundItem1 = true
		}
		if item["name"] == "item3" {
			foundItem3 = true
		}
		if item["name"] == "item2" {
			t.Error("Removed item should not be in result")
		}
	}

	if !foundItem1 || !foundItem3 {
		t.Error("Expected items not found in result")
	}
}

// TestMergeItems tests the mergeItems helper function
func TestMergeItems(t *testing.T) {
	existing := []map[string]interface{}{
		{"name": "item1", "data": "old-value1"},
		{"name": "item2", "data": "value2"},
	}

	new := []map[string]interface{}{
		{"name": "item1", "data": "new-value1"}, // Update existing
		{"name": "item3", "data": "value3"},     // Add new
	}

	result := mergeItems(existing, new)

	// Should have 3 items (item1 updated, item2 kept, item3 added)
	if len(result) != 3 {
		t.Errorf("Expected 3 items after merge, got %d", len(result))
	}

	// Verify items
	itemMap := make(map[string]string)
	for _, item := range result {
		if name, ok := item["name"].(string); ok {
			if data, ok := item["data"].(string); ok {
				itemMap[name] = data
			}
		}
	}

	// Check item1 was updated
	if itemMap["item1"] != "new-value1" {
		t.Errorf("Expected item1 to be updated to new-value1, got %s", itemMap["item1"])
	}

	// Check item2 was kept
	if itemMap["item2"] != "value2" {
		t.Errorf("Expected item2 to remain value2, got %s", itemMap["item2"])
	}

	// Check item3 was added
	if itemMap["item3"] != "value3" {
		t.Errorf("Expected item3 to be value3, got %s", itemMap["item3"])
	}
}

// TestMultipleContextRemoval tests removing multiple contexts sequentially
func TestMultipleContextRemoval(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config")

	originalConfig := `apiVersion: v1
kind: Config
clusters:
- name: cluster1
  cluster:
    server: https://cluster1.example.com
- name: cluster2
  cluster:
    server: https://cluster2.example.com
- name: cluster3
  cluster:
    server: https://cluster3.example.com
contexts:
- name: context1
  context:
    cluster: cluster1
    user: user1
- name: context2
  context:
    cluster: cluster2
    user: user2
- name: context3
  context:
    cluster: cluster3
    user: user3
users:
- name: user1
  user:
    token: token1
- name: user2
  user:
    token: token2
- name: user3
  user:
    token: token3
`

	// Write original config
	err := os.WriteFile(configPath, []byte(originalConfig), 0600)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Remove first context
	err = RemoveKubeconfigContext(originalConfig, "context1", configPath)
	if err != nil {
		t.Fatalf("Failed to remove context1: %v", err)
	}

	// Read and remove second context
	data, _ := os.ReadFile(configPath)
	err = RemoveKubeconfigContext(string(data), "context2", configPath)
	if err != nil {
		t.Fatalf("Failed to remove context2: %v", err)
	}

	// Verify final config
	finalData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read final config: %v", err)
	}
	final := string(finalData)

	// Should only have context3 remaining
	if strings.Contains(final, "context1") || strings.Contains(final, "context2") {
		t.Error("Removed contexts should not be in final config")
	}
	if !strings.Contains(final, "context3") {
		t.Error("Context3 should remain in final config")
	}
}
