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
