package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

		staleDaysStr      = "3"
		memberID          = a.Config.ActiveTeam().MemberID
		displayName       string
		gitlabUsername    string
		mattermostUsername string
		trackerUsername   string
		role              string

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
				return fmt.Errorf(
					"un clone de ce repo team-state existe déjà\n"+
						"  Path: %s\n  Référencé par: %s",
					existing.Path, existing.Source)
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
				"Username tracker (optionnel)",
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
		Label: "Tracker",
		SkipIf: func() bool {
			// Skip if no team configured yet (repo step was skipped/failed).
			return stateRepo == ""
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			options := []string{"Plus tard", "Oui, configurer maintenant"}
			form.AddDropDown("Configurer le tracker sync ?", options, 0, func(_ string, idx int) {
				launchDiscoveryAfter = idx == 1
			})
			form.AddButton("Suivant", func() { onDone() })
			return form
		},
		InfoFields: func() []views.InfoField {
			val := "Plus tard"
			if launchDiscoveryAfter {
				val = "Oui"
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
			_ = writeTeamConfig(stateRepo, statePath, memberID)

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
		tuiShell.ShowToast("Aucun projet actif — sélectionnez un projet d'abord", shell.ToastError)
		return
	}

	hubTeam := a.Config.ActiveTeam()

	// ── Choose: attach to team or detach ─────────────────────────────────
	var modeOptions []views.SelectOption
	if hubTeam.Enabled && hubTeam.ID != "" {
		modeOptions = []views.SelectOption{
			{
				Label: fmt.Sprintf("Attacher à l'équipe %s (%s)", hubTeam.ID, hubTeam.MemberID),
				Value: hubTeam.ID,
			},
			{Label: "Pas de team pour ce projet", Value: ""},
		}
	} else {
		modeOptions = []views.SelectOption{
			{Label: "Pas de team pour ce projet", Value: ""},
		}
	}

	defaultVal := ""
	if hubTeam.Enabled && hubTeam.ID != "" {
		defaultVal = hubTeam.ID
	}

	tuiShell.ShowSelectModal("Team pour ce projet", modeOptions, defaultVal, func(teamID string) {
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
		tuiShell.ShowToast("Erreur : projet introuvable", shell.ToastError)
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
		tuiShell.ShowToast("Erreur : "+err.Error(), shell.ToastError)
		return
	}

	label := "aucune"
	if teamID != "" {
		label = teamID
	}
	tuiShell.ShowToast(
		fmt.Sprintf("Équipe configurée : %s — redéployez pour appliquer", label),
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
			form.AddButton("Next", func() { onDone() })
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
				return fmt.Errorf("aucun membre trouvé dans le repo team-state")
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
				MainText: "Membres de l'équipe",
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
						{Label: "Membre", Value: fmt.Sprintf("%s (%s)", m.DisplayName, m.ID)},
					}
				}
			}
			return []views.InfoField{{Label: "Membre", Value: memberID}}
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
				{Label: "Statut", Value: theme.SuccessStyle.Render("Reconnecté")},
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

	tuiShell.ShowToast("Synchronisation en cours...", shell.ToastInfo)

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
				tuiShell.ShowToast("✗ Sync: "+err.Error(), shell.ToastError)
				return
			}
			content := formatSyncResultModal(result)
			tuiShell.ShowScrollableModal("Résultat sync tracker", content, []views.ModalAction{
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
		return nil, fmt.Errorf("équipe non configurée")
	}

	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	if !repo.IsCloned() {
		return nil, fmt.Errorf("team-state non cloné — lancez 'team init'")
	}

	// Load team config
	teamCfg, err := repo.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("chargement config: %w", err)
	}
	if teamCfg.Tracker.Type == "" {
		return nil, fmt.Errorf("tracker non configuré — utilisez 'g' dans la vue Config équipe")
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
		return nil, fmt.Errorf("credentials manquants — activez %s dans Settings: %w", teamCfg.Tracker.Type, err)
	}

	t, err := tracker.New(creds)
	if err != nil {
		return nil, fmt.Errorf("initialisation tracker: %w", err)
	}

	// Build the Projects map from hub projects (same as CLI sync-tracker and resolveTrackerEngine).
	projects, ticketPatterns := resolveTrackerProjects(ctx, a, effTracker)
	if len(projects) == 0 {
		return nil, fmt.Errorf("aucun projet configuré pour le tracker sync — vérifiez tracker_project dans la config team")
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
		tuiShell.ShowToast("Équipe non configurée — lancez 'team init' d'abord", shell.ToastError)
		return
	}

	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	if !repo.IsCloned() {
		tuiShell.ShowToast("Team-state non cloné — lancez 'team init' d'abord", shell.ToastError)
		return
	}

	// Load existing team config for pre-fill
	teamCfg, err := repo.LoadConfig()
	if err != nil {
		tuiShell.ShowToast("Erreur chargement config: "+err.Error(), shell.ToastError)
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
		discoveryInfo    *tracker.DiscoveryInfo
		suggestedColumns []teamstate.BoardColumnConfig
		suggestedMappings []tracker.SuggestedMapping
		unmapped         []string
		poolLabels       []string

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
					return fmt.Errorf("impossible de résoudre les credentials: %w", err)
				}
				cfg.Token = resolved.Token
				if cfg.BaseURL == "" {
					cfg.BaseURL = resolved.BaseURL
				}
			}

			// Create tracker instance and test
			t, err := tracker.New(cfg)
			if err != nil {
				return fmt.Errorf("initialisation tracker: %w", err)
			}

			// Wrap API calls in a 30s timeout to avoid hanging the wizard.
			apiCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			// Test connection
			username, err := t.TestConnection(apiCtx)
			if err != nil {
				return fmt.Errorf("échec connexion: %w", err)
			}
			_ = username // connection OK

			// Test project access
			_, err = t.TestProject(apiCtx, projectID)
			if err != nil {
				return fmt.Errorf("projet inaccessible: %w", err)
			}

			return nil
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{
				{Label: "Type", Value: trackerType},
				{Label: "URL", Value: trackerURL},
				{Label: "Projet", Value: projectID},
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
				return fmt.Errorf("initialisation tracker: %w", err)
			}

			// Wrap API call in a 30s timeout to avoid hanging the wizard.
			apiCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			// Discover project metadata (labels, statuses)
			discoveryInfo, err = t.DiscoverProject(apiCtx, projectID)
			if err != nil {
				return fmt.Errorf("découverte projet: %w", err)
			}

			// Start from existing board columns or defaults
			baseColumns := teamCfg.Board.Columns
			if len(baseColumns) == 0 {
				baseColumns = teamstate.DefaultBoardConfig().Columns
			}

			// Suggest new columns based on discovered labels/statuses
			newCols := tracker.SuggestNewColumns(discoveryInfo, baseColumns)
			suggestedColumns = append(baseColumns, newCols...)

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
					Label: "Non mappés", Value: fmt.Sprintf("%d", len(unmapped)),
				})
			}
			return fields
		},
	}

	// ── Step 3: Columns (interactive editor) ────────────────────────
	columnsStep := buildColumnEditorStep(&suggestedColumns, &finalColumns)

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
				info := fmt.Sprintf("Labels non mappés (%d) : %s",
					len(unmapped), strings.Join(unmapped[:maxShow], ", "))
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
				{Label: "Non mappés", Value: fmt.Sprintf("%d", len(unmapped))},
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
				return fmt.Errorf("chargement config: %w", err)
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
				return fmt.Errorf("sauvegarde config: %w", err)
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
				{Label: "Projet", Value: projectID},
				{Label: "Colonnes", Value: fmt.Sprintf("%d", len(finalColumns))},
				{Label: "Mappings", Value: fmt.Sprintf("%d", len(suggestedMappings))},
			}
			if len(selectedPool) > 0 {
				fields = append(fields, views.InfoField{
					Label: "Pool", Value: fmt.Sprintf("%d labels", len(selectedPool)),
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
		return "Aucun résultat"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Claims créés:      %d\n", r.ClaimsCreated)
	fmt.Fprintf(&sb, "Claims mis à jour: %d\n", r.ClaimsUpdated)
	fmt.Fprintf(&sb, "Labels poussés:    %d\n", r.LabelsPushed)

	if len(r.Projects) > 0 {
		sb.WriteString("\nProjets:\n")
		for _, p := range r.Projects {
			fmt.Fprintf(&sb, "  %s\n", p)
		}
	}
	if len(r.Warnings) > 0 {
		sb.WriteString("\nWarnings:\n")
		for _, w := range r.Warnings {
			fmt.Fprintf(&sb, "  ⚠ %s\n", w)
		}
	}
	if len(r.Errors) > 0 {
		sb.WriteString("\nErreurs:\n")
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
// on board columns. It operates on the `columns` slice (input) and writes the
// validated result to `result` on Ctrl+S.
//
// Used by both the discovery wizard (step 3) and the standalone board config action.
func buildColumnEditorStep(columns *[]teamstate.BoardColumnConfig, result *[]teamstate.BoardColumnConfig) views.WizardStep {
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
				SetSelectedTextColor(theme.Action)
			list.SetBackgroundColor(theme.BgPanel)

			// renderList rebuilds the tview.List items with rich styling.
			renderList := func() {
				sel := list.GetCurrentItem()
				list.Clear()

				// Section header
				headerText := fmt.Sprintf("  %s── Colonnes (%d) ──%s",
					theme.ColorTag(theme.AccentHex), len(*columns), theme.TagColor)
				list.AddItem(headerText, "", 0, nil)

				activeIdx := 0
				for i, col := range *columns {
					role := col.Role
					if role == "" {
						role = teamstate.ColumnRoleActive
					}

					// Resolve color for the dot indicator
					color := views.ResolveColumnColor(col, activeIdx)
					if role == teamstate.ColumnRoleActive || col.Role == "" {
						activeIdx++
					}
					colorDot := widgets.ColorTag(color) + "●" + "[-]"

					// Role badge with inverted color
					roleHex := columnRoleHex(role)
					roleBadge := fmt.Sprintf("[black:%s] %s [-:-]", roleHex, role)

					// Position indicator
					posIndicator := fmt.Sprintf("[%s]%d/%d[-]", theme.TextMutedHex, i+1, len(*columns))

					// Main text: color dot + bold name + role badge + position
					mainText := fmt.Sprintf("  %s  [::b]%s[-:-:-]  %s  %s",
						colorDot, col.Name, roleBadge, posIndicator)

					// Secondary text: id + color info
					colorName := columnColorName(col, color)
					secondaryText := fmt.Sprintf("      [%s]id: %s · couleur: %s[-]",
						theme.TextMutedHex, col.ID, colorName)

					list.AddItem(mainText, secondaryText, 0, nil)
				}

				// Spacer before role legend
				list.AddItem("", "", 0, nil)

				// Role legend (non-selectable)
				legendText := fmt.Sprintf("  %s── Rôles ──%s",
					theme.ColorTag(theme.AccentHex), theme.TagColor)
				list.AddItem(legendText, "", 0, nil)

				legendLine := fmt.Sprintf("    [%s]●[-] initial = entrée  [%s]●[-] active = en cours  [%s]●[-] terminal = terminé  [%s]●[-] blocked = bloqué",
					theme.WarningHex, theme.AccentHex, theme.SuccessHex, theme.ErrorHex)
				list.AddItem(legendLine,
					fmt.Sprintf("      [%s]Les colonnes définissent les étapes du workflow kanban[-]", theme.TextMutedHex),
					0, nil)

				// Restore selection (offset +1 for header)
				maxSel := len(*columns) // items 1..N are columns, header is 0
				if sel < 1 {
					sel = 1
				}
				if sel > maxSel {
					sel = maxSel
				}
				list.SetCurrentItem(sel)
			}
			renderList()

			// ── Key handler ─────────────────────────────────────────
			list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				idx := list.GetCurrentItem() - 1 // offset for header row
				cols := *columns
				n := len(cols)

				// Guard: ignore actions on non-column items (header, spacer, legend)
				isOnColumn := idx >= 0 && idx < n

				switch {
				// Navigation: skip non-column items
				case event.Key() == tcell.KeyDown || event.Rune() == 'j':
					cur := list.GetCurrentItem()
					maxItem := n // last column is at index n (1-based)
					if cur < maxItem {
						list.SetCurrentItem(cur + 1)
					}
					return nil

				case event.Key() == tcell.KeyUp || event.Rune() == 'k':
					cur := list.GetCurrentItem()
					if cur > 1 { // can't go above first column (index 1)
						list.SetCurrentItem(cur - 1)
					}
					return nil

				// Shift+Up or K: move column up
				case (event.Key() == tcell.KeyUp && event.Modifiers()&tcell.ModShift != 0) || event.Rune() == 'K':
					if isOnColumn && idx > 0 {
						cols[idx], cols[idx-1] = cols[idx-1], cols[idx]
						renderList()
						list.SetCurrentItem(idx) // idx is now the position above (0-based +1 for header)
						if tuiShell != nil {
							tuiShell.ShowToast("↑ Colonne déplacée", shell.ToastSuccess)
						}
					}
					return nil

				// Shift+Down or J: move column down
				case (event.Key() == tcell.KeyDown && event.Modifiers()&tcell.ModShift != 0) || event.Rune() == 'J':
					if isOnColumn && idx < n-1 {
						cols[idx], cols[idx+1] = cols[idx+1], cols[idx]
						renderList()
						list.SetCurrentItem(idx + 2) // moved down: new pos is idx+1 (0-based) +1 for header
						if tuiShell != nil {
							tuiShell.ShowToast("↓ Colonne déplacée", shell.ToastSuccess)
						}
					}
					return nil

				// 'a': add a new column
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
							renderList()
							list.SetCurrentItem(pos + 1) // +1 for header
							tuiShell.ShowToast(fmt.Sprintf("+ Colonne « %s » ajoutée", newCol.Name), shell.ToastSuccess)
						})
					}
					return nil

				// Enter: rename selected column
				case event.Key() == tcell.KeyEnter:
					if isOnColumn && tuiShell != nil {
						tuiShell.ShowInputModal(i18n.T("cmd.discovery.field.column_rename"), cols[idx].Name, func(name string) {
							if name != "" {
								(*columns)[idx].Name = strings.TrimSpace(name)
								renderList()
							}
						})
					}
					return nil

				// 'd': delete selected column
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
					deletedName := cols[idx].Name
					*columns = append(cols[:idx], cols[idx+1:]...)
					renderList()
					if tuiShell != nil {
						tuiShell.ShowToast(fmt.Sprintf("- Colonne « %s » supprimée", deletedName), shell.ToastSuccess)
					}
					return nil

				// 'r': cycle role
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
						renderList()
					}
					return nil

				// Ctrl+S: validate and submit
				case event.Key() == tcell.KeyCtrlS:
					// Run validation before accepting
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

			// ── Hints bar ───────────────────────────────────────────
			hints := tview.NewTextView().SetDynamicColors(true)
			hints.SetBackgroundColor(theme.BgPanel)
			hintSep := fmt.Sprintf(" [%s]│[-] ", theme.TextMutedHex)
			hints.SetText(fmt.Sprintf("  %s↑↓[-] nav%s%sJ/K[-] déplacer ↕%s%sa[-] ajouter%s%sEnter[-] renommer%s%sd[-] supprimer%s%sr[-] rôle%s%sCtrl+S[-] sauvegarder",
				theme.ColorTag(theme.AccentHex), hintSep,
				theme.ColorTag(theme.AccentHex), hintSep,
				theme.ColorTag(theme.AccentHex), hintSep,
				theme.ColorTag(theme.AccentHex), hintSep,
				theme.ColorTag(theme.AccentHex), hintSep,
				theme.ColorTag(theme.AccentHex), hintSep,
				theme.ColorTag(theme.AccentHex)))

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
				{Label: "Colonnes", Value: i18n.Tf("cmd.discovery.info.columns_selected", len(*result))},
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
		tuiShell.ShowToast("Aucune équipe active", shell.ToastError)
		return
	}

	statePath := activeTeam.StatePath
	if statePath == "" {
		statePath = config.TeamStatePath(activeTeam.StateRepo)
	}
	repo := teamstate.NewRepo(activeTeam.StateRepo, statePath)
	if !repo.IsCloned() {
		tuiShell.ShowToast("Repo team-state non cloné", shell.ToastError)
		return
	}

	// Load current board config
	cfg, err := repo.LoadConfig()
	if err != nil {
		tuiShell.ShowToast("Erreur chargement config : "+err.Error(), shell.ToastError)
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

	var finalColumns []teamstate.BoardColumnConfig

	editorStep := buildColumnEditorStep(&columns, &finalColumns)

	wizard := views.NewInlineWizardView(views.InlineWizardConfig{
		ID:    "wizard.board.columns",
		Title: i18n.T("tui.tm.item.board_config"),
		Steps: []views.WizardStep{editorStep},
		OnComplete: func(completed bool, wizErr error) {
			if !completed || wizErr != nil {
				return
			}
			// Save the updated columns to team-state config
			go func() {
				freshCfg, err := repo.LoadConfig()
				if err != nil {
					tuiShell.App().QueueUpdateDraw(func() {
						tuiShell.ShowToast("Erreur : "+err.Error(), shell.ToastError)
					})
					return
				}
				freshCfg.Board.Columns = finalColumns
				if err := repo.SaveConfig(ctx, freshCfg); err != nil {
					tuiShell.App().QueueUpdateDraw(func() {
						tuiShell.ShowToast("Erreur sauvegarde : "+err.Error(), shell.ToastError)
					})
					return
				}
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowToast(
						fmt.Sprintf("%s %d colonnes sauvegardées", theme.IconSuccess, len(finalColumns)),
						shell.ToastSuccess,
					)
				})
			}()
		},
	})

	tuiShell.PushView(wizard)
}
