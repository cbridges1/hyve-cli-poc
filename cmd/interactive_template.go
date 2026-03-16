package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

func runInteractiveTemplate() error {
	var action string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Template — what would you like to do?").
				Options(
					huh.NewOption("Create a template", "create"),
					huh.NewOption("Execute a template", "execute"),
					huh.NewOption("List templates", "list"),
					huh.NewOption("Show template details", "show"),
					huh.NewOption("Validate a template", "validate"),
					huh.NewOption("Delete a template", "delete"),
				).
				Value(&action),
		),
	).Run()
	if err != nil {
		return err
	}

	switch action {
	case "create":
		return interactiveTemplateCreate()
	case "execute":
		return interactiveTemplateExecute()
	case "list":
		listTemplates()
	case "show":
		return interactiveTemplateShow()
	case "validate":
		return interactiveTemplateValidate()
	case "delete":
		return interactiveTemplateDelete()
	}
	return nil
}

func interactiveTemplateCreate() error {
	var (
		name        string
		description string
		provider    string
		region      string
		nodesSizes  string
		clusterType string
	)

	err := newForm(
		huh.NewGroup(
			huh.NewInput().Title("Template name").Placeholder("my-template").Value(&name),
			huh.NewInput().Title("Description (optional)").Value(&description),
			huh.NewSelect[string]().
				Title("Cloud provider").
				Options(
					huh.NewOption("Civo", "civo"),
					huh.NewOption("AWS (EKS)", "aws"),
					huh.NewOption("GCP (GKE)", "gcp"),
					huh.NewOption("Azure (AKS)", "azure"),
				).
				Value(&provider),
		),
		huh.NewGroup(
			huh.NewInput().Title("Region").Placeholder("us-east-1").Value(&region),
			huh.NewInput().Title("Node sizes (comma-separated)").Placeholder("g4s.kube.medium").Value(&nodesSizes),
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

	createTemplate(name, description, provider, region, nodesSizes, clusterType, "", "")
	return nil
}

func interactiveTemplateExecute() error {
	var (
		templateName  string
		clusterName   string
		providerName  string
		org           string
		account       string
		vpcName       string
		eksRole       string
		nodeRole      string
		subscription  string
		resourceGroup string
		project       string
	)

	err := newForm(
		huh.NewGroup(
			huh.NewInput().Title("Template name").Value(&templateName),
			huh.NewInput().Title("New cluster name").Value(&clusterName),
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

	switch providerName {
	case "civo":
		err = newForm(
			huh.NewGroup(
				huh.NewInput().Title("Civo organization name").Value(&org),
			),
		).Run()
	case "aws":
		err = newForm(
			huh.NewGroup(
				huh.NewInput().Title("AWS account alias").Value(&account),
				huh.NewInput().Title("VPC name alias").Value(&vpcName),
				huh.NewInput().Title("EKS role alias").Value(&eksRole),
				huh.NewInput().Title("Node role alias").Value(&nodeRole),
			),
		).Run()
	case "gcp":
		err = newForm(
			huh.NewGroup(
				huh.NewInput().Title("GCP project alias").Value(&project),
			),
		).Run()
	case "azure":
		err = newForm(
			huh.NewGroup(
				huh.NewInput().Title("Azure subscription alias").Value(&subscription),
				huh.NewInput().Title("Resource group name").Value(&resourceGroup),
			),
		).Run()
	}
	if err != nil {
		return err
	}

	executeTemplate(templateName, clusterName, org, account, vpcName, eksRole, nodeRole, subscription, resourceGroup, project)
	return nil
}

func interactiveTemplateShow() error {
	var name string
	err := newForm(
		huh.NewGroup(huh.NewInput().Title("Template name").Value(&name)),
	).Run()
	if err != nil {
		return err
	}
	showTemplate(name)
	return nil
}

func interactiveTemplateValidate() error {
	var name string
	err := newForm(
		huh.NewGroup(huh.NewInput().Title("Template name to validate").Value(&name)),
	).Run()
	if err != nil {
		return err
	}
	validateTemplate(name)
	return nil
}

func interactiveTemplateDelete() error {
	var name string
	err := newForm(
		huh.NewGroup(huh.NewInput().Title("Template name to delete").Value(&name)),
	).Run()
	if err != nil {
		return err
	}

	var confirm bool
	err = newForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Delete template '%s'?", name)).
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

	deleteTemplate(name)
	return nil
}
