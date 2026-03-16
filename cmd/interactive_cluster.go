package cmd

import (
	gocontext "context"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"hyve/internal/types"
)

func runInteractiveCluster() error {
	for {
		var action string
		err := newForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Cluster — what would you like to do?").
					Options(
						huh.NewOption("Add a new cluster", "add"),
						huh.NewOption("Import an existing cluster", "import"),
						huh.NewOption("Modify an existing cluster", "modify"),
						huh.NewOption("Release a cluster from management", "release"),
						huh.NewOption("Delete a cluster", "delete"),
						huh.NewOption("Force-delete a cluster from cloud", "force-delete"),
						huh.NewOption("List clusters", "list"),
						huh.NewOption("← Back", "back"),
					).
					Value(&action),
			),
		).Run()
		if err != nil {
			return err
		}

		switch action {
		case "back":
			return errBack
		case "list":
			listClusters()
		case "add":
			if err := interactiveClusterAdd(); err != nil && err != errBack {
				return err
			}
		case "import":
			if err := interactiveClusterImport(); err != nil && err != errBack {
				return err
			}
		case "release":
			if err := interactiveClusterRelease(); err != nil && err != errBack {
				return err
			}
		case "modify":
			if err := interactiveClusterModify(); err != nil && err != errBack {
				return err
			}
		case "delete":
			if err := interactiveClusterDelete(); err != nil && err != errBack {
				return err
			}
		case "force-delete":
			if err := interactiveClusterForceDelete(); err != nil && err != errBack {
				return err
			}
		}
	}
}

func interactiveClusterAdd() error {
	var (
		clusterName      string
		providerName     string
		region           string
		nodesStr         string
		clusterType      string
		accountName      string
		projectName      string
		subscriptionName string
		orgName          string
		vpcName          string
		eksRoleName      string
		nodeRoleName     string
	)

	err := newForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Cluster name").
				Placeholder("my-cluster").
				Value(&clusterName),
			huh.NewSelect[string]().
				Title("Cloud provider").
				Options(
					huh.NewOption("Civo", "civo"),
					huh.NewOption("AWS (EKS)", "aws"),
					huh.NewOption("GCP (GKE)", "gcp"),
					huh.NewOption("Azure (AKS)", "azure"),
					huh.NewOption("← Back", "back"),
				).
				Value(&providerName),
		),
	).Run()
	if err != nil {
		return err
	}
	if providerName == "back" {
		return errBack
	}

	ctx := gocontext.Background()
	if err := selectFromGroups("Region", fetchRegionGroups(ctx, providerName, ""), "us-east-1", &region); err != nil {
		return err
	}
	if err := selectFromGroups("Node size", fetchNodeGroups(ctx, providerName, region, ""), "g4s.kube.medium", &nodesStr); err != nil {
		return err
	}

	// Cluster type is only applicable to Civo
	if providerName == "civo" {
		err = newForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Cluster type").
					Options(
						huh.NewOption("k3s (default)", ""),
						huh.NewOption("talos", "talos"),
					).
					Value(&clusterType),
			),
		).Run()
		if err != nil {
			return err
		}
	}

	// Provider-specific fields — use selects populated from config
	switch providerName {
	case "civo":
		if err := selectFromList("Civo organization", fetchCivoOrgNames(), &orgName); err != nil {
			return err
		}
	case "aws":
		if err := selectFromList("AWS account alias", fetchAWSAccountNames(), &accountName); err != nil {
			return err
		}
		if err := selectFromList("VPC alias", fetchAWSVPCNames(accountName), &vpcName); err != nil {
			return err
		}
		if err := selectFromList("EKS role alias", fetchAWSEKSRoleNames(accountName), &eksRoleName); err != nil {
			return err
		}
		if err := selectFromList("Node role alias", fetchAWSNodeRoleNames(accountName), &nodeRoleName); err != nil {
			return err
		}
	case "gcp":
		if err := selectFromList("GCP project alias", fetchGCPProjectNames(), &projectName); err != nil {
			return err
		}
	case "azure":
		if err := selectFromList("Azure subscription alias", fetchAzureSubscriptionNames(), &subscriptionName); err != nil {
			return err
		}
	}

	nodes := splitAndTrim(nodesStr, ",")
	var confirm bool
	summary := fmt.Sprintf("Add cluster '%s' on %s in %s with nodes: %s", clusterName, providerName, region, strings.Join(nodes, ", "))
	err = newForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(summary).
				Affirmative("Create").
				Negative("Cancel").
				Value(&confirm),
		),
	).Run()
	if err != nil {
		return err
	}
	if !confirm {
		return nil
	}

	addClusterFromCLI(clusterName, region, providerName, nodes, []types.NodeGroup{}, clusterType, accountName, projectName, subscriptionName, orgName, vpcName, eksRoleName, nodeRoleName)
	return nil
}

func interactiveClusterModify() error {
	clusterName := ""
	if err := selectFromList("Cluster to modify", fetchClusterNames(), &clusterName); err != nil {
		return err
	}

	// Determine provider from existing cluster definition so we can offer the right lists.
	var region, nodesStr, providerForModify string
	sm, _ := createStateManager(gocontext.Background())
	if sm != nil {
		defs, _ := sm.LoadClusterDefinitions()
		for _, d := range defs {
			if d.Metadata.Name == clusterName {
				providerForModify = d.Spec.Provider
				break
			}
		}
	}

	ctx2 := gocontext.Background()
	if err := selectFromGroupsOptional("New region", fetchRegionGroups(ctx2, providerForModify, ""), &region); err != nil {
		return err
	}
	if err := selectFromGroupsOptional("New node size", fetchNodeGroups(ctx2, providerForModify, region, ""), &nodesStr); err != nil {
		return err
	}

	modifyCmd.Flags().Set("region", region)
	if nodesStr != "" {
		for _, n := range splitAndTrim(nodesStr, ",") {
			modifyCmd.Flags().Set("nodes", n)
		}
	}
	modifyClusterFromCLI(modifyCmd, clusterName)
	return nil
}

func interactiveClusterDelete() error {
	clusterName := ""
	if err := selectFromList("Cluster to delete", fetchClusterNames(), &clusterName); err != nil {
		return err
	}

	var forceCloud bool
	err := newForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Delete from cloud immediately?").
				Description("No = remove from state only (GitOps reconcile handles cloud deletion)").
				Affirmative("Yes — delete from cloud now").
				Negative("No — GitOps").
				Value(&forceCloud),
		),
	).Run()
	if err != nil {
		return err
	}

	action := "remove from Git state (GitOps)"
	if forceCloud {
		action = "DELETE from cloud immediately"
	}
	var confirm bool
	err = newForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Confirm: %s cluster '%s'?", action, clusterName)).
				Affirmative("Yes, delete").
				Negative("Cancel").
				Value(&confirm),
		),
	).Run()
	if err != nil {
		return err
	}
	if !confirm {
		return nil
	}

	// forceCloud = user chose "Yes — delete from cloud now"
	// allowNoConfig=false: the cluster was selected from the local list so a config always exists
	deleteClusterFromCLI(clusterName, false, forceCloud)
	return nil
}

func interactiveClusterForceDelete() error {
	clusterName := ""
	if err := selectFromList("Cluster to force-delete", fetchClusterNames(), &clusterName); err != nil {
		return err
	}

	var providerName string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Cloud provider").
				Options(
					huh.NewOption("Civo", "civo"),
					huh.NewOption("AWS (EKS)", "aws"),
					huh.NewOption("GCP (GKE)", "gcp"),
					huh.NewOption("Azure (AKS)", "azure"),
				).
				Value(&providerName),
		),
	).Run()
	if err != nil {
		return err
	}

	// Account / org / project / subscription selection
	var accountAlias, projectName string
	switch providerName {
	case "civo":
		if err := selectFromList("Civo organization", fetchCivoOrgNames(), &accountAlias); err != nil {
			return err
		}
		projectName = accountAlias // used for token lookup in forceDeleteClusterFromCloud
	case "aws":
		if err := selectFromList("AWS account alias", fetchAWSAccountNames(), &accountAlias); err != nil {
			return err
		}
	case "gcp":
		if err := selectFromList("GCP project alias", fetchGCPProjectNames(), &projectName); err != nil {
			return err
		}
		accountAlias = projectName
	case "azure":
		if err := selectFromList("Azure subscription alias", fetchAzureSubscriptionNames(), &accountAlias); err != nil {
			return err
		}
	}

	ctxFD := gocontext.Background()
	var region string
	if err := selectFromGroups("Region", fetchRegionGroups(ctxFD, providerName, accountAlias), "us-east-1", &region); err != nil {
		return err
	}

	var confirm bool
	err = newForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Force-delete '%s' from %s/%s? This cannot be undone.", clusterName, providerName, region)).
				Affirmative("Yes, force-delete").
				Negative("Cancel").
				Value(&confirm),
		),
	).Run()
	if err != nil {
		return err
	}
	if !confirm {
		return nil
	}

	forceDeleteClusterFromCloud(clusterName, region, providerName, projectName, accountAlias)
	return nil
}

// splitAndTrim splits s by sep and trims whitespace from each element.
func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func interactiveClusterImport() error {
	if sm, _ := createStateManager(gocontext.Background()); sm != nil {
		if repoCfg, err := sm.LoadRepoConfig(); err == nil && repoCfg.Reconcile.StrictDelete {
			fmt.Println("❌ Import is disabled: this repository has strictDelete enabled.")
			fmt.Println("   In strict-delete mode hyve owns the full desired-state; importing an unmanaged cluster would cause it to be deleted on the next reconciliation.")
			return nil
		}
	}

	var (
		providerName string
		accountAlias string
		region       string
		clusterName  string
		vpcName      string
		eksRoleName  string
		nodeRoleName string
	)

	// Step 1: provider
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Cloud provider").
				Options(
					huh.NewOption("Civo", "civo"),
					huh.NewOption("AWS (EKS)", "aws"),
					huh.NewOption("GCP (GKE)", "gcp"),
					huh.NewOption("Azure (AKS)", "azure"),
					huh.NewOption("← Back", "back"),
				).
				Value(&providerName),
		),
	).Run()
	if err != nil {
		return err
	}
	if providerName == "back" {
		return errBack
	}

	// Step 2: account / org / project / subscription
	switch providerName {
	case "civo":
		if err := selectFromList("Civo organization", fetchCivoOrgNames(), &accountAlias); err != nil {
			return err
		}
	case "aws":
		if err := selectFromList("AWS account alias", fetchAWSAccountNames(), &accountAlias); err != nil {
			return err
		}
		if err := selectFromList("VPC alias", fetchAWSVPCNames(accountAlias), &vpcName); err != nil {
			return err
		}
		if err := selectFromList("EKS role alias", fetchAWSEKSRoleNames(accountAlias), &eksRoleName); err != nil {
			return err
		}
		if err := selectFromList("Node role alias", fetchAWSNodeRoleNames(accountAlias), &nodeRoleName); err != nil {
			return err
		}
	case "gcp":
		if err := selectFromList("GCP project alias", fetchGCPProjectNames(), &accountAlias); err != nil {
			return err
		}
	case "azure":
		if err := selectFromList("Azure subscription alias", fetchAzureSubscriptionNames(), &accountAlias); err != nil {
			return err
		}
	}

	// Step 3: region (pass accountAlias so we use the right credentials)
	ctx := gocontext.Background()
	if err := selectFromGroups("Region", fetchRegionGroups(ctx, providerName, accountAlias), "us-east-1", &region); err != nil {
		return err
	}

	// Step 4: cluster name — select from cloud or enter manually
	cloudNames := fetchCloudClusterNames(ctx, providerName, region, accountAlias)
	const manualKey = "__manual__"
	if len(cloudNames) > 0 {
		opts := make([]huh.Option[string], 0, len(cloudNames)+2)
		opts = append(opts, huh.NewOption("Enter manually...", manualKey))
		for _, n := range cloudNames {
			opts = append(opts, huh.NewOption(n, n))
		}
		opts = append(opts, huh.NewOption("← Back", "__back__"))

		selection := ""
		if err := newForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select cluster to import").
					Options(opts...).
					Value(&selection),
			),
		).Run(); err != nil {
			return err
		}
		switch selection {
		case "__back__":
			return errBack
		case manualKey:
			// fall through to manual input below
		default:
			clusterName = selection
		}
	}

	if clusterName == "" {
		if err := newForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Cluster name (must match the name in your cloud provider)").
					Placeholder("my-cluster").
					Value(&clusterName),
			),
		).Run(); err != nil {
			return err
		}
	}

	// Step 5: confirm
	// Map accountAlias back to the right field for importClusterFromCLI
	var orgName, projectName, subscriptionName, accountName string
	switch providerName {
	case "civo":
		orgName = accountAlias
	case "aws":
		accountName = accountAlias
	case "gcp":
		projectName = accountAlias
	case "azure":
		subscriptionName = accountAlias
	}

	var confirm bool
	summary := fmt.Sprintf("Import '%s' (%s, %s) into hyve — cloud cluster will NOT be reprovisioned", clusterName, providerName, region)
	err = newForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(summary).
				Affirmative("Import").
				Negative("Cancel").
				Value(&confirm),
		),
	).Run()
	if err != nil {
		return err
	}
	if !confirm {
		return nil
	}

	importClusterFromCLI(clusterName, region, providerName, nil, []types.NodeGroup{}, accountName, projectName, subscriptionName, orgName, vpcName, eksRoleName, nodeRoleName)
	return nil
}

func interactiveClusterRelease() error {
	if sm, _ := createStateManager(gocontext.Background()); sm != nil {
		if repoCfg, err := sm.LoadRepoConfig(); err == nil && repoCfg.Reconcile.StrictDelete {
			fmt.Println("❌ Release is disabled: this repository has strictDelete enabled.")
			fmt.Println("   In strict-delete mode removing a cluster definition would cause the cloud cluster to be deleted on the next reconciliation.")
			fmt.Println("   Use 'hyve cluster delete' instead.")
			return nil
		}
	}

	clusterName := ""
	if err := selectFromList("Cluster to release from management", fetchClusterNames(), &clusterName); err != nil {
		return err
	}

	var confirm bool
	err := newForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Release '%s' from hyve management? The cloud cluster will NOT be deleted.", clusterName)).
				Affirmative("Yes, release").
				Negative("Cancel").
				Value(&confirm),
		),
	).Run()
	if err != nil {
		return err
	}
	if !confirm {
		return nil
	}

	releaseClusterFromCLI(clusterName)
	return nil
}
