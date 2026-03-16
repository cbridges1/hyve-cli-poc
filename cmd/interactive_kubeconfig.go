package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

func runInteractiveKubeconfig() error {
	for {
		var action string
		err := newForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Kubeconfig — what would you like to do?").
					Options(
						huh.NewOption("Sync kubeconfigs from all clusters", "sync"),
						huh.NewOption("Get kubeconfig for a cluster", "get"),
						huh.NewOption("Use (merge + set active context)", "use"),
						huh.NewOption("Merge into ~/.kube/config", "merge"),
						huh.NewOption("Remove a kubeconfig", "remove"),
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
		case "sync":
			syncKubeconfigs()
		case "get":
			if err := interactiveKubeconfigGet(); err != nil && err != errBack {
				return err
			}
		case "use":
			if err := interactiveKubeconfigUse(); err != nil && err != errBack {
				return err
			}
		case "merge":
			if err := interactiveKubeconfigMerge(); err != nil && err != errBack {
				return err
			}
		case "remove":
			if err := interactiveKubeconfigRemove(); err != nil && err != errBack {
				return err
			}
		}
	}
}

func interactiveKubeconfigGet() error {
	clusterName := ""
	if err := selectFromList("Cluster", fetchKubeconfigClusterNames(), &clusterName); err != nil {
		return err
	}
	getKubeconfig(kubeconfigGetCmd, clusterName)
	return nil
}

func interactiveKubeconfigUse() error {
	clusterName := ""
	if err := selectFromList("Cluster", fetchKubeconfigClusterNames(), &clusterName); err != nil {
		return err
	}
	useKubeconfig(clusterName)
	return nil
}

func interactiveKubeconfigMerge() error {
	clusterName := ""
	if err := selectFromList("Cluster to merge", fetchKubeconfigClusterNames(), &clusterName); err != nil {
		return err
	}
	mergeKubeconfig(clusterName)
	return nil
}

func interactiveKubeconfigRemove() error {
	clusterName := ""
	if err := selectFromList("Cluster to remove kubeconfig for", fetchKubeconfigClusterNames(), &clusterName); err != nil {
		return err
	}

	var confirm bool
	err := newForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Remove kubeconfig for '%s'?", clusterName)).
				Affirmative("Yes, remove").
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

	removeKubeconfig(clusterName)
	return nil
}
