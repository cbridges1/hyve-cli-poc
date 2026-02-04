package template

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func setupTemplateTest(t *testing.T) (*Manager, string, func()) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	manager := NewManager(tmpDir)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return manager, tmpDir, cleanup
}

func TestNewManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	manager := NewManager(tmpDir)

	if manager == nil {
		t.Fatal("Expected manager to be created, got nil")
	}

	expectedPath := filepath.Join(tmpDir, "templates")
	if manager.templatesDir != expectedPath {
		t.Errorf("Expected templates dir %s, got %s", expectedPath, manager.templatesDir)
	}
}

func TestEnsureTemplatesDir(t *testing.T) {
	manager, tmpDir, cleanup := setupTemplateTest(t)
	defer cleanup()

	err := manager.EnsureTemplatesDir()
	if err != nil {
		t.Fatalf("Failed to ensure templates dir: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "templates")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Error("Expected templates directory to be created")
	}
}

func TestGetTemplatePath(t *testing.T) {
	manager, tmpDir, cleanup := setupTemplateTest(t)
	defer cleanup()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "without extension",
			input:    "my-template",
			expected: filepath.Join(tmpDir, "templates", "my-template.yaml"),
		},
		{
			name:     "with extension",
			input:    "my-template.yaml",
			expected: filepath.Join(tmpDir, "templates", "my-template.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.GetTemplatePath(tt.input)
			if result != tt.expected {
				t.Errorf("GetTemplatePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCreateTemplate(t *testing.T) {
	manager, _, cleanup := setupTemplateTest(t)
	defer cleanup()

	template := &Template{
		Metadata: TemplateMetadata{
			Name:        "test-template",
			Description: "Test template",
		},
		Spec: TemplateSpec{
			Provider:    "civo",
			Region:      "NYC1",
			Nodes:       []string{"g4s.kube.medium"},
			ClusterType: "k3s",
		},
	}

	err := manager.CreateTemplate(template)
	if err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	// Verify file exists
	templatePath := manager.GetTemplatePath("test-template")
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Error("Template file was not created")
	}

	// Verify defaults were set
	if template.APIVersion != "v1" {
		t.Errorf("Expected APIVersion 'v1', got %s", template.APIVersion)
	}

	if template.Kind != "Template" {
		t.Errorf("Expected Kind 'Template', got %s", template.Kind)
	}
}

func TestCreateTemplate_Duplicate(t *testing.T) {
	manager, _, cleanup := setupTemplateTest(t)
	defer cleanup()

	template := &Template{
		Metadata: TemplateMetadata{
			Name: "duplicate-template",
		},
		Spec: TemplateSpec{
			Provider: "civo",
			Region:   "NYC1",
			Nodes:    []string{"g4s.kube.medium"},
		},
	}

	// Create first template
	if err := manager.CreateTemplate(template); err != nil {
		t.Fatalf("Failed to create first template: %v", err)
	}

	// Try to create duplicate
	err := manager.CreateTemplate(template)
	if err == nil {
		t.Error("Expected error when creating duplicate template, got nil")
	}
}

func TestGetTemplate(t *testing.T) {
	manager, _, cleanup := setupTemplateTest(t)
	defer cleanup()

	// Create template
	original := &Template{
		Metadata: TemplateMetadata{
			Name:        "get-test-template",
			Description: "Test description",
		},
		Spec: TemplateSpec{
			Provider:    "civo",
			Region:      "PHX1",
			Nodes:       []string{"g4s.kube.large", "g4s.kube.large"},
			ClusterType: "k3s",
		},
	}

	if err := manager.CreateTemplate(original); err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	// Get template
	retrieved, err := manager.GetTemplate("get-test-template")
	if err != nil {
		t.Fatalf("Failed to get template: %v", err)
	}

	if retrieved.Metadata.Name != original.Metadata.Name {
		t.Errorf("Expected name %s, got %s", original.Metadata.Name, retrieved.Metadata.Name)
	}

	if retrieved.Metadata.Description != original.Metadata.Description {
		t.Errorf("Expected description %s, got %s", original.Metadata.Description, retrieved.Metadata.Description)
	}

	if retrieved.Spec.Provider != original.Spec.Provider {
		t.Errorf("Expected provider %s, got %s", original.Spec.Provider, retrieved.Spec.Provider)
	}

	if retrieved.Spec.Region != original.Spec.Region {
		t.Errorf("Expected region %s, got %s", original.Spec.Region, retrieved.Spec.Region)
	}

	if len(retrieved.Spec.Nodes) != len(original.Spec.Nodes) {
		t.Errorf("Expected %d nodes, got %d", len(original.Spec.Nodes), len(retrieved.Spec.Nodes))
	}
}

func TestGetTemplate_NotFound(t *testing.T) {
	manager, _, cleanup := setupTemplateTest(t)
	defer cleanup()

	_, err := manager.GetTemplate("nonexistent-template")
	if err == nil {
		t.Error("Expected error when getting nonexistent template, got nil")
	}
}

func TestListTemplates(t *testing.T) {
	manager, _, cleanup := setupTemplateTest(t)
	defer cleanup()

	// Create multiple templates
	templates := []*Template{
		{
			Metadata: TemplateMetadata{Name: "template-1"},
			Spec: TemplateSpec{
				Provider: "civo",
				Region:   "NYC1",
				Nodes:    []string{"g4s.kube.small"},
			},
		},
		{
			Metadata: TemplateMetadata{Name: "template-2"},
			Spec: TemplateSpec{
				Provider: "civo",
				Region:   "PHX1",
				Nodes:    []string{"g4s.kube.medium"},
			},
		},
		{
			Metadata: TemplateMetadata{Name: "template-3"},
			Spec: TemplateSpec{
				Provider: "civo",
				Region:   "FRA1",
				Nodes:    []string{"g4s.kube.large"},
			},
		},
	}

	for _, tmpl := range templates {
		if err := manager.CreateTemplate(tmpl); err != nil {
			t.Fatalf("Failed to create template %s: %v", tmpl.Metadata.Name, err)
		}
	}

	// List templates
	list, err := manager.ListTemplates()
	if err != nil {
		t.Fatalf("Failed to list templates: %v", err)
	}

	if len(list) != 3 {
		t.Errorf("Expected 3 templates, got %d", len(list))
	}
}

func TestListTemplates_Empty(t *testing.T) {
	manager, _, cleanup := setupTemplateTest(t)
	defer cleanup()

	// List templates from empty directory
	list, err := manager.ListTemplates()
	if err != nil {
		t.Fatalf("Failed to list templates: %v", err)
	}

	if len(list) != 0 {
		t.Errorf("Expected 0 templates, got %d", len(list))
	}
}

func TestDeleteTemplate(t *testing.T) {
	manager, _, cleanup := setupTemplateTest(t)
	defer cleanup()

	// Create template
	template := &Template{
		Metadata: TemplateMetadata{
			Name: "delete-test-template",
		},
		Spec: TemplateSpec{
			Provider: "civo",
			Region:   "NYC1",
			Nodes:    []string{"g4s.kube.medium"},
		},
	}

	if err := manager.CreateTemplate(template); err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	// Delete template
	if err := manager.DeleteTemplate("delete-test-template"); err != nil {
		t.Fatalf("Failed to delete template: %v", err)
	}

	// Verify deletion
	_, err := manager.GetTemplate("delete-test-template")
	if err == nil {
		t.Error("Expected error when getting deleted template, got nil")
	}
}

func TestDeleteTemplate_NotFound(t *testing.T) {
	manager, _, cleanup := setupTemplateTest(t)
	defer cleanup()

	err := manager.DeleteTemplate("nonexistent-template")
	if err == nil {
		t.Error("Expected error when deleting nonexistent template, got nil")
	}
}

func TestConvertToClusterDefinition(t *testing.T) {
	manager, _, cleanup := setupTemplateTest(t)
	defer cleanup()

	template := &Template{
		Metadata: TemplateMetadata{
			Name:        "convert-test-template",
			Description: "Template for conversion",
		},
		Spec: TemplateSpec{
			Provider:    "civo",
			Region:      "NYC1",
			Nodes:       []string{"g4s.kube.large", "g4s.kube.large"},
			ClusterType: "k3s",
		},
	}
	template.Spec.Ingress.Enabled = true
	template.Spec.Ingress.LoadBalancer = true
	template.Spec.Ingress.ChartVersion = "4.7.1"

	clusterName := "my-cluster"
	clusterDef := manager.ConvertToClusterDefinition(template, clusterName)

	if clusterDef == nil {
		t.Fatal("Expected cluster definition to be created, got nil")
	}

	if clusterDef.Metadata.Name != clusterName {
		t.Errorf("Expected cluster name %s, got %s", clusterName, clusterDef.Metadata.Name)
	}

	if clusterDef.Metadata.Region != template.Spec.Region {
		t.Errorf("Expected region %s, got %s", template.Spec.Region, clusterDef.Metadata.Region)
	}

	if clusterDef.Spec.Provider != template.Spec.Provider {
		t.Errorf("Expected provider %s, got %s", template.Spec.Provider, clusterDef.Spec.Provider)
	}

	if len(clusterDef.Spec.Nodes) != len(template.Spec.Nodes) {
		t.Errorf("Expected %d nodes, got %d", len(template.Spec.Nodes), len(clusterDef.Spec.Nodes))
	}

	if clusterDef.Spec.ClusterType != template.Spec.ClusterType {
		t.Errorf("Expected cluster type %s, got %s", template.Spec.ClusterType, clusterDef.Spec.ClusterType)
	}

	if clusterDef.Spec.Ingress.Enabled != template.Spec.Ingress.Enabled {
		t.Errorf("Expected ingress enabled %v, got %v", template.Spec.Ingress.Enabled, clusterDef.Spec.Ingress.Enabled)
	}

	if clusterDef.Spec.Ingress.LoadBalancer != template.Spec.Ingress.LoadBalancer {
		t.Errorf("Expected ingress load balancer %v, got %v", template.Spec.Ingress.LoadBalancer, clusterDef.Spec.Ingress.LoadBalancer)
	}

	if clusterDef.Spec.Ingress.ChartVersion != template.Spec.Ingress.ChartVersion {
		t.Errorf("Expected ingress chart version %s, got %s", template.Spec.Ingress.ChartVersion, clusterDef.Spec.Ingress.ChartVersion)
	}

	if clusterDef.APIVersion != "v1" {
		t.Errorf("Expected API version v1, got %s", clusterDef.APIVersion)
	}

	if clusterDef.Kind != "Cluster" {
		t.Errorf("Expected kind Cluster, got %s", clusterDef.Kind)
	}
}

func TestExecuteTemplate(t *testing.T) {
	manager, _, cleanup := setupTemplateTest(t)
	defer cleanup()

	// Create template
	template := &Template{
		Metadata: TemplateMetadata{
			Name:        "execute-test-template",
			Description: "Template for execution",
		},
		Spec: TemplateSpec{
			Provider:    "civo",
			Region:      "PHX1",
			Nodes:       []string{"g4s.kube.medium"},
			ClusterType: "k3s",
			Workflows: TemplateWorkflowsSpec{
				OnCreated: []string{"setup-monitoring", "deploy-app"},
			},
		},
	}

	if err := manager.CreateTemplate(template); err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	// Execute template
	ctx := context.Background()
	clusterName := "test-cluster"
	retrievedTemplate, clusterDef, err := manager.ExecuteTemplate(ctx, "execute-test-template", clusterName)

	if err != nil {
		t.Fatalf("Failed to execute template: %v", err)
	}

	if retrievedTemplate == nil {
		t.Fatal("Expected template to be returned, got nil")
	}

	if clusterDef == nil {
		t.Fatal("Expected cluster definition to be returned, got nil")
	}

	if retrievedTemplate.Metadata.Name != template.Metadata.Name {
		t.Errorf("Expected template name %s, got %s", template.Metadata.Name, retrievedTemplate.Metadata.Name)
	}

	if clusterDef.Metadata.Name != clusterName {
		t.Errorf("Expected cluster name %s, got %s", clusterName, clusterDef.Metadata.Name)
	}

	if len(retrievedTemplate.Spec.Workflows.OnCreated) != 2 {
		t.Errorf("Expected 2 onCreated workflows, got %d", len(retrievedTemplate.Spec.Workflows.OnCreated))
	}
}

func TestExecuteTemplate_NotFound(t *testing.T) {
	manager, _, cleanup := setupTemplateTest(t)
	defer cleanup()

	ctx := context.Background()
	_, _, err := manager.ExecuteTemplate(ctx, "nonexistent-template", "test-cluster")

	if err == nil {
		t.Error("Expected error when executing nonexistent template, got nil")
	}
}

func TestTemplateWithWorkflows(t *testing.T) {
	manager, _, cleanup := setupTemplateTest(t)
	defer cleanup()

	template := &Template{
		Metadata: TemplateMetadata{
			Name:        "workflow-template",
			Description: "Template with workflows",
		},
		Spec: TemplateSpec{
			Provider:    "civo",
			Region:      "NYC1",
			Nodes:       []string{"g4s.kube.large"},
			ClusterType: "k3s",
			Workflows: TemplateWorkflowsSpec{
				OnCreated: []string{"setup", "deploy"},
				OnDestroy: []string{"cleanup"},
			},
		},
	}

	if err := manager.CreateTemplate(template); err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	// Retrieve and verify workflows
	retrieved, err := manager.GetTemplate("workflow-template")
	if err != nil {
		t.Fatalf("Failed to get template: %v", err)
	}

	if len(retrieved.Spec.Workflows.OnCreated) != 2 {
		t.Errorf("Expected 2 onCreated workflows, got %d", len(retrieved.Spec.Workflows.OnCreated))
	}

	if len(retrieved.Spec.Workflows.OnDestroy) != 1 {
		t.Errorf("Expected 1 onDestroy workflow, got %d", len(retrieved.Spec.Workflows.OnDestroy))
	}

	expectedOnCreated := []string{"setup", "deploy"}
	for i, workflow := range retrieved.Spec.Workflows.OnCreated {
		if workflow != expectedOnCreated[i] {
			t.Errorf("Expected onCreated workflow %s, got %s", expectedOnCreated[i], workflow)
		}
	}

	if retrieved.Spec.Workflows.OnDestroy[0] != "cleanup" {
		t.Errorf("Expected onDestroy workflow 'cleanup', got %s", retrieved.Spec.Workflows.OnDestroy[0])
	}
}

func TestTemplateWithIngress(t *testing.T) {
	manager, _, cleanup := setupTemplateTest(t)
	defer cleanup()

	template := &Template{
		Metadata: TemplateMetadata{
			Name: "ingress-template",
		},
		Spec: TemplateSpec{
			Provider:    "civo",
			Region:      "NYC1",
			Nodes:       []string{"g4s.kube.medium"},
			ClusterType: "k3s",
		},
	}
	template.Spec.Ingress.Enabled = true
	template.Spec.Ingress.LoadBalancer = true
	template.Spec.Ingress.ChartVersion = "4.7.1"

	if err := manager.CreateTemplate(template); err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	// Retrieve and verify ingress settings
	retrieved, err := manager.GetTemplate("ingress-template")
	if err != nil {
		t.Fatalf("Failed to get template: %v", err)
	}

	if !retrieved.Spec.Ingress.Enabled {
		t.Error("Expected ingress to be enabled")
	}

	if !retrieved.Spec.Ingress.LoadBalancer {
		t.Error("Expected load balancer to be enabled")
	}

	if retrieved.Spec.Ingress.ChartVersion != "4.7.1" {
		t.Errorf("Expected chart version 4.7.1, got %s", retrieved.Spec.Ingress.ChartVersion)
	}
}
