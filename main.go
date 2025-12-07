package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/civo/civogo"
	"gopkg.in/yaml.v3"

	"civo-cluster-deploy/internal/cloudflare"
	"civo-cluster-deploy/internal/cluster"
	"civo-cluster-deploy/internal/config"
	"civo-cluster-deploy/internal/firewall"
	"civo-cluster-deploy/internal/ingress"
	"civo-cluster-deploy/internal/state"
	"civo-cluster-deploy/internal/types"
)

// CLIParams holds command line parameters
type CLIParams struct {
	Action        string
	ClusterName   string
	Region        string
	Nodes         string
	ClusterType   string
	MasterCluster bool
}

func main() {
	// Parse command line flags
	cliParams := parseCLIFlags()

	// If CLI parameters are provided, handle them and exit
	if cliParams.Action != "" {
		handleCLIAction(cliParams)
		return
	}

	// Otherwise, run normal reconciliation mode
	runReconciliationMode()
}

// parseCLIFlags parses command line flags and returns CLIParams
func parseCLIFlags() CLIParams {
	var params CLIParams

	flag.StringVar(&params.Action, "action", "", "Action to perform: add, modify, or delete")
	flag.StringVar(&params.ClusterName, "cluster-name", "", "Name of the cluster")
	flag.StringVar(&params.Region, "region", "PHX1", "Region for the cluster")
	flag.StringVar(&params.Nodes, "nodes", "g4s.kube.small", "Comma-separated list of node sizes (e.g., g4s.kube.small,g4s.kube.medium)")
	flag.StringVar(&params.ClusterType, "cluster-type", "k3s", "Type of Kubernetes cluster")
	flag.BoolVar(&params.MasterCluster, "master-cluster", false, "Whether this is a master cluster")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nReconciliation mode (no flags): Reconciles clusters based on YAML files in state/clusters/\n\n")
		fmt.Fprintf(os.Stderr, "CLI mode flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Add a new worker cluster with 2 medium nodes\n")
		fmt.Fprintf(os.Stderr, "  %s -action=add -cluster-name=worker-1 -region=PHX1 -nodes=g4s.kube.medium,g4s.kube.medium\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Add a new master cluster with 1 small node\n")
		fmt.Fprintf(os.Stderr, "  %s -action=add -cluster-name=master-1 -region=PHX1 -nodes=g4s.kube.small -master-cluster=true\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Modify an existing cluster to have 3 nodes (small, medium, large)\n")
		fmt.Fprintf(os.Stderr, "  %s -action=modify -cluster-name=worker-1 -nodes=g4s.kube.small,g4s.kube.medium,g4s.kube.large\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Delete a cluster\n")
		fmt.Fprintf(os.Stderr, "  %s -action=delete -cluster-name=worker-1\n\n", os.Args[0])
	}

	flag.Parse()
	return params
}

// handleCLIAction handles CLI-based actions
func handleCLIAction(params CLIParams) {
	// Validate required parameters
	if params.ClusterName == "" {
		log.Fatal("cluster-name is required when using action parameter")
	}

	// Validate action
	validActions := []string{"add", "modify", "delete"}
	validAction := false
	for _, action := range validActions {
		if params.Action == action {
			validAction = true
			break
		}
	}
	if !validAction {
		log.Fatalf("Invalid action '%s'. Valid actions are: %s", params.Action, strings.Join(validActions, ", "))
	}

	// Create state directory if it doesn't exist
	stateDir := "state/clusters"
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		log.Fatalf("Failed to create state directory: %v", err)
	}

	filePath := filepath.Join(stateDir, params.ClusterName+".yaml")

	switch params.Action {
	case "add":
		addClusterFromCLI(params, filePath)
	case "modify":
		modifyClusterFromCLI(params, filePath)
	case "delete":
		deleteClusterFromCLI(params, filePath)
	}
}

// addClusterFromCLI creates a new cluster YAML file from CLI parameters
func addClusterFromCLI(params CLIParams, filePath string) {
	// Check if file already exists
	if _, err := os.Stat(filePath); err == nil {
		log.Fatalf("Cluster %s already exists. Use 'modify' action to update it.", params.ClusterName)
	}

	// Parse nodes string into array
	var nodeArray []string
	if params.Nodes != "" {
		nodeArray = strings.Split(params.Nodes, ",")
		// Trim whitespace from each node
		for i, node := range nodeArray {
			nodeArray[i] = strings.TrimSpace(node)
		}
	}

	// Create cluster definition
	clusterDef := types.ClusterDefinition{
		APIVersion: "v1",
		Kind:       "Cluster",
		Metadata: types.ClusterMetadata{
			Name:   params.ClusterName,
			Region: params.Region,
		},
		Spec: types.ClusterSpec{
			Nodes:         nodeArray,
			ClusterType:   params.ClusterType,
			MasterCluster: params.MasterCluster,
			Firewall: types.FirewallSpec{
				Enabled: true,
				Rules: []types.FirewallRule{
					{
						Protocol:  "tcp",
						StartPort: "6443",
						EndPort:   "6443",
						Cidr:      []string{"0.0.0.0/0"},
						Direction: "ingress",
					},
				},
			},
			Ingress: types.IngressSpec{
				Enabled:      true,
				LoadBalancer: true,
			},
		},
	}

	// Marshal to YAML
	data, err := yaml.Marshal(&clusterDef)
	if err != nil {
		log.Fatalf("Failed to marshal cluster definition: %v", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		log.Fatalf("Failed to write cluster definition file: %v", err)
	}

	log.Printf("Created cluster definition file: %s", filePath)
	log.Printf("Cluster %s configuration:", params.ClusterName)
	log.Printf("  Region: %s", params.Region)
	log.Printf("  Nodes: %v", nodeArray)
	log.Printf("  Cluster Type: %s", params.ClusterType)
	log.Printf("  Master Cluster: %t", params.MasterCluster)
}

// modifyClusterFromCLI updates an existing cluster YAML file with CLI parameters
func modifyClusterFromCLI(params CLIParams, filePath string) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Fatalf("Cluster %s does not exist. Use 'add' action to create it.", params.ClusterName)
	}

	// Read existing file
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to read existing cluster file: %v", err)
	}

	// Parse existing cluster definition
	var clusterDef types.ClusterDefinition
	if err := yaml.Unmarshal(data, &clusterDef); err != nil {
		log.Fatalf("Failed to parse existing cluster definition: %v", err)
	}

	// Update only the fields that were provided (non-default values)
	if flag.Lookup("region").Value.String() != flag.Lookup("region").DefValue {
		clusterDef.Metadata.Region = params.Region
	}
	if flag.Lookup("nodes").Value.String() != flag.Lookup("nodes").DefValue {
		// Parse nodes string into array
		var nodeArray []string
		if params.Nodes != "" {
			nodeArray = strings.Split(params.Nodes, ",")
			// Trim whitespace from each node
			for i, node := range nodeArray {
				nodeArray[i] = strings.TrimSpace(node)
			}
		}
		clusterDef.Spec.Nodes = nodeArray
	}
	if flag.Lookup("cluster-type").Value.String() != flag.Lookup("cluster-type").DefValue {
		clusterDef.Spec.ClusterType = params.ClusterType
	}
	if flag.Lookup("master-cluster").Value.String() != flag.Lookup("master-cluster").DefValue {
		clusterDef.Spec.MasterCluster = params.MasterCluster
	}

	// Marshal to YAML
	updatedData, err := yaml.Marshal(&clusterDef)
	if err != nil {
		log.Fatalf("Failed to marshal updated cluster definition: %v", err)
	}

	// Write updated file
	if err := os.WriteFile(filePath, updatedData, 0644); err != nil {
		log.Fatalf("Failed to write updated cluster definition file: %v", err)
	}

	log.Printf("Updated cluster definition file: %s", filePath)
	log.Printf("Cluster %s updated configuration:", params.ClusterName)
	log.Printf("  Region: %s", clusterDef.Metadata.Region)
	log.Printf("  Nodes: %v", clusterDef.Spec.Nodes)
	log.Printf("  Cluster Type: %s", clusterDef.Spec.ClusterType)
	log.Printf("  Master Cluster: %t", clusterDef.Spec.MasterCluster)
}

// deleteClusterFromCLI removes a cluster YAML file
func deleteClusterFromCLI(params CLIParams, filePath string) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Fatalf("Cluster %s does not exist.", params.ClusterName)
	}

	// Remove the file
	if err := os.Remove(filePath); err != nil {
		log.Fatalf("Failed to delete cluster definition file: %v", err)
	}

	log.Printf("Deleted cluster definition file: %s", filePath)
	log.Printf("Cluster %s has been removed from configuration", params.ClusterName)
}

// runReconciliationMode runs the original reconciliation logic
func runReconciliationMode() {
	// Initialize configuration manager
	configMgr := config.NewManager()
	apiKey := configMgr.GetCivoToken()
	if apiKey == "" {
		log.Fatal("CIVO_TOKEN environment variable is required")
	}

	cloudflareToken := configMgr.GetCloudflareToken()
	domain := configMgr.GetDomain()

	var cloudflareMgr *cloudflare.Manager
	if cloudflareToken != "" && domain != "" {
		var err error
		cloudflareMgr, err = cloudflare.NewManager(cloudflareToken, domain)
		if err != nil {
			log.Fatalf("Failed to initialize Cloudflare manager: %v", err)
		}
		log.Printf("Cloudflare integration enabled for domain: %s", domain)
	} else {
		log.Println("Cloudflare integration disabled - missing CLOUDFLARE_API_TOKEN or DOMAIN")
	}

	// Initialize state manager
	stateMgr := state.NewManager("state/clusters")

	// Load cluster definitions
	clusterDefs, err := stateMgr.LoadClusterDefinitions()
	if err != nil {
		log.Fatalf("Failed to load cluster definitions: %v", err)
	}

	// Validate cluster definitions
	err = stateMgr.ValidateClusterDefinitions(clusterDefs)
	if err != nil {
		log.Fatalf("Invalid cluster configuration: %v", err)
	}

	// Order clusters so master cluster comes first
	clusterDefs = stateMgr.OrderClusters(clusterDefs)

	ctx := context.Background()

	// Group clusters by region for efficient processing, maintaining order
	regionClusters := make(map[string][]types.ClusterDefinition)
	for _, clusterDef := range clusterDefs {
		region := clusterDef.Metadata.Region
		regionClusters[region] = append(regionClusters[region], clusterDef)
	}

	// If no cluster definitions found, we still need to check for orphaned clusters
	if len(clusterDefs) == 0 {
		log.Println("No cluster definitions found in state/clusters directory")
		cleanupAllRegions(ctx, apiKey, clusterDefs)

		// Also cleanup Cloudflare load balancers if enabled
		if cloudflareMgr != nil {
			err := cloudflareMgr.RemoveAllLoadBalancers(ctx)
			if err != nil {
				log.Printf("Failed to cleanup Cloudflare load balancers: %v", err)
			}
		}
		return
	}

	// Process each region separately
	for region, clusters := range regionClusters {
		log.Printf("Processing region: %s", region)

		client, err := civogo.NewClient(apiKey, region)
		if err != nil {
			log.Fatalf("Failed to create Civo client for region %s: %v", region, err)
		}

		// Initialize managers for this region
		clusterMgr := cluster.NewManager(client)
		firewallMgr := firewall.NewManager(client)
		ingressMgr := ingress.NewManager(client)

		// First, reconcile all desired clusters with master cluster dependency check
		masterClusterReady := false
		for _, clusterDef := range clusters {
			// Check if this is a master cluster or if master cluster is ready
			if clusterDef.Spec.MasterCluster {
				// This is the master cluster, process it first
				err := reconcileCluster(ctx, clusterMgr, firewallMgr, ingressMgr, cloudflareMgr, stateMgr, clusterDef)
				if err != nil {
					log.Printf("Failed to reconcile master cluster %s: %v", clusterDef.Metadata.Name, err)
					continue
				}

				// Check if master cluster is now ready
				updatedClusters := []types.ClusterDefinition{clusterDef}
				if stateMgr.IsMasterClusterReady(updatedClusters, clusterMgr) {
					masterClusterReady = true
					log.Printf("Master cluster %s is ready, can now process other clusters", clusterDef.Metadata.Name)
				}
			} else {
				// This is a regular cluster, check if master cluster is ready
				if !masterClusterReady {
					// Re-check if master cluster is ready from all clusters in this region
					if !stateMgr.IsMasterClusterReady(clusters, clusterMgr) {
						log.Printf("Skipping cluster %s - master cluster is not ready yet", clusterDef.Metadata.Name)
						continue
					}
					masterClusterReady = true
				}

				// Process regular cluster
				err := reconcileCluster(ctx, clusterMgr, firewallMgr, ingressMgr, cloudflareMgr, stateMgr, clusterDef)
				if err != nil {
					log.Printf("Failed to reconcile cluster %s: %v", clusterDef.Metadata.Name, err)
				}
			}
		}

		// Then, cleanup orphaned resources
		err = cleanupOrphanedResources(ctx, clusterMgr, firewallMgr, clusters)
		if err != nil {
			log.Printf("Failed to cleanup orphaned resources in region %s: %v", region, err)
		}
	}

	log.Println("Cluster reconciliation completed")
}

// reconcileCluster handles the reconciliation of a single cluster
func reconcileCluster(ctx context.Context, clusterMgr *cluster.Manager, firewallMgr *firewall.Manager, ingressMgr *ingress.Manager, cloudflareMgr *cloudflare.Manager, stateMgr *state.Manager, clusterDef types.ClusterDefinition) error {
	action := clusterMgr.DetermineAction(ctx, clusterDef)

	switch action {
	case types.ActionCreate:
		return createCluster(ctx, clusterMgr, firewallMgr, ingressMgr, cloudflareMgr, stateMgr, clusterDef)
	case types.ActionUpdate:
		return updateCluster(ctx, clusterMgr, ingressMgr, cloudflareMgr, stateMgr, clusterDef)
	case types.ActionDelete:
		return deleteCluster(ctx, clusterMgr, firewallMgr, ingressMgr, cloudflareMgr, stateMgr, clusterDef)
	case types.ActionNone:
		log.Printf("Cluster %s is up to date, no action needed", clusterDef.Metadata.Name)
		return nil
	default:
		log.Printf("Unknown action for cluster %s: %v", clusterDef.Metadata.Name, action)
		return nil
	}
}

// createCluster creates a new cluster with optional firewall, ingress controller, and Cloudflare integration
func createCluster(ctx context.Context, clusterMgr *cluster.Manager, firewallMgr *firewall.Manager, ingressMgr *ingress.Manager, cloudflareMgr *cloudflare.Manager, stateMgr *state.Manager, clusterDef types.ClusterDefinition) error {
	// First check if cluster already exists (idempotent behavior)
	existingCluster, err := clusterMgr.FindByName(clusterDef.Metadata.Name)
	if err != nil {
		return err
	}

	if existingCluster != nil {
		log.Printf("Cluster %s already exists with ID %s, updating status and continuing",
			clusterDef.Metadata.Name, existingCluster.ID)

		log.Printf("Cluster %s already exists and is in %s state", clusterDef.Metadata.Name, existingCluster.Status)
		return nil
	}

	// Create firewall if requested
	var firewallID string
	if clusterDef.Spec.Firewall.Enabled && len(clusterDef.Spec.Firewall.Rules) > 0 {
		fw, err := firewallMgr.CreateFromDefinition(ctx, clusterDef)
		if err != nil {
			return err
		}
		firewallID = fw.ID
		log.Printf("Created/reused firewall with ID: %s", firewallID)
	}

	// Create the cluster
	createdCluster, err := clusterMgr.Create(ctx, clusterDef, firewallID)
	if err != nil {
		return err
	}

	// Wait for the cluster to be ready
	err = clusterMgr.WaitForReady(ctx, createdCluster.ID)
	if err != nil {
		return err
	}
	log.Println("Cluster is ready!")

	// Deploy ingress controller if enabled
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
	return nil
}

// updateCluster updates an existing cluster
func updateCluster(ctx context.Context, clusterMgr *cluster.Manager, ingressMgr *ingress.Manager, cloudflareMgr *cloudflare.Manager, stateMgr *state.Manager, clusterDef types.ClusterDefinition) error {
	err := clusterMgr.Update(ctx, clusterDef)
	if err != nil {
		return err
	}

	return nil
}

// deleteCluster deletes a cluster and its associated firewall, ingress controller, and Cloudflare endpoint
func deleteCluster(ctx context.Context, clusterMgr *cluster.Manager, firewallMgr *firewall.Manager, ingressMgr *ingress.Manager, cloudflareMgr *cloudflare.Manager, stateMgr *state.Manager, clusterDef types.ClusterDefinition) error {
	// Find the actual cluster by name
	existingCluster, err := clusterMgr.FindByName(clusterDef.Metadata.Name)
	if err != nil {
		return err
	}

	if existingCluster == nil {
		log.Printf("Cluster %s not found, nothing to delete", clusterDef.Metadata.Name)
		return nil
	}

	log.Printf("Deleting cluster %s with ID %s", clusterDef.Metadata.Name, existingCluster.ID)

	// Get load balancer IP before deleting cluster (for Cloudflare cleanup)
	var loadBalancerIP string
	if clusterDef.Spec.Ingress.Enabled && clusterDef.Spec.Ingress.LoadBalancer {
		loadBalancerIP, _ = ingressMgr.GetLoadBalancerIP(ctx, existingCluster.ID)
	}

	// Remove ingress controller if it was deployed
	if clusterDef.Spec.Ingress.Enabled {
		err := ingressMgr.RemoveIngressController(ctx, existingCluster.ID)
		if err != nil {
			log.Printf("Failed to remove ingress controller: %v", err)
		}
	}

	// Delete the cluster
	err = clusterMgr.Delete(ctx, existingCluster.ID)
	if err != nil {
		return err
	}

	// Delete associated firewall
	err = firewallMgr.DeleteForCluster(ctx, clusterDef, existingCluster.FirewallID)
	if err != nil {
		log.Printf("Failed to delete firewall: %v", err)
	}

	// Remove from Cloudflare load balancer if enabled
	if cloudflareMgr != nil && loadBalancerIP != "" {
		err := cloudflareMgr.RemoveEndpoint(ctx, "clusters", loadBalancerIP)
		if err != nil {
			log.Printf("Failed to remove cluster from Cloudflare load balancer: %v", err)
		} else {
			log.Printf("Removed cluster %s from Cloudflare load balancer", clusterDef.Metadata.Name)
		}
	}

	log.Printf("Cluster %s deleted successfully!", clusterDef.Metadata.Name)
	return nil
}

// cleanupOrphanedResources removes clusters and firewalls that are no longer defined
func cleanupOrphanedResources(ctx context.Context, clusterMgr *cluster.Manager, firewallMgr *firewall.Manager, clusters []types.ClusterDefinition) error {
	// Find and cleanup orphaned clusters
	orphanedClusters, err := clusterMgr.FindOrphaned(clusters)
	if err != nil {
		return err
	}

	err = clusterMgr.CleanupOrphaned(ctx, orphanedClusters)
	if err != nil {
		return err
	}

	// Find and cleanup orphaned firewalls
	orphanedFirewalls, err := firewallMgr.FindOrphaned(clusters)
	if err != nil {
		return err
	}

	err = firewallMgr.CleanupOrphaned(ctx, orphanedFirewalls)
	if err != nil {
		return err
	}

	return nil
}

// cleanupAllRegions handles cleanup when no clusters are defined
func cleanupAllRegions(ctx context.Context, apiKey string, clusterDefs []types.ClusterDefinition) {
	// Use a default region to check for orphaned clusters (you may want to check all regions)
	defaultRegion := "PHX1"
	client, err := civogo.NewClient(apiKey, defaultRegion)
	if err != nil {
		log.Fatalf("Failed to create Civo client: %v", err)
	}

	clusterMgr := cluster.NewManager(client)
	firewallMgr := firewall.NewManager(client)

	err = cleanupOrphanedResources(ctx, clusterMgr, firewallMgr, clusterDefs)
	if err != nil {
		log.Printf("Failed to cleanup orphaned resources: %v", err)
	}
}
