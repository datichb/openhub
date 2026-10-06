package cmd

import (
	"context"
	"log/slog"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Session launchers — inline prompts au lieu de modal overlays
// ─────────────────────────────────────────────────────────────────────────────

// actionOpencode returns a menu action callback that launches opencode
// via the unified launcher pipeline. Used by the "coder" omnibar command
// and the Hub Home session links.
func actionOpencode(agent, sessionPrompt string) func() {
	return func() {
		if tuiShell == nil {
			return
		}
		launchSessionWithPrompt(agent, sessionPrompt)
	}
}

// launchSessionWithPrompt is the common session launch logic with SuspendAndExec.
// It passes only agent and prompt to opencode — never oh-specific flags.
func launchSessionWithPrompt(agent, sessionPrompt string) {
	if tuiShell == nil {
		return
	}

	if _, err := opencode.FindBinary(); err != nil {
		tuiShell.ShowToast(i18n.T("tui.session.opencode_not_found"), shell.ToastError)
		return
	}

	a := MustApp()

	// Same default everywhere (B7): free session = orchestrator.
	if agent == "" {
		agent = "orchestrator"
	}

	// If no active project, try to resolve one — or prompt the user to choose.
	project, err := resolveActiveProject(a)
	if err != nil {
		// No project could be resolved — check if there are multiple projects
		// and offer a selector (ADR-032 Phase 3: dynamic project selection in team mode).
		projects, _ := a.Projects.List(context.Background(), domain.ProjectStatusActive)
		if len(projects) == 0 {
			tuiShell.ShowToast(i18n.T("tui.session.no_project_configured"), shell.ToastWarning)
			return
		}
		if len(projects) == 1 {
			project = &projects[0]
		} else {
			// Multiple projects — show selector, then launch
			opts := make([]views.SelectOption, len(projects))
			for i, p := range projects {
				opts[i] = views.SelectOption{Label: p.Name, Value: p.ID}
			}
			tuiShell.ShowSelectModal(i18n.T("tui.session.choose_project"), opts, "", func(selected string) {
				for i := range projects {
					if projects[i].ID == selected {
						launchSessionForProject(a, &projects[i], agent, sessionPrompt)
						return
					}
				}
			})
			return
		}
	}

	launchSessionForProject(a, project, agent, sessionPrompt)
}

// launchSessionForProject launches an opencode session on the given project
// via the unified launcher pipeline (session tracking, credentials, team events).
func launchSessionForProject(a *app.App, project *domain.Project, agent, sessionPrompt string) {
	runTUILaunch(a, launcher.LaunchOpts{
		ProjectID:   project.ID,
		ProjectPath: project.Path,
		Agent:       agent,
		Prompt:      sessionPrompt,
		SkipSummary: true,
		SkipConfirm: true,
		SkipDeploy:  true,
	})
}

// runTUILaunch is the single TUI entry point for launching a session (B6).
// The launch runs off the event loop (bundle compilation and server start take
// a few seconds); every UI call it makes is marshalled back onto the event
// loop, including the terminal suspension of the legacy/inline client.
func runTUILaunch(a *app.App, opts launcher.LaunchOpts) {
	if tuiShell == nil {
		return
	}
	sh := tuiShell
	ui := launcher.NewTUIUI(
		func(fn func() error) error {
			done := make(chan error, 1)
			sh.App().QueueUpdateDraw(func() { done <- sh.SuspendAndExec(fn) })
			return <-done
		},
		func(msg string, ok bool) {
			sh.App().QueueUpdateDraw(func() { sh.ShowToastMsg(msg, ok) })
		},
	)
	go func() {
		if err := launcher.New(a, ui).Launch(context.Background(), opts); err != nil {
			slog.Warn("TUI session ended with error", "error", err)
			sh.App().QueueUpdateDraw(func() { sh.ShowToastMsg(err.Error(), false) })
		}
	}()
}
