package types

// IngressSpec represents nginx ingress controller configuration
type IngressSpec struct {
	Enabled      bool   `yaml:"enabled"`
	LoadBalancer bool   `yaml:"loadBalancer"`
	ChartVersion string `yaml:"chartVersion,omitempty"` // Specific helm chart version to install
}

// WorkflowsSpec defines workflows to run on cluster lifecycle events
type WorkflowsSpec struct {
	OnCreated []string `yaml:"onCreated,omitempty"` // Workflows to run after cluster creation
	OnDestroy []string `yaml:"onDestroy,omitempty"` // Workflows to run before cluster destruction
}

// ClusterSpec represents the desired cluster configuration
type ClusterSpec struct {
	Provider    string        `yaml:"provider"`
	Nodes       []string      `yaml:"nodes"`
	ClusterType string        `yaml:"clusterType"`
	Ingress     IngressSpec   `yaml:"ingress"`
	Workflows   WorkflowsSpec `yaml:"workflows,omitempty"`

	// GCP-specific configuration
	GCPProject   string `yaml:"gcpProject,omitempty"`   // GCP project name alias
	GCPProjectID string `yaml:"gcpProjectId,omitempty"` // GCP project ID (resolved from alias)

	// AWS-specific configuration
	AWSAccount    string `yaml:"awsAccount,omitempty"`    // AWS account name alias
	AWSAccountID  string `yaml:"awsAccountId,omitempty"`  // AWS account ID (resolved from alias)
	AWSVPCName    string `yaml:"awsVpcName,omitempty"`    // AWS VPC name alias
	AWSVPCID      string `yaml:"awsVpcId,omitempty"`      // AWS VPC ID (resolved from alias)
	AWSEKSRole    string `yaml:"awsEksRole,omitempty"`    // AWS EKS role name alias
	AWSEKSRoleARN string `yaml:"awsEksRoleArn,omitempty"` // AWS EKS role ARN (resolved from alias)
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
