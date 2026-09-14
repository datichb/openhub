package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
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

	// ── Build and push the inline wizard ────────────────────────────
	wizard := views.NewInlineWizardView(views.InlineWizardConfig{
		ID:    "wizard.team.init",
		Title: i18n.T("tui.team.init"),
		Steps: []views.WizardStep{
			repoStep, credStep, configStep, identityStep, notifStep, policiesStep,
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
