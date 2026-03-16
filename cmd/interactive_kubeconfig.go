package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

func runInteractiveKubeconfig() error {
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
				).
				Value(&action),
		),
	).Run()
	if err != nil {
		return err
	}

	switch action {
	case "sync":
		syncKubeconfigs()
	case "get":
		return interactiveKubeconfigGet()
	case "use":
		return interactiveKubeconfigUse()
	case "merge":
		return interactiveKubeconfigMerge()
	case "remove":
		return interactiveKubeconfigRemove()
	}
	return nil
}

func interactiveKubeconfigGet() error {
	var clusterName string
	err := newForm(
		huh.NewGroup(
			huh.NewInput().Title("Cluster name").Value(&clusterName),
		),
	).Run()
	if err != nil {
		return err
	}
	getKubeconfig(kubeconfigGetCmd, clusterName)
	return nil
}

func interactiveKubeconfigUse() error {
	var clusterName string
	err := newForm(
		huh.NewGroup(
			huh.NewInput().Title("Cluster name").Value(&clusterName),
		),
	).Run()
	if err != nil {
		return err
	}
	useKubeconfig(clusterName)
	return nil
}

func interactiveKubeconfigMerge() error {
	var clusterName string
	err := newForm(
		huh.NewGroup(
			huh.NewInput().Title("Cluster name to merge").Value(&clusterName),
		),
	).Run()
	if err != nil {
		return err
	}
	mergeKubeconfig(clusterName)
	return nil
}

func interactiveKubeconfigRemove() error {
	var clusterName string
	err := newForm(
		huh.NewGroup(
			huh.NewInput().Title("Cluster name to remove kubeconfig for").Value(&clusterName),
		),
	).Run()
	if err != nil {
		return err
	}

	var confirm bool
	err = newForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Remove kubeconfig for cluster '%s'?", clusterName)).
				Affirmative("Yes, remove").
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

	removeKubeconfig(clusterName)
	return nil
}
