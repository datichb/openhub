package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/tui/common"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// tuiShell holds a reference to the running shell for action callbacks.
var tuiShell *shell.Shell

// tuiContext is the context of the running shell, which ends with it; the
// callbacks of the views that read the clone of the team off the event loop
// take it. Background when no shell runs (tests, standalone views).
func tuiContext() context.Context {
	if sh := tuiShell; sh != nil {
		return sh.Context()
	}
	return context.Background()
}

// tuiLaunch says how the TUI shell opens. The zero value is the usual start:
// project mode inside a registered project, the hub elsewhere.
type tuiLaunch struct {
	// Project is the name of the project to open in project mode.
	Project string
	// Team opens the team mode on this team (its ID may be empty: the team of
	// the hub).
	Team *views.ActiveTeam
	// Hub opens the hub even inside a registered project.
	Hub bool
	// View is a view opened on top of the home view of the mode
	// (`oh board`, `oh team board`, `oh dashboard`); Esc goes back to it.
	View string
	// Notice is a warning shown as a toast once the shell is up (the
	// terminal is then on the alternate screen: the message printed before
	// is only seen at exit).
	Notice string
}

// runTUIWithProject launches the TUI, optionally activating project mode
// immediately for the given project name.
func runTUIWithProject(projectName string) error {
	return runTUI(tuiLaunch{Project: projectName})
}

// runTUI launches the TUI shell as l says.
func runTUI(l tuiLaunch) error {
	a := MustApp()
	projectName := l.Project

	// ── First-run detection ─────────────────────────────────────────────
	// If no provider is configured, the inline wizard will be pushed into
	// the TUI shell (see below). If the TUI cannot launch (no TTY, --no-tui),
	// redirect to the CLI 'oh init' command.
	firstRun := needsFirstRunWizard(a) && projectName == "" && l.View == ""
	if firstRun && !canLaunchTUI() {
		fmt.Fprintln(a.IO.Out, i18n.T("cmd.init.wizard_no_tui_hint"))
		return nil
	}

	homeViewID := "home"

	// Resolve initial project if requested
	var initialProject *views.ActiveProject
	var p *domain.Project
	if projectName != "" && a.Projects != nil {
		var err error
		p, err = a.Projects.GetByName(context.Background(), projectName)
		if err != nil || p == nil {
			return fmt.Errorf(i18n.T("cmd.tui.project_not_found"), projectName)
		}
	} else if projectName == "" && a.Projects != nil && !l.Hub && l.Team == nil {
		// Inside a registered project: project mode; elsewhere: hub, no question.
		if projects, err := a.Projects.List(context.Background(), domain.ProjectStatusActive); err == nil {
			p = projectAtCwd(projects)
		}
	}
	if p != nil {
		initialProject = &views.ActiveProject{
			ID:     p.ID,
			Name:   p.Name,
			Path:   p.Path,
			Branch: resolveGitBranch(p.Path),
		}
		homeViewID = "project.mode"
	}
	if l.Team != nil {
		homeViewID = "team.mode"
	}

	// Create the notification store upfront so it can be shared between the
	// shell (which populates it via showToast) and the NotificationsView.
	notifStore := shell.NewNotificationStore(200)

	// ── Pre-warm secret store ──────────────────────────────────────────
	// If filecrypt fallback is in use, the first Get() triggers a passphrase
	// prompt via huh (BubbleTea). This MUST happen before tview takes over
	// the terminal, otherwise the two TUI frameworks will conflict.
	if a.Secrets != nil {
		_, _ = a.Secrets.Get(context.Background(), "_probe_")
	}

	builtViews := buildViews(a)

	cfg := shell.Config{
		ProjectName:   a.Config.Name,
		Commands:      buildCommands(a),
		Views:         builtViews,
		HomeViewID:    homeViewID,
		InitialViewID: l.View,
		Notifications: notifStore,
		BeforeQuit:    func(quit func()) { beforeQuit(a, tuiShell)(quit) },
		Compact:       func() string { return a.Config.CLI.Compact },
		TeamsProvider: func() []views.SelectOption { // B12
			var out []views.SelectOption
			for _, t := range a.Config.Teams {
				if t.Enabled {
					out = append(out, views.SelectOption{Label: t.DisplayName(), Value: t.ID})
				}
			}
			return out
		},
		// Ctrl+T enters the project mode (BL-24): both run off the event loop.
		ProjectsProvider: func() []views.SelectOption {
			projects, _ := a.Projects.List(context.Background(), domain.ProjectStatusActive)
			out := make([]views.SelectOption, 0, len(projects))
			for _, p := range projects {
				out = append(out, views.SelectOption{Label: p.Name, Value: p.ID})
			}
			return out
		},
		ProjectResolver: func(id string) *views.ActiveProject {
			p, err := a.Projects.Get(context.Background(), id)
			if err != nil || p == nil {
				return nil
			}
			return &views.ActiveProject{ID: p.ID, Name: p.Name, Path: p.Path, Branch: resolveGitBranch(p.Path)}
		},
	}

	views.ToolName = toolName
	tuiShell = shell.New(cfg)
	views.SetTeamSyncHook(tuiReplayWorkflowQueue)

	// ── Install TUI-aware slog handler ──────────────────────────────────
	// Redirect slog output to the TUI toast/notification system instead of
	// writing to stderr (which corrupts the tview terminal).
	originalSlogHandler := slog.Default().Handler()
	originalLogOutput := log.Writer()
	tuiHandler := shell.NewTUILogHandler(tuiShell, notifStore, slog.LevelWarn)
	slog.SetDefault(slog.New(tuiHandler))
	log.SetOutput(io.Discard) // suppress stdlib log writes (e.g. toast.go legacy)

	// Pre-set the active project before navigating home so project.mode renders it
	if initialProject != nil {
		tuiShell.SetActiveProject(initialProject)
	}
	if l.Team != nil {
		tuiShell.SetActiveTeam(l.Team)
	}

	// Determine the navigation mode for the home view
	homeMode := views.ModeHub
	switch cfg.HomeViewID {
	case "project.mode":
		homeMode = views.ModeProject
	case "team.mode":
		homeMode = views.ModeTeam
	}
	tuiShell.NavigateHome(cfg.HomeViewID, homeMode)

	// ── Push first-run wizard inline if needed ──────────────────────────
	if firstRun {
		lockPath := filepath.Join(config.HubDir(), "init.lock")
		if err := acquireInitLock(lockPath); err != nil {
			slog.Warn("skipping first-run wizard: another init may be running", "error", err)
		} else {
			wizard := buildFirstRunInlineWizard(a)
			tuiShell.PushView(wizard)
			// Lock is released when the TUI exits (deferred below).
			defer releaseInitLock(lockPath)
		}
	}

	// v5 sessions: presence heartbeat + recap of what ran while oh was closed.
	stopPresence := startTUIPresence(context.Background())
	defer stopPresence()
	sessCtx, stopSessions := context.WithCancel(context.Background())
	defer stopSessions()
	if tuiSess != nil {
		tuiSess.start(sessCtx)
	}
	safego.Go(func() {
		time.Sleep(500 * time.Millisecond)
		showAbsenceRecap(context.Background(), a, tuiShell) // reads off the event loop
		if l.Notice != "" {
			tuiShell.ShowToast(l.Notice, shell.ToastWarning)
		}
		offerKeysNoticeInTUI(context.Background(), a, tuiShell, firstRun)
		offerCleanupInTUI(context.Background(), a, tuiShell)
	})

	err := tuiShell.Run()

	// ── Restore original logging ────────────────────────────────────────
	slog.SetDefault(slog.New(originalSlogHandler))
	log.SetOutput(originalLogOutput)

	return err
}

// canLaunchTUI returns true if the environment supports launching the TUI shell.
func canLaunchTUI() bool {
	return common.UseRichTUI()
}
