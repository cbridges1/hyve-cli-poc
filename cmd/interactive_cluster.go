package cmd

import (
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
						huh.NewOption("Modify an existing cluster", "modify"),
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

	err = newForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Region").
				Placeholder("us-east-1").
				Value(&region),
			huh.NewInput().
				Title("Node sizes (comma-separated)").
				Placeholder("g4s.kube.medium").
				Value(&nodesStr),
		),
	).Run()
	if err != nil {
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

	var region, nodesStr string
	err := newForm(
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

	deleteClusterFromCLI(clusterName, forceCloud, false)
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

	var region, projectName string
	err = newForm(
		huh.NewGroup(
			huh.NewInput().Title("Region").Value(&region),
		),
	).Run()
	if err != nil {
		return err
	}

	if providerName == "gcp" {
		if err := selectFromList("GCP project alias", fetchGCPProjectNames(), &projectName); err != nil {
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
