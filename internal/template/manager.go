package template

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"civo-cluster-deploy/internal/types"
)

// Manager handles cluster template operations
type Manager struct {
	templatesDir string
}

// NewManager creates a new template manager
func NewManager(repoPath string) *Manager {
	return &Manager{
		templatesDir: filepath.Join(repoPath, "templates"),
	}
}

// EnsureTemplatesDir ensures the templates directory exists
func (m *Manager) EnsureTemplatesDir() error {
	return os.MkdirAll(m.templatesDir, 0755)
}

// GetTemplatePath returns the path to a template file
func (m *Manager) GetTemplatePath(name string) string {
	// Ensure .yaml extension
	if !strings.HasSuffix(name, ".yaml") {
		name = name + ".yaml"
	}
	return filepath.Join(m.templatesDir, name)
}

// CreateTemplate creates a new template file
func (m *Manager) CreateTemplate(template *Template) error {
	if err := m.EnsureTemplatesDir(); err != nil {
		return fmt.Errorf("failed to ensure templates directory: %w", err)
	}

	templatePath := m.GetTemplatePath(template.Metadata.Name)

	// Check if template already exists
	if _, err := os.Stat(templatePath); err == nil {
		return fmt.Errorf("template '%s' already exists", template.Metadata.Name)
	}

	// Set defaults
	if template.APIVersion == "" {
		template.APIVersion = "v1"
	}
	if template.Kind == "" {
		template.Kind = "Template"
	}

	// Marshal to YAML
	data, err := yaml.Marshal(template)
	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}

	// Write to file
	if err := os.WriteFile(templatePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write template file: %w", err)
	}

	return nil
}

// GetTemplate reads a template from disk
func (m *Manager) GetTemplate(name string) (*Template, error) {
	templatePath := m.GetTemplatePath(name)

	data, err := os.ReadFile(templatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("template '%s' not found", name)
		}
		return nil, fmt.Errorf("failed to read template: %w", err)
	}

	var template Template
	if err := yaml.Unmarshal(data, &template); err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	return &template, nil
}

// ListTemplates lists all available templates
func (m *Manager) ListTemplates() ([]*Template, error) {
	if _, err := os.Stat(m.templatesDir); os.IsNotExist(err) {
		return []*Template{}, nil
	}

	entries, err := os.ReadDir(m.templatesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read templates directory: %w", err)
	}

	var templates []*Template
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".yaml")
		template, err := m.GetTemplate(name)
		if err != nil {
			continue // Skip invalid templates
		}

		templates = append(templates, template)
	}

	return templates, nil
}

// DeleteTemplate deletes a template file
func (m *Manager) DeleteTemplate(name string) error {
	templatePath := m.GetTemplatePath(name)

	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		return fmt.Errorf("template '%s' not found", name)
	}

	if err := os.Remove(templatePath); err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}

	return nil
}

// ConvertToClusterDefinition converts a template to a cluster definition
func (m *Manager) ConvertToClusterDefinition(template *Template, clusterName string) *types.ClusterDefinition {
	return &types.ClusterDefinition{
		APIVersion: "v1",
		Kind:       "Cluster",
		Metadata: types.ClusterMetadata{
			Name:   clusterName,
			Region: template.Spec.Region,
		},
		Spec: types.ClusterSpec{
			Provider:    template.Spec.Provider,
			Nodes:       template.Spec.Nodes,
			ClusterType: template.Spec.ClusterType,
			Ingress: types.IngressSpec{
				Enabled:      template.Spec.Ingress.Enabled,
				LoadBalancer: template.Spec.Ingress.LoadBalancer,
				ChartVersion: template.Spec.Ingress.ChartVersion,
			},
			Workflows: types.WorkflowsSpec{
				OnCreated: template.Spec.Workflows.OnCreated,
				OnDestroy: template.Spec.Workflows.OnDestroy,
			},
		},
	}
}

// ExecuteTemplate creates a cluster from a template
func (m *Manager) ExecuteTemplate(ctx context.Context, templateName, clusterName string) (*Template, *types.ClusterDefinition, error) {
	// Get template
	template, err := m.GetTemplate(templateName)
	if err != nil {
		return nil, nil, err
	}

	// Convert to cluster definition
	clusterDef := m.ConvertToClusterDefinition(template, clusterName)

	return template, clusterDef, nil
}
