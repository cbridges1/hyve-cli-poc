package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

func runInteractiveWorkflow() error {
	var action string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Workflow — what would you like to do?").
				Options(
					huh.NewOption("Create a workflow", "create"),
					huh.NewOption("Run a workflow", "run"),
					huh.NewOption("List workflows", "list"),
					huh.NewOption("Show workflow details", "show"),
					huh.NewOption("Validate a workflow", "validate"),
					huh.NewOption("Delete a workflow", "delete"),
				).
				Value(&action),
		),
	).Run()
	if err != nil {
		return err
	}

	switch action {
	case "create":
		return interactiveWorkflowCreate()
	case "run":
		return interactiveWorkflowRun()
	case "list":
		listWorkflows()
	case "show":
		return interactiveWorkflowShow()
	case "validate":
		return interactiveWorkflowValidate()
	case "delete":
		return interactiveWorkflowDelete()
	}
	return nil
}

func interactiveWorkflowCreate() error {
	var mode string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Create from").
				Options(
					huh.NewOption("Default template", "template"),
					huh.NewOption("Existing YAML file", "file"),
				).
				Value(&mode),
		),
	).Run()
	if err != nil {
		return err
	}

	if mode == "file" {
		var fromFile string
		err = newForm(
			huh.NewGroup(
				huh.NewInput().Title("Path to YAML file").Placeholder("./workflow.yaml").Value(&fromFile),
			),
		).Run()
		if err != nil {
			return err
		}
		createWorkflowFromFile(fromFile)
		return nil
	}

	var name, description string
	err = newForm(
		huh.NewGroup(
			huh.NewInput().Title("Workflow name").Placeholder("deploy-app").Value(&name),
			huh.NewInput().Title("Description (optional)").Value(&description),
		),
	).Run()
	if err != nil {
		return err
	}
	createWorkflowTemplate(name, description)
	return nil
}

func interactiveWorkflowRun() error {
	var (
		name       string
		cluster    string
		showLogs   bool
		showOutput bool
	)

	showLogs = true // default

	err := newForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Workflow name").
				Value(&name),
			huh.NewInput().
				Title("Cluster (leave blank to run locally)").
				Value(&cluster),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Show execution logs?").
				Affirmative("Yes").
				Negative("No").
				Value(&showLogs),
			huh.NewConfirm().
				Title("Show step outputs?").
				Affirmative("Yes").
				Negative("No").
				Value(&showOutput),
		),
	).Run()
	if err != nil {
		return err
	}

	runWorkflow(name, cluster, showLogs, showOutput)
	return nil
}

func interactiveWorkflowShow() error {
	var name string
	err := newForm(
		huh.NewGroup(
			huh.NewInput().Title("Workflow name").Value(&name),
		),
	).Run()
	if err != nil {
		return err
	}
	showWorkflow(name)
	return nil
}

func interactiveWorkflowValidate() error {
	var name string
	err := newForm(
		huh.NewGroup(
			huh.NewInput().Title("Workflow name to validate").Value(&name),
		),
	).Run()
	if err != nil {
		return err
	}
	validateWorkflow(name)
	return nil
}

func interactiveWorkflowDelete() error {
	var name string
	err := newForm(
		huh.NewGroup(
			huh.NewInput().Title("Workflow name to delete").Value(&name),
		),
	).Run()
	if err != nil {
		return err
	}

	var confirm bool
	err = newForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Delete workflow '%s'?", name)).
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

	deleteWorkflow(name, true)
	return nil
}
