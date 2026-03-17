package cmd

import (
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"hyve/cmd/cfg"
	"hyve/cmd/cluster"
	gitpkg "hyve/cmd/git"
	"hyve/cmd/kube"
	"hyve/cmd/shared"
	"hyve/cmd/tpl"
	"hyve/cmd/wf"
)

var interactiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Launch the interactive TUI",
	Long:  "Navigate and run any Hyve command through a guided terminal user interface.",
	RunE: func(cmd *cobra.Command, args []string) error {
		for {
			var section string
			err := shared.NewForm(
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
				runErr = cluster.RunInteractive()
			case "git":
				runErr = gitpkg.RunInteractive()
			case "config":
				runErr = cfg.RunInteractive()
			case "workflow":
				runErr = wf.RunInteractive()
			case "template":
				runErr = tpl.RunInteractive()
			case "kubeconfig":
				runErr = kube.RunInteractive()
			}
			// ErrBack from a top-level section just returns to this menu
			if runErr == huh.ErrUserAborted {
				return nil
			}
			if runErr != nil && runErr != shared.ErrBack {
				return runErr
			}
		}
	},
}

func init() {
	interactiveCmd.Aliases = []string{"tui"}
}
