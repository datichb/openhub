package cmd

import (
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
)

// startAlias runs `oh start` and its modes as workflows (O8, O15):
//
//	oh start [--prompt]      → oh run feature (prompt = first text input)
//	oh start --dev [-t <id>] → oh run ticket --tickets <id> (picker when -t is absent)
//	oh start --onboard       → oh run onboarding (-i refresh=true with --refresh)
//	oh start --parallel      → oh run ticket --tickets a,b,…
//	oh start --sweep <goal>  → oh run sweep (goal = -i goal, else first text input)
//	--worktree <branch>      → --location new (branch input or worktree branch)
//
// An explicit --agent, a missing workflow or opencode V1 keep the former
// launch. When the dev picker selects an epic, the former launch runs with
// that selection (one session for the epic), returned in dev. project is the
// resolved project when the alias resolved it (reused by the former launch).
func startAlias(cmd *cobra.Command, a *app.App) (handled bool, project *domain.Project, dev *devSelection, err error) {
	f := cmd.Flags()
	if agent, _ := f.GetString("agent"); agent != "" {
		return false, nil, nil, nil
	}
	al, ok := startAliasFor(cmd)
	if !ok {
		return false, nil, nil, nil
	}
	devMode, _ := f.GetBool("dev")
	ticket, _ := f.GetString("ticket")
	if !aliasAvailable(cmd.Context(), al.Workflow) {
		return false, nil, nil, nil
	}
	projectID, _ := f.GetString("project")
	if project, err = resolveProject(cmd.Context(), a, projectID); err != nil {
		return true, nil, nil, err
	}
	if devMode {
		sel, err := handleDevMode(cmd, a, project, project.Path)
		if err != nil {
			return true, project, nil, err
		}
		if sel.Epic != "" || len(sel.Tickets) != 1 {
			return false, project, &sel, nil
		}
		al.Opts.Tickets = sel.Tickets
		if ticket != "" {
			al.Old += " -t " + ticket
		}
	}
	al.Opts.Project = project
	return true, project, nil, runAlias(cmd, al)
}

// startAliasFor maps the `oh start` flags to the workflow alias (ok is
// false when the former launch must run: --parallel without tickets).
func startAliasFor(cmd *cobra.Command) (al workflowAlias, ok bool) {
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

	al = workflowAlias{Recap: recap, Opts: runOptions{Provider: providerFlag, LooseInputs: map[string]string{}}}
	if f.Changed("worktree") {
		wt, _ := f.GetString("worktree")
		al.Opts.Location, al.Opts.Branch = "new", wt
		al.Opts.LooseInputs["branch"] = wt
	}
	switch {
	case parallel:
		if len(tickets) == 0 {
			return al, false
		}
		al.Old, al.Workflow, al.Opts.Tickets = "oh start --parallel --tickets", "ticket", tickets
	case sweepGoal != "":
		al.Old, al.Workflow, al.Opts.Text = "oh start --sweep", "sweep", sweepGoal
		al.Opts.LooseInputs["goal"] = sweepGoal
		if s, _ := f.GetString("sweep-strategy"); s != "" {
			al.Opts.LooseInputs["strategy"] = s
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
	return al, true
}
