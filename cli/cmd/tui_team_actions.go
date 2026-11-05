package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// « Créer une équipe » and « Reconfigurer » (teamAssistant)
// ─────────────────────────────────────────────────────────────────────────────

// actionTeamInit is « Créer une équipe »: a new repository, always, even when
// the hub has teams already.
func actionTeamInit() { openTeamAssistant(nil) }

// actionTeamReconfigure is « Reconfigurer l'équipe »: the assistant on the team
// that is open.
func actionTeamReconfigure() {
	if tuiShell == nil {
		return
	}
	a := MustApp()
	// The team that is open: the one of the team mode (tuiTeamID), else the
	// one the commands work on.
	rt, err := ResolveTeam(tuiShell.Context(), a)
	team := a.Config.FindTeam(rt.TeamID)
	if err != nil || team == nil {
		tuiShell.ShowToast(i18n.T("v54a.init.nothing_to_reconfigure_tui"), shell.ToastWarning)
		return
	}
	reconfigureTeam(team)
}

// reconfigureTeam opens the assistant on team (the action of the detail of a
// team on the team it shows).
func reconfigureTeam(team *config.TeamConfig) {
	if tuiShell == nil {
		return
	}
	if archivedTeam(*team) {
		tuiShell.ShowToast(archivedTeamMessage(*team), shell.ToastWarning)
		return
	}
	if !reconfigurable(*team) {
		tuiShell.ShowToast(i18n.T("v54a.init.nothing_to_reconfigure_tui"), shell.ToastWarning)
		return
	}
	openTeamAssistant(team)
}

// openTeamAssistant pushes the assistant that creates a team (team nil) or
// reconfigures team. Nothing is cloned, pulled or pushed before it is on the
// screen: the probe of the access and the clone are steps with a spinner.
func openTeamAssistant(team *config.TeamConfig) {
	if tuiShell == nil {
		return
	}
	a := MustApp()
	ctx := tuiShell.Context()
	w := newTeamAssistant(ctx, a, team)

	// ── Tracker discovery (optional) ──────────────────────────────────
	var launchDiscoveryAfter bool
	trackerStep := views.WizardStep{
		Label:  i18n.T("cmd.init.wizard_step_tracker_setup"),
		SkipIf: func() bool { return w.repo == nil },
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()

			trackerConfigured := false
			if teamCfg, err := w.repo.LoadConfig(ctx); err == nil && teamCfg.Tracker.Type != "" {
				trackerConfigured = true
				hint := i18n.T("cmd.init.wizard_tracker_configured") + "\n" +
					i18n.Tf("cmd.init.wizard_tracker_info",
						teamCfg.Tracker.Type,
						teamCfg.Tracker.TrackerURL,
						teamCfg.Tracker.TrackerProject,
					)
				form.AddTextView("", hint, 60, 3, true, false)
			} else {
				form.AddTextView("", i18n.T("cmd.init.wizard_tracker_not_configured"), 60, 2, true, false)
			}

			options := []string{i18n.T("cmd.init.wizard_select_placeholder")}
			if trackerConfigured {
				options = append(options,
					i18n.T("cmd.init.wizard_tracker_keep_existing"),
					i18n.T("cmd.init.wizard_tracker_configure_now"),
					i18n.T("cmd.init.wizard_tracker_configure_later"),
				)
				form.AddDropDown(i18n.T("cmd.init.wizard_step_tracker_setup"), options, 0, func(_ string, idx int) {
					launchDiscoveryAfter = idx == 2 // 1=keep, 2=reconfigure, 3=later
				})
			} else {
				options = append(options,
					i18n.T("cmd.init.wizard_tracker_configure_now"),
					i18n.T("cmd.init.wizard_tracker_configure_later"),
				)
				form.AddDropDown(i18n.T("cmd.init.wizard_step_tracker_setup"), options, 0, func(_ string, idx int) {
					launchDiscoveryAfter = idx == 1 // 1=configure, 2=later
				})
			}
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		InfoFields: func() []views.InfoField {
			val := i18n.T("cmd.init.wizard_tracker_configure_later")
			if launchDiscoveryAfter {
				val = i18n.T("cmd.init.wizard_tracker_configure_now")
			}
			return []views.InfoField{{Label: "Tracker", Value: val}}
		},
	}

	title := i18n.T("v54a.team.create.title")
	if team != nil {
		title = i18n.Tf("v54a.team.reconfigure.title", team.DisplayName())
	}
	wizard := views.NewInlineWizardView(views.InlineWizardConfig{
		ID:                 "wizard.team.init",
		Title:              title,
		Steps:              append(w.steps(), trackerStep),
		SummaryTargetView:  "team.detail",
		SummaryTargetLabel: i18n.T("wizard.summary.goto_team_detail"),
		OnComplete: func(completed bool, err error) {
			// hub.toml was written when the card of the member was (teamAssistant)
			if !completed || err != nil || !launchDiscoveryAfter {
				return
			}
			// Small delay to let the summary screen render before pushing the new wizard.
			safego.Go(func() {
				time.Sleep(200 * time.Millisecond)
				if tuiShell != nil {
					queueOnShell(tuiShell, func() {
						actionTrackerDiscovery()
					})
				}
			})
		},
	})

	tuiShell.PushView(wizard)
}

// ─────────────────────────────────────────────────────────────────────────────
// Takeover brief enrichment
// ─────────────────────────────────────────────────────────────────────────────

func runTakeoverEnrich(a *app.App, project, ticketID string) error {
	// Enriched by the brief-enrich workflow (headless session of the project).
	var proj *domain.Project
	if a.Projects != nil {
		proj, _ = a.Projects.GetByName(context.Background(), project)
	}
	if proj == nil {
		return errors.New(i18n.T("cmd.run.no_project"))
	}
	// The briefs of the project are in the team-state of its team.
	team, err := teamOfProject(context.Background(), a, proj)
	if err != nil {
		return err
	}
	if !team.Enabled {
		return team.notEnabledError()
	}
	repo := teamstate.NewRepo(team.StateRepo, team.StatePath)

	enriched, err := runBriefEnrich(tuiShell.Context(), a, proj, ticketID, io.Discard)
	if err != nil {
		return fmt.Errorf("enrichment: %w", err)
	}

	briefsDir := filepath.Join(repo.Path(), "projects", project, "takeover-briefs")
	entries, _ := os.ReadDir(briefsDir)
	var latestBase string
	for _, e := range entries {
		name := e.Name()
		if len(name) > len(ticketID)+1 && name[:len(ticketID)] == ticketID &&
			strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".enriched.md") {
			latestBase = strings.TrimSuffix(name, ".md")
		}
	}
	if latestBase == "" {
		latestBase = ticketID
	}

	enrichedContent := fmt.Sprintf("# Takeover Brief (enrichi): %s\n\n%s", ticketID, enriched)
	enrichedFile := filepath.Join(briefsDir, latestBase+".enriched.md")
	if err := os.WriteFile(enrichedFile, []byte(enrichedContent), 0o644); err != nil {
		return fmt.Errorf(i18n.T("cmd.tui_team_actions.writing_enriched_brief"), err)
	}

	relPath := filepath.Join("projects", project, "takeover-briefs", latestBase+".enriched.md")
	ctx := tuiShell.Context()
	if err := repo.CommitAndPush(ctx, fmt.Sprintf("takeover: enriched brief for %s/%s", project, ticketID), relPath); err != nil {
		slog.Warn("enriched brief saved locally but sync failed", "ticket", ticketID, "error", err)
	}

	return nil
}
