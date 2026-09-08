package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Session launchers — inline prompts au lieu de modal overlays
// ─────────────────────────────────────────────────────────────────────────────

// actionOpencode returns a menu action callback that suspends the TUI and launches opencode.
func actionOpencode(agent, extraArg string) func() {
	return func() {
		if tuiShell == nil {
			return
		}

		// Pre-flight: check binary exists
		if _, err := opencode.FindBinary(); err != nil {
			tuiShell.ShowToast("opencode non trouvé", shell.ToastError)
			return
		}

		a := MustApp()
		project, err := resolveActiveProject(a)
		if err != nil {
			tuiShell.ShowToast("Aucun projet actif", shell.ToastWarning)
			return
		}

		opts := opencode.StartOpts{
			ProjectPath: project.Path,
			ProjectID:   project.ID,
			Agent:       agent,
		}
		if extraArg != "" {
			opts.ExtraArgs = []string{extraArg}
		}

		resolveProviderCreds(a, project, &opts)

		err = tuiShell.SuspendAndExec(func() error {
			return opencode.Run(opts)
		})
		if err != nil {
			slog.Warn("opencode session ended with error", "error", err)
			tuiShell.ShowToast(fmt.Sprintf("Session: %s", err), shell.ToastWarning)
		} else {
			tuiShell.ShowToast("Session terminée", shell.ToastSuccess)
		}
	}
}

func actionStartLauncher() {
	if tuiShell == nil {
		return
	}
	tuiShell.ShowSessionLauncher(shell.SessionLaunchConfig{
		Title: "Lancer une session",
		Options: []shell.SessionOption{
			{Label: "Standard", Description: "Session interactive classique", Agent: ""},
			{Label: "Dev (ticket)", Description: "Session orientée développement", Agent: "", ExtraArgs: []string{"--dev"}},
			{Label: "Onboard", Description: "Session d'onboarding projet", Agent: "", ExtraArgs: []string{"--onboard"}},
		},
		OnLaunch: func(opt shell.SessionOption) {
			launchOpencode(opt.Agent, opt.ExtraArgs...)
		},
	})
}

func actionAuditLauncher() {
	if tuiShell == nil {
		return
	}
	tuiShell.ShowSessionLauncher(shell.SessionLaunchConfig{
		Title: "Lancer un audit",
		Options: []shell.SessionOption{
			{Label: "Sécurité", Description: "Audit de sécurité (OWASP, injections, auth)", Agent: "auditor", ExtraArgs: []string{"--type", "security"}},
			{Label: "Performance", Description: "Audit de performance (N+1, mémoire, CPU)", Agent: "auditor", ExtraArgs: []string{"--type", "performance"}},
			{Label: "Architecture", Description: "Audit d'architecture (couplage, patterns)", Agent: "auditor", ExtraArgs: []string{"--type", "architecture"}},
			{Label: "Accessibilité", Description: "Audit a11y (WCAG, ARIA, contraste)", Agent: "auditor", ExtraArgs: []string{"--type", "accessibility"}},
			{Label: "Éco-conception", Description: "Audit impact environnemental", Agent: "auditor", ExtraArgs: []string{"--type", "ecodesign"}},
			{Label: "Observabilité", Description: "Audit logs, traces, métriques", Agent: "auditor", ExtraArgs: []string{"--type", "observability"}},
		},
		OnLaunch: func(opt shell.SessionOption) {
			launchOpencode(opt.Agent, opt.ExtraArgs...)
		},
	})
}

func actionReviewLauncher() {
	if tuiShell == nil {
		return
	}

	options := []shell.SessionOption{
		{Label: "Standard", Description: "Code review classique", Agent: "reviewer"},
		{Label: "Adversarial", Description: "Review adversariale (trouver les failles)", Agent: "reviewer", ExtraArgs: []string{"--mode", "adversarial"}},
		{Label: "Edge cases", Description: "Review orientée cas limites", Agent: "reviewer", ExtraArgs: []string{"--mode", "edge-case"}},
		{Label: "Complète", Description: "Review complète (tous les modes)", Agent: "reviewer", ExtraArgs: []string{"--mode", "all"}},
	}

	a := MustApp()
	if a.Config.MCP.Gitlab.WriteEnabled {
		options = append(options, shell.SessionOption{
			Label: "Publish", Description: "Publier pour review (crée MR + notifie l'équipe)", Agent: "reviewer", ExtraArgs: []string{"--publish"},
		})
	}

	tuiShell.ShowSessionLauncher(shell.SessionLaunchConfig{
		Title:   "Lancer une review",
		Options: options,
		OnLaunch: func(opt shell.SessionOption) {
			launchOpencode(opt.Agent, opt.ExtraArgs...)
		},
	})
}

func actionDebugLauncher() {
	if tuiShell == nil {
		return
	}
	tuiShell.ShowInputModal("Description du problème", "", func(issue string) {
		args := []string{}
		if issue != "" {
			args = append(args, "--issue", issue)
		}
		launchOpencode("debugger", args...)
	})
}

// launchOpencode is the common session launch logic with SuspendAndExec.
func launchOpencode(agent string, extraArgs ...string) {
	if tuiShell == nil {
		return
	}

	if _, err := opencode.FindBinary(); err != nil {
		tuiShell.ShowToast("opencode non trouvé", shell.ToastError)
		return
	}

	a := MustApp()

	// If no active project, try to resolve one — or prompt the user to choose.
	project, err := resolveActiveProject(a)
	if err != nil {
		// No project could be resolved — check if there are multiple projects
		// and offer a selector (ADR-032 Phase 3: dynamic project selection in team mode).
		projects, _ := a.Projects.List(context.Background(), domain.ProjectStatusActive)
		if len(projects) == 0 {
			tuiShell.ShowToast("Aucun projet configuré", shell.ToastWarning)
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
			tuiShell.ShowSelectModal("Choisir un projet", opts, "", func(selected string) {
				for i := range projects {
					if projects[i].ID == selected {
						launchOpcodeForProject(a, &projects[i], agent, extraArgs...)
						return
					}
				}
			})
			return
		}
	}

	launchOpcodeForProject(a, project, agent, extraArgs...)
}

// launchOpcodeForProject launches an opencode session on the given project.
func launchOpcodeForProject(a *app.App, project *domain.Project, agent string, extraArgs ...string) {
	opts := opencode.StartOpts{
		ProjectPath: project.Path,
		ProjectID:   project.ID,
		Agent:       agent,
		ExtraArgs:   extraArgs,
	}
	resolveProviderCreds(a, project, &opts)

	err := tuiShell.SuspendAndExec(func() error {
		return opencode.Run(opts)
	})
	if err != nil {
		tuiShell.ShowToast("Session terminée avec erreur", shell.ToastWarning)
	} else {
		tuiShell.ShowToast("Session terminée", shell.ToastSuccess)
	}
}

// launchOpcodeAtPath launches an opencode session at an arbitrary filesystem path.
// Used by board quick actions where the path may be a worktree, not the project base.
// The projectID is used for credential resolution — the project itself is unchanged.
func launchOpcodeAtPath(launchPath, projectID, agent string, extraArgs ...string) {
	if tuiShell == nil {
		return
	}

	if _, err := opencode.FindBinary(); err != nil {
		tuiShell.ShowToast("opencode non trouvé", shell.ToastError)
		return
	}

	a := MustApp()

	// Resolve the project from its ID for credential lookup.
	project, err := a.Projects.Get(context.Background(), projectID)
	if err != nil {
		tuiShell.ShowToast("Projet introuvable", shell.ToastWarning)
		return
	}

	opts := opencode.StartOpts{
		ProjectPath: launchPath, // worktree or base — NOT necessarily project.Path
		ProjectID:   project.ID,
		Agent:       agent,
		ExtraArgs:   extraArgs,
	}
	resolveProviderCreds(a, project, &opts)

	err = tuiShell.SuspendAndExec(func() error {
		return opencode.Run(opts)
	})
	if err != nil {
		slog.Warn("quick-action session ended with error", "error", err, "path", launchPath)
		tuiShell.ShowToast("Session terminée avec erreur", shell.ToastWarning)
	} else {
		tuiShell.ShowToast("Session terminée", shell.ToastSuccess)
	}
}
