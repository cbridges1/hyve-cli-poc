package cmd

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// hyveTheme returns a yellow-and-blue huh theme for all interactive forms.
func hyveTheme() *huh.Theme {
	t := huh.ThemeBase()

	var (
		yellow   = lipgloss.Color("#F5C518")
		blue     = lipgloss.Color("#4A9FD5")
		blueDark = lipgloss.Color("#1E6FA8")
		muted    = lipgloss.Color("#6B7280")
		white    = lipgloss.Color("#F9FAFB")
		red      = lipgloss.Color("#EF4444")
	)

	// Focused field styles
	t.Focused.Base = t.Focused.Base.BorderForeground(blue)
	t.Focused.Card = t.Focused.Base

	t.Focused.Title = t.Focused.Title.Foreground(yellow).Bold(true)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(yellow).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(muted)

	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(red)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(red)

	// Select / multi-select
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(yellow)
	t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(yellow)
	t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(yellow)
	t.Focused.Option = t.Focused.Option.Foreground(white)
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(yellow)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(blue)
	t.Focused.SelectedPrefix = lipgloss.NewStyle().Foreground(blue).SetString("[✓] ")
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(white)
	t.Focused.UnselectedPrefix = lipgloss.NewStyle().Foreground(muted).SetString("[ ] ")

	// Buttons
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(yellow).Background(blueDark).Bold(true)
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(muted).Background(lipgloss.Color("#1F2937"))

	// Text input
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(yellow)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(muted)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(blue)
	t.Focused.TextInput.Text = t.Focused.TextInput.Text.Foreground(white)

	// Blurred inherits focused then overrides border
	t.Blurred = t.Focused
	t.Blurred.Base = t.Blurred.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.Card = t.Blurred.Base
	t.Blurred.NextIndicator = lipgloss.NewStyle()
	t.Blurred.PrevIndicator = lipgloss.NewStyle()

	t.Group.Title = t.Focused.Title
	t.Group.Description = t.Focused.Description

	return t
}

// newForm wraps huh.NewForm and applies the Hyve theme automatically.
func newForm(groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).WithTheme(hyveTheme())
}

var interactiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Launch the interactive TUI",
	Long:  "Navigate and run any Hyve command through a guided terminal user interface.",
	RunE: func(cmd *cobra.Command, args []string) error {
		for {
			var section string
			err := newForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Hyve — what would you like to do?").
						Options(
							huh.NewOption("cluster    — manage Kubernetes clusters", "cluster"),
							huh.NewOption("git        — manage Git repositories", "git"),
							huh.NewOption("config     — provider credentials & config", "config"),
							huh.NewOption("workflow   — automated pipelines", "workflow"),
							huh.NewOption("template   — reusable cluster patterns", "template"),
							huh.NewOption("kubeconfig — cluster access", "kubeconfig"),
							huh.NewOption("Quit", "quit"),
						).
						Value(&section),
				),
			).Run()
			if err == huh.ErrUserAborted {
				return nil
			}
			if err != nil {
				return err
			}

			if section == "quit" {
				return nil
			}

			var runErr error
			switch section {
			case "cluster":
				runErr = runInteractiveCluster()
			case "git":
				runErr = runInteractiveGit()
			case "config":
				runErr = runInteractiveConfig()
			case "workflow":
				runErr = runInteractiveWorkflow()
			case "template":
				runErr = runInteractiveTemplate()
			case "kubeconfig":
				runErr = runInteractiveKubeconfig()
			}
			// errBack from a top-level section just returns to this menu
			if runErr == huh.ErrUserAborted {
				return nil
			}
			if runErr != nil && runErr != errBack {
				return runErr
			}
		}
	},
}

func init() {
	interactiveCmd.Aliases = []string{"tui"}
}
