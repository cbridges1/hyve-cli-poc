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

// optionGroup holds a named group of select options for two-level menus.
type optionGroup struct {
	Name    string
	Options []huh.Option[string]
}

// selectFromGroups shows a two-level select: first the group name, then the
// items inside that group. "← Back to categories" at the item level loops back
// to the group list. "Enter manually..." at the group level falls through to a
// free-text input. "← Back" at the group level returns errBack.
func selectFromGroups(title string, groups []optionGroup, placeholder string, value *string) error {
	const manualKey = "__manual__"
	const backKey = "__back__"
	for {
		groupOpts := make([]huh.Option[string], 0, len(groups)+2)
		groupOpts = append(groupOpts, huh.NewOption("Enter manually...", manualKey))
		for _, g := range groups {
			groupOpts = append(groupOpts, huh.NewOption(g.Name, g.Name))
		}
		groupOpts = append(groupOpts, huh.NewOption("← Back", backKey))

		selectedGroup := ""
		if err := newForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(title + " — select category").
					Options(groupOpts...).
					Value(&selectedGroup),
			),
		).Run(); err != nil {
			return err
		}
		if selectedGroup == backKey {
			return errBack
		}
		if selectedGroup == manualKey {
			return newForm(
				huh.NewGroup(
					huh.NewInput().Title(title).Placeholder(placeholder).Value(value),
				),
			).Run()
		}

		// Find the chosen group's items
		var items []huh.Option[string]
		for _, g := range groups {
			if g.Name == selectedGroup {
				items = g.Options
				break
			}
		}
		itemOpts := make([]huh.Option[string], 0, len(items)+1)
		itemOpts = append(itemOpts, items...)
		itemOpts = append(itemOpts, huh.NewOption("← Back to categories", backKey))

		selection := ""
		if err := newForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(title + " — " + selectedGroup).
					Options(itemOpts...).
					Value(&selection),
			),
		).Run(); err != nil {
			return err
		}
		if selection == backKey {
			continue // re-show group list
		}
		*value = selection
		return nil
	}
}

// selectFromGroupsOptional is like selectFromGroups but prepends a
// "No change (keep current)" option at the group level. Used in modify flows.
func selectFromGroupsOptional(title string, groups []optionGroup, value *string) error {
	const manualKey = "__manual__"
	const backKey = "__back__"
	const noChangeKey = "__nochange__"
	for {
		groupOpts := make([]huh.Option[string], 0, len(groups)+3)
		groupOpts = append(groupOpts, huh.NewOption("Enter manually...", manualKey))
		groupOpts = append(groupOpts, huh.NewOption("No change (keep current)", noChangeKey))
		for _, g := range groups {
			groupOpts = append(groupOpts, huh.NewOption(g.Name, g.Name))
		}
		groupOpts = append(groupOpts, huh.NewOption("← Back", backKey))

		selectedGroup := ""
		if err := newForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(title + " — select category").
					Options(groupOpts...).
					Value(&selectedGroup),
			),
		).Run(); err != nil {
			return err
		}
		if selectedGroup == backKey {
			return errBack
		}
		if selectedGroup == noChangeKey {
			*value = ""
			return nil
		}
		if selectedGroup == manualKey {
			return newForm(
				huh.NewGroup(
					huh.NewInput().Title(title + " (leave blank to keep current)").Value(value),
				),
			).Run()
		}

		var items []huh.Option[string]
		for _, g := range groups {
			if g.Name == selectedGroup {
				items = g.Options
				break
			}
		}
		itemOpts := make([]huh.Option[string], 0, len(items)+1)
		itemOpts = append(itemOpts, items...)
		itemOpts = append(itemOpts, huh.NewOption("← Back to categories", backKey))

		selection := ""
		if err := newForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(title + " — " + selectedGroup).
					Options(itemOpts...).
					Value(&selection),
			),
		).Run(); err != nil {
			return err
		}
		if selection == backKey {
			continue
		}
		*value = selection
		return nil
	}
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
