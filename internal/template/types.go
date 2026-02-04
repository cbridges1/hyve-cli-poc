package template

// TemplateMetadata represents template metadata
type TemplateMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
}

// TemplateWorkflowsSpec defines workflows to run on cluster lifecycle events
type TemplateWorkflowsSpec struct {
	OnCreated []string `yaml:"onCreated,omitempty"` // Workflows to run after cluster creation
	OnDestroy []string `yaml:"onDestroy,omitempty"` // Workflows to run before cluster destruction
}

// TemplateSpec represents the template specification
type TemplateSpec struct {
	Provider    string   `yaml:"provider"`
	Region      string   `yaml:"region"`
	Nodes       []string `yaml:"nodes"`
	ClusterType string   `yaml:"clusterType"`
	Ingress     struct {
		Enabled      bool   `yaml:"enabled"`
		LoadBalancer bool   `yaml:"loadBalancer"`
		ChartVersion string `yaml:"chartVersion,omitempty"`
	} `yaml:"ingress"`
	Workflows TemplateWorkflowsSpec `yaml:"workflows,omitempty"` // Workflows to run on lifecycle events
}

// Template represents a complete cluster template definition
type Template struct {
	APIVersion string           `yaml:"apiVersion"`
	Kind       string           `yaml:"kind"`
	Metadata   TemplateMetadata `yaml:"metadata"`
	Spec       TemplateSpec     `yaml:"spec"`
}
