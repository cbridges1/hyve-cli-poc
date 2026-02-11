package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"civo-cluster-deploy/internal/cluster"
	"civo-cluster-deploy/internal/provider"
	"civo-cluster-deploy/internal/providerconfig"
	"civo-cluster-deploy/internal/repository"
	"civo-cluster-deploy/internal/types"
)

func exportClusterInfo(ctx context.Context, apiKey string, clusterDef types.ClusterDefinition) error {
	factory := provider.NewFactory()

	// Build provider options based on cluster type
	providerName := clusterDef.Spec.Provider
	if providerName == "" {
		providerName = "civo"
	}

	opts := provider.ProviderOptions{
		Region: clusterDef.Metadata.Region,
	}

	// Only set API key for Civo provider
	if providerName == "civo" {
		opts.APIKey = apiKey
	}

	// Handle GCP-specific configuration
	if providerName == "gcp" {
		if clusterDef.Spec.GCPProjectID != "" {
			opts.ProjectID = clusterDef.Spec.GCPProjectID
		} else if clusterDef.Spec.GCPProject != "" {
			// Resolve from alias
			repoMgr, err := repository.NewManager()
			if err == nil {
				defer repoMgr.Close()
				if currentRepo, err := repoMgr.GetCurrentRepository(); err == nil {
					pcMgr := providerconfig.NewManager(currentRepo.LocalPath)
					if projectID, err := pcMgr.GetGCPProjectID(clusterDef.Spec.GCPProject); err == nil {
						opts.ProjectID = projectID
					}
				}
			}
		}
	}

	prov, err := factory.CreateProviderWithOptions(providerName, opts)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	clusterMgr := cluster.NewManager(prov)

	clusterInfo, err := clusterMgr.GetClusterInfo(ctx, clusterDef.Metadata.Name)
	if err != nil {
		return fmt.Errorf("failed to get cluster info: %w", err)
	}

	if clusterInfo.Status != "ACTIVE" {
		log.Printf("Cluster %s is not active (status: %s), skipping export", clusterInfo.Name, clusterInfo.Status)
		return nil
	}

	if githubEnv := os.Getenv("GITHUB_ENV"); githubEnv != "" {
		log.Printf("Exporting cluster information to GitHub Actions environment")
		file, err := os.OpenFile(githubEnv, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("Warning: Failed to open GITHUB_ENV file: %v", err)
		} else {
			defer file.Close()

			fmt.Fprintf(file, "HYVE_CLUSTER_NAME=%s\n", clusterInfo.Name)
			fmt.Fprintf(file, "HYVE_CLUSTER_IP_ADDRESS=%s\n", clusterInfo.IPAddress)
			fmt.Fprintf(file, "HYVE_CLUSTER_ACCESS_PORT=%s\n", clusterInfo.AccessPort)
			fmt.Fprintf(file, "HYVE_CLUSTER_ID=%s\n", clusterInfo.ID)
			fmt.Fprintf(file, "HYVE_CLUSTER_STATUS=%s\n", clusterInfo.Status)
			fmt.Fprintf(file, "HYVE_CLUSTER_KUBECONFIG=%s\n", clusterInfo.Kubeconfig)

			log.Printf("✅ Exported cluster information to GitHub Actions environment:")
			log.Printf("  HYVE_CLUSTER_NAME=%s", clusterInfo.Name)
			log.Printf("  HYVE_CLUSTER_IP_ADDRESS=%s", clusterInfo.IPAddress)
			log.Printf("  HYVE_CLUSTER_ACCESS_PORT=%s", clusterInfo.AccessPort)
			log.Printf("  HYVE_CLUSTER_ID=%s", clusterInfo.ID)
			log.Printf("  HYVE_CLUSTER_STATUS=%s", clusterInfo.Status)
			log.Printf("  HYVE_CLUSTER_KUBECONFIG=<kubeconfig content>")
		}
	}

	os.Setenv("HYVE_CLUSTER_NAME", clusterInfo.Name)
	os.Setenv("HYVE_CLUSTER_IP_ADDRESS", clusterInfo.IPAddress)
	os.Setenv("HYVE_CLUSTER_ACCESS_PORT", clusterInfo.AccessPort)
	os.Setenv("HYVE_CLUSTER_ID", clusterInfo.ID)
	os.Setenv("HYVE_CLUSTER_STATUS", clusterInfo.Status)
	os.Setenv("HYVE_CLUSTER_KUBECONFIG", clusterInfo.Kubeconfig)

	return nil
}
