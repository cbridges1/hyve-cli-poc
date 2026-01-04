package cmd

import (
	"context"
	"log"

	"github.com/spf13/cobra"

	"civo-cluster-deploy/internal/config"
	"civo-cluster-deploy/internal/reconcile"
	"civo-cluster-deploy/internal/state"
)

var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Reconcile clusters based on YAML files",
	Long: `Reconcile clusters by reading cluster definitions from YAML files in the state/clusters directory
and ensuring the actual infrastructure matches the desired state.`,
	Run: func(cmd *cobra.Command, args []string) {
		runReconciliation()
	},
}

func runReconciliation() {
	configMgr := config.NewManager()
	apiKey := configMgr.GetCivoToken()
	if apiKey == "" {
		log.Fatal("CIVO_TOKEN environment variable is required")
	}

	stateMgr := state.NewManager("state/clusters")

	clusterDefs, err := stateMgr.LoadClusterDefinitions()
	if err != nil {
		log.Fatalf("Failed to load cluster definitions: %v", err)
	}

	err = stateMgr.ValidateClusterDefinitions(clusterDefs)
	if err != nil {
		log.Fatalf("Invalid cluster configuration: %v", err)
	}

	clusterDefs = stateMgr.OrderClusters(clusterDefs)

	ctx := context.Background()

	reconciler := reconcile.NewReconciler(apiKey, stateMgr)
	err = reconciler.ReconcileAll(ctx, clusterDefs)
	if err != nil {
		log.Fatalf("Reconciliation failed: %v", err)
	}

	log.Println("Cluster reconciliation completed")
}
