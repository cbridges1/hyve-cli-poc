package cmd

import (
	gocontext "context"
	"errors"
	"github.com/charmbracelet/huh"
	"hyve/internal/providerconfig"
	"hyve/internal/repository"
	"hyve/internal/template"
	"hyve/internal/workflow"
)

// errBack is the sentinel returned when the user selects "← Back" in any menu.
var errBack = errors.New("back")

// ── List helpers ─────────────────────────────────────────────────────────────
// Each helper returns a slice of name strings. On error (e.g. no repo
// configured) it logs a warning and returns nil so callers can fall back to a
// free-text input.

func fetchClusterNames() []string {
	sm, _ := createStateManager(gocontext.Background())
	defs, err := sm.LoadClusterDefinitions()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(defs))
	for _, d := range defs {
		names = append(names, d.Metadata.Name)
	}
	return names
}

func fetchWorkflowNames() []string {
	mgr, err := workflow.NewManager(getWorkflowLocalPath())
	if err != nil {
		return nil
	}
	list, err := mgr.ListWorkflows()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(list))
	for _, w := range list {
		names = append(names, w.Metadata.Name)
	}
	return names
}

func fetchTemplateNames() []string {
	repoMgr, err := repository.NewManager()
	if err != nil {
		return nil
	}
	defer repoMgr.Close()
	repo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		return nil
	}
	mgr := template.NewManager(repo.LocalPath)
	list, err := mgr.ListTemplates()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(list))
	for _, t := range list {
		names = append(names, t.Metadata.Name)
	}
	return names
}

func fetchGitRepoNames() []string {
	repoMgr, err := repository.NewManager()
	if err != nil {
		return nil
	}
	defer repoMgr.Close()
	repos, err := repoMgr.ListRepositories()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(repos))
	for _, r := range repos {
		names = append(names, r.Name)
	}
	return names
}

func fetchKubeconfigClusterNames() []string {
	mgr, _, err := createKubeconfigManager() // defined in kubeconfig.go
	if err != nil {
		return nil
	}
	list, err := mgr.ListKubeconfigs()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(list))
	for _, k := range list {
		names = append(names, k.ClusterName)
	}
	return names
}

func fetchAWSAccountNames() []string {
	mgr := providerconfig.NewManager(getRepoPath())
	accounts, err := mgr.ListAWSAccounts()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(accounts))
	for _, a := range accounts {
		names = append(names, a.Name)
	}
	return names
}

func fetchAWSEKSRoleNames(account string) []string {
	mgr := providerconfig.NewManager(getRepoPath())
	roles, err := mgr.ListAWSEKSRoles(account)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(roles))
	for _, r := range roles {
		names = append(names, r.Name)
	}
	return names
}

func fetchAWSNodeRoleNames(account string) []string {
	mgr := providerconfig.NewManager(getRepoPath())
	roles, err := mgr.ListAWSNodeRoles(account)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(roles))
	for _, r := range roles {
		names = append(names, r.Name)
	}
	return names
}

func fetchAWSVPCNames(account string) []string {
	mgr := providerconfig.NewManager(getRepoPath())
	vpcs, err := mgr.ListAWSVPCs(account)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(vpcs))
	for _, v := range vpcs {
		names = append(names, v.Name)
	}
	return names
}

func fetchGCPProjectNames() []string {
	mgr := providerconfig.NewManager(getRepoPath())
	projects, err := mgr.ListGCPProjects()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(projects))
	for _, p := range projects {
		names = append(names, p.Name)
	}
	return names
}

func fetchAzureSubscriptionNames() []string {
	mgr := providerconfig.NewManager(getRepoPath())
	subs, err := mgr.ListAzureSubscriptions()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(subs))
	for _, s := range subs {
		names = append(names, s.Name)
	}
	return names
}

func fetchAzureResourceGroupNames(subscription string) []string {
	mgr := providerconfig.NewManager(getRepoPath())
	rgs, err := mgr.ListAzureResourceGroups(subscription)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(rgs))
	for _, rg := range rgs {
		names = append(names, rg.Name)
	}
	return names
}

func fetchCivoOrgNames() []string {
	mgr := providerconfig.NewManager(getRepoPath())
	orgs, err := mgr.ListCivoOrganizations()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(orgs))
	for _, o := range orgs {
		names = append(names, o.Name)
	}
	return names
}

// ── Provider region / node lists ─────────────────────────────────────────────

func regionOptionsForProvider(provider string) []string {
	switch provider {
	case "civo":
		return []string{"PHX1", "NYC1", "FRA1", "LON1"}
	case "aws":
		return []string{
			"us-east-1", "us-east-2", "us-west-1", "us-west-2",
			"eu-west-1", "eu-west-2", "eu-central-1",
			"ap-southeast-1", "ap-southeast-2", "ap-northeast-1",
			"ca-central-1", "sa-east-1",
		}
	case "gcp":
		return []string{
			"us-central1", "us-east1", "us-east4", "us-west1", "us-west2",
			"europe-west1", "europe-west2", "europe-west3", "europe-west4",
			"asia-east1", "asia-northeast1", "asia-southeast1",
			"australia-southeast1",
		}
	case "azure":
		return []string{
			"eastus", "eastus2", "westus", "westus2", "centralus",
			"northeurope", "westeurope",
			"eastasia", "southeastasia", "japaneast",
			"australiaeast", "canadacentral", "brazilsouth",
		}
	}
	return nil
}

func nodeOptionsForProvider(provider string) []string {
	switch provider {
	case "civo":
		return []string{
			"g4s.kube.xsmall", "g4s.kube.small", "g4s.kube.medium",
			"g4s.kube.large", "g4s.kube.xlarge",
		}
	case "aws":
		return []string{
			"t3.small", "t3.medium", "t3.large", "t3.xlarge", "t3.2xlarge",
			"m5.large", "m5.xlarge", "m5.2xlarge", "m5.4xlarge",
			"c5.large", "c5.xlarge", "c5.2xlarge", "c5.4xlarge",
			"r5.large", "r5.xlarge",
		}
	case "gcp":
		return []string{
			"e2-micro", "e2-small", "e2-medium",
			"e2-standard-2", "e2-standard-4", "e2-standard-8", "e2-standard-16",
			"n2-standard-2", "n2-standard-4", "n2-standard-8", "n2-standard-16",
			"c2-standard-4", "c2-standard-8",
		}
	case "azure":
		return []string{
			"Standard_B2s", "Standard_B4ms",
			"Standard_DS2_v2", "Standard_DS3_v2", "Standard_DS4_v2",
			"Standard_D2s_v3", "Standard_D4s_v3", "Standard_D8s_v3",
			"Standard_E2s_v3", "Standard_E4s_v3",
			"Standard_F4s_v2", "Standard_F8s_v2",
		}
	}
	return nil
}

// selectOrInput shows a select with known options plus "Enter manually..." and
// "← Back". If the user picks "Enter manually..." a free-text input is shown.
// Returns errBack when the user selects back.
func selectOrInput(title, placeholder string, options []string, value *string) error {
	if len(options) == 0 {
		return newForm(
			huh.NewGroup(
				huh.NewInput().Title(title).Placeholder(placeholder).Value(value),
			),
		).Run()
	}

	const manualKey = "__manual__"
	opts := make([]huh.Option[string], 0, len(options)+2)
	for _, o := range options {
		opts = append(opts, huh.NewOption(o, o))
	}
	opts = append(opts, huh.NewOption("Enter manually...", manualKey))
	opts = append(opts, huh.NewOption("← Back", "__back__"))

	selection := ""
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(title).
				Options(opts...).
				Value(&selection),
		),
	).Run()
	if err != nil {
		return err
	}
	if selection == "__back__" {
		return errBack
	}
	if selection == manualKey {
		return newForm(
			huh.NewGroup(
				huh.NewInput().Title(title).Placeholder(placeholder).Value(value),
			),
		).Run()
	}
	*value = selection
	return nil
}

// selectOrInputOptional is like selectOrInput but prepends a "No change (keep
// current)" option that sets value to "". Used in modify flows.
func selectOrInputOptional(title string, options []string, value *string) error {
	if len(options) == 0 {
		return newForm(
			huh.NewGroup(
				huh.NewInput().Title(title + " (leave blank to keep current)").Value(value),
			),
		).Run()
	}

	const manualKey = "__manual__"
	opts := make([]huh.Option[string], 0, len(options)+3)
	opts = append(opts, huh.NewOption("No change (keep current)", ""))
	for _, o := range options {
		opts = append(opts, huh.NewOption(o, o))
	}
	opts = append(opts, huh.NewOption("Enter manually...", manualKey))
	opts = append(opts, huh.NewOption("← Back", "__back__"))

	selection := ""
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(title).
				Options(opts...).
				Value(&selection),
		),
	).Run()
	if err != nil {
		return err
	}
	if selection == "__back__" {
		return errBack
	}
	if selection == manualKey {
		return newForm(
			huh.NewGroup(
				huh.NewInput().Title(title + " (leave blank to keep current)").Value(value),
			),
		).Run()
	}
	*value = selection
	return nil
}

// ── Select helpers ───────────────────────────────────────────────────────────

// selectFromList presents a huh.Select populated with the given names plus a
// "← Back" entry. Returns errBack when the user picks back.
func selectFromList(title string, names []string, value *string) error {
	if len(names) == 0 {
		// Fall back to free-text when list is empty
		return newForm(
			huh.NewGroup(
				huh.NewInput().Title(title).Value(value),
			),
		).Run()
	}

	opts := make([]huh.Option[string], 0, len(names)+1)
	for _, n := range names {
		opts = append(opts, huh.NewOption(n, n))
	}
	opts = append(opts, huh.NewOption("← Back", "__back__"))

	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(title).
				Options(opts...).
				Value(value),
		),
	).Run()
	if err != nil {
		return err
	}
	if *value == "__back__" {
		return errBack
	}
	return nil
}
