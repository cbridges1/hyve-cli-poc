package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"hyve/internal/types"
)

func runInteractiveCluster() error {
	var action string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Cluster — what would you like to do?").
				Options(
					huh.NewOption("Add a new cluster", "add"),
					huh.NewOption("Modify an existing cluster", "modify"),
					huh.NewOption("Delete a cluster", "delete"),
					huh.NewOption("Force-delete a cluster from cloud", "force-delete"),
					huh.NewOption("List clusters", "list"),
				).
				Value(&action),
		),
	).Run()
	if err != nil {
		return err
	}

	switch action {
	case "add":
		return interactiveClusterAdd()
	case "modify":
		return interactiveClusterModify()
	case "delete":
		return interactiveClusterDelete()
	case "force-delete":
		return interactiveClusterForceDelete()
	case "list":
		listClusters()
	}
	return nil
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
				).
				Value(&providerName),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Region").
				Placeholder("us-east-1").
				Value(&region),
			huh.NewInput().
				Title("Node sizes (comma-separated)").
				Placeholder("g4s.kube.medium").
				Value(&nodesStr),
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

	// Provider-specific fields
	switch providerName {
	case "civo":
		err = newForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Civo organization name").
					Placeholder("my-org").
					Value(&orgName),
			),
		).Run()
	case "aws":
		err = newForm(
			huh.NewGroup(
				huh.NewInput().
					Title("AWS account alias").
					Placeholder("prod").
					Value(&accountName),
				huh.NewInput().
					Title("VPC name alias").
					Placeholder("eks-vpc").
					Value(&vpcName),
				huh.NewInput().
					Title("EKS role name alias").
					Placeholder("eks-role").
					Value(&eksRoleName),
				huh.NewInput().
					Title("Node role name alias").
					Placeholder("node-role").
					Value(&nodeRoleName),
			),
		).Run()
	case "gcp":
		err = newForm(
			huh.NewGroup(
				huh.NewInput().
					Title("GCP project name alias").
					Placeholder("my-project").
					Value(&projectName),
			),
		).Run()
	case "azure":
		err = newForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Azure subscription name alias").
					Placeholder("prod-sub").
					Value(&subscriptionName),
			),
		).Run()
	}
	if err != nil {
		return err
	}

	var confirm bool
	nodes := splitAndTrim(nodesStr, ",")
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
		fmt.Println("Cancelled.")
		return nil
	}

	addClusterFromCLI(clusterName, region, providerName, nodes, []types.NodeGroup{}, clusterType, accountName, projectName, subscriptionName, orgName, vpcName, eksRoleName, nodeRoleName)
	return nil
}

func interactiveClusterModify() error {
	var clusterName string
	err := newForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Cluster name to modify").
				Value(&clusterName),
		),
	).Run()
	if err != nil {
		return err
	}

	var (
		region   string
		nodesStr string
	)

	err = newForm(
		huh.NewGroup(
			huh.NewInput().
				Title("New region (leave blank to keep current)").
				Value(&region),
			huh.NewInput().
				Title("New node sizes, comma-separated (leave blank to keep current)").
				Value(&nodesStr),
		),
	).Run()
	if err != nil {
		return err
	}

	// Build a fake cobra.Command with the flags set so we can reuse modifyClusterFromCLI
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
	var (
		clusterName string
		force       bool
		forceCloud  bool
	)

	err := newForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Cluster name to delete").
				Value(&clusterName),
			huh.NewConfirm().
				Title("Delete from cloud immediately (--force)?").
				Affirmative("Yes").
				Negative("No (GitOps — remove YAML and reconcile)").
				Value(&forceCloud),
		),
	).Run()
	if err != nil {
		return err
	}

	var confirm bool
	action := "remove from state (GitOps)"
	if forceCloud {
		action = "DELETE from cloud immediately"
	}
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
		fmt.Println("Cancelled.")
		return nil
	}

	deleteClusterFromCLI(clusterName, forceCloud, force)
	return nil
}

func interactiveClusterForceDelete() error {
	var (
		clusterName  string
		region       string
		providerName string
		projectName  string
	)

	err := newForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Cluster name").
				Value(&clusterName),
			huh.NewSelect[string]().
				Title("Cloud provider").
				Options(
					huh.NewOption("Civo", "civo"),
					huh.NewOption("AWS (EKS)", "aws"),
					huh.NewOption("GCP (GKE)", "gcp"),
					huh.NewOption("Azure (AKS)", "azure"),
				).
				Value(&providerName),
			huh.NewInput().
				Title("Region").
				Value(&region),
		),
	).Run()
	if err != nil {
		return err
	}

	if providerName == "gcp" {
		err = newForm(
			huh.NewGroup(
				huh.NewInput().
					Title("GCP project name alias").
					Value(&projectName),
			),
		).Run()
		if err != nil {
			return err
		}
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
		fmt.Println("Cancelled.")
		return nil
	}

	forceDeleteClusterFromCloud(clusterName, region, providerName, projectName)
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
