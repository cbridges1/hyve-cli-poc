package cmd

import (
	gocontext "context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"hyve/internal/cluster"
	"hyve/internal/config"
	"hyve/internal/provider"
	"hyve/internal/providerconfig"
	"hyve/internal/repository"
	"hyve/internal/types"
)

// deleteClusterFromCLI deletes a cluster. When deleteFromCloud is true the
// cluster is removed from the cloud provider BEFORE the YAML is touched and
// BEFORE reconciliation runs, guaranteeing the cloud resource is gone first.
// allowNoConfig permits cloud deletion even when no YAML exists (--force-cloud).
func deleteClusterFromCLI(clusterName string, allowNoConfig bool, deleteFromCloud bool) {
	ctx := gocontext.Background()
	stateMgr, stateDir := createStateManager(ctx)
	filePath := filepath.Join(stateDir, clusterName+".yaml")

	var clusterDef types.ClusterDefinition
	configExists := false

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if deleteFromCloud && allowNoConfig {
			// --force --force-cloud: allow cloud deletion even without a config file.
			log.Printf("⚠️ Configuration file not found, but --force --force-cloud specified")
			clusterDef.Metadata.Name = clusterName
		} else {
			log.Fatalf("Cluster %s configuration does not exist. Use --force --force-cloud to delete from cloud provider anyway.", clusterName)
		}
	} else {
		configExists = true
		data, err := os.ReadFile(filePath)
		if err != nil {
			log.Fatalf("Failed to read cluster definition: %v", err)
		}
		if err := yaml.Unmarshal(data, &clusterDef); err != nil {
			log.Fatalf("Failed to parse cluster definition: %v", err)
		}
	}

	// Step 1: delete from cloud FIRST so the resource is gone before we touch
	// git state or run reconciliation.
	if deleteFromCloud {
		log.Printf("🗑️ Deleting cluster '%s' from cloud provider...", clusterName)
		if err := deleteClusterExplicitly(ctx, clusterDef); err != nil {
			log.Fatalf("❌ Failed to delete cluster '%s' from cloud provider: %v\n\n"+
				"Configuration file was NOT removed to prevent orphaned cluster state.\n"+
				"Please resolve the issue and try again.", clusterName, err)
		}
		log.Printf("✅ Cloud cluster '%s' deleted", clusterName)
	} else {
		log.Printf("📝 Removing cluster YAML — cloud deletion will be handled by reconciliation")
	}

	// Step 2: remove the YAML from git state.
	if configExists {
		if err := os.Remove(filePath); err != nil {
			log.Fatalf("Failed to delete cluster definition file: %v", err)
		}
		commitStateChanges(ctx, stateMgr, fmt.Sprintf("Delete cluster %s", clusterName))
		log.Printf("Deleted cluster definition file: %s", filePath)
		log.Printf("Cluster %s has been removed from configuration", clusterName)
	} else {
		log.Printf("📝 No configuration file to remove")
	}

	// Step 3: clean up stored kubeconfig.
	cleanupClusterKubeconfig(clusterName)

	// Step 4: reconcile. When deleteFromCloud=true the cluster is already gone
	// from the cloud so reconciliation is a no-op for this cluster; it still
	// ensures any remaining desired state is applied.
	runReconciliation("")
}

// cleanupClusterKubeconfig removes a cluster's kubeconfig from both the Hyve
// database and the active ~/.kube/config context list.
func cleanupClusterKubeconfig(clusterName string) {
	// Remove from the Hyve encrypted database
	kubeconfigMgr, _, err := createKubeconfigManager()
	if err != nil {
		log.Printf("⚠️  Could not open kubeconfig database: %v", err)
	} else {
		defer kubeconfigMgr.Close()
		if err := kubeconfigMgr.DeleteKubeconfig(clusterName); err != nil {
			log.Printf("⚠️  Failed to remove kubeconfig from database: %v", err)
		} else {
			log.Printf("🗑️  Removed kubeconfig for '%s' from Hyve database", clusterName)
		}
	}

	// Remove from ~/.kube/config
	removeKubeconfig(clusterName)
}

// deleteClusterExplicitly deletes a cluster by name directly from the provider
// This ensures deletion even if the cluster doesn't appear in provider API listings
func deleteClusterExplicitly(ctx gocontext.Context, clusterDef types.ClusterDefinition) error {
	clusterName := clusterDef.Metadata.Name
	region := clusterDef.Metadata.Region
	providerName := clusterDef.Spec.Provider
	if providerName == "" {
		providerName = "civo" // default for backward compatibility
	}

	// Create provider with appropriate options
	prov, err := createProviderForClusterDef(clusterDef)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Create cluster manager
	clusterMgr := cluster.NewManager(prov)

	log.Printf("🔍 Explicitly searching for cluster '%s' in region %s (provider: %s)...", clusterName, region, providerName)

	// Try to find the cluster by name
	existingCluster, err := clusterMgr.FindByName(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to search for cluster: %w", err)
	}

	if existingCluster == nil {
		log.Printf("✅ Cluster '%s' not found in cloud provider, may already be deleted", clusterName)
		return nil
	}

	log.Printf("🗑️ Found cluster '%s' with ID %s, explicitly deleting...", clusterName, existingCluster.ID)

	// Delete the cluster explicitly by ID
	err = clusterMgr.Delete(ctx, existingCluster.ID)
	if err != nil {
		return fmt.Errorf("failed to delete cluster %s (ID: %s): %w", clusterName, existingCluster.ID, err)
	}

	log.Printf("✅ Successfully deleted cluster '%s' from cloud provider", clusterName)
	return nil
}

// createProviderForClusterDef creates a provider with appropriate options for a cluster definition
func createProviderForClusterDef(clusterDef types.ClusterDefinition) (provider.Provider, error) {
	providerName := clusterDef.Spec.Provider
	if providerName == "" {
		providerName = "civo" // default
	}

	providerFactory := provider.NewFactory()

	opts := provider.ProviderOptions{
		Region: clusterDef.Metadata.Region,
	}

	// Populate AccountName so the factory can resolve named env vars.
	switch strings.ToLower(providerName) {
	case "civo":
		opts.AccountName = clusterDef.Spec.CivoOrganization
	case "aws":
		opts.AccountName = clusterDef.Spec.AWSAccount
	case "gcp":
		opts.AccountName = clusterDef.Spec.GCPProject
	case "azure":
		opts.AccountName = clusterDef.Spec.AzureSubscription
	}

	// Handle Civo-specific configuration
	if providerName == "civo" {
		configMgr := config.NewManager()
		apiKey := configMgr.GetCivoToken(clusterDef.Spec.CivoOrganization)
		if apiKey == "" {
			apiKey = os.Getenv("CIVO_TOKEN")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("Civo API token not found. Please run 'hyve config civo token set --org %s' or set CIVO_TOKEN environment variable", clusterDef.Spec.CivoOrganization)
		}
		opts.APIKey = apiKey
	}

	// Handle GCP-specific configuration
	if providerName == "gcp" {
		// Use stored project ID if available, otherwise resolve from alias
		if clusterDef.Spec.GCPProjectID != "" {
			opts.ProjectID = clusterDef.Spec.GCPProjectID
			log.Printf("Using GCP project ID '%s'", clusterDef.Spec.GCPProjectID)
		} else if clusterDef.Spec.GCPProject != "" {
			gcpRepoMgr, err := repository.NewManager()
			if err == nil {
				defer gcpRepoMgr.Close()
				if currentRepo, err := gcpRepoMgr.GetCurrentRepository(); err == nil {
					pcMgr := providerconfig.NewManager(currentRepo.LocalPath)
					projectID, err := pcMgr.GetGCPProjectID(clusterDef.Spec.GCPProject)
					if err != nil {
						return nil, fmt.Errorf("failed to resolve GCP project '%s': %w", clusterDef.Spec.GCPProject, err)
					}
					opts.ProjectID = projectID
					log.Printf("Using GCP project '%s' (ID: %s)", clusterDef.Spec.GCPProject, projectID)
				}
			}
		}
	}

	// Handle Azure-specific configuration
	if providerName == "azure" {
		opts.AzureResourceGroup = clusterDef.Spec.AzureResourceGroup
		if clusterDef.Spec.AzureSubscriptionID != "" {
			opts.AzureSubscriptionID = clusterDef.Spec.AzureSubscriptionID
		} else if clusterDef.Spec.AzureSubscription != "" {
			azureRepoMgr, err := repository.NewManager()
			if err == nil {
				defer azureRepoMgr.Close()
				if currentRepo, err := azureRepoMgr.GetCurrentRepository(); err == nil {
					pcMgr := providerconfig.NewManager(currentRepo.LocalPath)
					subscriptionID, err := pcMgr.GetAzureSubscriptionID(clusterDef.Spec.AzureSubscription)
					if err != nil {
						return nil, fmt.Errorf("failed to resolve Azure subscription '%s': %w", clusterDef.Spec.AzureSubscription, err)
					}
					opts.AzureSubscriptionID = subscriptionID
					log.Printf("Using Azure subscription '%s' (ID: %s)", clusterDef.Spec.AzureSubscription, subscriptionID)
				}
			}
		}
	}

	return providerFactory.CreateProviderWithOptions(providerName, opts)
}

// forceDeleteClusterFromCloud deletes a cluster by name from the cloud provider across multiple regions
func forceDeleteClusterFromCloud(clusterName, region, providerName, projectName string, accountAlias ...string) {
	alias := ""
	if len(accountAlias) > 0 {
		alias = accountAlias[0]
	}
	ctx := gocontext.Background()

	regions := []string{region}
	if region == "" {
		// Search common regions based on provider
		switch providerName {
		case "civo":
			regions = []string{"PHX1", "NYC1", "FRA1", "LON1"}
		case "gcp":
			regions = []string{"us-central1", "us-east1", "us-west1", "europe-west1"}
		case "aws":
			regions = []string{"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1"}
		case "azure":
			regions = []string{"eastus", "westus2", "westeurope", "southeastasia"}
		default:
			regions = []string{"PHX1"}
		}
		log.Printf("🔍 No region specified, searching common %s regions: %v", providerName, regions)
	}

	// Build provider options
	opts := provider.ProviderOptions{}

	// Handle Civo-specific configuration
	if providerName == "civo" {
		configMgr := config.NewManager()
		apiKey := configMgr.GetCivoToken(projectName)
		if apiKey == "" {
			apiKey = os.Getenv("CIVO_TOKEN")
		}
		if apiKey == "" {
			log.Fatalf("Civo API token not found. Please run 'hyve config civo token set --org %s' or set CIVO_TOKEN environment variable", projectName)
		}
		opts.APIKey = apiKey
	}

	// Resolve provider config manager for alias lookups
	var fdPcMgr *providerconfig.Manager
	{
		fdRepoMgr, err := repository.NewManager()
		if err == nil {
			defer fdRepoMgr.Close()
			if fdCurrentRepo, err := fdRepoMgr.GetCurrentRepository(); err == nil {
				fdPcMgr = providerconfig.NewManager(fdCurrentRepo.LocalPath)
			}
		}
	}

	// Handle GCP-specific configuration
	if providerName == "gcp" && projectName != "" {
		if fdPcMgr == nil {
			log.Fatalf("Failed to load repository configuration for GCP project lookup")
		}
		projectID, err := fdPcMgr.GetGCPProjectID(projectName)
		if err != nil {
			log.Fatalf("GCP project alias '%s' not found in repository configuration.\n"+
				"Use 'hyve config gcp project add --name %s --id <project-id>' to add it.", projectName, projectName)
		}
		opts.ProjectID = projectID
		log.Printf("Using GCP project '%s' (ID: %s)", projectName, projectID)
		// GKE's list/get API with a specific region misses zonal clusters. The
		// wildcard "-" searches all zones and regions in a single API call.
		regions = []string{"-"}
	}

	// Handle AWS-specific configuration
	if providerName == "aws" && alias != "" && fdPcMgr != nil {
		keyID, secret, tok, err := fdPcMgr.GetAWSCredentials(alias)
		if err == nil {
			opts.AccessKeyID = keyID
			opts.SecretAccessKey = secret
			opts.SessionToken = tok
		}
		opts.AccountName = alias
	}

	// Handle Azure-specific configuration
	if providerName == "azure" && alias != "" && fdPcMgr != nil {
		subID, _ := fdPcMgr.GetAzureSubscriptionID(alias)
		tenantID, clientID, clientSecret, err := fdPcMgr.GetAzureCredentials(alias)
		if err == nil {
			opts.AzureTenantID = tenantID
			opts.AzureClientID = clientID
			opts.AzureClientSecret = clientSecret
		}
		opts.AzureSubscriptionID = subID
		opts.AccountName = alias
		// Leave AzureResourceGroup empty — FindClusterByName will auto-detect
		// it from the cluster's ARM ID by doing a subscription-wide list.
	}

	providerFactory := provider.NewFactory()
	found := false

	for _, r := range regions {
		log.Printf("🔍 Searching for cluster '%s' in region %s (provider: %s)...", clusterName, r, providerName)

		opts.Region = r
		prov, err := providerFactory.CreateProviderWithOptions(providerName, opts)
		if err != nil {
			log.Printf("Failed to create provider for region %s: %v", r, err)
			continue
		}

		clusterMgr := cluster.NewManager(prov)

		existingCluster, err := clusterMgr.FindByName(ctx, clusterName)
		if err != nil {
			log.Printf("Failed to search for cluster in region %s: %v", r, err)
			continue
		}

		if existingCluster == nil {
			log.Printf("❌ Cluster '%s' not found in region %s", clusterName, r)
			continue
		}

		found = true
		log.Printf("✅ Found cluster '%s' in region %s with ID %s", clusterName, r, existingCluster.ID)
		log.Printf("🗑️ Force deleting cluster '%s'...", clusterName)

		// Delete the cluster
		err = clusterMgr.Delete(ctx, existingCluster.ID)
		if err != nil {
			log.Fatalf("Failed to delete cluster %s (ID: %s): %v", clusterName, existingCluster.ID, err)
		}

		log.Printf("✅ Successfully force deleted cluster '%s' from region %s", clusterName, r)
		break
	}

	if !found {
		log.Printf("❌ Cluster '%s' not found in any searched regions", clusterName)
		log.Printf("💡 Try specifying a specific region with --region flag")
	}
}
