package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/components/summary"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

var teamCmd = &cobra.Command{
	Use:   "team",
	Short: i18n.T("cmd.team.short"),
	Long:  i18n.T("cmd.team.long"),
	PersistentPreRunE: teamPreRunE,
}

// teamRepo is the shared team-state repository instance, resolved once by
// teamPreRunE and reused by all team subcommands. Commands that need to work
// before the repo exists (e.g. `team init`) override PersistentPreRunE to nil.
var teamRepo *teamstate.Repo

// teamPreRunE resolves the team-state repo for all team subcommands.
// It skips resolution if the command is `team init` (repo doesn't exist yet).
func teamPreRunE(cmd *cobra.Command, _ []string) error {
	// Skip for commands that don't need an existing repo
	if cmd.Name() == "init" {
		return nil
	}
	a := MustApp()
	ctx := cmd.Context()
	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}
	teamRepo = repo
	return nil
}

var teamInitCmd = &cobra.Command{
	Use:   "init",
	Short: i18n.T("cmd.team.init.short"),
	Long:  i18n.T("cmd.team.init.long"),
	RunE:  runTeamInit,
}

var teamStatusCmd = &cobra.Command{
	Use:   "status",
	Short: i18n.T("cmd.team.status.short"),
	RunE:  runTeamStatus,
}

var teamActivityCmd = &cobra.Command{
	Use:   "activity",
	Short: i18n.T("cmd.team.activity.short"),
	RunE:  runTeamActivity,
}

func init() {
	rootCmd.AddCommand(teamCmd)
	teamCmd.AddCommand(teamInitCmd)
	teamCmd.AddCommand(teamStatusCmd)
	teamCmd.AddCommand(teamActivityCmd)

	teamActivityCmd.Flags().Bool("today", false, "Show only today's events")
	teamActivityCmd.Flags().Bool("week", false, "Show last 7 days")
	teamActivityCmd.Flags().String("member", "", "Filter by member ID")
	teamActivityCmd.Flags().String("project", "", "Filter by project")
	teamActivityCmd.Flags().Int("limit", 20, "Maximum number of events to display")

	teamStatusCmd.Flags().Bool("detail", false, i18n.T("cmd.team.status.flags.detail"))
}

func runTeamInit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	a := MustApp()

	// ══════════════════════════════════════════════════════════════════════════
	// PRE-WIZARD: Prerequisite check
	// ══════════════════════════════════════════════════════════════════════════
	if _, err := os.Stat(config.ConfigPath()); os.IsNotExist(err) {
		return fmt.Errorf("%s", i18n.Tf("cmd.team.init.hub_not_configured", theme.Bold.Render("oh init")))
	}

	// ══════════════════════════════════════════════════════════════════════════
	// PRE-WIZARD: Preamble (visible, no pause)
	// ══════════════════════════════════════════════════════════════════════════
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, theme.Title.Render("  Team Setup  "))
	fmt.Fprintln(a.IO.Out)

	preamble := fmt.Sprintf("  %s %s\n  %s %s\n  %s %s",
		theme.SuccessStyle.Render(theme.IconSuccess), i18n.T("cmd.team.init.prereq_hub"),
		theme.SuccessStyle.Render(theme.IconSuccess), i18n.T("cmd.team.init.prereq_repo"),
		theme.SuccessStyle.Render(theme.IconSuccess), i18n.T("cmd.team.init.prereq_ssh"),
	)
	fmt.Fprintln(a.IO.Out, theme.Box.Render(preamble))
	fmt.Fprintln(a.IO.Out)

	// ══════════════════════════════════════════════════════════════════════════
	// PRE-WIZARD: Repo URL + clone/pull (only when already configured)
	// ══════════════════════════════════════════════════════════════════════════
	var stateRepo string
	var statePath string
	var repo *teamstate.Repo

	// Determine local path
	statePath = a.Config.ActiveTeam().StatePath
	if statePath == "" {
		statePath = config.DefaultTeamStatePath()
	}

	// If already configured in hub.toml, clone/pull immediately (skip Step 0)
	if a.Config.ActiveTeam().StateRepo != "" {
		stateRepo = a.Config.ActiveTeam().StateRepo
		repo = teamstate.NewRepo(stateRepo, statePath)
		if repo.IsCloned() {
			if err := repo.Pull(ctx); err != nil {
				// Non-fatal: continue with local state but warn the user
				slog.Warn("team.init.pull_failed", "error", err)
				fmt.Fprintf(a.IO.ErrOut, "%s synchronisation échouée, contenu local utilisé\n",
					theme.WarningStyle.Render(theme.IconWarning))
			}
		} else {
			if err := repo.Clone(ctx); err != nil {
				return fmt.Errorf("cloning team-state: %w", err)
			}
		}
		if err := repo.InitStructure(ctx); err != nil {
			return fmt.Errorf("initializing structure: %w", err)
		}
	}

	// ══════════════════════════════════════════════════════════════════════════
	// STATE DETECTION (deferred if repo not yet cloned)
	// ══════════════════════════════════════════════════════════════════════════
	var hasConfig, hasPolicies bool
	if repo != nil {
		hasConfig = repo.HasConfig()
		hasPolicies = repo.HasPolicies()
	}

	// ══════════════════════════════════════════════════════════════════════════
	// BUILD WIZARD STEPS (tview v2 — cell-buffer rendering)
	// ══════════════════════════════════════════════════════════════════════════

	// Variables shared across steps (captured by closures)
	var (
		// Config step
		staleDaysStr string

		// Identity step
		memberID           string
		displayName        string
		gitlabUsername     string
		mattermostUsername string
		role               string

		// Notifications step
		webhookURL string
		channel    string
		botName    string

		// Policies step
		selectedPolicies []string
	)

	// Pre-fill from existing state (only if repo already cloned)
	var existingCfg *teamstate.TeamConfig
	var hasMember bool
	if repo != nil {
		existingCfg, _ = repo.LoadConfig()
		if hasConfig && existingCfg != nil {
			staleDaysStr = strconv.Itoa(existingCfg.Takeover.StaleDays)
			webhookURL = existingCfg.Notification.MattermostWebhook
			channel = existingCfg.Notification.Channel
			botName = existingCfg.Notification.BotName
		} else {
			staleDaysStr = "3"
			botName = "OpenHub"
		}

		// Pre-fill member ID from hub.toml if available
		if a.Config.ActiveTeam().MemberID != "" {
			memberID = a.Config.ActiveTeam().MemberID
		}

		// Pre-fill member profile if exists
		hasMember = memberID != "" && repo.HasMember(memberID)
		if hasMember {
			existing, err := repo.GetMember(memberID)
			if err == nil && existing != nil {
				displayName = existing.DisplayName
				gitlabUsername = existing.GitLabUsername
				mattermostUsername = existing.MattermostUsername
				role = existing.Role
			}
		}
	} else {
		staleDaysStr = "3"
		botName = "OpenHub"
		if a.Config.ActiveTeam().MemberID != "" {
			memberID = a.Config.ActiveTeam().MemberID
		}
	}

	// ── Step 0: Repo team-state ──
	repoStep := views.WizardStep{
		Label:      i18n.T("cmd.team.init.step_repo"),
		Processing: i18n.T("cmd.team.init.processing_repo"),
		Required:   true,
		SkipIf: func() bool {
			// Skip if repo URL already configured (clone/pull done above)
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
			form.AddButton("Next", func() {
				onDone()
			})
			return form
		},
		OnDone: func() error {
			// ── Duplicate clone check ─────────────────────────────────────────
			// Before cloning, verify that no other clone of this remote already
			// exists locally. Two clones of the same team-state repo will diverge
			// and cause inconsistent state. We block and offer to reuse instead.
			if existing, found := findExistingCloneForRemote(ctx, a, stateRepo); found {
				// Propose reusing the existing clone.
				// The WizardStep OnDone runs on the tview event loop so we cannot
				// use fmt.Scanln. Return a structured error that the wizard renders.
				return fmt.Errorf(
					"un clone de ce repo team-state existe déjà\n"+
						"  Path: %s\n"+
						"  Référencé par: %s\n\n"+
						"Pour réutiliser ce clone, configurez state_path = %q\n"+
						"dans la config projet (oh deploy) plutôt que d'en créer un nouveau.\n"+
						"Si ce clone est obsolète, supprimez-le d'abord: rm -rf %s",
					existing.Path, existing.Source, existing.Path, existing.Path,
				)
			}

			// Clone or pull the repo
			repo = teamstate.NewRepo(stateRepo, statePath)
			if repo.IsCloned() {
				if err := repo.Pull(ctx); err != nil {
					// Non-fatal: continue with local state
					_ = err
				}
			} else {
				if err := repo.Clone(ctx); err != nil {
					return fmt.Errorf("cloning team-state: %w", err)
				}
			}
			if err := repo.InitStructure(ctx); err != nil {
				return fmt.Errorf("initializing structure: %w", err)
			}

			// Now that repo is available, pre-fill state for subsequent steps
			hasConfig = repo.HasConfig()
			hasPolicies = repo.HasPolicies()
			existingCfg, _ = repo.LoadConfig()
			if hasConfig && existingCfg != nil {
				staleDaysStr = strconv.Itoa(existingCfg.Takeover.StaleDays)
				webhookURL = existingCfg.Notification.MattermostWebhook
				channel = existingCfg.Notification.Channel
				botName = existingCfg.Notification.BotName
			}
			hasMember = memberID != "" && repo.HasMember(memberID)
			if hasMember {
				existing, err := repo.GetMember(memberID)
				if err == nil && existing != nil {
					displayName = existing.DisplayName
					gitlabUsername = existing.GitLabUsername
					mattermostUsername = existing.MattermostUsername
					role = existing.Role
				}
			}
			return nil
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{
				{Label: "Repo", Value: stateRepo},
			}
		},
	}

	// ── Step 1: Config globale ──
	configStep := views.WizardStep{
		Label:      i18n.T("cmd.team.init.step_config"),
		Processing: i18n.T("cmd.team.init.processing_config"),
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			form.AddInputField(
				i18n.T("cmd.team.init.config_stale_days"),
				staleDaysStr, 0, nil,
				func(text string) { staleDaysStr = text })
			form.AddButton("Next", func() { onDone() })
			return form
		},
		OnDone: func() error {
			days, err := strconv.Atoi(staleDaysStr)
			if err != nil || days <= 0 {
				days = 3
			}
			if hasConfig && existingCfg != nil {
				// Only save if changed
				if days == existingCfg.Takeover.StaleDays {
					return nil
				}
				existingCfg.Takeover.StaleDays = days
				if err := repo.SaveConfig(ctx, existingCfg); err != nil {
					return err
				}
			} else {
				cfg := &teamstate.TeamConfig{
					Notification: teamstate.NotificationConfig{
						Enabled: false,
						BotName: "OpenHub",
					},
					Takeover: teamstate.TakeoverConfig{
						StaleDays: days,
					},
					Parallel: teamstate.ParallelConfig{
						MaxSessions:    3,
						PortRangeStart: 4100,
						AutoMergeBeads: true,
					},
				}
				if err := repo.SaveConfig(ctx, cfg); err != nil {
					return err
				}
			}
			return nil
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{
				{Label: "Stale days", Value: staleDaysStr},
			}
		},
	}

	// ── Step 2: Identité ──
	var identityStep views.WizardStep
	if !hasMember {
		identityStep = views.WizardStep{
			Label:      i18n.T("cmd.team.init.step_identity"),
			Processing: i18n.T("cmd.team.init.processing_identity"),
			Validate: func() string {
				if strings.TrimSpace(memberID) == "" {
					return i18n.T("cmd.team.init.validate.member_id_required")
				}
				return ""
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddInputField(
					i18n.T("cmd.team.init.identity_id"),
					memberID, 0, nil,
					func(text string) { memberID = text })
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
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				member := teamstate.Member{
					ID:                 memberID,
					DisplayName:        displayName,
					GitLabUsername:     gitlabUsername,
					MattermostUsername: mattermostUsername,
					Role:               role,
					DefaultMode:        "semi-auto",
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
	} else {
		// Member exists — show pre-filled form for update (Esc to skip)
		identityStep = views.WizardStep{
			Label:      i18n.T("cmd.team.init.step_identity"),
			Processing: i18n.T("cmd.team.init.processing_identity"),
			Validate: func() string {
				if strings.TrimSpace(displayName) == "" {
					return i18n.T("cmd.team.init.validate.display_name_required")
				}
				return ""
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
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
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				member := teamstate.Member{
					ID:                 memberID,
					DisplayName:        displayName,
					GitLabUsername:     gitlabUsername,
					MattermostUsername: mattermostUsername,
					Role:               role,
					DefaultMode:        "semi-auto",
				}
				if err := repo.UpdateMember(ctx, member); err != nil {
					return err
				}
				return repo.CommitAndPush(ctx, fmt.Sprintf("team: update member %s", memberID), "members.toml")
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{
					{Label: "Member", Value: memberID},
					{Label: "Name", Value: displayName},
					{Label: "Role", Value: role},
				}
			},
		}
	}

	// ── Step 3: Notifications ──
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
			form.AddButton("Next", func() { onDone() })
			return form
		},
		OnDone: func() error {
			if webhookURL == "" {
				return nil // nothing to configure
			}
			cfg, err := repo.LoadConfig()
			if err != nil {
				return err
			}
			// Only save if something changed
			if cfg.Notification.MattermostWebhook == webhookURL &&
				cfg.Notification.Channel == channel &&
				cfg.Notification.BotName == botName {
				return nil
			}
			cfg.Notification.MattermostWebhook = webhookURL
			cfg.Notification.Channel = channel
			cfg.Notification.BotName = botName
			cfg.Notification.Enabled = true
			if err := repo.SaveConfig(ctx, cfg); err != nil {
				return err
			}
			return nil
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

	// ── Step 4: Policies ──
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
			form.AddButton("Confirm", func() {
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
				// Merge with existing
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

	// ══════════════════════════════════════════════════════════════════════════
	// LAUNCH WIZARD (tview alt-screen, cell-buffer, no banding)
	// ══════════════════════════════════════════════════════════════════════════
	wizResult := views.RunWizard(views.WizardConfig{
		Layout: layout.Config{
			ProjectName: a.Config.Name,
			Command:     "team init",
			StatusHints: i18n.T("wizard.hints.default"),
		},
		Steps: []views.WizardStep{repoStep, configStep, identityStep, notifStep, policiesStep},
	})
	if wizResult.Aborted {
		return nil
	}
	if wizResult.Err != nil {
		return wizResult.Err
	}

	// ══════════════════════════════════════════════════════════════════════════
	// POST-WIZARD: Update hub.toml + summary
	// ══════════════════════════════════════════════════════════════════════════

	// If nothing was configured (all steps skipped, no prior config), treat as abort
	if stateRepo == "" && a.Config.ActiveTeam().StateRepo == "" {
		return nil
	}

	// Resolve memberID (may have been set in wizard)
	if memberID == "" {
		memberID = a.Config.ActiveTeam().MemberID
	}

	// Write hub.toml team config (upsert — safe to call multiple times)
	needsWrite := a.Config.ActiveTeam().StateRepo == "" ||
		(a.Config.ActiveTeam().MemberID != memberID && memberID != "")
	if needsWrite {
		if err := writeTeamConfig(stateRepo, statePath, memberID); err != nil {
			return err
		}
	}

	// Summary card
	summaryTitle := i18n.T("cmd.team.init.done")
	if hasConfig || hasPolicies {
		summaryTitle = i18n.T("cmd.team.init.done_update")
	}

	fields := []summary.Field{
		{Label: "Repo", Value: maskRepoURL(stateRepo)},
		{Label: "Clone", Value: shortenPath(statePath)},
	}
	if memberID != "" {
		memberDesc := memberID
		if displayName != "" {
			memberDesc = fmt.Sprintf("%s (%s)", displayName, role)
		}
		fields = append(fields, summary.Field{Label: i18n.T("cmd.team.init.recap.member"), Value: memberDesc})
	}
	if staleDaysStr != "" {
		fields = append(fields, summary.Field{Label: i18n.T("cmd.team.init.recap.stale_days"), Value: staleDaysStr})
	}
	if webhookURL != "" {
		fields = append(fields, summary.Field{
			Label: i18n.T("cmd.team.init.recap.notifs"),
			Value: fmt.Sprintf("mattermost · %s", maskSecret(webhookURL)),
		})
	} else {
		fields = append(fields, summary.Field{Label: i18n.T("cmd.team.init.recap.notifs"), Value: i18n.T("cmd.team.init.recap.notifs_none")})
	}
	if len(selectedPolicies) > 0 {
		fields = append(fields, summary.Field{
			Label: i18n.T("cmd.team.init.recap.policies"),
			Value: fmt.Sprintf("%s (%d)", strings.Join(selectedPolicies, ", "), len(selectedPolicies)),
		})
	}

	fmt.Fprint(a.IO.Out, summary.Render(summary.Config{
		Title:     summaryTitle,
		Icon:      theme.IconSuccess,
		IconColor: theme.LipSuccess,
		Fields:    fields,
		Footer:    i18n.Tf("cmd.team.init.done_hint_status", "oh team status"),
	}))

	return nil
}

// buildRecommendedPolicies creates a policy map from the selected recommended policy keys.
func buildRecommendedPolicies(selected []string) map[string]teamstate.Policy {
	all := map[string]teamstate.Policy{
		"branch_naming": {
			Type:        teamstate.PolicyTypeRegex,
			Rule:        `^(feat|fix|chore|refactor|docs|test|ci)/[a-z0-9-]+`,
			Enforcement: teamstate.EnforcementRefuse,
			Message:     i18n.T("cmd.team.init.policy_msg.branch_naming"),
		},
		"commit_format": {
			Type:        teamstate.PolicyTypeRegex,
			Rule:        `^(feat|fix|chore|refactor|docs|test|ci)(\(.+\))?: .+`,
			Enforcement: teamstate.EnforcementWarn,
			Message:     i18n.T("cmd.team.init.policy_msg.commit_format"),
		},
		"max_ticket_wip": {
			Type:        teamstate.PolicyTypeLimit,
			Max:         2,
			Enforcement: teamstate.EnforcementWarn,
			Message:     i18n.T("cmd.team.init.policy_msg.max_wip"),
		},
		"review_required": {
			Type:        teamstate.PolicyTypeBoolean,
			Enabled:     true,
			Enforcement: teamstate.EnforcementRefuse,
			Message:     i18n.T("cmd.team.init.policy_msg.review_required"),
		},
	}

	result := make(map[string]teamstate.Policy, len(selected))
	for _, key := range selected {
		if p, ok := all[key]; ok {
			result[key] = p
		}
	}
	return result
}

func runTeamStatus(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo := teamRepo

	// Pull latest
	if err := repo.Pull(ctx); err != nil {
		fmt.Fprintf(a.IO.ErrOut, "%s %s\n",
			theme.WarningStyle.Render(theme.IconWarning),
			i18n.Tf("cmd.team.status.sync_error", err))
	}

	detail, _ := cmd.Flags().GetBool("detail")

	// List members
	members, err := repo.ListMembers()
	if err != nil {
		return fmt.Errorf("listing members: %w", err)
	}

	if len(members) == 0 {
		fmt.Fprintf(a.IO.Out, "\n%s %s\n\n",
			theme.Subtitle.Render(theme.IconInfo),
			i18n.Tf("cmd.team.status.empty", theme.Bold.Render("oh team init")))
		return nil
	}

	// List all claims
	claims, err := repo.ListClaims("")
	if err != nil {
		return fmt.Errorf("listing claims: %w", err)
	}

	// Index claims by member
	claimsByMember := make(map[string][]teamstate.Claim)
	for _, c := range claims {
		claimsByMember[c.ClaimedBy] = append(claimsByMember[c.ClaimedBy], c)
	}

	// Counters
	activeCount := 0
	reviewCount := 0
	blockedCount := 0
	for _, c := range claims {
		switch c.Status {
		case "in_progress":
			activeCount++
		case "review":
			reviewCount++
		case "blocked":
			blockedCount++
		}
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, theme.Title.Render(i18n.Tf("cmd.team.status.title", len(members))))
	fmt.Fprintln(a.IO.Out)

	w := tabwriter.NewWriter(a.IO.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n",
		theme.Bold.Render(i18n.T("cmd.team.status.header_member")),
		theme.Bold.Render(i18n.T("cmd.team.status.header_role")),
		theme.Bold.Render(i18n.T("cmd.team.status.header_ticket")),
		theme.Bold.Render(i18n.T("cmd.team.status.header_status")),
		theme.Bold.Render(i18n.T("cmd.team.status.header_since")))
	fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", "──────", "────", "──────", "──────", "─────")

	for _, m := range members {
		memberClaims := claimsByMember[m.ID]

		if len(memberClaims) == 0 {
			fmt.Fprintf(w, "  %s\t%s\t%s\t\t\n",
				m.DisplayName, m.Role,
				theme.Subtitle.Render(i18n.T("cmd.team.status.idle")))
			continue
		}

		for i, c := range memberClaims {
			since := c.ClaimedAt
			if !c.LastActivity.IsZero() {
				since = c.LastActivity
			}
			sinceStr := formatDuration(time.Since(since).Truncate(time.Minute))

			memberCol := m.DisplayName
			roleCol := m.Role
			if i > 0 {
				memberCol = ""
				roleCol = ""
			}

			ticketStr := fmt.Sprintf("%s/%s", c.Project, c.TicketID)
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n",
				memberCol, roleCol, ticketStr, c.Status, sinceStr)

			// Detail mode: show sub-beads
			if detail {
				subBeads := fetchSubBeadsJSON(c.TicketID)
				if len(subBeads) > 0 {
					completed := 0
					for j, sb := range subBeads {
						icon := theme.IconDot
						switch sb.Status {
						case "completed":
							icon = theme.IconSuccess
							completed++
						case "in_progress":
							icon = theme.IconInfo
						}
						prefix := "├"
						if j == len(subBeads)-1 {
							prefix = "└"
						}
						fmt.Fprintf(w, "  \t\t  %s %s %s\t%s\t\n",
							prefix, icon, sb.ID, sb.Status)
					}
					fmt.Fprintf(w, "  \t\t  %s\t\t\n",
						theme.Subtitle.Render(i18n.Tf("cmd.team.status.progress", completed, len(subBeads))))
				}
			}
		}
	}
	w.Flush()

	// Summary line
	fmt.Fprintf(a.IO.Out, "\n  %s\n\n",
		i18n.Tf("cmd.team.status.summary",
			activeCount, theme.IconDot,
			reviewCount, theme.IconDot,
			blockedCount))

	return nil
}

func runTeamActivity(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo := teamRepo

	if err := repo.Pull(ctx); err != nil {
		fmt.Fprintf(a.IO.ErrOut, "%s Impossible de synchroniser: %v\n",
			theme.WarningStyle.Render(theme.IconWarning), err)
	}

	// Determine time filter
	var since time.Time
	today, _ := cmd.Flags().GetBool("today")
	week, _ := cmd.Flags().GetBool("week")
	switch {
	case today:
		now := time.Now()
		since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case week:
		since = time.Now().AddDate(0, 0, -7)
	default:
		since = time.Now().AddDate(0, 0, -1) // Default: last 24h
	}

	project, _ := cmd.Flags().GetString("project")
	limit, _ := cmd.Flags().GetInt("limit")

	events, err := repo.ListEvents(project, since)
	if err != nil {
		return fmt.Errorf("listing events: %w", err)
	}

	// Filter by member if specified
	member, _ := cmd.Flags().GetString("member")
	if member != "" {
		filtered := events[:0]
		for _, e := range events {
			if e.Actor == member {
				filtered = append(filtered, e)
			}
		}
		events = filtered
	}

	// Apply limit
	totalCount := len(events)
	if len(events) > limit {
		events = events[:limit]
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, theme.Title.Render("  Team Activity  "))
	fmt.Fprintln(a.IO.Out)

	if len(events) == 0 {
		fmt.Fprintf(a.IO.Out, "  %s %s\n",
			theme.Subtitle.Render(theme.IconInfo), i18n.T("cmd.team.no_activity"))
		return nil
	}

	for _, e := range events {
		ts := e.Timestamp.Local().Format("15:04")
		icon := eventIcon(e.Type)
		desc := formatEvent(e)
		fmt.Fprintf(a.IO.Out, "  %s %s %s %s\n",
			theme.Subtitle.Render(ts), icon, theme.Bold.Render(e.Actor), desc)
	}

	if totalCount > limit {
		fmt.Fprintf(a.IO.Out, "\n  %s %s\n",
			theme.Subtitle.Render(theme.IconInfo),
			i18n.Tf("cmd.team.activity.truncated", limit, totalCount))
	}
	fmt.Fprintln(a.IO.Out)

	return nil
}

// ensureTeamRepo returns a ready Repo or an error if team is not configured.
// It uses the hub-level team config. For project-aware resolution, use ensureTeamRepoForProject.
func ensureTeamRepo(ctx context.Context, a *app.App) (*teamstate.Repo, error) {
	return ensureTeamRepoForProject(ctx, a, nil)
}

// ensureTeamRepoForProject returns a ready Repo using the effective team config
// for the given project (nil = hub-level only).
func ensureTeamRepoForProject(ctx context.Context, a *app.App, project *domain.Project) (*teamstate.Repo, error) {
	tc := resolvedTeamConfig(a, project)
	if !tc.Enabled {
		return nil, fmt.Errorf("fonctions d'équipe non activées. Lance %s d'abord",
			theme.Bold.Render("oh team init"))
	}
	statePath := tc.StatePath
	if statePath == "" {
		statePath = config.DefaultTeamStatePath()
	}
	repo := teamstate.NewRepo(tc.StateRepo, statePath)
	if !repo.IsCloned() {
		return nil, fmt.Errorf("repo team-state non trouvé dans %s. Lance %s",
			statePath, theme.Bold.Render("oh team init"))
	}
	return repo, nil
}

// writeTeamConfig updates hub.toml with team settings using the new [[teams]] format.
// It updates the in-memory Config.Teams and persists via config.Save().
func writeTeamConfig(stateRepo, statePath, memberID string) error {
	a := MustApp()
	id := config.RepoNameFromRemote(stateRepo)

	newTeam := config.TeamConfig{
		ID:        id,
		Enabled:   true,
		StateRepo: stateRepo,
		StatePath: statePath,
		MemberID:  memberID,
	}

	// Update or append
	found := false
	for i := range a.Config.Teams {
		if a.Config.Teams[i].ID == id || a.Config.Teams[i].StateRepo == stateRepo {
			a.Config.Teams[i] = newTeam
			found = true
			break
		}
	}
	if !found {
		a.Config.Teams = append(a.Config.Teams, newTeam)
	}
	// Clear legacy field
	a.Config.Team = config.TeamConfig{}

	return config.Save(a.Config)
}

func eventIcon(eventType string) string {
	switch eventType {
	case teamstate.EventSessionComplete:
		return theme.SuccessStyle.Render(theme.IconSuccess)
	case teamstate.EventReviewReady:
		return theme.SuccessStyle.Render(theme.IconInfo)
	case teamstate.EventAuditFinding:
		return theme.WarningStyle.Render(theme.IconWarning)
	case teamstate.EventClaimTaken:
		return theme.Subtitle.Render(theme.IconArrow)
	case teamstate.EventClaimConflict:
		return theme.ErrorStyle.Render(theme.IconWarning)
	case teamstate.EventClaimTransferred:
		return theme.Subtitle.Render(theme.IconArrow)
	case teamstate.EventClaimReleased:
		return theme.Subtitle.Render(theme.IconDot)
	case teamstate.EventWikiProposal:
		return theme.SuccessStyle.Render(theme.IconInfo)
	case teamstate.EventWikiAccepted:
		return theme.SuccessStyle.Render(theme.IconSuccess)
	default:
		return theme.Subtitle.Render(theme.IconDot)
	}
}

func formatEvent(e teamstate.Event) string {
	switch e.Type {
	case teamstate.EventSessionComplete:
		ticket := e.Ticket
		if ticket == "" {
			ticket = "session"
		}
		return fmt.Sprintf("a terminé %s/%s", e.Project, ticket)
	case teamstate.EventReviewReady:
		return fmt.Sprintf("review prête pour %s/%s", e.Project, e.Ticket)
	case teamstate.EventAuditFinding:
		return fmt.Sprintf("audit findings sur %s/%s", e.Project, e.Ticket)
	case teamstate.EventClaimTaken:
		return fmt.Sprintf("a pris %s/%s", e.Project, e.Ticket)
	case teamstate.EventClaimConflict:
		return fmt.Sprintf("conflit de claim sur %s/%s", e.Project, e.Ticket)
	case teamstate.EventClaimTransferred:
		return fmt.Sprintf("transfert %s/%s", e.Project, e.Ticket)
	case teamstate.EventClaimReleased:
		return fmt.Sprintf("a libéré %s/%s", e.Project, e.Ticket)
	case teamstate.EventWikiProposal:
		return "a proposé une entrée wiki"
	case teamstate.EventWikiAccepted:
		return "entrée wiki acceptée"
	default:
		return e.Type
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}

// ── Duplicate clone detection ─────────────────────────────────────────────────

// existingClone holds information about an existing clone of a team-state repo.
type existingClone struct {
	// Path is the local filesystem path of the existing clone.
	Path string
	// Source describes where this clone is referenced ("hub" or the project name).
	Source string
}

// findExistingCloneForRemote scans all known team-state configurations (hub.toml
// and every registered project) to detect whether a clone of remoteURL already
// exists locally. This is used by oh team init to prevent creating duplicate
// clones of the same remote — a situation that leads to diverged local state.
//
// Returns the first matching clone found, or (existingClone{}, false) if none.
func findExistingCloneForRemote(ctx context.Context, a *app.App, remoteURL string) (existingClone, bool) {
	// Normalize for comparison (strip trailing slashes, .git suffix)
	norm := normalizeRemoteURL(remoteURL)

	// 1. Check hub-level team config
	if a.Config.ActiveTeam().StateRepo != "" &&
		normalizeRemoteURL(a.Config.ActiveTeam().StateRepo) == norm &&
		a.Config.ActiveTeam().StatePath != "" {
		repo := teamstate.NewRepo(a.Config.ActiveTeam().StateRepo, a.Config.ActiveTeam().StatePath)
		if repo.IsCloned() {
			return existingClone{
				Path:   a.Config.ActiveTeam().StatePath,
				Source: "configuration hub",
			}, true
		}
	}

	// 2. Check all registered projects
	projects, err := a.Projects.List(ctx, "")
	if err != nil {
		return existingClone{}, false
	}
	for _, p := range projects {
		if p.TeamConfig == nil {
			continue
		}
		if p.TeamConfig.StateRepo == "" ||
			normalizeRemoteURL(p.TeamConfig.StateRepo) != norm {
			continue
		}
		statePath := p.TeamConfig.StatePath
		if statePath == "" {
			continue
		}
		repo := teamstate.NewRepo(p.TeamConfig.StateRepo, statePath)
		if repo.IsCloned() {
			return existingClone{
				Path:   statePath,
				Source: fmt.Sprintf("projet %s", p.Name),
			}, true
		}
	}

	return existingClone{}, false
}

// normalizeRemoteURL strips trailing slashes and the .git suffix for comparison.
func normalizeRemoteURL(u string) string {
	u = strings.TrimRight(u, "/")
	u = strings.TrimSuffix(u, ".git")
	return u
}

// collectUsedMemberIDs returns a set of member IDs that are actively referenced
// by at least one team configuration (hub-level or project-level).
// Used to detect orphan entries in members.toml.
func collectUsedMemberIDs(ctx context.Context, a *app.App) map[string]bool {
	used := make(map[string]bool)

	// Hub-level member_id
	if a.Config.ActiveTeam().MemberID != "" {
		used[a.Config.ActiveTeam().MemberID] = true
	}

	// Per-project member IDs
	projects, err := a.Projects.List(ctx, "")
	if err != nil {
		return used
	}
	for _, p := range projects {
		if p.TeamConfig != nil && p.TeamConfig.MemberID != "" {
			used[p.TeamConfig.MemberID] = true
		}
	}
	return used
}

// maskRepoURL strips credentials and scheme from a repo URL for safe display.
// e.g. "https://oauth2:token@gitlab.com/group/repo.git" → "gitlab.com/group/repo.git"
func maskRepoURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	u.User = nil
	return u.Host + u.Path
}

// shortenPath replaces the user's home directory with "~" for display.
func shortenPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}
