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
	"golang.org/x/term"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/components/summary"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

var teamCmd = &cobra.Command{
	Use:               "team",
	Short:             i18n.T("cmd.team.short"),
	Long:              i18n.T("cmd.team.long"),
	PersistentPreRunE: teamPreRunE,
}

// teamRepo is the shared team-state repository instance, resolved once by
// teamPreRunE and reused by all team subcommands. Commands that need to work
// before the repo exists (e.g. `team init`) override PersistentPreRunE to nil.
var teamRepo *teamstate.Repo

// teamPreRunE resolves the team-state repo for all team subcommands.
// It skips resolution if the command is `team init` or `team rejoin` (repo doesn't exist yet).
func teamPreRunE(cmd *cobra.Command, _ []string) error {
	// Skip for commands that don't need an existing repo
	if cmd.Name() == "init" || cmd.Name() == "rejoin" {
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

var teamRejoinCmd = &cobra.Command{
	Use:   "rejoin",
	Short: i18n.T("cmd.team.rejoin.short"),
	Long: i18n.T("cmd.team.rejoin.long") + "\n\n" +
		"NOTE: Do not run rejoin while the TUI is actively performing team\n" +
		"operations on the same team-state clone. The in-process lock does not\n" +
		"protect against concurrent access from separate processes.",
	RunE: runTeamRejoin,
}

func init() {
	rootCmd.AddCommand(teamCmd)
	teamCmd.AddCommand(teamInitCmd)
	teamCmd.AddCommand(teamStatusCmd)
	teamCmd.AddCommand(teamActivityCmd)
	teamCmd.AddCommand(teamRejoinCmd)

	teamActivityCmd.Flags().Bool("today", false, "Show only today's events")
	teamActivityCmd.Flags().Bool("week", false, "Show last 7 days")
	teamActivityCmd.Flags().String("member", "", "Filter by member ID")
	teamActivityCmd.Flags().String("project", "", "Filter by project")
	teamActivityCmd.Flags().Int("limit", 20, "Maximum number of events to display")

	teamStatusCmd.Flags().Bool("detail", false, i18n.T("cmd.team.status.flags.detail"))

	teamRejoinCmd.Flags().String("repo", "", i18n.T("cmd.team.rejoin.flags.repo"))
	teamRejoinCmd.Flags().String("member-id", "", i18n.T("cmd.team.rejoin.flags.member_id"))
	teamRejoinCmd.Flags().Bool("no-retro-tag", false, i18n.T("cmd.team.rejoin.flags.no_retro_tag"))
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
		trackerUsername    string
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
				trackerUsername = existing.TrackerUsername
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
					trackerUsername = existing.TrackerUsername
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

	// ── Build shared team step state ──
	tss := &teamStepState{
		Ctx:               ctx,
		Repo:              repo,
		HasConfig:         hasConfig,
		HasPolicies:       hasPolicies,
		ExistingCfg:       existingCfg,
		HasMember:         hasMember,
		StaleDaysStr:      staleDaysStr,
		MemberID:          memberID,
		DisplayName:       displayName,
		GitLabUsername:     gitlabUsername,
		TrackerUsername:    trackerUsername,
		MattermostUsername: mattermostUsername,
		Role:              role,
		WebhookURL:        webhookURL,
		Channel:           channel,
		BotName:           botName,
	}
	stepOpts := teamStepOpts{} // CLI wizard has no extra SkipIf
	configStep := buildTeamConfigStep(tss, stepOpts)
	identityStep := buildTeamIdentityStep(tss, stepOpts)
	notifStep := buildTeamNotifStep(tss, stepOpts)
	policiesStep := buildTeamPoliciesStep(tss, stepOpts)

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
	// POST-WIZARD: Sync state from shared struct, then update hub.toml + summary
	// ══════════════════════════════════════════════════════════════════════════
	memberID = tss.MemberID
	displayName = tss.DisplayName
	role = tss.Role
	staleDaysStr = tss.StaleDaysStr
	webhookURL = tss.WebhookURL
	selectedPolicies = tss.SelectedPolicies

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

	// ── Tracker hint ──
	fmt.Fprintf(a.IO.Out, "  %s %s\n\n",
		theme.InfoStyle.Render(theme.IconInfo),
		i18n.T("cmd.team.tracker_hint"),
	)

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
		case "validation":
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
	case teamstate.EventWikiRejected:
		return theme.ErrorStyle.Render(theme.IconWarning)
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
	case teamstate.EventWikiRejected:
		return "proposition wiki rejetée"
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
		resolved := config.ResolveTeamForProject(a.Config, &p)
		if !resolved.Enabled || resolved.StateRepo == "" {
			continue
		}
		if normalizeRemoteURL(resolved.StateRepo) != norm {
			continue
		}
		statePath := resolved.StatePath
		if statePath == "" {
			continue
		}
		repo := teamstate.NewRepo(resolved.StateRepo, statePath)
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
		resolved := config.ResolveTeamForProject(a.Config, &p)
		if resolved.Enabled && resolved.MemberID != "" {
			used[resolved.MemberID] = true
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

// ══════════════════════════════════════════════════════════════════════════════
// oh team rejoin
// ══════════════════════════════════════════════════════════════════════════════

func runTeamRejoin(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	a := MustApp()

	// Prerequisite check
	if _, err := os.Stat(config.ConfigPath()); os.IsNotExist(err) {
		return fmt.Errorf("%s", i18n.Tf("cmd.team.rejoin.hub_not_configured", theme.Bold.Render("oh init")))
	}

	repoURL, _ := cmd.Flags().GetString("repo")
	memberID, _ := cmd.Flags().GetString("member-id")
	noRetroTag, _ := cmd.Flags().GetBool("no-retro-tag")

	// ── Step 1: Repo URL ──
	if repoURL == "" {
		return fmt.Errorf("--repo is required: URL of the team-state Git repository")
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, theme.Title.Render("  Team Rejoin  "))
	fmt.Fprintln(a.IO.Out)

	// ── Step 2: Clone/pull and list members ──
	fmt.Fprintf(a.IO.Out, "  %s Cloning team-state...\n", theme.WarningStyle.Render(theme.IconDot))
	_, members, _, err := listTeamMembers(ctx, repoURL, "")
	if err != nil {
		return err
	}
	if len(members) == 0 {
		return fmt.Errorf("no members found in the team-state repository")
	}

	// ── Step 3: Member selection ──
	if memberID == "" {
		// Interactive: list members and let user choose
		fmt.Fprintln(a.IO.Out)
		fmt.Fprintln(a.IO.Out, i18n.T("cmd.team.rejoin.select_member"))
		fmt.Fprintln(a.IO.Out)
		for i, m := range members {
			gitlab := ""
			if m.GitLabUsername != "" {
				gitlab = fmt.Sprintf(" — gitlab: %s", m.GitLabUsername)
			}
			fmt.Fprintf(a.IO.Out, "  %d. %s (%s)%s\n", i+1, m.DisplayName, m.ID, gitlab)
		}
		fmt.Fprintln(a.IO.Out)
		fmt.Fprintf(a.IO.Out, "%s", i18n.Tf("cmd.team.rejoin.choose", len(members)))

		var choice int
		if _, err := fmt.Fscanln(a.IO.In, &choice); err != nil || choice < 1 || choice > len(members) {
			return fmt.Errorf("invalid selection")
		}
		memberID = members[choice-1].ID
	} else {
		// Verify the member exists
		found := false
		for _, m := range members {
			if m.ID == memberID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("member %q not found in the team-state repository", memberID)
		}
	}

	// ── Step 3b: GitLab token prompt (conditional) ──
	var selectedMember *teamstate.Member
	for i := range members {
		if members[i].ID == memberID {
			selectedMember = &members[i]
			break
		}
	}
	if selectedMember != nil && selectedMember.GitLabUsername != "" {
		teamID := config.RepoNameFromRemote(repoURL)
		source := gitlabTokenSource(ctx, a, teamID)

		fmt.Fprintln(a.IO.Out)
		if source != "" {
			// Token available — offer choice
			fmt.Fprintf(a.IO.Out, "  %s %s\n",
				theme.InfoStyle.Render(theme.IconInfo),
				i18n.Tf("cmd.init.wizard_rejoin_token_reuse", source),
			)
			fmt.Fprintf(a.IO.Out, "  [1] %s\n", i18n.T("cmd.init.wizard_rejoin_token_reuse_short"))
			fmt.Fprintf(a.IO.Out, "  [2] %s\n", i18n.T("cmd.init.wizard_rejoin_token_new"))
			fmt.Fprintf(a.IO.Out, "  [3] %s\n", i18n.T("cmd.init.wizard_rejoin_token_skip"))
			fmt.Fprintf(a.IO.Out, "  Choice [1]: ")
			var choiceStr string
			fmt.Fscanln(a.IO.In, &choiceStr)
			switch choiceStr {
			case "2":
				// Enter new token
				fmt.Fprintf(a.IO.Out, "  %s: ", i18n.T("cmd.init.wizard_rejoin_gitlab_token_label"))
				if term.IsTerminal(int(os.Stdin.Fd())) {
					tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
					fmt.Fprintln(a.IO.Out)
					if err == nil && len(tokenBytes) > 0 && a.Secrets != nil {
						if sErr := a.Secrets.Set(ctx, config.TeamGitLabTokenKey(teamID), string(tokenBytes)); sErr != nil {
							slog.Error("failed to save GitLab token to keychain", "team", teamID, "error", sErr)
						}
					}
				}
			case "3":
				// Skip — do nothing
			default:
				// "1" or empty — reuse existing token
			}
		} else {
			// No token — offer to enter or skip
			fmt.Fprintf(a.IO.Out, "  %s %s\n",
				theme.WarningStyle.Render(theme.IconDot),
				i18n.T("cmd.init.wizard_rejoin_gitlab_hint"),
			)
			fmt.Fprintf(a.IO.Out, "  [1] %s\n", i18n.T("cmd.init.wizard_rejoin_token_new"))
			fmt.Fprintf(a.IO.Out, "  [2] %s\n", i18n.T("cmd.init.wizard_rejoin_token_skip"))
			fmt.Fprintf(a.IO.Out, "  Choice [1]: ")
			var choiceStr string
			fmt.Fscanln(a.IO.In, &choiceStr)
			if choiceStr != "2" {
				fmt.Fprintf(a.IO.Out, "  %s: ", i18n.T("cmd.init.wizard_rejoin_gitlab_token_label"))
				if term.IsTerminal(int(os.Stdin.Fd())) {
					tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
					fmt.Fprintln(a.IO.Out)
					if err == nil && len(tokenBytes) > 0 && a.Secrets != nil {
						if sErr := a.Secrets.Set(ctx, config.TeamGitLabTokenKey(teamID), string(tokenBytes)); sErr != nil {
							slog.Error("failed to save GitLab token to keychain", "team", teamID, "error", sErr)
						}
					}
				}
			}
		}
	}

	// ── Step 4: Validate GitLab identity + write config ──
	fmt.Fprintf(a.IO.Out, "\n  %s %s\n", theme.WarningStyle.Render(theme.IconDot), i18n.T("cmd.team.rejoin.validating"))
	result, err := teamRejoinCore(ctx, a, teamRejoinParams{
		StateRepo: repoURL,
		MemberID:  memberID,
	})
	if err != nil {
		return err
	}

	// ── Step 4b: Handle identity mismatch (bot token or wrong user) ──
	if result.IdentityMismatch != nil {
		fmt.Fprintln(a.IO.Out)
		// Show clear error message
		var msg string
		if result.IdentityMismatch.IsBot {
			msg = i18n.Tf("cmd.init.wizard_identity_bot_detected", result.IdentityMismatch.AuthenticatedAs)
		} else {
			msg = i18n.Tf("cmd.init.wizard_identity_user_mismatch",
				result.IdentityMismatch.AuthenticatedAs,
				result.IdentityMismatch.MemberID,
				result.IdentityMismatch.ExpectedUser,
			)
		}
		fmt.Fprintf(a.IO.Out, "  %s %s\n",
			theme.WarningStyle.Render(theme.IconDot), msg,
		)
		fmt.Fprintf(a.IO.Out, "  [1] %s\n", i18n.T("cmd.init.wizard_identity_verify"))
		fmt.Fprintf(a.IO.Out, "  [2] %s\n", i18n.T("cmd.init.wizard_identity_skip"))
		fmt.Fprintf(a.IO.Out, "  Choice [2]: ")
		var choiceStr string
		fmt.Fscanln(a.IO.In, &choiceStr)
		if choiceStr == "1" {
			fmt.Fprintf(a.IO.Out, "  %s: ", i18n.T("cmd.init.wizard_rejoin_gitlab_token_label"))
			if term.IsTerminal(int(os.Stdin.Fd())) {
				tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(a.IO.Out)
				if err == nil && len(tokenBytes) > 0 {
					teamID := config.RepoNameFromRemote(repoURL)
					if a.Secrets != nil {
						if sErr := a.Secrets.Set(ctx, config.TeamGitLabTokenKey(teamID), string(tokenBytes)); sErr != nil {
							slog.Error("failed to save GitLab token to keychain", "team", teamID, "error", sErr)
						}
					}
					// Re-validate
					sp := config.TeamStatePath(repoURL)
					repo := teamstate.NewRepo(repoURL, sp)
					member, _ := repo.GetMember(memberID)
					if member != nil {
						if err := validateGitLabIdentity(ctx, a, repo, member); err != nil {
							fmt.Fprintf(a.IO.Out, "  %s %s\n",
								theme.WarningStyle.Render(theme.IconDot), err.Error(),
							)
						} else {
							fmt.Fprintf(a.IO.Out, "  %s Identity verified\n",
								theme.SuccessStyle.Render(theme.IconSuccess),
							)
						}
					}
				}
			}
		}
	}

	// Success output
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "  %s %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.Tf("cmd.team.rejoin.success", result.Member.DisplayName, result.Member.ID, result.TeamName),
	)

	if result.EventCount > 0 {
		fmt.Fprintf(a.IO.Out, "  %s %s\n",
			theme.InfoStyle.Render(theme.IconInfo),
			i18n.Tf("cmd.team.rejoin.events_found", result.EventCount),
		)
	}

	// ── Step 5: Retro-tag sessions ──
	if !noRetroTag {
		count, err := retroTagSessions(ctx, memberID)
		if err != nil {
			slog.Warn("retro-tag failed", "error", err)
		} else if count > 0 {
			fmt.Fprintf(a.IO.Out, "  %s %s\n",
				theme.SuccessStyle.Render(theme.IconSuccess),
				i18n.Tf("cmd.team.rejoin.retro_tagged", count),
			)
		}
	}

	// ── Tracker hint ──
	fmt.Fprintf(a.IO.Out, "\n  %s %s\n",
		theme.InfoStyle.Render(theme.IconInfo),
		i18n.T("cmd.team.tracker_hint"),
	)

	fmt.Fprintln(a.IO.Out)
	return nil
}
