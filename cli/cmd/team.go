package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/common"
	"github.com/datichb/openhub/cli/internal/tui/views/wizard"
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
		repoForm := huh.NewForm(
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
	// BUILD WIZARD STEPS
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
	configStep := wizard.StepConfig{
		Label: i18n.T("cmd.team.init.step_config"),
		Form: huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(i18n.T("cmd.team.init.config_stale_days")).
					Description(i18n.T("cmd.team.init.config_stale_days_desc")).
					Placeholder("3").
					Value(&staleDaysStr),
			).Title(i18n.T("cmd.team.init.config_create_title")),
		),
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
	}

	// ── Step 2: Identité ──
	var identityStep wizard.StepConfig
	if !hasMember {
		identityStep = wizard.StepConfig{
			Label: i18n.T("cmd.team.init.step_identity"),
			Form: huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title(i18n.T("cmd.team.init.identity_id")).
						Description(i18n.T("cmd.team.init.identity_id_desc")).
						Value(&memberID).
						Validate(func(s string) error {
							if s == "" {
								return fmt.Errorf("%s", i18n.T("cmd.team.init.identity_id_required"))
							}
							return nil
						}),
					huh.NewInput().
						Title(i18n.T("cmd.team.init.identity_display")).
						Value(&displayName),
					huh.NewInput().
						Title(i18n.T("cmd.team.init.identity_gitlab")).
						Value(&gitlabUsername),
					huh.NewInput().
						Title(i18n.T("cmd.team.init.identity_mattermost")).
						Value(&mattermostUsername),
					huh.NewSelect[string]().
						Title(i18n.T("cmd.team.init.identity_role")).
						Options(
							huh.NewOption(i18n.T("cmd.team.init.identity_role_lead"), "lead"),
							huh.NewOption(i18n.T("cmd.team.init.identity_role_dev"), "dev"),
							huh.NewOption(i18n.T("cmd.team.init.identity_role_reviewer"), "reviewer"),
						).
						Value(&role),
				).Title(i18n.T("cmd.team.init.identity_title")),
			),
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
		}
	} else {
		// Member exists — show pre-filled form for update (Esc to skip)
		identityStep = wizard.StepConfig{
			Label: i18n.T("cmd.team.init.step_identity"),
			Form: huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title(i18n.T("cmd.team.init.identity_display")).
						Value(&displayName),
					huh.NewInput().
						Title(i18n.T("cmd.team.init.identity_gitlab")).
						Value(&gitlabUsername),
					huh.NewInput().
						Title(i18n.T("cmd.team.init.identity_mattermost")).
						Value(&mattermostUsername),
					huh.NewSelect[string]().
						Title(i18n.T("cmd.team.init.identity_role")).
						Options(
							huh.NewOption(i18n.T("cmd.team.init.identity_role_lead"), "lead"),
							huh.NewOption(i18n.T("cmd.team.init.identity_role_dev"), "dev"),
							huh.NewOption(i18n.T("cmd.team.init.identity_role_reviewer"), "reviewer"),
						).
						Value(&role),
				).Title(i18n.T("cmd.team.init.identity_title")),
			),
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
		}
	}

	// ── Step 3: Notifications ──
	notifStep := wizard.StepConfig{
		Label: i18n.T("cmd.team.init.step_notifications"),
		Form: huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(i18n.T("cmd.team.init.notif_webhook")).
					Description(i18n.T("cmd.team.init.notif_webhook_desc")).
					Placeholder(i18n.T("cmd.team.init.notif_webhook_placeholder")).
					Value(&webhookURL),
				huh.NewInput().
					Title(i18n.T("cmd.team.init.notif_channel")).
					Placeholder(i18n.T("cmd.team.init.notif_channel_placeholder")).
					Value(&channel),
				huh.NewInput().
					Title(i18n.T("cmd.team.init.notif_bot_name")).
					Value(&botName),
			).Title(i18n.T("cmd.team.init.notif_title")),
		),
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
	}

	// ── Step 4: Policies ──
	policiesStep := wizard.StepConfig{
		Label: i18n.T("cmd.team.init.step_policies"),
		Form: huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title(i18n.T("cmd.team.init.policies_select")).
					Description(i18n.T("cmd.team.init.policies_select_desc")).
					Options(
						huh.NewOption(i18n.T("cmd.team.init.policies_branch_naming"), "branch_naming"),
						huh.NewOption(i18n.T("cmd.team.init.policies_commit_format"), "commit_format"),
						huh.NewOption(i18n.T("cmd.team.init.policies_max_wip"), "max_ticket_wip"),
						huh.NewOption(i18n.T("cmd.team.init.policies_review_required"), "review_required"),
					).
					Value(&selectedPolicies),
			).Title(i18n.T("cmd.team.init.policies_title")),
		),
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
	}

	// ══════════════════════════════════════════════════════════════════════════
	// LAUNCH WIZARD (BubbleTea alt-screen, step bar + full width)
	// ══════════════════════════════════════════════════════════════════════════
	steps := []wizard.StepConfig{configStep, identityStep, notifStep, policiesStep}

	model := wizard.New("Team Setup", nil, steps)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("wizard error: %w", err)
	}

	wiz := finalModel.(wizard.Model)
	if wiz.Aborted() {
		return nil
	}
	if wiz.Err() != nil {
		return wiz.Err()
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

	// Summary
	fmt.Fprintln(a.IO.Out)
	if hasConfig || hasPolicies {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.T("cmd.team.init.done_update"))
	} else {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.T("cmd.team.init.done"))
	}
	fmt.Fprintf(a.IO.Out, "  %s\n",
		i18n.Tf("cmd.team.init.done_hint_status", common.Bold.Render("oh team status")))
	fmt.Fprintf(a.IO.Out, "  %s\n",
		i18n.Tf("cmd.team.init.done_hint_deploy", common.Bold.Render("oh deploy")))

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
				subBeads := fetchSubBeads(c.TicketID)
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
