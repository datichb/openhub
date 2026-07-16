package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/common"
	"github.com/datichb/openhub/cli/internal/tui/components/summary"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

var teamCmd = &cobra.Command{
	Use:   "team",
	Short: "Gestion de l'équipe et de la collaboration",
	Long: `Commandes pour le travail en équipe : initialisation, statut,
activité, et gestion du wiki partagé.`,
}

var teamInitCmd = &cobra.Command{
	Use:   "init",
	Short: i18n.T("cmd.team.init.short"),
	Long:  i18n.T("cmd.team.init.long"),
	RunE:  runTeamInit,
}

var teamStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Affiche qui travaille sur quoi",
	RunE:  runTeamStatus,
}

var teamActivityCmd = &cobra.Command{
	Use:   "activity",
	Short: "Journal d'activité de l'équipe",
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

	teamStatusCmd.Flags().Bool("detail", false, "Affiche les sous-tickets et la progression")
}

func runTeamInit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	a := MustApp()

	// ══════════════════════════════════════════════════════════════════════════
	// PRE-WIZARD: Prerequisite check
	// ══════════════════════════════════════════════════════════════════════════
	if _, err := os.Stat(config.ConfigPath()); os.IsNotExist(err) {
		return fmt.Errorf("%s", i18n.Tf("cmd.team.init.hub_not_configured", common.Bold.Render("oh init")))
	}

	// ══════════════════════════════════════════════════════════════════════════
	// PRE-WIZARD: Preamble (visible, no pause)
	// ══════════════════════════════════════════════════════════════════════════
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, common.Title.Render("  Team Setup  "))
	fmt.Fprintln(a.IO.Out)

	preamble := fmt.Sprintf("  %s %s\n  %s %s\n  %s %s",
		common.SuccessStyle.Render(common.IconSuccess), i18n.T("cmd.team.init.prereq_hub"),
		common.SuccessStyle.Render(common.IconSuccess), i18n.T("cmd.team.init.prereq_repo"),
		common.SuccessStyle.Render(common.IconSuccess), i18n.T("cmd.team.init.prereq_ssh"),
	)
	fmt.Fprintln(a.IO.Out, common.Box.Render(preamble))
	fmt.Fprintln(a.IO.Out)

	// ══════════════════════════════════════════════════════════════════════════
	// PRE-WIZARD: Repo URL + clone/pull
	// ══════════════════════════════════════════════════════════════════════════
	var stateRepo string

	// If already configured in hub.toml, reuse
	if a.Config.Team.StateRepo != "" {
		stateRepo = a.Config.Team.StateRepo
	} else {
		repoForm := common.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(i18n.T("cmd.team.init.repo_url_title")).
					Description(i18n.T("cmd.team.init.repo_url_desc")).
					Placeholder(i18n.T("cmd.team.init.repo_url_placeholder")).
					Value(&stateRepo).
					Validate(func(s string) error {
						if s == "" {
							return fmt.Errorf("%s", i18n.T("cmd.team.init.repo_url_required"))
						}
						return nil
					}),
			),
		)
		if err := repoForm.Run(); err != nil {
			return err
		}
	}

	// Determine local path
	statePath := a.Config.Team.StatePath
	if statePath == "" {
		statePath = config.DefaultTeamStatePath()
	}

	// Clone or pull
	repo := teamstate.NewRepo(stateRepo, statePath)
	if repo.IsCloned() {
		fmt.Fprintf(a.IO.Out, "%s %s\n", common.Subtitle.Render(common.IconArrow), i18n.T("cmd.team.init.pulling"))
		if err := repo.Pull(ctx); err != nil {
			// Non-fatal: continue with local state
			fmt.Fprintf(a.IO.Out, "%s pull: %v\n", common.WarningStyle.Render(common.IconWarning), err)
		} else {
			fmt.Fprintf(a.IO.Out, "%s %s\n", common.SuccessStyle.Render(common.IconSuccess), i18n.T("cmd.team.init.pulled"))
		}
	} else {
		fmt.Fprintf(a.IO.Out, "%s %s\n", common.Subtitle.Render(common.IconArrow), i18n.T("cmd.team.init.cloning"))
		if err := repo.Clone(ctx); err != nil {
			return fmt.Errorf("cloning team-state: %w", err)
		}
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.Tf("cmd.team.init.cloned", statePath))
	}

	// Initialize directory structure
	if err := repo.InitStructure(ctx); err != nil {
		return fmt.Errorf("initializing structure: %w", err)
	}

	// ══════════════════════════════════════════════════════════════════════════
	// STATE DETECTION
	// ══════════════════════════════════════════════════════════════════════════
	hasConfig := repo.HasConfig()
	hasPolicies := repo.HasPolicies()

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

	// Pre-fill from existing state
	existingCfg, _ := repo.LoadConfig()
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
	if a.Config.Team.MemberID != "" {
		memberID = a.Config.Team.MemberID
	}

	// Pre-fill member profile if exists
	hasMember := memberID != "" && repo.HasMember(memberID)
	if hasMember {
		existing, err := repo.GetMember(memberID)
		if err == nil && existing != nil {
			displayName = existing.DisplayName
			gitlabUsername = existing.GitLabUsername
			mattermostUsername = existing.MattermostUsername
			role = existing.Role
		}
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
				if err := repo.SaveConfig(existingCfg); err != nil {
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
				if err := repo.SaveConfig(cfg); err != nil {
					return err
				}
			}
			return repo.CommitAndPush(ctx, "team: configure config.toml", "config.toml")
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
				if err := repo.AddMember(member); err != nil {
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
				if err := repo.UpdateMember(member); err != nil {
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
			if err := repo.SaveConfig(cfg); err != nil {
				return err
			}
			return repo.CommitAndPush(ctx, "team: configure notifications", "config.toml")
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
			if err := repo.SavePolicies(policies); err != nil {
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
			StatusHints: "enter confirm · esc skip",
		},
		Steps: []views.WizardStep{configStep, identityStep, notifStep, policiesStep},
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

	// Resolve memberID (may have been set in wizard)
	if memberID == "" {
		memberID = a.Config.Team.MemberID
	}

	// Write hub.toml team config if not already there
	if a.Config.Team.StateRepo == "" {
		if err := writeTeamConfig(stateRepo, statePath, memberID); err != nil {
			return err
		}
	} else if a.Config.Team.MemberID != memberID && memberID != "" {
		// Update member_id if changed
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
		{Label: "Repo", Value: stateRepo},
	}
	if memberID != "" {
		memberDesc := memberID
		if displayName != "" {
			memberDesc = fmt.Sprintf("%s (%s)", displayName, role)
		}
		fields = append(fields, summary.Field{Label: "Member", Value: memberDesc})
	}
	if len(selectedPolicies) > 0 {
		fields = append(fields, summary.Field{Label: "Policies", Value: fmt.Sprintf("%d active", len(selectedPolicies))})
	}

	fmt.Fprint(a.IO.Out, summary.Render(summary.Config{
		Title:     summaryTitle,
		Icon:      common.IconSuccess,
		IconColor: common.Success,
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
			Message:     "Le nom de branche doit suivre le format type/description-kebab-case",
		},
		"commit_format": {
			Type:        teamstate.PolicyTypeRegex,
			Rule:        `^(feat|fix|chore|refactor|docs|test|ci)(\(.+\))?: .+`,
			Enforcement: teamstate.EnforcementWarn,
			Message:     "Le commit devrait suivre Conventional Commits",
		},
		"max_ticket_wip": {
			Type:        teamstate.PolicyTypeLimit,
			Max:         2,
			Enforcement: teamstate.EnforcementWarn,
			Message:     "Maximum 2 tickets en parallèle par membre",
		},
		"review_required": {
			Type:        teamstate.PolicyTypeBoolean,
			Enabled:     true,
			Enforcement: teamstate.EnforcementRefuse,
			Message:     "Une review est requise avant merge",
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

	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}

	// Pull latest
	if err := repo.Pull(ctx); err != nil {
		fmt.Fprintf(a.IO.ErrOut, "%s Impossible de synchroniser: %v\n",
			common.WarningStyle.Render(common.IconWarning), err)
	}

	detail, _ := cmd.Flags().GetBool("detail")

	// List members
	members, err := repo.ListMembers()
	if err != nil {
		return fmt.Errorf("listing members: %w", err)
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
	fmt.Fprintln(a.IO.Out, common.Title.Render(fmt.Sprintf("  Team: %d members  ", len(members))))
	fmt.Fprintln(a.IO.Out)

	w := tabwriter.NewWriter(a.IO.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n",
		common.Bold.Render("Member"),
		common.Bold.Render("Role"),
		common.Bold.Render("Ticket"),
		common.Bold.Render("Status"),
		common.Bold.Render("Since"))
	fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", "──────", "────", "──────", "──────", "─────")

	for _, m := range members {
		memberClaims := claimsByMember[m.ID]

		if len(memberClaims) == 0 {
			fmt.Fprintf(w, "  %s\t%s\t%s\t\t\n",
				m.DisplayName, m.Role,
				common.Subtitle.Render("— (idle)"))
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
						icon := common.IconDot
						switch sb.Status {
						case "completed":
							icon = common.IconSuccess
							completed++
						case "in_progress":
							icon = common.IconInfo
						}
						prefix := "├"
						if j == len(subBeads)-1 {
							prefix = "└"
						}
						fmt.Fprintf(w, "  \t\t  %s %s %s\t%s\t\n",
							prefix, icon, sb.ID, sb.Status)
					}
					fmt.Fprintf(w, "  \t\t  %s\t\t\n",
						common.Subtitle.Render(fmt.Sprintf("Progress: %d/%d", completed, len(subBeads))))
				}
			}
		}
	}
	w.Flush()

	// Summary line
	fmt.Fprintf(a.IO.Out, "\n  %d tickets actifs %s %d en review %s %d blocked\n\n",
		activeCount, common.IconDot,
		reviewCount, common.IconDot,
		blockedCount)

	return nil
}

func runTeamActivity(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}

	if err := repo.Pull(ctx); err != nil {
		fmt.Fprintf(a.IO.ErrOut, "%s Impossible de synchroniser: %v\n",
			common.WarningStyle.Render(common.IconWarning), err)
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
	if len(events) > limit {
		events = events[:limit]
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, common.Title.Render("  Team Activity  "))
	fmt.Fprintln(a.IO.Out)

	if len(events) == 0 {
		fmt.Fprintf(a.IO.Out, "  %s %s\n",
			common.Subtitle.Render(common.IconInfo), i18n.T("cmd.team.no_activity"))
		return nil
	}

	for _, e := range events {
		ts := e.Timestamp.Local().Format("15:04")
		icon := eventIcon(e.Type)
		desc := formatEvent(e)
		fmt.Fprintf(a.IO.Out, "  %s %s %s %s\n",
			common.Subtitle.Render(ts), icon, common.Bold.Render(e.Actor), desc)
	}
	fmt.Fprintln(a.IO.Out)

	return nil
}

// ensureTeamRepo returns a ready Repo or an error if team is not configured.
func ensureTeamRepo(ctx context.Context, a *app.App) (*teamstate.Repo, error) {
	if !a.Config.Team.Enabled {
		return nil, fmt.Errorf("fonctions d'équipe non activées. Lance %s d'abord",
			common.Bold.Render("oh team init"))
	}
	statePath := a.Config.Team.StatePath
	if statePath == "" {
		statePath = config.DefaultTeamStatePath()
	}
	repo := teamstate.NewRepo(a.Config.Team.StateRepo, statePath)
	if !repo.IsCloned() {
		return nil, fmt.Errorf("repo team-state non trouvé dans %s. Lance %s",
			statePath, common.Bold.Render("oh team init"))
	}
	return repo, nil
}

// writeTeamConfig updates hub.toml with team settings.
func writeTeamConfig(stateRepo, statePath, memberID string) error {
	cfgPath := config.ConfigPath()
	content, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("reading hub.toml: %w", err)
	}

	// Append team section
	teamSection := fmt.Sprintf(`
[team]
enabled = true
state_repo = %q
state_path = %q
member_id = %q
`, stateRepo, statePath, memberID)

	newContent := string(content) + teamSection
	return os.WriteFile(cfgPath, []byte(newContent), 0o600)
}

func eventIcon(eventType string) string {
	switch eventType {
	case teamstate.EventSessionComplete:
		return common.SuccessStyle.Render(common.IconSuccess)
	case teamstate.EventReviewReady:
		return common.SuccessStyle.Render(common.IconInfo)
	case teamstate.EventAuditFinding:
		return common.WarningStyle.Render(common.IconWarning)
	case teamstate.EventClaimTaken:
		return common.Subtitle.Render(common.IconArrow)
	case teamstate.EventClaimConflict:
		return common.ErrorStyle.Render(common.IconWarning)
	case teamstate.EventClaimTransferred:
		return common.Subtitle.Render(common.IconArrow)
	case teamstate.EventClaimReleased:
		return common.Subtitle.Render(common.IconDot)
	case teamstate.EventWikiProposal:
		return common.SuccessStyle.Render(common.IconInfo)
	case teamstate.EventWikiAccepted:
		return common.SuccessStyle.Render(common.IconSuccess)
	default:
		return common.Subtitle.Render(common.IconDot)
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
