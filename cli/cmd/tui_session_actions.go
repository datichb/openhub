package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/charmbracelet/huh"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/prompt"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Audit type descriptions — mirrors audit_review_debug.go validTypes map.
// ─────────────────────────────────────────────────────────────────────────────

var auditTypeDescriptions = map[string]string{
	"security":      "sécurité (vulnérabilités, injections, secrets, dépendances)",
	"performance":   "performance (fuites mémoire, N+1, rendering, bundle size)",
	"architecture":  "architecture (couplage, cohésion, patterns, dette technique)",
	"accessibility": "accessibilité (WCAG, ARIA, contraste, navigation clavier)",
	"ecodesign":     "éco-conception (empreinte carbone, poids, requêtes inutiles)",
	"observability": "observabilité (logs, traces, métriques, alerting, SLI/SLO)",
	"privacy":       "vie privée (RGPD, données personnelles, consentement, rétention)",
}

// buildAuditPrompt constructs the --prompt value for an audit session.
func buildAuditPrompt(auditType string) string {
	desc := auditTypeDescriptions[auditType]
	if desc == "" {
		desc = auditType
	}
	return i18n.Tf("cmd.audit.prompt", auditType, desc)
}

// buildReviewPrompt constructs the --prompt value for a review session.
func buildReviewPrompt(mode string) string {
	base := i18n.T("cmd.review.prompt")
	if mode == "" {
		return base
	}
	return "[MODE:" + mode + "] " + base
}

// buildDebugPrompt constructs the --prompt value for a debug session.
func buildDebugPrompt(issue string) string {
	if issue == "" {
		return i18n.T("cmd.debug.prompt_default")
	}
	return i18n.Tf("cmd.debug.prompt", issue)
}

// ─────────────────────────────────────────────────────────────────────────────
// translateOhFlags converts oh-specific CLI flags (passed as extraArgs by views)
// into opencode-compatible (agent, prompt) pairs.
//
// This is the central translation layer that prevents oh flags like --dev,
// --type, --mode, --issue from being forwarded to the opencode binary
// which does not understand them.
// ─────────────────────────────────────────────────────────────────────────────

func translateOhFlags(agent string, extraArgs ...string) (resolvedAgent, resolvedPrompt string) {
	resolvedAgent = agent

	for i := 0; i < len(extraArgs); i++ {
		switch extraArgs[i] {
		case "--dev":
			// Dev mode requires interactive ticket picker — caller must handle.
			// Return a sentinel (agent=orchestrator-dev, prompt="") so caller knows.
			return "orchestrator-dev", ""
		case "--onboard":
			return "onboarder", buildOnboardPromptForTUI()
		case "--quick":
			// Quick = plain session, no special flags needed.
			return agent, ""
		case "--type":
			if i+1 < len(extraArgs) {
				auditType := extraArgs[i+1]
				return "auditor", buildAuditPrompt(auditType)
			}
		case "--mode":
			if i+1 < len(extraArgs) {
				mode := extraArgs[i+1]
				return "reviewer", buildReviewPrompt(mode)
			}
		case "--issue":
			if i+1 < len(extraArgs) {
				issue := extraArgs[i+1]
				return "debugger", buildDebugPrompt(issue)
			}
		case "--publish":
			return "reviewer", "[PUBLISH] " + buildReviewPrompt("")
		}
	}

	return resolvedAgent, ""
}

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

func actionAuditLauncher() {
	if tuiShell == nil {
		return
	}
	tuiShell.ShowSessionLauncher(shell.SessionLaunchConfig{
		Title: "Lancer un audit",
		Options: []shell.SessionOption{
			{Label: "Sécurité", Description: "Audit de sécurité (OWASP, injections, auth)", Agent: "auditor"},
			{Label: "Performance", Description: "Audit de performance (N+1, mémoire, CPU)", Agent: "auditor"},
			{Label: "Architecture", Description: "Audit d'architecture (couplage, patterns)", Agent: "auditor"},
			{Label: "Accessibilité", Description: "Audit a11y (WCAG, ARIA, contraste)", Agent: "auditor"},
			{Label: "Éco-conception", Description: "Audit impact environnemental", Agent: "auditor"},
			{Label: "Observabilité", Description: "Audit logs, traces, métriques", Agent: "auditor"},
		},
		OnLaunch: func(opt shell.SessionOption) {
			// Map label → audit type
			auditTypeByLabel := map[string]string{
				"Sécurité":       "security",
				"Performance":    "performance",
				"Architecture":   "architecture",
				"Accessibilité":  "accessibility",
				"Éco-conception": "ecodesign",
				"Observabilité":  "observability",
			}
			auditType := auditTypeByLabel[opt.Label]
			if auditType == "" {
				auditType = "security"
			}
			launchSessionWithPrompt("auditor", buildAuditPrompt(auditType))
		},
	})
}

func actionReviewLauncher() {
	if tuiShell == nil {
		return
	}

	options := []shell.SessionOption{
		{Label: "Standard", Description: "Code review classique", Agent: "reviewer"},
		{Label: "Adversarial", Description: "Review adversariale (trouver les failles)", Agent: "reviewer"},
		{Label: "Edge cases", Description: "Review orientée cas limites", Agent: "reviewer"},
		{Label: "Complète", Description: "Review complète (tous les modes)", Agent: "reviewer"},
	}

	a := MustApp()
	if a.Config.MCP.Gitlab.WriteEnabled {
		options = append(options, shell.SessionOption{
			Label: "Publish", Description: "Publier pour review (crée MR + notifie l'équipe)", Agent: "reviewer",
		})
	}

	tuiShell.ShowSessionLauncher(shell.SessionLaunchConfig{
		Title:   "Lancer une review",
		Options: options,
		OnLaunch: func(opt shell.SessionOption) {
			// Map label → review mode
			modeByLabel := map[string]string{
				"Standard":    "",
				"Adversarial": "adversarial",
				"Edge cases":  "edge-case",
				"Complète":    "all",
			}
			if opt.Label == "Publish" {
				// Publish is a special workflow — not a standard session launch.
				// Delegate to the review publish logic (same as oh review --publish).
				launchSessionWithPrompt("reviewer", "[PUBLISH] "+buildReviewPrompt(""))
				return
			}
			mode := modeByLabel[opt.Label]
			launchSessionWithPrompt("reviewer", buildReviewPrompt(mode))
		},
	})
}

func actionDebugLauncher() {
	if tuiShell == nil {
		return
	}
	tuiShell.ShowInputModal("Description du problème", "", func(issue string) {
		launchSessionWithPrompt("debugger", buildDebugPrompt(issue))
	})
}

// launchSessionWithPrompt is the common session launch logic with SuspendAndExec.
// It passes only agent and prompt to opencode — never oh-specific flags.
func launchSessionWithPrompt(agent, sessionPrompt string) {
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
	l := launcher.New(a, launcher.NewTUIUI(tuiShell.SuspendAndExec, tuiShell.ShowToastMsg))
	err := l.Launch(context.Background(), launcher.LaunchOpts{
		ProjectID:   project.ID,
		ProjectPath: project.Path,
		Agent:       agent,
		Prompt:      sessionPrompt,
		SkipSummary: true,
		SkipConfirm: true,
		SkipDeploy:  true, // TUI auto-deploy is handled at project mode entry
	})
	if err != nil {
		slog.Warn("TUI session ended with error", "error", err)
	}
}

// launchSessionAtPath launches an opencode session at an arbitrary filesystem path.
// Used by board quick actions where the path may be a worktree, not the project base.
// The projectID is used for credential resolution — the project itself is unchanged.
func launchSessionAtPath(launchPath, projectID, agent, sessionPrompt string) {
	if tuiShell == nil {
		return
	}

	if _, err := opencode.FindBinary(); err != nil {
		tuiShell.ShowToast("opencode non trouvé", shell.ToastError)
		return
	}

	a := MustApp()

	l := launcher.New(a, launcher.NewTUIUI(tuiShell.SuspendAndExec, tuiShell.ShowToastMsg))
	err := l.Launch(context.Background(), launcher.LaunchOpts{
		ProjectID:   projectID,
		ProjectPath: launchPath,
		Agent:       agent,
		Prompt:      sessionPrompt,
		SkipSummary: true,
		SkipConfirm: true,
		SkipDeploy:  true, // board path: deploy is handled upstream
	})
	if err != nil {
		slog.Warn("quick-action session ended with error", "error", err, "path", launchPath)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Dev mode TUI — mirrors handleDevMode from start_dev_mode.go but adapted
// for the TUI context (no cobra.Command, uses SuspendAndExec for the picker).
// ─────────────────────────────────────────────────────────────────────────────

// launchDevSession handles the --dev flow from the TUI:
// resolve project, run the dev mode handler (ticket picker), then launch opencode.
func launchDevSession() {
	if tuiShell == nil {
		return
	}

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

	// The dev mode handler uses huh forms (interactive terminal picker).
	// We need to run it inside SuspendAndExec so the TUI is suspended.
	var devAgent, devPrompt string
	var devErr error

	err = tuiShell.SuspendAndExec(func() error {
		devAgent, devPrompt, devErr = handleDevModeTUI(a, project)
		if devErr != nil {
			return devErr
		}
		// Already inside SuspendAndExec — use CLI UI (no re-suspend).
		l := launcher.New(a, launcher.NewCLIUI(nil))
		return l.Launch(context.Background(), launcher.LaunchOpts{
			ProjectID:   project.ID,
			ProjectPath: project.Path,
			Agent:       devAgent,
			Prompt:      devPrompt,
			SkipSummary: true,
			SkipConfirm: true,
			SkipDeploy:  true,
		})
	})
	if devErr != nil {
		tuiShell.ShowToast(fmt.Sprintf("Dev: %s", devErr), shell.ToastWarning)
	} else if err != nil {
		tuiShell.ShowToast("Session terminée avec erreur", shell.ToastWarning)
	} else {
		tuiShell.ShowToast("Session terminée", shell.ToastSuccess)
	}
}

// buildOnboardPromptForTUI constructs the onboard prompt for the current project.
func buildOnboardPromptForTUI() string {
	a := MustApp()
	project, err := resolveActiveProject(a)
	if err != nil {
		return ""
	}
	hubDir := findHubDir()
	return prompt.BuildOnboardPrompt(project, hubDir, prompt.WikiExists(project.Path))
}

// handleDevModeTUI is the TUI version of handleDevMode (start_dev_mode.go).
// It runs the same ticket picker workflow but without cobra.Command dependency.
// Must be called from within SuspendAndExec (needs terminal control for huh forms).
func handleDevModeTUI(a *app.App, project *domain.Project) (agentName, devPrompt string, err error) {
	if err := beads.Available(); err != nil {
		return "", "", fmt.Errorf("%s", i18n.T("cmd.start.dev_no_bd"))
	}

	ctx := context.Background()

	// Pull team-state for claim awareness
	var teamRepo *teamstate.Repo
	if teamEnabledForProject(a, project) {
		tc := resolvedTeamConfig(a, project)
		statePath := tc.StatePath
		if statePath == "" {
			statePath = defaultTeamStatePath(a)
		}
		teamRepo = teamstate.NewRepo(tc.StateRepo, statePath)
		if teamRepo.IsCloned() {
			_ = teamRepo.Pull(ctx)
		}
	}

	// Query tickets — include both ready (todo) and in-progress (resumable)
	epics, err := beads.ListEpicsWithDevPickableChildren(project.Path)
	if err != nil {
		slog.Warn("failed to list epics", "error", err)
	}

	withLabel, withoutLabel, err := beads.DevPickableOrphanTickets(project.Path, "")
	if err != nil {
		return "", "", fmt.Errorf("querying tickets: %w", err)
	}

	totalOptions := len(epics) + len(withLabel) + len(withoutLabel)
	if totalOptions == 0 {
		return "", "", fmt.Errorf("aucun ticket disponible (todo ou en cours) avec le label %q", "ai-delegated")
	}

	// Build picker
	type pickerItem struct {
		label  string
		isEpic bool
		epicID string
		ticket beads.Ticket
	}

	var items []pickerItem
	for _, e := range epics {
		items = append(items, pickerItem{
			label:  fmt.Sprintf("[Epic] %s (%d tickets)", e.Ticket.Title, e.ReadyCount),
			isEpic: true,
			epicID: e.Ticket.ID,
			ticket: e.Ticket,
		})
	}
	for _, t := range withLabel {
		items = append(items, pickerItem{
			label:  fmt.Sprintf("[ai-delegated]%s %s — %s", devStatusTag(t.Status), t.ID, t.Title),
			isEpic: false,
			ticket: t,
		})
	}
	for _, t := range withoutLabel {
		items = append(items, pickerItem{
			label:  fmt.Sprintf("%s%s — %s", t.ID, devStatusTag(t.Status), t.Title),
			isEpic: false,
			ticket: t,
		})
	}

	options := make([]huh.Option[int], len(items))
	for i, item := range items {
		options[i] = huh.NewOption(item.label, i)
	}

	var selectedIdx int
	form := theme.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title(i18n.T("cmd.start.dev_picker_title")).
				Options(options...).
				Value(&selectedIdx),
		),
	)
	if err := form.Run(); err != nil {
		return "", "", err
	}

	selected := items[selectedIdx]

	// Resolve tickets for selected item
	var tickets []beads.Ticket
	if selected.isEpic {
		children, err := beads.DevPickableChildren(project.Path, selected.epicID)
		if err != nil {
			return "", "", fmt.Errorf("querying epic children: %w", err)
		}
		tickets = children
		fmt.Printf("  %s %s\n",
			theme.SuccessStyle.Render(theme.IconSuccess),
			i18n.Tf("cmd.start.dev_selected_epic", selected.ticket.Title, len(tickets)))
	} else {
		tickets = []beads.Ticket{selected.ticket}
		fmt.Printf("  %s %s\n",
			theme.SuccessStyle.Render(theme.IconSuccess),
			i18n.Tf("cmd.start.dev_selected_ticket", selected.ticket.ID, selected.ticket.Title))
	}

	// Auto-claim selected ticket
	if teamRepo != nil && teamRepo.IsCloned() && a.Config.ActiveTeam().MemberID != "" {
		claimID := selected.ticket.ID
		if selected.isEpic {
			claimID = selected.epicID
		}
		autoClaimTicket(ctx, a, teamRepo, project, claimID)
	}

	fmt.Printf("  %s %s\n",
		theme.SuccessStyle.Render(theme.IconArrow), i18n.T("cmd.start.dev_launching"))

	devPrompt = prompt.BuildDevPrompt(tickets)
	return "orchestrator-dev", devPrompt, nil
}
