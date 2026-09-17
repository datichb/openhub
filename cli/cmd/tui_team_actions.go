package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
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

		selectedPolicies []string
		hasMember        bool
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
					return fmt.Errorf("cloning team-state: %w", err)
				}
			}
			if err := repo.InitStructure(ctx); err != nil {
				return fmt.Errorf("init structure: %w", err)
			}
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
	credStep := views.WizardStep{
		Label: i18n.T("cmd.team.init.step_credentials"),
		SkipIf: func() bool {
			return !teamstate.IsHTTPS(stateRepo)
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			username := "oauth2"
			token := ""
			authChoice := "provide"
			authOptions := []string{
				i18n.T("cmd.team.init.cred_provide"),
				i18n.T("cmd.team.init.cred_skip"),
				i18n.T("cmd.team.init.cred_public"),
			}
			form.AddDropDown(i18n.T("cmd.team.init.cred_auth_mode"), authOptions, 0,
				func(_ string, idx int) {
					switch idx {
					case 0:
						authChoice = "provide"
					case 1:
						authChoice = "skip"
					case 2:
						authChoice = "public"
					}
				})
			form.AddInputField("Username", username, 0, nil,
				func(text string) { username = text })
			form.AddPasswordField("Token", token, 0, '*',
				func(text string) { token = text })
			form.AddButton(i18n.T("wizard.hint.submit"), func() {
				if authChoice == "provide" && token == "" {
					return // block submit without token
				}
				onDone()
			})

			// Capture shared vars for OnDone
			_ = &username
			_ = &token
			_ = &authChoice
			return form
		},
		OnDone: func() error {
			// The Form closure captures username/token/authChoice but since we need
			// them in OnDone, we access them through the step Form closure.
			// For now, credentials are configured via the form submit.
			return nil
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{{Label: "Auth", Value: "configured"}}
		},
	}

	// ── Step 1: Global Config ───────────────────────────────────────
	configStep := views.WizardStep{
		Label:      i18n.T("cmd.team.init.step_config"),
		Processing: i18n.T("cmd.team.init.processing_config"),
		SkipIf:     func() bool { return repo == nil },
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			form.AddInputField(
				i18n.T("cmd.team.init.config_stale_days"),
				staleDaysStr, 0, nil,
				func(text string) { staleDaysStr = text })
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			days, err := strconv.Atoi(staleDaysStr)
			if err != nil || days <= 0 {
				days = 3
			}
			if hasConfig && existingCfg != nil {
				if days == existingCfg.Takeover.StaleDays {
					return nil
				}
				existingCfg.Takeover.StaleDays = days
				return repo.SaveConfig(ctx, existingCfg)
			}
			cfg := &teamstate.TeamConfig{
				Notification: teamstate.NotificationConfig{
					Enabled: false,
					BotName: "OpenHub",
				},
				Takeover: teamstate.TakeoverConfig{StaleDays: days},
				Parallel: teamstate.ParallelConfig{
					MaxSessions:    3,
					PortRangeStart: 4100,
					AutoMergeBeads: true,
				},
			}
			return repo.SaveConfig(ctx, cfg)
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{{Label: "Stale days", Value: staleDaysStr}}
		},
	}

	// ── Step 2: Identity ────────────────────────────────────────────
	identityStep := views.WizardStep{
		Label:      i18n.T("cmd.team.init.step_identity"),
		Processing: i18n.T("cmd.team.init.processing_identity"),
		Required:   true,
		SkipIf:     func() bool { return repo == nil },
		Validate: func() string {
			if strings.TrimSpace(memberID) == "" {
				return i18n.T("cmd.team.init.validate.member_id_required")
			}
			return ""
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			if !hasMember {
				form.AddInputField(
					i18n.T("cmd.team.init.identity_id"),
					memberID, 0, nil,
					func(text string) { memberID = text })
			}
			form.AddInputField(
				i18n.T("cmd.team.init.identity_display"),
				displayName, 0, nil,
				func(text string) { displayName = text })
			form.AddInputField(
				i18n.T("cmd.team.init.identity_gitlab"),
				gitlabUsername, 0, nil,
				func(text string) { gitlabUsername = text })
			form.AddInputField(
				i18n.T("cmd.team.init.identity_mattermost"),
				mattermostUsername, 0, nil,
				func(text string) { mattermostUsername = text })
			form.AddInputField(
				i18n.T("tui.team.identity_tracker_username"),
				trackerUsername, 0, nil,
				func(text string) { trackerUsername = text })
			roles := []string{"lead", "dev", "reviewer"}
			roleIdx := 0
			for i, r := range roles {
				if r == role {
					roleIdx = i
					break
				}
			}
			form.AddDropDown(
				i18n.T("cmd.team.init.identity_role"),
				roles, roleIdx,
				func(_ string, idx int) { role = roles[idx] })
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			member := teamstate.Member{
				ID:                 memberID,
				DisplayName:        displayName,
				GitLabUsername:     gitlabUsername,
				TrackerUsername:    trackerUsername,
				MattermostUsername: mattermostUsername,
				Role:               role,
				DefaultMode:        "semi-auto",
			}
			if hasMember {
				if err := repo.UpdateMember(ctx, member); err != nil {
					return err
				}
				return repo.CommitAndPush(ctx, fmt.Sprintf("team: update member %s", memberID), "members.toml")
			}
			if err := repo.AddMember(ctx, member); err != nil {
				if err == teamstate.ErrMemberExists {
					return nil
				}
				return err
			}
			return repo.CommitAndPush(ctx, fmt.Sprintf("team: add member %s", memberID), "members.toml")
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{
				{Label: "Member", Value: memberID},
				{Label: "Name", Value: displayName},
				{Label: "Role", Value: role},
			}
		},
	}

	// ── Step 3: Notifications ───────────────────────────────────────
	notifStep := views.WizardStep{
		Label:      i18n.T("cmd.team.init.step_notifications"),
		Processing: i18n.T("cmd.team.init.processing_notifications"),
		SkipIf:     func() bool { return repo == nil },
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			form.AddInputField(
				i18n.T("cmd.team.init.notif_webhook"),
				webhookURL, 0, nil,
				func(text string) { webhookURL = text })
			form.AddInputField(
				i18n.T("cmd.team.init.notif_channel"),
				channel, 0, nil,
				func(text string) { channel = text })
			form.AddInputField(
				i18n.T("cmd.team.init.notif_bot_name"),
				botName, 0, nil,
				func(text string) { botName = text })
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			if webhookURL == "" {
				return nil
			}
			cfg, err := repo.LoadConfig()
			if err != nil {
				return err
			}
			if cfg.Notification.MattermostWebhook == webhookURL &&
				cfg.Notification.Channel == channel &&
				cfg.Notification.BotName == botName {
				return nil
			}
			cfg.Notification.MattermostWebhook = webhookURL
			cfg.Notification.Channel = channel
			cfg.Notification.BotName = botName
			cfg.Notification.Enabled = true
			return repo.SaveConfig(ctx, cfg)
		},
		InfoFields: func() []views.InfoField {
			if webhookURL == "" {
				return []views.InfoField{{Label: "Notifications", Value: "skipped"}}
			}
			return []views.InfoField{
				{Label: "Webhook", Value: webhookURL},
				{Label: "Channel", Value: channel},
				{Label: "Bot", Value: botName},
			}
		},
	}

	// ── Step 4: Policies ────────────────────────────────────────────
	policiesStep := views.WizardStep{
		Label:      i18n.T("cmd.team.init.step_policies"),
		Processing: i18n.T("cmd.team.init.processing_policies"),
		SkipIf:     func() bool { return repo == nil },
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			branchNaming := false
			commitFormat := false
			maxWip := false
			reviewRequired := false
			form.AddCheckbox(i18n.T("cmd.team.init.policies_branch_naming"), false,
				func(checked bool) { branchNaming = checked })
			form.AddCheckbox(i18n.T("cmd.team.init.policies_commit_format"), false,
				func(checked bool) { commitFormat = checked })
			form.AddCheckbox(i18n.T("cmd.team.init.policies_max_wip"), false,
				func(checked bool) { maxWip = checked })
			form.AddCheckbox(i18n.T("cmd.team.init.policies_review_required"), false,
				func(checked bool) { reviewRequired = checked })
			form.AddButton(i18n.T("wizard.hint.submit"), func() {
				selectedPolicies = nil
				if branchNaming {
					selectedPolicies = append(selectedPolicies, "branch_naming")
				}
				if commitFormat {
					selectedPolicies = append(selectedPolicies, "commit_format")
				}
				if maxWip {
					selectedPolicies = append(selectedPolicies, "max_ticket_wip")
				}
				if reviewRequired {
					selectedPolicies = append(selectedPolicies, "review_required")
				}
				onDone()
			})
			return form
		},
		OnDone: func() error {
			if len(selectedPolicies) == 0 {
				return nil
			}
			policies := buildRecommendedPolicies(selectedPolicies)
			if hasPolicies {
				existing, _ := repo.LoadPolicies("")
				for _, ep := range existing {
					if _, ok := policies[ep.Name]; !ok {
						policies[ep.Name] = ep
					}
				}
			}
			if err := repo.SavePolicies(ctx, policies); err != nil {
				return err
			}
			commitMsg := "team: init policies"
			if hasPolicies {
				commitMsg = "team: update policies"
			}
			return repo.CommitAndPush(ctx, commitMsg, "policies.toml")
		},
		InfoFields: func() []views.InfoField {
			if len(selectedPolicies) == 0 {
				return []views.InfoField{{Label: "Policies", Value: "none"}}
			}
			return []views.InfoField{
				{Label: "Policies", Value: fmt.Sprintf("%d active", len(selectedPolicies))},
			}
		},
	}

	// ── Step 7: Tracker discovery (optional) ──────────────────────────
	var launchDiscoveryAfter bool
	trackerStep := views.WizardStep{
		Label: i18n.T("tui.team.tracker_step"),
		SkipIf: func() bool {
			// Skip if no team configured yet (repo step was skipped/failed).
			return stateRepo == ""
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			options := []string{i18n.T("tui.team.tracker_later"), i18n.T("tui.team.tracker_configure_now")}
			form.AddDropDown(i18n.T("tui.team.tracker_configure_prompt"), options, 0, func(_ string, idx int) {
				launchDiscoveryAfter = idx == 1
			})
			form.AddButton(i18n.T("tui.team.tracker_next"), func() { onDone() })
			return form
		},
		InfoFields: func() []views.InfoField {
			val := i18n.T("tui.team.tracker_later")
			if launchDiscoveryAfter {
				val = i18n.T("tui.team.tracker_info_yes")
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
// Team Rejoin action (reconnect to existing team after reinstall)
// ─────────────────────────────────────────────────────────────────────────────

func actionTeamRejoin() {
	if tuiShell == nil {
		return
	}

	a := MustApp()
	ctx := tuiShell.Context()

	// ── Shared state (captured by step closures) ────────────────────
	var (
		stateRepo string
		statePath string
		members   []teamstate.Member
		memberID  string
	)

	// ── Step 0: Repository URL ──────────────────────────────────────
	repoStep := views.WizardStep{
		Label:    i18n.T("cmd.team.init.step_repo"),
		Required: true,
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
			form.AddButton(i18n.T("tui.team.rejoin.next"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			statePath = config.TeamStatePath(stateRepo)
			var err error
			_, members, err = listTeamMembers(ctx, stateRepo, statePath)
			if err != nil {
				return err
			}
			if len(members) == 0 {
				return errors.New(i18n.T("tui.team.rejoin.no_members"))
			}
			return nil
		},
		Processing: i18n.T("cmd.team.init.processing_repo"),
		InfoFields: func() []views.InfoField {
			return []views.InfoField{{Label: "Repo", Value: stateRepo}}
		},
	}

	// ── Step 0b: HTTPS Credentials (conditional) ────────────────────
	var credUsername = "oauth2"
	var credToken string
	var credAuthChoice = "provide"

	httpsCredStep := views.WizardStep{
		Label: i18n.T("cmd.team.init.step_credentials"),
		SkipIf: func() bool {
			return !teamstate.IsHTTPS(stateRepo)
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			authOptions := []string{
				i18n.T("cmd.team.init.cred_provide"),
				i18n.T("cmd.team.init.cred_skip"),
				i18n.T("cmd.team.init.cred_public"),
			}
			form.AddDropDown(i18n.T("cmd.team.init.cred_auth_mode"), authOptions, 0,
				func(_ string, idx int) {
					switch idx {
					case 0:
						credAuthChoice = "provide"
					case 1:
						credAuthChoice = "skip"
					case 2:
						credAuthChoice = "public"
					}
				})
			form.AddInputField("Username", credUsername, 0, nil,
				func(text string) { credUsername = text })
			form.AddPasswordField("Token", credToken, 0, '*',
				func(text string) { credToken = text })
			form.AddButton(i18n.T("wizard.hint.submit"), func() {
				if credAuthChoice == "provide" && credToken == "" {
					return
				}
				onDone()
			})
			return form
		},
		OnDone: func() error {
			if credAuthChoice == "provide" && credToken != "" {
				return teamstate.ConfigureCredential(ctx, stateRepo, credUsername, credToken)
			}
			return nil
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{{Label: "Auth", Value: "configured"}}
		},
	}

	// ── Step 1: Member selection ────────────────────────────────────
	memberStep := views.WizardStep{
		Label:    i18n.T("cmd.team.rejoin.select_member"),
		Required: true,
		CustomView: func(_ *tview.Application, container *tview.Flex, onDone func()) {
			list := widgets.NewSectionedList()

			var items []widgets.SectionItem
			items = append(items, widgets.SectionItem{
				MainText: i18n.T("tui.team.rejoin.members_header"),
				IsHeader: true,
			})
			for _, m := range members {
				label := fmt.Sprintf("%s (%s)", m.DisplayName, m.ID)
				secondary := ""
				if m.GitLabUsername != "" {
					secondary = fmt.Sprintf("gitlab: %s", m.GitLabUsername)
				}
				if m.Role != "" {
					if secondary != "" {
						secondary += " — "
					}
					secondary += m.Role
				}
				items = append(items, widgets.SectionItem{
					MainText:      label,
					SecondaryText: secondary,
					Reference:     m.ID,
				})
			}
			list.SetItems(items)

			list.SetItemSelectedFunc(func(_ int, item widgets.SectionItem) {
				if id, ok := item.Reference.(string); ok {
					memberID = id
				}
				onDone()
			})

			container.AddItem(list, 0, 1, true)
		},
		InfoFields: func() []views.InfoField {
			if memberID == "" {
				return nil
			}
			for _, m := range members {
				if m.ID == memberID {
					return []views.InfoField{
						{Label: i18n.T("tui.team.rejoin.member_label"), Value: fmt.Sprintf("%s (%s)", m.DisplayName, m.ID)},
					}
				}
			}
			return []views.InfoField{{Label: i18n.T("tui.team.rejoin.member_label"), Value: memberID}}
		},
	}

	// ── Step 2: GitLab identity validation + config write ───────────
	validateStep := views.WizardStep{
		Label:      i18n.T("cmd.team.rejoin.validating"),
		Processing: i18n.T("cmd.team.rejoin.validating"),
		OnDone: func() error {
			result, err := teamRejoinCore(ctx, a, teamRejoinParams{
				StateRepo: stateRepo,
				StatePath: statePath,
				MemberID:  memberID,
			})
			if err != nil {
				return err
			}

			// Retro-tag sessions in background
			go func() {
				bgCtx := context.Background()
				count, err := retroTagSessions(bgCtx, memberID)
				if err == nil && count > 0 && tuiShell != nil {
					tuiShell.App().QueueUpdateDraw(func() {
						tuiShell.ShowToast(
							i18n.Tf("cmd.team.rejoin.retro_tagged", count),
							shell.ToastSuccess,
						)
					})
				}
			}()

			// Show result summary via toast
			if tuiShell != nil {
				msg := i18n.Tf("cmd.team.rejoin.success", result.Member.DisplayName, result.Member.ID, result.TeamName)
				if result.EventCount > 0 {
					msg += " — " + i18n.Tf("cmd.team.rejoin.events_found", result.EventCount)
				}
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowToast(msg, shell.ToastSuccess)
				})
			}
			return nil
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{
				{Label: i18n.T("tui.team.rejoin.status_label"), Value: theme.SuccessStyle.Render(i18n.T("tui.team.rejoin.reconnected"))},
			}
		},
	}

	wizard := views.NewInlineWizardView(views.InlineWizardConfig{
		ID:    "wizard.team.rejoin",
		Title: i18n.T("tui.team.rejoin"),
		Steps: []views.WizardStep{
			repoStep,
			httpsCredStep,
			memberStep,
			validateStep,
		},
	})

	tuiShell.PushView(wizard)
}

// ─────────────────────────────────────────────────────────────────────────────
// Takeover brief enrichment
// ─────────────────────────────────────────────────────────────────────────────

func runTakeoverEnrich(a *app.App, project, ticketID string) error {
	repo := teamstate.NewRepo(a.Config.ActiveTeam().StateRepo, a.Config.ActiveTeam().StatePath)

	content, err := repo.ReadBrief(project, ticketID)
	if err != nil {
		return fmt.Errorf("reading brief: %w", err)
	}

	enriched, err := opencode.RunHeadless(opencode.HeadlessOpts{
		Agent:  "brief-enricher",
		Prompt: content,
	})
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
		return fmt.Errorf("writing enriched brief: %w", err)
	}

	relPath := filepath.Join("projects", project, "takeover-briefs", latestBase+".enriched.md")
	ctx := tuiShell.Context()
	_ = repo.CommitAndPush(ctx, fmt.Sprintf("takeover: enriched brief for %s/%s", project, ticketID), relPath)

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Sync Tracker action (omnibar + view touche 's')
// ─────────────────────────────────────────────────────────────────────────────

// actionSyncTracker is the omnibar action for "sync tracker".
func actionSyncTracker() {
	if tuiShell == nil {
		return
	}
	a := MustApp()
	ctx := tuiShell.Context()

	tuiShell.ShowToast(i18n.T("tui.team.sync.in_progress"), shell.ToastInfo)

	go func() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		result, err := runSyncTrackerForTUI(a, ctx)
		select {
		case <-ctx.Done():
			return
		default:
		}
		tuiShell.App().QueueUpdateDraw(func() {
			if err != nil {
				tuiShell.ShowToast(i18n.T("tui.team.sync.error_prefix")+err.Error(), shell.ToastError)
				return
			}
			content := formatSyncResultModal(result)
			tuiShell.ShowScrollableModal(i18n.T("tui.team.sync.result_title"), content, []views.ModalAction{
				{Label: "OK", Callback: func() {}},
			})
		})
	}()
}

// runSyncTrackerForTUI executes the tracker sync and returns a result for display.
func runSyncTrackerForTUI(a *app.App, ctx context.Context) (*views.SyncTrackerResult, error) {
	// Resolve team config
	tc := resolvedTeamConfig(a, nil)
	if !tc.Enabled {
		return nil, errors.New(i18n.T("tui.team.sync.team_not_configured"))
	}

	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	if !repo.IsCloned() {
		return nil, errors.New(i18n.T("tui.team.sync.not_cloned"))
	}

	// Load team config
	teamCfg, err := repo.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("tui.team.sync.config_load_error"), err)
	}
	if teamCfg.Tracker.Type == "" {
		return nil, errors.New(i18n.T("tui.team.sync.tracker_not_configured"))
	}

	// Merge shared team-state config with local hub.toml overrides.
	effTracker := tracker.ResolveTrackerConfig(
		&teamCfg.Tracker,
		a.Config.Tracker,
		resolveWriteEnabledForTracker(a, teamCfg),
	)

	// Build credential source
	credSrc := buildCredentialSource(a, teamCfg.MCP, &teamCfg.Tracker)
	trackerType := tracker.Type(effTracker.Type)

	creds, err := tracker.ResolveCredentials(ctx, credSrc, trackerType)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Tf("tui.team.sync.credentials_missing", teamCfg.Tracker.Type), err)
	}

	t, err := tracker.New(creds)
	if err != nil {
		return nil, fmt.Errorf("initialisation tracker: %w", err)
	}

	// Build the Projects map from hub projects (same as CLI sync-tracker and resolveTrackerEngine).
	projects, ticketPatterns := resolveTrackerProjects(ctx, a, effTracker)
	if len(projects) == 0 {
		return nil, errors.New(i18n.T("tui.team.sync.no_projects"))
	}

	engineCfg := teamstate.TrackerConfig{
		Type:                 effTracker.Type,
		Enabled:              effTracker.Enabled,
		AutoSync:             effTracker.AutoSync,
		SyncIntervalMinutes:  effTracker.SyncIntervalMinutes,
		AutoPlanAssigned:     effTracker.AutoPlanAssigned,
		MaxAutoPlanPerMember: effTracker.MaxAutoPlanPerMember,
		AutoPlanUnassigned:   effTracker.AutoPlanUnassigned,
		UnassignedLabels:     effTracker.UnassignedLabels,
		MaxUnassignedIssues:  effTracker.MaxUnassignedIssues,
		PushLabels:           effTracker.PushLabels,
		TicketPatterns:       ticketPatterns,
		Projects:             projects,
		StatusMapping:        effTracker.StatusMapping,
		LabelStatusMapping:   effTracker.LabelStatusMapping,
	}
	engine := tracker.NewEngine(t, repo, engineCfg, config.HubDir())

	// Pull before sync
	_ = repo.Pull(ctx)

	// Run sync
	syncResult, err := engine.Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("sync: %w", err)
	}

	// Convert to view-friendly result
	result := &views.SyncTrackerResult{
		ClaimsCreated: syncResult.ClaimsCreated,
		ClaimsUpdated: syncResult.ClaimsUpdated,
		LabelsPushed:  syncResult.LabelsPushed,
	}
	for _, p := range syncResult.Projects {
		result.Projects = append(result.Projects, fmt.Sprintf("%s: %d fetched, %d created, %d updated", p.ProjectID, p.IssuesFetched, p.ClaimsCreated, p.ClaimsUpdated))
	}
	for _, w := range syncResult.Warnings {
		result.Warnings = append(result.Warnings, w.Message)
	}
	for _, e := range syncResult.Errors {
		result.Errors = append(result.Errors, e.Error())
	}

	return result, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Tracker Discovery Wizard (C6)
// ─────────────────────────────────────────────────────────────────────────────

// actionTrackerDiscovery launches the 5-step tracker discovery wizard.
// It connects to the configured tracker, discovers project metadata (labels,
// statuses), suggests board columns and label→column mappings, and saves
// the resulting configuration to the team-state repo.
//
// The wizard follows the same inline-wizard pattern as actionTeamInit.
func actionTrackerDiscovery() {
	if tuiShell == nil {
		return
	}

	a := MustApp()
	ctx := tuiShell.Context()

	// ── Resolve team config (same pattern as runSyncTrackerForTUI) ───
	tc := resolvedTeamConfig(a, nil)
	if !tc.Enabled {
		tuiShell.ShowToast(i18n.T("tui.team.discovery.team_not_configured"), shell.ToastError)
		return
	}

	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	if !repo.IsCloned() {
		tuiShell.ShowToast(i18n.T("tui.team.discovery.not_cloned"), shell.ToastError)
		return
	}

	// Load existing team config for pre-fill
	teamCfg, err := repo.LoadConfig()
	if err != nil {
		tuiShell.ShowToast(i18n.T("tui.team.discovery.config_load_error")+err.Error(), shell.ToastError)
		return
	}

	// ── Shared state (captured by step closures) ────────────────────
	var (
		// Step 1: Connection
		trackerType = teamCfg.Tracker.Type // pre-fill from existing config
		trackerURL  string
		projectID   = teamCfg.Tracker.TrackerProject
		token       string // resolved at runtime, not pre-filled

		// Step 2: Discovery results
		discoveryInfo     *tracker.DiscoveryInfo
		suggestedColumns  []teamstate.BoardColumnConfig
		suggestedMappings []tracker.SuggestedMapping
		unmapped          []string
		poolLabels        []string

		// Step 3: Final column selection
		finalColumns []teamstate.BoardColumnConfig

		// Step 4: Mappings (accepted as-is from suggestions)
		// suggestedMappings is reused directly

		// Step 5: Pool label selection
		selectedPool []string

		// Summary: TOML preview built after save
		tomlPreview string
	)

	// Pre-fill tracker URL from team-state config or MCP config
	if teamCfg.Tracker.TrackerURL != "" {
		trackerURL = teamCfg.Tracker.TrackerURL
	} else if trackerType != "" {
		// Try to get URL from MCP shared config
		if mcpCfg, ok := teamCfg.MCP[trackerType]; ok && mcpCfg.URL != "" {
			trackerURL = mcpCfg.URL
		}
	}

	// ── Step 1: Connection & Project ────────────────────────────────
	connStep := views.WizardStep{
		Label:      i18n.T("cmd.discovery.step_connection"),
		Required:   true,
		Processing: i18n.T("cmd.discovery.processing_connection"),
		Validate: func() string {
			if strings.TrimSpace(trackerType) == "" {
				return i18n.T("cmd.discovery.validate.type_required")
			}
			if strings.TrimSpace(trackerURL) == "" {
				return i18n.T("cmd.discovery.validate.url_required")
			}
			if strings.TrimSpace(projectID) == "" {
				return i18n.T("cmd.discovery.validate.project_required")
			}
			return ""
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()

			// Tracker type dropdown
			typeOptions := []string{"gitlab", "jira"}
			typeIdx := 0
			for i, t := range typeOptions {
				if t == trackerType {
					typeIdx = i
					break
				}
			}
			form.AddDropDown(i18n.T("cmd.discovery.field.tracker_type"), typeOptions, typeIdx,
				func(_ string, idx int) { trackerType = typeOptions[idx] })

			// URL
			form.AddInputField(i18n.T("cmd.discovery.field.tracker_url"), trackerURL, 0, nil,
				func(text string) { trackerURL = text })

			// Project ID
			form.AddInputField(i18n.T("cmd.discovery.field.tracker_project"), projectID, 0, nil,
				func(text string) { projectID = text })

			// Token (optional — if empty, will try keychain/env)
			form.AddPasswordField(i18n.T("cmd.discovery.field.token"), token, 0, '*',
				func(text string) { token = text })

			form.AddButton(i18n.T("cmd.discovery.btn.test"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			// Build tracker config for connection test
			cfg := tracker.Config{
				Type:    tracker.Type(trackerType),
				BaseURL: trackerURL,
			}

			// Resolve token: explicit > keychain > env
			if token != "" {
				cfg.Token = token
			} else {
				credSrc := buildCredentialSource(a, teamCfg.MCP, &teamCfg.Tracker)
				resolved, err := tracker.ResolveCredentials(ctx, credSrc, tracker.Type(trackerType))
				if err != nil {
					return fmt.Errorf("%s: %w", i18n.T("tui.team.discovery.credentials_error"), err)
				}
				cfg.Token = resolved.Token
				if cfg.BaseURL == "" {
					cfg.BaseURL = resolved.BaseURL
				}
			}

			// Create tracker instance and test
			t, err := tracker.New(cfg)
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("tui.team.discovery.tracker_init_error"), err)
			}

			// Wrap API calls in a 30s timeout to avoid hanging the wizard.
			apiCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			// Test connection
			username, err := t.TestConnection(apiCtx)
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("tui.team.discovery.connection_failed"), err)
			}
			_ = username // connection OK

			// Test project access
			_, err = t.TestProject(apiCtx, projectID)
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("tui.team.discovery.project_inaccessible"), err)
			}

			return nil
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{
				{Label: "Type", Value: trackerType},
				{Label: "URL", Value: trackerURL},
				{Label: i18n.T("tui.team.discovery.info_project"), Value: projectID},
			}
		},
	}

	// ── Step 2: Discovery (processing only, no form) ────────────────
	discoveryStep := views.WizardStep{
		Label:      i18n.T("cmd.discovery.step_discovery"),
		Processing: i18n.T("cmd.discovery.processing_discovery"),
		// No Form — this is a processing-only step
		OnDone: func() error {
			// Build tracker instance (same as step 1 but we need it again)
			cfg := tracker.Config{
				Type:    tracker.Type(trackerType),
				BaseURL: trackerURL,
			}
			if token != "" {
				cfg.Token = token
			} else {
				credSrc := buildCredentialSource(a, teamCfg.MCP, &teamCfg.Tracker)
				resolved, err := tracker.ResolveCredentials(ctx, credSrc, tracker.Type(trackerType))
				if err != nil {
					return fmt.Errorf("credentials: %w", err)
				}
				cfg.Token = resolved.Token
				if cfg.BaseURL == "" {
					cfg.BaseURL = resolved.BaseURL
				}
			}

			t, err := tracker.New(cfg)
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("tui.team.discovery.tracker_init_error"), err)
			}

			// Wrap API call in a 30s timeout to avoid hanging the wizard.
			apiCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			// Discover project metadata (labels, statuses)
			discoveryInfo, err = t.DiscoverProject(apiCtx, projectID)
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("tui.team.discovery.discovery_error"), err)
			}

			// Start from existing board columns or defaults
			baseColumns := teamCfg.Board.Columns
			if len(baseColumns) == 0 {
				baseColumns = teamstate.DefaultBoardConfig().Columns
			}

			// Suggest new columns based on discovered labels/statuses
			newCols := tracker.SuggestNewColumns(discoveryInfo, baseColumns)
			suggestedColumns = make([]teamstate.BoardColumnConfig, len(baseColumns))
			copy(suggestedColumns, baseColumns)
			suggestedColumns = append(suggestedColumns, newCols...)

			// Suggest label→column and status→column mappings
			suggestedMappings = tracker.SuggestMappings(discoveryInfo, tracker.Type(trackerType), suggestedColumns)

			// Identify unmapped items
			unmapped = tracker.UnmappedItems(discoveryInfo, suggestedMappings)

			// Suggest pool labels (labels mapped to the initial column)
			boardCfg := teamstate.BoardConfig{Columns: suggestedColumns}
			poolLabels = tracker.SuggestPoolLabels(suggestedMappings, boardCfg)

			return nil
		},
		InfoFields: func() []views.InfoField {
			if discoveryInfo == nil {
				return nil
			}
			fields := []views.InfoField{
				{Label: "Labels", Value: i18n.Tf("cmd.discovery.info.labels_found", len(discoveryInfo.Labels))},
				{Label: "Mappings", Value: i18n.Tf("cmd.discovery.info.mappings_suggested", len(suggestedMappings))},
			}
			if len(unmapped) > 0 {
				fields = append(fields, views.InfoField{
					Label: i18n.T("tui.team.discovery.info_unmapped"), Value: fmt.Sprintf("%d", len(unmapped)),
				})
			}
			return fields
		},
	}

	// ── Step 3: Columns (interactive editor) ────────────────────────
	columnsStep := buildColumnEditorStep(&suggestedColumns, &finalColumns, nil, nil, "")

	// ── Step 4: Mappings ────────────────────────────────────────────
	// Track which mappings the user accepts (pre-checked by default).
	var acceptedMappingFlags []bool

	mappingStep := views.WizardStep{
		Label:      i18n.T("cmd.discovery.step_mappings"),
		Processing: i18n.T("cmd.discovery.processing_mappings"),
		SkipIf: func() bool {
			return len(suggestedMappings) == 0
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()

			// Show unmapped labels as info (read-only).
			if len(unmapped) > 0 {
				maxShow := 5
				if len(unmapped) < maxShow {
					maxShow = len(unmapped)
				}
				info := i18n.Tf("tui.team.discovery.unmapped_labels", len(unmapped), strings.Join(unmapped[:maxShow], ", "))
				if len(unmapped) > maxShow {
					info += "..."
				}
				form.AddTextView("Info", info, 0, 2, false, false)
			}

			// Interactive checkboxes for each suggested mapping.
			acceptedMappingFlags = make([]bool, len(suggestedMappings))
			for i, m := range suggestedMappings {
				acceptedMappingFlags[i] = true // pre-checked
				idx := i
				label := fmt.Sprintf("%s → %s (%s)", m.Source, m.ColumnID, m.Confidence)
				form.AddCheckbox(label, true, func(checked bool) {
					acceptedMappingFlags[idx] = checked
				})
			}

			form.AddButton(i18n.T("cmd.discovery.btn.accept"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			// Filter suggestedMappings to only accepted ones.
			var accepted []tracker.SuggestedMapping
			for i, m := range suggestedMappings {
				if i < len(acceptedMappingFlags) && acceptedMappingFlags[i] {
					accepted = append(accepted, m)
				}
			}
			suggestedMappings = accepted
			return nil
		},
		InfoFields: func() []views.InfoField {
			accepted := 0
			for _, f := range acceptedMappingFlags {
				if f {
					accepted++
				}
			}
			return []views.InfoField{
				{Label: "Mappings", Value: i18n.Tf("cmd.discovery.info.mappings_accepted", accepted)},
				{Label: i18n.T("tui.team.discovery.info_unmapped"), Value: fmt.Sprintf("%d", len(unmapped))},
			}
		},
	}

	// ── Step 5: Pool labels ─────────────────────────────────────────
	poolStep := views.WizardStep{
		Label:      i18n.T("cmd.discovery.step_pool"),
		Processing: i18n.T("cmd.discovery.processing_save"),
		SkipIf: func() bool {
			// Skip if no pool labels were suggested
			return len(poolLabels) == 0
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()

			if len(poolLabels) == 0 {
				form.AddTextView("Info", i18n.T("cmd.discovery.pool.none_detected"), 0, 2, false, false)
				form.AddButton(i18n.T("cmd.discovery.btn.save"), func() { onDone() })
				return form
			}

			form.AddTextView("Info",
				i18n.T("cmd.discovery.pool.explanation"), 0, 4, false, false)

			poolEnabled := make([]bool, len(poolLabels))
			for i := range poolLabels {
				poolEnabled[i] = true
				idx := i
				form.AddCheckbox(poolLabels[i], true, func(checked bool) {
					poolEnabled[idx] = checked
				})
			}

			form.AddButton(i18n.T("cmd.discovery.btn.save"), func() {
				selectedPool = nil
				for i, l := range poolLabels {
					if poolEnabled[i] {
						selectedPool = append(selectedPool, l)
					}
				}
				onDone()
			})
			return form
		},
		OnDone: func() error {
			// ── Save everything to team-state config ────────────
			cfg, err := repo.LoadConfig()
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("tui.team.sync.config_load_error"), err)
			}

			// Tracker connection settings
			cfg.Tracker.Type = trackerType
			cfg.Tracker.Enabled = true
			if trackerURL != "" {
				cfg.Tracker.TrackerURL = trackerURL
			}
			cfg.Tracker.TrackerProject = projectID

			// Board columns
			cfg.Board.Columns = finalColumns

			// Label/status mappings
			if trackerType == "gitlab" {
				if cfg.Tracker.LabelStatusMapping == nil {
					cfg.Tracker.LabelStatusMapping = make(map[string]string)
				}
				for _, m := range suggestedMappings {
					cfg.Tracker.LabelStatusMapping[m.Source] = m.ColumnID
				}
			} else {
				// Jira: use both StatusMapping and LabelStatusMapping
				if cfg.Tracker.StatusMapping == nil {
					cfg.Tracker.StatusMapping = make(map[string]string)
				}
				if cfg.Tracker.LabelStatusMapping == nil {
					cfg.Tracker.LabelStatusMapping = make(map[string]string)
				}
				for _, m := range suggestedMappings {
					if m.Origin == "status" {
						cfg.Tracker.StatusMapping[m.Source] = m.ColumnID
					} else {
						cfg.Tracker.LabelStatusMapping[m.Source] = m.ColumnID
					}
				}
			}

			// Pool labels
			if len(selectedPool) > 0 {
				cfg.Tracker.AutoPlanUnassigned = true
				cfg.Tracker.UnassignedLabels = selectedPool
			}

			// Save config and commit
			if err := repo.SaveConfig(ctx, cfg); err != nil {
				return fmt.Errorf("%s: %w", i18n.T("tui.team.board_config.save_error_label"), err)
			}

			// Build TOML preview for the summary screen.
			type previewColumn struct {
				ID   string `toml:"id"`
				Name string `toml:"name"`
				Role string `toml:"role,omitempty"`
			}
			type previewBoard struct {
				Columns []previewColumn `toml:"columns"`
			}
			type previewTracker struct {
				LabelStatusMapping map[string]string `toml:"label_status_mapping,omitempty"`
				StatusMapping      map[string]string `toml:"status_mapping,omitempty"`
				AutoPlanUnassigned bool              `toml:"auto_plan_unassigned,omitempty"`
				UnassignedLabels   []string          `toml:"unassigned_labels,omitempty"`
			}
			type previewRoot struct {
				Board   previewBoard   `toml:"board"`
				Tracker previewTracker `toml:"tracker"`
			}

			cols := make([]previewColumn, len(cfg.Board.Columns))
			for i, c := range cfg.Board.Columns {
				cols[i] = previewColumn{ID: c.ID, Name: c.Name, Role: c.Role}
			}
			preview := previewRoot{
				Board: previewBoard{Columns: cols},
				Tracker: previewTracker{
					LabelStatusMapping: cfg.Tracker.LabelStatusMapping,
					StatusMapping:      cfg.Tracker.StatusMapping,
					AutoPlanUnassigned: cfg.Tracker.AutoPlanUnassigned,
					UnassignedLabels:   cfg.Tracker.UnassignedLabels,
				},
			}
			if raw, err := toml.Marshal(preview); err == nil {
				tomlPreview = strings.TrimSpace(string(raw))
			}

			return repo.CommitAndPush(ctx,
				fmt.Sprintf("tracker: discovery config for %s (%s)", projectID, trackerType),
				"config.toml")
		},
		InfoFields: func() []views.InfoField {
			fields := []views.InfoField{
				{Label: "Tracker", Value: fmt.Sprintf("%s @ %s", trackerType, trackerURL)},
				{Label: i18n.T("tui.team.discovery.info_project"), Value: projectID},
				{Label: i18n.T("tui.team.columns.header_label"), Value: fmt.Sprintf("%d", len(finalColumns))},
				{Label: "Mappings", Value: fmt.Sprintf("%d", len(suggestedMappings))},
			}
			if len(selectedPool) > 0 {
				fields = append(fields, views.InfoField{
					Label: "Pool", Value: i18n.Tf("tui.team.discovery.pool_labels_count", len(selectedPool)),
				})
			}
			if tomlPreview != "" {
				fields = append(fields, views.InfoField{
					Label: "Config TOML", Value: tomlPreview,
				})
			}
			return fields
		},
	}

	// ── Build and push the inline wizard ────────────────────────────
	wizard := views.NewInlineWizardView(views.InlineWizardConfig{
		ID:    "wizard.tracker.discovery",
		Title: i18n.T("cmd.discovery.title"),
		Steps: []views.WizardStep{
			connStep,
			discoveryStep,
			columnsStep,
			mappingStep,
			poolStep,
		},
		SummaryTargetView:  "team.board",
		SummaryTargetLabel: i18n.T("cmd.discovery.summary.target"),
	})

	tuiShell.PushView(wizard)
}

func formatSyncResultModal(r *views.SyncTrackerResult) string {
	if r == nil {
		return i18n.T("tui.team.sync.no_result")
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n", i18n.Tf("tui.team.sync.claims_created", r.ClaimsCreated))
	fmt.Fprintf(&sb, "%s\n", i18n.Tf("tui.team.sync.claims_updated", r.ClaimsUpdated))
	fmt.Fprintf(&sb, "%s\n", i18n.Tf("tui.team.sync.labels_pushed", r.LabelsPushed))

	if len(r.Projects) > 0 {
		sb.WriteString("\n" + i18n.T("tui.team.sync.projects_header") + "\n")
		for _, p := range r.Projects {
			fmt.Fprintf(&sb, "  %s\n", p)
		}
	}
	if len(r.Warnings) > 0 {
		sb.WriteString("\n" + i18n.T("tui.team.sync.warnings_header") + "\n")
		for _, w := range r.Warnings {
			fmt.Fprintf(&sb, "  ⚠ %s\n", w)
		}
	}
	if len(r.Errors) > 0 {
		sb.WriteString("\n" + i18n.T("tui.team.sync.errors_header") + "\n")
		for _, e := range r.Errors {
			fmt.Fprintf(&sb, "  ✗ %s\n", e)
		}
	}
	return sb.String()
}

// ─────────────────────────────────────────────────────────────────────────────
// Board column editor (shared helper + standalone action)
// ─────────────────────────────────────────────────────────────────────────────

// columnEditorRoles is the cycle order for the 'r' key in the column editor.
var columnEditorRoles = []string{
	teamstate.ColumnRoleInitial,
	teamstate.ColumnRoleActive,
	teamstate.ColumnRoleTerminal,
	teamstate.ColumnRoleBlocked,
}

// buildColumnEditorStep creates a WizardStep with an interactive column editor.
// The editor allows adding, removing, renaming, reordering, and cycling roles
// on board columns. When labelMappings/statusMappings are non-nil, the editor
// also supports a "mappings mode" (toggled via 'm') for managing the label→column
// translation rules.
//
// Used by both the discovery wizard (step 3, mappings=nil) and the standalone
// board config action (mappings loaded from team-state).
func buildColumnEditorStep(
	columns *[]teamstate.BoardColumnConfig,
	result *[]teamstate.BoardColumnConfig,
	labelMappings *map[string]string, //nolint:gocritic // ptrToRefParam: pointer needed to lazily initialize nil maps in the caller
	statusMappings *map[string]string, //nolint:gocritic // ptrToRefParam: pointer needed to lazily initialize nil maps in the caller
	trackerType string,
) views.WizardStep {
	return views.WizardStep{
		Label:      i18n.T("cmd.discovery.step_columns"),
		Processing: i18n.T("cmd.discovery.processing_columns"),
		CustomView: func(app *tview.Application, container *tview.Flex, onDone func()) {
			list := tview.NewList().
				ShowSecondaryText(true).
				SetHighlightFullLine(true).
				SetMainTextColor(theme.FgPrimary).
				SetSecondaryTextColor(theme.FgSecondary).
				SetSelectedBackgroundColor(theme.BgElement).
				SetSelectedTextColor(theme.FgPrimary)
			list.SetBackgroundColor(theme.BgPanel)

			hints := tview.NewTextView().SetDynamicColors(true)
			hints.SetBackgroundColor(theme.BgPanel)
			hintSep := fmt.Sprintf(" [%s]│[-] ", theme.TextMutedHex)

			// ── State ──
			hasMappings := labelMappings != nil
			var editingMappingsFor string // "" = columns mode, "col_id" = mappings mode

			// ── Helpers ──
			labelsForCol := func(colID string) (labels []string, statuses []string) {
				if labelMappings != nil {
					for k, v := range *labelMappings {
						if v == colID {
							labels = append(labels, k)
						}
					}
				}
				if statusMappings != nil {
					for k, v := range *statusMappings {
						if v == colID {
							statuses = append(statuses, k)
						}
					}
				}
				sort.Strings(labels)
				sort.Strings(statuses)
				return
			}

			truncLabels := func(items []string, max int) string {
				if len(items) == 0 {
					return "—"
				}
				if len(items) <= max {
					return strings.Join(items, ", ")
				}
				return strings.Join(items[:max], ", ") + fmt.Sprintf(" +%d", len(items)-max)
			}

			type mappingRef struct {
				kind string // "label" or "status"
				key  string
			}

			// ── Render functions ──
			var renderColumns func()
			var renderMappings func()

			updateHints := func() {
				if editingMappingsFor == "" {
					h := fmt.Sprintf("  %s↑↓[-] nav%s%sJ/K[-] %s%s%sa[-] %s%s%sEnter[-] %s%s%sd[-] %s%s%sr[-] %s",
						theme.ColorTag(theme.AccentHex), hintSep,
						theme.ColorTag(theme.AccentHex), i18n.T("tui.team.columns.hint_move"), hintSep,
						theme.ColorTag(theme.AccentHex), i18n.T("tui.team.columns.hint_add"), hintSep,
						theme.ColorTag(theme.AccentHex), i18n.T("tui.team.columns.hint_rename"), hintSep,
						theme.ColorTag(theme.AccentHex), i18n.T("tui.team.columns.hint_delete"), hintSep,
						theme.ColorTag(theme.AccentHex), i18n.T("tui.team.columns.hint_role"))
					if hasMappings {
						h += fmt.Sprintf("%s%sm[-] mappings", hintSep, theme.ColorTag(theme.AccentHex))
					}
					h += fmt.Sprintf("%s%sCtrl+S[-] %s", hintSep, theme.ColorTag(theme.AccentHex), i18n.T("tui.team.columns.hint_save"))
					hints.SetText(h)
				} else {
					hints.SetText(fmt.Sprintf("  %s↑↓[-] nav%s%sa[-] %s%s%sEnter[-] %s%s%sd[-] %s%s%sq[-] %s",
						theme.ColorTag(theme.AccentHex), hintSep,
						theme.ColorTag(theme.AccentHex), i18n.T("tui.team.columns.hint_add_label"), hintSep,
						theme.ColorTag(theme.AccentHex), i18n.T("tui.team.columns.hint_rename"), hintSep,
						theme.ColorTag(theme.AccentHex), i18n.T("tui.team.columns.hint_delete"), hintSep,
						theme.ColorTag(theme.AccentHex), i18n.T("tui.team.columns.hint_back_columns")))
				}
			}

			renderColumns = func() {
				sel := list.GetCurrentItem()
				list.Clear()

				headerText := fmt.Sprintf("  %s── %s (%d) ──%s",
					theme.ColorTag(theme.AccentHex), i18n.T("tui.team.columns.header_label"), len(*columns), theme.TagColor)
				list.AddItem(headerText, "", 0, nil)

				activeIdx := 0
				for i, col := range *columns {
					role := col.Role
					if role == "" {
						role = teamstate.ColumnRoleActive
					}
					color := views.ResolveColumnColor(col, activeIdx)
					if role == teamstate.ColumnRoleActive || col.Role == "" {
						activeIdx++
					}
					colorDot := widgets.ColorTag(color) + "●" + "[-]"
					roleHex := columnRoleHex(role)
					roleBadge := fmt.Sprintf("[black:%s] %s [-:-]", roleHex, role)
					posIndicator := fmt.Sprintf("[%s]%d/%d[-]", theme.TextMutedHex, i+1, len(*columns))

					mainText := fmt.Sprintf("  %s  [::b]%s[-:-:-]  %s  %s",
						colorDot, col.Name, roleBadge, posIndicator)

					colorName := columnColorName(col, color)
					secParts := []string{
						fmt.Sprintf("id: %s", col.ID),
						fmt.Sprintf(i18n.T("tui.team.columns.color_label"), colorName),
					}
					if hasMappings {
						labels, statuses := labelsForCol(col.ID)
						all := make([]string, 0, len(labels)+len(statuses))
						all = append(all, labels...)
						all = append(all, statuses...)
						secParts = append(secParts, fmt.Sprintf(i18n.T("tui.team.columns.labels_label"), truncLabels(all, 3)))
					}
					secondaryText := fmt.Sprintf("      [%s]%s[-]",
						theme.TextMutedHex, strings.Join(secParts, " · "))

					list.AddItem(mainText, secondaryText, 0, nil)
				}

				list.AddItem("", "", 0, nil)
				legendText := fmt.Sprintf("  %s── %s ──%s",
					theme.ColorTag(theme.AccentHex), i18n.T("tui.team.columns.roles_header"), theme.TagColor)
				list.AddItem(legendText, "", 0, nil)
				legendLine := fmt.Sprintf("    [%s]●[-] %s  [%s]●[-] %s  [%s]●[-] %s  [%s]●[-] %s",
					theme.WarningHex, i18n.T("tui.team.columns.role_initial_desc"),
					theme.AccentHex, i18n.T("tui.team.columns.role_active_desc"),
					theme.SuccessHex, i18n.T("tui.team.columns.role_terminal_desc"),
					theme.ErrorHex, i18n.T("tui.team.columns.role_blocked_desc"))
				list.AddItem(legendLine,
					fmt.Sprintf("      [%s]%s[-]", theme.TextMutedHex, i18n.T("tui.team.columns.roles_explanation")),
					0, nil)

				maxSel := len(*columns)
				if sel < 1 {
					sel = 1
				}
				if sel > maxSel {
					sel = maxSel
				}
				list.SetCurrentItem(sel)
				updateHints()
			}

			renderMappings = func() {
				list.Clear()

				colName := editingMappingsFor
				for _, c := range *columns {
					if c.ID == editingMappingsFor {
						colName = c.Name
						break
					}
				}

				headerText := fmt.Sprintf("  %s── %s → %s ──%s",
					theme.ColorTag(theme.AccentHex), i18n.T("tui.team.columns.mappings_title"), colName, theme.TagColor)
				list.AddItem(headerText, "", 0, nil)

				labels, statuses := labelsForCol(editingMappingsFor)

				// Labels section
				labelHeader := fmt.Sprintf("    %s── %s ──%s",
					theme.ColorTag(theme.TextMutedHex), i18n.T("tui.team.columns.labels_section"), theme.TagColor)
				list.AddItem(labelHeader, "", 0, nil)

				if len(labels) == 0 {
					list.AddItem(
						fmt.Sprintf("    [%s]%s[-]", theme.TextMutedHex, i18n.T("tui.team.columns.no_label_mapped")),
						fmt.Sprintf("      [%s]%s[-]", theme.TextMutedHex, i18n.T("tui.team.columns.press_a_to_add")),
						0, nil)
				}
				for _, l := range labels {
					list.AddItem(
						fmt.Sprintf("    %s▸[-] %s", theme.ColorTag(theme.AccentHex), l),
						fmt.Sprintf("      [%s]→ %s[-]", theme.TextMutedHex, editingMappingsFor),
						0, nil)
				}

				// Statuses section (Jira only)
				if trackerType == "jira" && statusMappings != nil {
					statusHeader := fmt.Sprintf("    %s── Statuses (Jira) ──%s",
						theme.ColorTag(theme.TextMutedHex), theme.TagColor)
					list.AddItem(statusHeader, "", 0, nil)

					if len(statuses) == 0 {
						list.AddItem(
							fmt.Sprintf("    [%s]%s[-]", theme.TextMutedHex, i18n.T("tui.team.columns.no_status_mapped")),
							fmt.Sprintf("      [%s]%s[-]", theme.TextMutedHex, i18n.T("tui.team.columns.press_a_to_add")),
							0, nil)
					}
					for _, s := range statuses {
						list.AddItem(
							fmt.Sprintf("    %s▸[-] %s", theme.ColorTag(theme.InfoHex), s),
							fmt.Sprintf("      [%s]→ %s[-]", theme.TextMutedHex, editingMappingsFor),
							0, nil)
					}
				}

				// Select first mapping item (skip headers)
				total := list.GetItemCount()
				sel := 1
				for sel < total {
					main, _ := list.GetItemText(sel)
					if strings.Contains(main, "──") || main == "" {
						sel++
					} else {
						break
					}
				}
				if sel >= total {
					sel = 0
				}
				list.SetCurrentItem(sel)
				updateHints()
			}

			// ── Mapping item resolution ──
			resolveMappingItem := func() *mappingRef {
				if editingMappingsFor == "" {
					return nil
				}
				idx := list.GetCurrentItem()
				main, _ := list.GetItemText(idx)
				if strings.Contains(main, "──") || strings.Contains(main, i18n.T("tui.team.columns.no_label_mapped")) || strings.Contains(main, i18n.T("tui.team.columns.no_status_mapped")) || main == "" {
					return nil
				}
				// Strip tview color tags to get the raw name
				name := main
				for strings.Contains(name, "[") && strings.Contains(name, "]") {
					start := strings.Index(name, "[")
					end := strings.Index(name, "]")
					if end > start {
						name = name[:start] + name[end+1:]
					} else {
						break
					}
				}
				name = strings.TrimPrefix(name, "▸ ")
				name = strings.TrimSpace(name)
				if name == "" {
					return nil
				}
				if labelMappings != nil {
					for k, v := range *labelMappings {
						if k == name && v == editingMappingsFor {
							return &mappingRef{kind: "label", key: k}
						}
					}
				}
				if statusMappings != nil {
					for k, v := range *statusMappings {
						if k == name && v == editingMappingsFor {
							return &mappingRef{kind: "status", key: k}
						}
					}
				}
				return nil
			}

			isHeaderLine := func(listIdx int) bool {
				main, _ := list.GetItemText(listIdx)
				return strings.Contains(main, "──") || main == "" || strings.Contains(main, i18n.T("tui.team.columns.no_label_mapped")) || strings.Contains(main, i18n.T("tui.team.columns.no_status_mapped"))
			}

			renderColumns()

			// ── Key handler ─────────────────────────────────────────
			list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				// ═══════════════════════════════════════════════════
				// MODE: Mappings
				// ═══════════════════════════════════════════════════
				if editingMappingsFor != "" {
					switch {
					case event.Rune() == 'q':
						editingMappingsFor = ""
						renderColumns()
						return nil

					case event.Key() == tcell.KeyDown || event.Rune() == 'j':
						cur := list.GetCurrentItem()
						total := list.GetItemCount()
						next := cur + 1
						for next < total && isHeaderLine(next) {
							next++
						}
						if next < total {
							list.SetCurrentItem(next)
						}
						return nil

					case event.Key() == tcell.KeyUp || event.Rune() == 'k':
						cur := list.GetCurrentItem()
						prev := cur - 1
						for prev >= 0 && isHeaderLine(prev) {
							prev--
						}
						if prev >= 0 {
							list.SetCurrentItem(prev)
						}
						return nil

					case event.Rune() == 'a':
						if tuiShell != nil {
							tuiShell.ShowInputModal(i18n.T("tui.team.columns.label_name_prompt"), "", func(name string) {
								if name == "" {
									return
								}
								name = strings.TrimSpace(name)
								if labelMappings != nil {
									if *labelMappings == nil {
										*labelMappings = make(map[string]string)
									}
									(*labelMappings)[name] = editingMappingsFor
								}
								renderMappings()
								if tuiShell != nil {
									tuiShell.ShowToast(i18n.Tf("tui.team.columns.label_added", name, editingMappingsFor), shell.ToastSuccess)
								}
							})
						}
						return nil

					case event.Key() == tcell.KeyEnter:
						ref := resolveMappingItem()
						if ref != nil && tuiShell != nil {
							tuiShell.ShowInputModal(i18n.T("tui.team.columns.rename_prompt"), ref.key, func(newName string) {
								if newName == "" || newName == ref.key {
									return
								}
								newName = strings.TrimSpace(newName)
								if ref.kind == "label" && labelMappings != nil {
									delete(*labelMappings, ref.key)
									(*labelMappings)[newName] = editingMappingsFor
								} else if ref.kind == "status" && statusMappings != nil {
									delete(*statusMappings, ref.key)
									(*statusMappings)[newName] = editingMappingsFor
								}
								renderMappings()
							})
						}
						return nil

					case event.Rune() == 'd':
						ref := resolveMappingItem()
						if ref != nil {
							if ref.kind == "label" && labelMappings != nil {
								delete(*labelMappings, ref.key)
							} else if ref.kind == "status" && statusMappings != nil {
								delete(*statusMappings, ref.key)
							}
							renderMappings()
							if tuiShell != nil {
								tuiShell.ShowToast(i18n.Tf("tui.team.columns.mapping_deleted", ref.key), shell.ToastSuccess)
							}
						}
						return nil
					}
					return event
				}

				// ═══════════════════════════════════════════════════
				// MODE: Columns
				// ═══════════════════════════════════════════════════
				idx := list.GetCurrentItem() - 1
				cols := *columns
				n := len(cols)
				isOnColumn := idx >= 0 && idx < n

				switch {
				case event.Key() == tcell.KeyDown || event.Rune() == 'j':
					cur := list.GetCurrentItem()
					if cur < n {
						list.SetCurrentItem(cur + 1)
					}
					return nil

				case event.Key() == tcell.KeyUp || event.Rune() == 'k':
					cur := list.GetCurrentItem()
					if cur > 1 {
						list.SetCurrentItem(cur - 1)
					}
					return nil

				case (event.Key() == tcell.KeyUp && event.Modifiers()&tcell.ModShift != 0) || event.Rune() == 'K':
					if isOnColumn && idx > 0 {
						cols[idx], cols[idx-1] = cols[idx-1], cols[idx]
						renderColumns()
						list.SetCurrentItem(idx)
						if tuiShell != nil {
							tuiShell.ShowToast(i18n.T("tui.team.columns.moved_up"), shell.ToastSuccess)
						}
					}
					return nil

				case (event.Key() == tcell.KeyDown && event.Modifiers()&tcell.ModShift != 0) || event.Rune() == 'J':
					if isOnColumn && idx < n-1 {
						cols[idx], cols[idx+1] = cols[idx+1], cols[idx]
						renderColumns()
						list.SetCurrentItem(idx + 2)
						if tuiShell != nil {
							tuiShell.ShowToast(i18n.T("tui.team.columns.moved_down"), shell.ToastSuccess)
						}
					}
					return nil

				case event.Rune() == 'a':
					if tuiShell != nil {
						tuiShell.ShowInputModal(i18n.T("cmd.discovery.field.column_name"), "", func(name string) {
							if name == "" {
								return
							}
							id := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "_"))
							role := tracker.SuggestColumnRole(name)
							newCol := teamstate.BoardColumnConfig{ID: id, Name: strings.TrimSpace(name), Role: role}
							pos := idx + 1
							if pos < 0 {
								pos = 0
							}
							if pos > n {
								pos = n
							}
							updated := make([]teamstate.BoardColumnConfig, 0, n+1)
							updated = append(updated, cols[:pos]...)
							updated = append(updated, newCol)
							updated = append(updated, cols[pos:]...)
							*columns = updated
							renderColumns()
							list.SetCurrentItem(pos + 1)
							tuiShell.ShowToast(i18n.Tf("tui.team.columns.column_added", newCol.Name), shell.ToastSuccess)
						})
					}
					return nil

				case event.Key() == tcell.KeyEnter:
					if isOnColumn && tuiShell != nil {
						tuiShell.ShowInputModal(i18n.T("cmd.discovery.field.column_rename"), cols[idx].Name, func(name string) {
							if name != "" {
								(*columns)[idx].Name = strings.TrimSpace(name)
								renderColumns()
							}
						})
					}
					return nil

				case event.Rune() == 'd':
					if !isOnColumn {
						return nil
					}
					if n <= 2 {
						if tuiShell != nil {
							tuiShell.ShowToast(i18n.T("cmd.discovery.validate.min_columns"), shell.ToastError)
						}
						return nil
					}
					deletedID := cols[idx].ID
					deletedName := cols[idx].Name
					copy(cols[idx:], cols[idx+1:])
					*columns = cols[:len(cols)-1]
					cleaned := 0
					if labelMappings != nil {
						for k, v := range *labelMappings {
							if v == deletedID {
								delete(*labelMappings, k)
								cleaned++
							}
						}
					}
					if statusMappings != nil {
						for k, v := range *statusMappings {
							if v == deletedID {
								delete(*statusMappings, k)
								cleaned++
							}
						}
					}
					renderColumns()
					msg := i18n.Tf("tui.team.columns.column_deleted", deletedName)
					if cleaned > 0 {
						msg += i18n.Tf("tui.team.columns.mappings_cleaned", cleaned)
					}
					if tuiShell != nil {
						tuiShell.ShowToast(msg, shell.ToastSuccess)
					}
					return nil

				case event.Rune() == 'r':
					if isOnColumn {
						current := cols[idx].Role
						if current == "" {
							current = teamstate.ColumnRoleActive
						}
						nextIdx := 0
						for i, r := range columnEditorRoles {
							if r == current {
								nextIdx = (i + 1) % len(columnEditorRoles)
								break
							}
						}
						(*columns)[idx].Role = columnEditorRoles[nextIdx]
						renderColumns()
					}
					return nil

				case event.Rune() == 'm':
					if hasMappings && isOnColumn {
						editingMappingsFor = cols[idx].ID
						renderMappings()
					}
					return nil

				case event.Key() == tcell.KeyCtrlS:
					if len(*columns) < 2 {
						if tuiShell != nil {
							tuiShell.ShowToast(i18n.T("cmd.discovery.validate.min_columns"), shell.ToastError)
						}
						return nil
					}
					hasTerminal := false
					for _, c := range *columns {
						if c.Role == teamstate.ColumnRoleTerminal {
							hasTerminal = true
							break
						}
					}
					if !hasTerminal {
						if tuiShell != nil {
							tuiShell.ShowToast(i18n.T("cmd.discovery.validate.need_terminal"), shell.ToastError)
						}
						return nil
					}
					*result = make([]teamstate.BoardColumnConfig, len(*columns))
					copy(*result, *columns)
					onDone()
					return nil
				}

				return event
			})

			container.AddItem(list, 0, 1, true)
			container.AddItem(hints, 1, 0, false)
			app.SetFocus(list)
		},
		Validate: func() string {
			if len(*result) < 2 {
				return i18n.T("cmd.discovery.validate.min_columns")
			}
			hasTerminal := false
			for _, c := range *result {
				if c.Role == teamstate.ColumnRoleTerminal {
					hasTerminal = true
					break
				}
			}
			if !hasTerminal {
				return i18n.T("cmd.discovery.validate.need_terminal")
			}
			return ""
		},
		OnDone: func() error {
			return nil
		},
		InfoFields: func() []views.InfoField {
			names := make([]string, len(*result))
			for i, c := range *result {
				names[i] = c.Name
			}
			return []views.InfoField{
				{Label: i18n.T("tui.team.columns.header_label"), Value: i18n.Tf("cmd.discovery.info.columns_selected", len(*result))},
				{Label: "Layout", Value: strings.Join(names, " → ")},
			}
		},
	}
}

// ── Column editor helpers ───────────────────────────────────────────────────

// columnRoleHex returns the hex background color for a role badge.
func columnRoleHex(role string) string {
	switch role {
	case teamstate.ColumnRoleInitial:
		return theme.WarningHex
	case teamstate.ColumnRoleTerminal:
		return theme.SuccessHex
	case teamstate.ColumnRoleBlocked:
		return theme.ErrorHex
	default:
		return theme.AccentHex
	}
}

// columnColorName returns a human-readable color description.
func columnColorName(col teamstate.BoardColumnConfig, resolved tcell.Color) string {
	if col.Color != "" {
		return col.Color
	}
	r, g, b := resolved.RGB()
	return fmt.Sprintf("auto (#%02x%02x%02x)", r, g, b)
}

// ─────────────────────────────────────────────────────────────────────────────
// Board column config action (standalone, no tracker required)
// ─────────────────────────────────────────────────────────────────────────────

func actionBoardColumnConfig() {
	if tuiShell == nil {
		return
	}

	a := MustApp()
	ctx := tuiShell.Context()

	// Resolve the active team
	activeTeam := a.Config.ActiveTeam()
	if !activeTeam.Enabled {
		tuiShell.ShowToast(i18n.T("tui.team.board_config.no_team"), shell.ToastError)
		return
	}

	statePath := activeTeam.StatePath
	if statePath == "" {
		statePath = config.TeamStatePath(activeTeam.StateRepo)
	}
	repo := teamstate.NewRepo(activeTeam.StateRepo, statePath)
	if !repo.IsCloned() {
		tuiShell.ShowToast(i18n.T("tui.team.board_config.not_cloned"), shell.ToastError)
		return
	}

	// Load current board config
	cfg, err := repo.LoadConfig()
	if err != nil {
		tuiShell.ShowToast(i18n.T("tui.team.board_config.config_load_error")+err.Error(), shell.ToastError)
		return
	}

	// Use existing columns or defaults
	var columns []teamstate.BoardColumnConfig
	if cfg.Board.HasCustomColumns() {
		columns = make([]teamstate.BoardColumnConfig, len(cfg.Board.Columns))
		copy(columns, cfg.Board.Columns)
	} else {
		def := teamstate.DefaultBoardConfig()
		columns = make([]teamstate.BoardColumnConfig, len(def.Columns))
		copy(columns, def.Columns)
	}

	// Load label/status mappings for the integrated editor
	labelMappings := cfg.Tracker.LabelStatusMapping
	if labelMappings == nil {
		labelMappings = make(map[string]string)
	}
	statusMappings := cfg.Tracker.StatusMapping
	if statusMappings == nil {
		statusMappings = make(map[string]string)
	}
	tType := cfg.Tracker.Type

	var finalColumns []teamstate.BoardColumnConfig

	editorStep := buildColumnEditorStep(&columns, &finalColumns, &labelMappings, &statusMappings, tType)

	wizard := views.NewInlineWizardView(views.InlineWizardConfig{
		ID:    "wizard.board.columns",
		Title: i18n.T("tui.tm.item.board_config"),
		Steps: []views.WizardStep{editorStep},
		OnComplete: func(completed bool, wizErr error) {
			if !completed || wizErr != nil {
				return
			}
			// Save columns + mappings to team-state config
			go func() {
				freshCfg, err := repo.LoadConfig()
				if err != nil {
					tuiShell.App().QueueUpdateDraw(func() {
						tuiShell.ShowToast(i18n.T("tui.team.board_config.error_prefix")+err.Error(), shell.ToastError)
					})
					return
				}
				freshCfg.Board.Columns = finalColumns
				freshCfg.Tracker.LabelStatusMapping = labelMappings
				freshCfg.Tracker.StatusMapping = statusMappings
				if err := repo.SaveConfig(ctx, freshCfg); err != nil {
					tuiShell.App().QueueUpdateDraw(func() {
						tuiShell.ShowToast(i18n.T("tui.team.board_config.save_error")+err.Error(), shell.ToastError)
					})
					return
				}
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowToast(
						i18n.Tf("tui.team.board_config.saved", theme.IconSuccess, len(finalColumns)),
						shell.ToastSuccess,
					)
				})
			}()
		},
	})

	tuiShell.PushView(wizard)
}
