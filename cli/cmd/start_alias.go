package cmd

import (
	"errors"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// startAlias runs `oh start` and its modes as workflows (O8, O15):
//
//	oh start [--prompt]      → oh run feature (prompt = first text input)
//	oh start --agent <id>    → oh run libre --agent <id> (prompt = request)
//	oh start --dev [-t <id>] → oh run ticket --tickets <id> (picker when -t is absent)
//	oh start --onboard       → oh run onboarding (-i refresh=true with --refresh)
//	oh start --parallel      → oh run ticket --tickets a,b,…
//	oh start --sweep <goal>  → oh run sweep (goal = -i goal, else first text input)
//	--worktree <branch>      → --location new (branch input or worktree branch)
func startAlias(cmd *cobra.Command, a *app.App) error {
	f := cmd.Flags()
	al := startAliasFor(cmd)
	devMode, _ := f.GetBool("dev")
	ticket, _ := f.GetString("ticket")
	projectID, _ := f.GetString("project")
	project, err := resolveProject(cmd.Context(), a, projectID)
	if err != nil {
		return err
	}
	if devMode {
		sel, err := handleDevMode(cmd, a, project, project.Path)
		if err != nil {
			return err
		}
		if len(sel.Tickets) == 0 {
			return errors.New(i18n.T("cmd.v1.unsupported.dev_no_ticket"))
		}
		al.Opts.Tickets = sel.Tickets
		if sel.Epic != "" && len(sel.Tickets) > 1 {
			one, err := askEpicSessions(len(sel.Tickets))
			if err != nil {
				return err
			}
			al.Opts.OneSession = one
		}
		if ticket != "" {
			al.Old += " -t " + ticket
		}
	}
	al.Opts.Project = project
	return runAlias(cmd, al)
}

// startAliasFor maps the `oh start` flags to the workflow alias
// (--parallel without tickets is refused by runStart).
func startAliasFor(cmd *cobra.Command) (al workflowAlias) {
	f := cmd.Flags()
	parallel, _ := f.GetBool("parallel")
	sweepGoal, _ := f.GetString("sweep")
	devMode, _ := f.GetBool("dev")
	onboard, _ := f.GetBool("onboard")
	refresh, _ := f.GetBool("refresh")
	recap, _ := f.GetBool("recap")
	tickets, _ := f.GetStringSlice("tickets")
	userPrompt, _ := f.GetString("prompt")
	providerFlag, _ := f.GetString("provider")
	agent, _ := f.GetString("agent")

	al = workflowAlias{Recap: recap, Opts: runOptions{Provider: providerFlag, LooseInputs: map[string]string{}}}
	if f.Changed("worktree") {
		wt, _ := f.GetString("worktree")
		al.Opts.Location, al.Opts.Branch = "new", wt
		al.Opts.LooseInputs["branch"] = wt
	}
	switch {
	case agent != "":
		al.Old, al.Workflow, al.Opts.Agent, al.Opts.Text = "oh start --agent "+agent, "libre", agent, userPrompt
	case parallel:
		al.Old, al.Workflow, al.Opts.Tickets = "oh start --parallel --tickets", "ticket", tickets
	case sweepGoal != "":
		al.Old, al.Workflow, al.Opts.Text = "oh start --sweep", "sweep", sweepGoal
		al.Opts.LooseInputs["goal"] = sweepGoal
		for flag, input := range map[string]string{"sweep-strategy": "strategy", "sweep-verify-cmd": "verify_cmd"} {
			if v, _ := f.GetString(flag); v != "" {
				al.Opts.LooseInputs[input] = v
			}
		}
		if f.Changed("sweep-verify") {
			al.Opts.LooseInputs["verify"], _ = f.GetString("sweep-verify")
		}
		for flag, input := range map[string]string{"sweep-tasks": "tasks", "sweep-include": "include", "sweep-exclude": "exclude"} {
			if v, _ := f.GetStringSlice(flag); len(v) > 0 {
				sep := ", "
				if flag == "sweep-tasks" {
					sep = "\n"
				}
				al.Opts.LooseInputs[input] = strings.Join(v, sep)
			}
		}
		if dry, _ := f.GetBool("sweep-dry-run"); dry {
			al.Opts.LooseInputs["dry_run"] = "true"
		}
	case devMode:
		al.Old, al.Workflow = "oh start --dev", "ticket"
	case onboard:
		al.Old, al.Workflow = "oh start --onboard", "onboarding"
		if refresh {
			al.Opts.LooseInputs["refresh"] = "true"
		}
	default:
		al.Old, al.Workflow, al.Opts.Text = "oh start", "feature", userPrompt
	}
	return al
}

// askEpicSessions asks how to run the tickets of an epic: one session for
// the whole epic (default, as before) or one session per ticket.
func askEpicSessions(n int) (oneSession bool, err error) {
	choice := "one"
	err = theme.NewForm(huh.NewGroup(huh.NewSelect[string]().Title(i18n.Tf("cmd.alias.epic_title", n)).
		Options(huh.NewOption(i18n.T("cmd.alias.epic_one"), "one"), huh.NewOption(i18n.Tf("cmd.alias.epic_each", n), "each")).
		Value(&choice))).Run()
	return choice == "one", err
}
