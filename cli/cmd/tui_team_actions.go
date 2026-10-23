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
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Team Init action (hub-level)
// ─────────────────────────────────────────────────────────────────────────────

func actionTeamInit() {
	if tuiShell == nil {
		return
	}

	a := MustApp()
	ctx := tuiShell.Context()

	// ── Shared state (captured by step closures) ────────────────────
	credState := &httpsCredState{Username: "oauth2", AuthChoice: "provide"}
	var (
		stateRepo string
		statePath = config.DefaultTeamStatePath()
		repo      *teamstate.Repo

		hasConfig, hasPolicies bool
		existingCfg            *teamstate.TeamConfig

		staleDaysStr       = "3"
		memberID           = a.Config.ActiveTeam().MemberID
		displayName        string
		gitlabUsername     string
		mattermostUsername string
		trackerUsername    string
		role               string

		webhookURL string
		channel    string
		botName    = "OpenHub"

		hasMember bool
	)

	// Pre-fill from existing hub.toml if already configured
	if at := a.Config.ActiveTeam(); at.StatePath != "" {
		statePath = at.StatePath
	}
	if at := a.Config.ActiveTeam(); at.StateRepo != "" {
		stateRepo = at.StateRepo
		repo = teamstate.NewRepo(stateRepo, statePath)
		if repo.IsCloned() {
			_ = repo.Pull(ctx)
			_ = repo.InitStructure(ctx)
			credState.Probe(ctx, stateRepo)
			hasConfig = repo.HasConfig()
			hasPolicies = repo.HasPolicies()
			existingCfg, _ = repo.LoadConfig()
			if hasConfig && existingCfg != nil {
				staleDaysStr = fmt.Sprintf("%d", existingCfg.Takeover.StaleDays)
				webhookURL = existingCfg.Notification.MattermostWebhook
				channel = existingCfg.Notification.Channel
				botName = existingCfg.Notification.BotName
			}
			hasMember = memberID != "" && repo.HasMember(memberID)
			if hasMember {
				if m, err := repo.GetMember(memberID); err == nil && m != nil {
					displayName = m.DisplayName
					gitlabUsername = m.GitLabUsername
					mattermostUsername = m.MattermostUsername
					trackerUsername = m.TrackerUsername
					role = m.Role
				}
			}
		}
	}

	// ── Step 0: Repository ──────────────────────────────────────────
	repoStep := views.WizardStep{
		Label:    i18n.T("cmd.team.init.step_repo"),
		Required: true,
		SkipIf: func() bool {
			return a.Config.ActiveTeam().StateRepo != ""
		},
		Validate: func() string {
			if strings.TrimSpace(stateRepo) == "" {
				return i18n.T("cmd.team.init.validate.repo_required")
			}
			return ""
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			form.AddInputField(
				i18n.T("cmd.team.init.repo_url_title"),
				stateRepo, 0, nil,
				func(text string) { stateRepo = text })
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		Processing: i18n.T("cmd.team.init.processing_repo"),
		OnDone: func() error {
			if existing, found := findExistingCloneForRemote(ctx, a, stateRepo); found {
				return errors.New(
					i18n.Tf("tui.team.clone_exists", existing.Path, existing.Source))
			}
			repo = teamstate.NewRepo(stateRepo, statePath)
			if repo.IsCloned() {
				_ = repo.Pull(ctx)
			} else {
				if err := repo.Clone(ctx); err != nil {
					return fmt.Errorf(i18n.T("cmd.tui_team_actions.cloning_team_state"), err)
				}
			}
			if err := repo.InitStructure(ctx); err != nil {
				return fmt.Errorf(i18n.T("cmd.tui_team_actions.init_structure"), err)
			}
			credState.Probe(ctx, stateRepo)
			// Pre-fill for subsequent steps
			hasConfig = repo.HasConfig()
			hasPolicies = repo.HasPolicies()
			existingCfg, _ = repo.LoadConfig()
			if hasConfig && existingCfg != nil {
				staleDaysStr = fmt.Sprintf("%d", existingCfg.Takeover.StaleDays)
				webhookURL = existingCfg.Notification.MattermostWebhook
				channel = existingCfg.Notification.Channel
				botName = existingCfg.Notification.BotName
			}
			hasMember = memberID != "" && repo.HasMember(memberID)
			if hasMember {
				if m, err := repo.GetMember(memberID); err == nil && m != nil {
					displayName = m.DisplayName
					gitlabUsername = m.GitLabUsername
					mattermostUsername = m.MattermostUsername
					trackerUsername = m.TrackerUsername
					role = m.Role
				}
			}
			return nil
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{{Label: "Repo", Value: stateRepo}}
		},
	}

	// ── Step 0b: HTTPS Credentials ──────────────────────────────────
	credStep := buildHTTPSCredStep(ctx, &stateRepo, credState, nil, a.Secrets)

	// ── Build shared team step state ────────────────────────────────
	tss := &teamStepState{
		Ctx:                ctx,
		Repo:               repo,
		HasConfig:          hasConfig,
		HasPolicies:        hasPolicies,
		ExistingCfg:        existingCfg,
		HasMember:          hasMember,
		StaleDaysStr:       staleDaysStr,
		MemberID:           memberID,
		DisplayName:        displayName,
		GitLabUsername:     gitlabUsername,
		TrackerUsername:    trackerUsername,
		MattermostUsername: mattermostUsername,
		Role:               role,
		WebhookURL:         webhookURL,
		Channel:            channel,
		BotName:            botName,
	}
	stepOpts := teamStepOpts{}

	configStep := buildTeamConfigStep(tss, stepOpts)
	identityStep := buildTeamIdentityStep(tss, stepOpts)
	notifStep := buildTeamNotifStep(tss, stepOpts)
	policiesStep := buildTeamPoliciesStep(tss, stepOpts)

	// ── Step 7: Tracker discovery (optional) ──────────────────────────
	var launchDiscoveryAfter bool
	trackerStep := views.WizardStep{
		Label: i18n.T("cmd.init.wizard_step_tracker_setup"),
		SkipIf: func() bool {
			return stateRepo == ""
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()

			sp := statePath
			if sp == "" {
				sp = config.TeamStatePath(stateRepo)
			}
			repo := teamstate.NewRepo(stateRepo, sp)
			trackerConfigured := false
			if teamCfg, err := repo.LoadConfig(); err == nil && teamCfg.Tracker.Type != "" {
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

	// ── Build and push the inline wizard ────────────────────────────
	wizard := views.NewInlineWizardView(views.InlineWizardConfig{
		ID:    "wizard.team.init",
		Title: i18n.T("tui.team.init"),
		Steps: []views.WizardStep{
			repoStep, credStep, configStep, identityStep, notifStep, policiesStep, trackerStep,
		},
		SummaryTargetView:  "team.detail",
		SummaryTargetLabel: i18n.T("wizard.summary.goto_team_detail"),
		OnComplete: func(completed bool, err error) {
			if !completed || err != nil {
				return
			}
			// Sync state from shared struct (builders mutate the struct, not local vars)
			memberID = tss.MemberID

			// Persist hub.toml team config
			if stateRepo == "" {
				stateRepo = a.Config.ActiveTeam().StateRepo
			}
			if stateRepo == "" {
				return
			}
			if memberID == "" {
				memberID = a.Config.ActiveTeam().MemberID
			}
			if err := writeTeamConfig(stateRepo, statePath, memberID); err != nil {
				slog.Warn("failed to write team config", "error", err)
			}

			// Launch discovery wizard if requested.
			if launchDiscoveryAfter {
				// Small delay to let the summary screen render before pushing the new wizard.
				go func() {
					time.Sleep(200 * time.Millisecond)
					if tuiShell != nil {
						tuiShell.App().QueueUpdateDraw(func() {
							actionTrackerDiscovery()
						})
					}
				}()
			}
		},
	})

	tuiShell.PushView(wizard)
}

// ─────────────────────────────────────────────────────────────────────────────
// Team Configure action (project-level)
// ─────────────────────────────────────────────────────────────────────────────

// actionTeamConfigure opens a modal chain to fully configure the team for the
// currently active project:
//   - inherit  → use hub team as-is
//   - custom   → full setup (clone, init, add/update member, persist)
//   - disabled → opt out for this project
func actionTeamConfigure() {
	if tuiShell == nil {
		return
	}

	a := MustApp()

	project := tuiShell.ActiveProject()
	if project == nil {
		tuiShell.ShowToast(i18n.T("tui.team.configure.no_project"), shell.ToastError)
		return
	}

	hubTeam := a.Config.ActiveTeam()

	// ── Choose: attach to team or detach ─────────────────────────────────
	var modeOptions []views.SelectOption
	if hubTeam.Enabled && hubTeam.ID != "" {
		modeOptions = []views.SelectOption{
			{
				Label: i18n.Tf("tui.team.configure.attach_team", hubTeam.ID, hubTeam.MemberID),
				Value: hubTeam.ID,
			},
			{Label: i18n.T("tui.team.configure.no_team"), Value: ""},
		}
	} else {
		modeOptions = []views.SelectOption{
			{Label: i18n.T("tui.team.configure.no_team"), Value: ""},
		}
	}

	defaultVal := ""
	if hubTeam.Enabled && hubTeam.ID != "" {
		defaultVal = hubTeam.ID
	}

	tuiShell.ShowSelectModal(i18n.T("tui.team.configure.modal_title"), modeOptions, defaultVal, func(teamID string) {
		go func() {
			tuiShell.App().QueueUpdateDraw(func() {
				applyProjectTeamID(a, project.ID, teamID)
			})
		}()
	})
}

// applyProjectTeamID persists the team ID for a project in the DB.
//
// IMPORTANT: This function MUST be called from within the tview event loop
// (e.g. from a QueueUpdateDraw callback or a tview handler). It calls ShowToast
// directly — never via QueueUpdateDraw — to avoid a nested-QueueUpdateDraw deadlock.
func applyProjectTeamID(a *app.App, projectID, teamID string) {
	ctx := context.Background()
	p, err := a.Projects.Get(ctx, projectID)
	if err != nil {
		tuiShell.ShowToast(i18n.T("tui.team.configure.project_not_found"), shell.ToastError)
		return
	}

	if teamID == "" {
		p.TeamID = nil
	} else {
		p.TeamID = &teamID
	}
	// Clear legacy TeamConfig to avoid confusion
	p.TeamConfig = nil

	if err := a.Projects.Update(ctx, p); err != nil {
		tuiShell.ShowToast(i18n.T("tui.team.configure.error_prefix")+err.Error(), shell.ToastError)
		return
	}

	label := i18n.T("tui.team.configure.team_none")
	if teamID != "" {
		label = teamID
	}
	tuiShell.ShowToast(
		i18n.Tf("tui.team.configure.team_set", label),
		shell.ToastSuccess,
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// Takeover brief enrichment
// ─────────────────────────────────────────────────────────────────────────────

func runTakeoverEnrich(a *app.App, project, ticketID string) error {
	repo := teamstate.NewRepo(a.Config.ActiveTeam().StateRepo, a.Config.ActiveTeam().StatePath)

	// Enriched by the brief-enrich workflow (headless session of the project).
	var proj *domain.Project
	if a.Projects != nil {
		proj, _ = a.Projects.GetByName(context.Background(), project)
	}
	if proj == nil {
		return errors.New(i18n.T("cmd.run.no_project"))
	}
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
