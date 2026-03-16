package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
)

func runInteractiveTemplate() error {
	for {
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
			listTemplates()
		case "create":
			if err := interactiveTemplateCreate(); err != nil && err != errBack {
				return err
			}
		case "execute":
			if err := interactiveTemplateExecute(); err != nil && err != errBack {
				return err
			}
		case "show":
			if err := interactiveTemplateShow(); err != nil && err != errBack {
				return err
			}
		case "validate":
			if err := interactiveTemplateValidate(); err != nil && err != errBack {
				return err
			}
		case "delete":
			if err := interactiveTemplateDelete(); err != nil && err != errBack {
				return err
			}
		}
	}
}

func interactiveTemplateCreate() error {
	var (
		name           string
		description    string
		provider       string
		region         string
		nodesSizes     string
		clusterType    string
		onCreatedNames []string
		onDestroyNames []string
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
					huh.NewOption("← Back", "back"),
				).
				Value(&provider),
		),
	).Run()
	if err != nil {
		return err
	}
	if provider == "back" {
		return errBack
	}

	ctx := context.Background()
	if err := selectFromGroups("Region", fetchRegionGroups(ctx, provider, ""), "us-east-1", &region); err != nil {
		return err
	}
	if err := selectFromGroups("Node size", fetchNodeGroups(ctx, provider, region, ""), "g4s.kube.medium", &nodesSizes); err != nil {
		return err
	}

	// Cluster type is only applicable to Civo
	if provider == "civo" {
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

	// Workflow attachment — optional last step
	if wfNames := fetchWorkflowNames(); len(wfNames) > 0 {
		makeOpts := func() []huh.Option[string] {
			opts := make([]huh.Option[string], len(wfNames))
			for i, wf := range wfNames {
				opts[i] = huh.NewOption(wf, wf)
			}
			return opts
		}
		err = newForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("On-created workflows (optional — space to select, enter to confirm)").
					Options(makeOpts()...).
					Value(&onCreatedNames),
				huh.NewMultiSelect[string]().
					Title("On-destroy workflows (optional — space to select, enter to confirm)").
					Options(makeOpts()...).
					Value(&onDestroyNames),
			),
		).Run()
		if err != nil {
			return err
		}
	}

	onCreatedStr := strings.Join(onCreatedNames, ",")
	onDestroyStr := strings.Join(onDestroyNames, ",")
	createTemplate(name, description, provider, region, nodesSizes, clusterType, onCreatedStr, onDestroyStr)
	return nil
}

func interactiveTemplateExecute() error {
	templateName := ""
	if err := selectFromList("Template to execute", fetchTemplateNames(), &templateName); err != nil {
		return err
	}

	var clusterName string
	err := newForm(
		huh.NewGroup(
			huh.NewInput().Title("New cluster name").Value(&clusterName),
		),
	).Run()
	if err != nil {
		return err
	}

	executeTemplate(templateName, clusterName, "", "", "", "", "", "", "", "")
	return nil
}

func interactiveTemplateShow() error {
	name := ""
	if err := selectFromList("Template to show", fetchTemplateNames(), &name); err != nil {
		return err
	}
	showTemplate(name)
	return nil
}

func interactiveTemplateValidate() error {
	name := ""
	if err := selectFromList("Template to validate", fetchTemplateNames(), &name); err != nil {
		return err
	}
	validateTemplate(name)
	return nil
}

func interactiveTemplateDelete() error {
	name := ""
	if err := selectFromList("Template to delete", fetchTemplateNames(), &name); err != nil {
		return err
	}

	var confirm bool
	err := newForm(
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
		return nil
	}

	deleteTemplate(name)
	return nil
}
