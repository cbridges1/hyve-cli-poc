package types

// FirewallRule represents a firewall rule configuration
type FirewallRule struct {
	Protocol  string   `yaml:"protocol"`
	StartPort string   `yaml:"startPort"`
	EndPort   string   `yaml:"endPort"`
	Cidr      []string `yaml:"cidr"`
	Direction string   `yaml:"direction"`
}

// FirewallSpec represents firewall configuration
type FirewallSpec struct {
	Enabled bool           `yaml:"enabled"`
	Rules   []FirewallRule `yaml:"rules"`
}

// IngressSpec represents nginx ingress controller configuration
type IngressSpec struct {
	Enabled      bool   `yaml:"enabled"`
	LoadBalancer bool   `yaml:"loadBalancer"`
	ChartVersion string `yaml:"chartVersion,omitempty"` // Specific helm chart version to install
}

// ClusterSpec represents the desired cluster configuration
type ClusterSpec struct {
	Provider      string       `yaml:"provider"`
	Nodes         []string     `yaml:"nodes"`
	ClusterType   string       `yaml:"clusterType"`
	MasterCluster bool         `yaml:"masterCluster,omitempty"`
	Firewall      FirewallSpec `yaml:"firewall"`
	Ingress       IngressSpec  `yaml:"ingress"`
}

// ClusterMetadata represents cluster metadata
type ClusterMetadata struct {
	Name   string `yaml:"name"`
	Region string `yaml:"region"`
}

// ClusterDefinition represents a complete cluster definition
type ClusterDefinition struct {
	APIVersion string          `yaml:"apiVersion"`
	Kind       string          `yaml:"kind"`
	Metadata   ClusterMetadata `yaml:"metadata"`
	Spec       ClusterSpec     `yaml:"spec"`
}

// ReconcileAction represents the type of action to take on a cluster
type ReconcileAction int

const (
	ActionNone ReconcileAction = iota
	ActionCreate
	ActionUpdate
	ActionDelete
)
