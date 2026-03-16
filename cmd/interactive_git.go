package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

func runInteractiveGit() error {
	var category string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Git — what would you like to do?").
				Options(
					huh.NewOption("Repository management", "repo"),
					huh.NewOption("Branch management", "branch"),
					huh.NewOption("Pull / push / sync", "sync"),
				).
				Value(&category),
		),
	).Run()
	if err != nil {
		return err
	}

	switch category {
	case "repo":
		return interactiveGitRepo()
	case "branch":
		return interactiveGitBranch()
	case "sync":
		return interactiveGitSync()
	}
	return nil
}

// ── Repository ───────────────────────────────────────────────────────────────

func interactiveGitRepo() error {
	var action string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Repository — action").
				Options(
					huh.NewOption("Add repository", "add"),
					huh.NewOption("List repositories", "list"),
					huh.NewOption("Use (switch to) repository", "use"),
					huh.NewOption("Show status", "status"),
					huh.NewOption("Remove repository", "remove"),
				).
				Value(&action),
		),
	).Run()
	if err != nil {
		return err
	}

	switch action {
	case "add":
		return interactiveGitRepoAdd()
	case "list":
		listGitRepositories()
	case "use":
		return interactiveGitRepoUse()
	case "status":
		showGitStatus()
	case "remove":
		return interactiveGitRepoRemove()
	}
	return nil
}

func interactiveGitRepoAdd() error {
	var (
		name       string
		repoURL    string
		username   string
		setCurrent bool
	)

	err := newForm(
		huh.NewGroup(
			huh.NewInput().Title("Repository alias").Placeholder("production").Value(&name),
			huh.NewInput().Title("Repository URL").Placeholder("https://github.com/org/hyve-state.git").Value(&repoURL),
			huh.NewInput().Title("Git username (optional)").Value(&username),
			huh.NewConfirm().
				Title("Set as current active repository?").
				Affirmative("Yes").
				Negative("No").
				Value(&setCurrent),
		),
	).Run()
	if err != nil {
		return err
	}

	addGitRepository(name, repoURL, username, setCurrent)
	return nil
}

func interactiveGitRepoUse() error {
	var name string
	err := newForm(
		huh.NewGroup(
			huh.NewInput().Title("Repository alias to switch to").Value(&name),
		),
	).Run()
	if err != nil {
		return err
	}
	switchToRepository(name)
	return nil
}

func interactiveGitRepoRemove() error {
	var name string
	err := newForm(
		huh.NewGroup(
			huh.NewInput().Title("Repository alias to remove").Value(&name),
		),
	).Run()
	if err != nil {
		return err
	}

	var confirm bool
	err = newForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Remove repository '%s'?", name)).
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

	removeGitRepository(name)
	return nil
}

// ── Branch ───────────────────────────────────────────────────────────────────

func interactiveGitBranch() error {
	var action string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Branch — action").
				Options(
					huh.NewOption("List branches", "list"),
					huh.NewOption("Create branch", "create"),
					huh.NewOption("Switch branch", "switch"),
					huh.NewOption("Delete branch", "delete"),
				).
				Value(&action),
		),
	).Run()
	if err != nil {
		return err
	}

	switch action {
	case "list":
		listGitBranches()
	case "create":
		return interactiveGitBranchCreate()
	case "switch":
		return interactiveGitBranchSwitch()
	case "delete":
		return interactiveGitBranchDelete()
	}
	return nil
}

func interactiveGitBranchCreate() error {
	var (
		branchName   string
		switchBranch bool
		push         bool
	)

	err := newForm(
		huh.NewGroup(
			huh.NewInput().Title("New branch name").Placeholder("feature/my-feature").Value(&branchName),
			huh.NewConfirm().
				Title("Switch to new branch after creating?").
				Affirmative("Yes").
				Negative("No").
				Value(&switchBranch),
			huh.NewConfirm().
				Title("Push to remote?").
				Affirmative("Yes").
				Negative("No").
				Value(&push),
		),
	).Run()
	if err != nil {
		return err
	}

	createGitBranch(branchName, switchBranch, push)
	return nil
}

func interactiveGitBranchSwitch() error {
	var (
		branchName string
		pull       bool
	)

	err := newForm(
		huh.NewGroup(
			huh.NewInput().Title("Branch name to switch to").Value(&branchName),
			huh.NewConfirm().
				Title("Pull latest changes after switching?").
				Affirmative("Yes").
				Negative("No").
				Value(&pull),
		),
	).Run()
	if err != nil {
		return err
	}

	switchGitBranch(branchName, pull)
	return nil
}

func interactiveGitBranchDelete() error {
	var (
		branchName string
		force      bool
	)

	err := newForm(
		huh.NewGroup(
			huh.NewInput().Title("Branch name to delete").Value(&branchName),
			huh.NewConfirm().
				Title("Force delete (even if not merged)?").
				Affirmative("Force delete").
				Negative("Safe delete").
				Value(&force),
		),
	).Run()
	if err != nil {
		return err
	}

	var confirm bool
	err = newForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Delete branch '%s'?", branchName)).
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

	deleteGitBranch(branchName, force)
	return nil
}

// ── Sync ─────────────────────────────────────────────────────────────────────

func interactiveGitSync() error {
	var action string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Sync — action").
				Options(
					huh.NewOption("Pull latest changes", "pull"),
					huh.NewOption("Push changes", "push"),
					huh.NewOption("Sync (pull then push)", "sync"),
				).
				Value(&action),
		),
	).Run()
	if err != nil {
		return err
	}

	switch action {
	case "pull":
		pullGitChanges()
	case "push":
		return interactiveGitPush()
	case "sync":
		return interactiveGitSyncChanges()
	}
	return nil
}

func interactiveGitPush() error {
	var message string
	err := newForm(
		huh.NewGroup(
			huh.NewInput().Title("Commit message").Placeholder("Update cluster config").Value(&message),
		),
	).Run()
	if err != nil {
		return err
	}
	pushGitChanges(message)
	return nil
}

func interactiveGitSyncChanges() error {
	var message string
	err := newForm(
		huh.NewGroup(
			huh.NewInput().Title("Commit message").Placeholder("Update cluster config").Value(&message),
		),
	).Run()
	if err != nil {
		return err
	}
	syncGitChanges(message)
	return nil
}
