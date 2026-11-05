package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/glyph"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/textfmt"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

var teamCmd = &cobra.Command{
	Use:               "team",
	Short:             i18n.T("cmd.team.short"),
	Long:              i18n.T("cmd.team.long"),
	PersistentPreRunE: teamPreRunE,
}

func init() { addTeamFlag(teamCmd) }

// teamRepo is the shared team-state repository instance, resolved once by
// teamPreRunE and reused by all team subcommands. Commands that need to work
// before the repo exists (e.g. `team init`) override PersistentPreRunE to nil.
var teamRepo *teamstate.Repo

// teamPreRunE resolves the team the subcommand acts on (ResolveTeam: --team,
// else the project, else the default team) and its team-state repo. The
// choice is kept in the context of the command, so that the repository and
// the member of the command come from the same team.
// It skips the repo if the command is `team init`, `team rejoin` (repo doesn't exist yet) or
// `team profile` (the team service checks the clone of the team it is asked for).
func teamPreRunE(cmd *cobra.Command, _ []string) error {
	teamFlagMiddleware(cmd)
	// Skip for commands that don't need an existing repo (promote works on
	// a solo team-state, which is never the active team).
	if cmd.Name() == "init" || cmd.Name() == "rejoin" || cmd.Name() == "promote" || cmd.Name() == "profile" {
		// profile and promote act on a team that must exist (promote: a solo
		// space); init and rejoin create the team they act on.
		if err := refuseUnusedTeamFlag(cmd); err != nil {
			return err
		}
		if id := teamAskOf(cmd.Context()).Team; id != "" && cmd.Name() != "init" && cmd.Name() != "rejoin" {
			if _, _, err := MustApp().Config.PickTeam(config.TeamSelection{Flag: id}); err != nil {
				return errors.New(i18n.Tf("cmd.teams.not_found", id, theme.Bold.Render("oh teams list")))
			}
		}
		return nil
	}
	a := MustApp()
	ctx := cmd.Context()
	t, err := ResolveTeam(ctx, a)
	if err != nil {
		return err
	}
	repo, err := ensureTeamRepoFor(t)
	if err != nil {
		return err
	}
	cmd.SetContext(withResolvedTeam(ctx, t))
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
	RunE:  runTeamRejoin,
}

func init() {
	rootCmd.AddCommand(teamCmd)
	teamCmd.AddCommand(teamInitCmd)
	teamCmd.AddCommand(teamStatusCmd)
	teamCmd.AddCommand(teamActivityCmd)
	teamCmd.AddCommand(teamRejoinCmd)

	teamActivityCmd.Flags().Bool("today", false, i18n.T("cmd.team.activity.flags.today"))
	teamActivityCmd.Flags().Bool("week", false, i18n.T("cmd.team.activity.flags.week"))
	teamActivityCmd.Flags().String("member", "", i18n.T("cmd.team.activity.flags.member"))
	teamActivityCmd.Flags().String("project", "", i18n.T("cmd.team.activity.flags.project"))
	teamActivityCmd.Flags().Int("limit", 20, i18n.T("cmd.team.activity.flags.limit"))

	teamStatusCmd.Flags().Bool("detail", false, i18n.T("cmd.team.status.flags.detail"))

	addTeamRejoinFlags(teamRejoinCmd)
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
	members, err := repo.ListMembers(ctx)
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.team.listing_members"), err)
	}

	if len(members) == 0 {
		fmt.Fprintf(a.IO.Out, "\n%s %s\n\n",
			theme.Subtitle.Render(theme.IconInfo),
			i18n.Tf("cmd.team.status.empty", theme.Bold.Render("oh team init")))
		return nil
	}

	// List all claims
	claims, err := repo.ListClaims(ctx, "")
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.team.listing_claims"), err)
	}

	names := views.NewNames(registryNames(ctx, a), members)

	// Index claims by member
	claimsByMember := make(map[string][]teamstate.Claim)
	for _, c := range claims {
		claimsByMember[c.ClaimedBy] = append(claimsByMember[c.ClaimedBy], c)
	}

	// Counters, by the roles of the columns of the board of the team
	var board teamstate.BoardConfig
	if cfg, cfgErr := repo.LoadConfig(ctx); cfgErr == nil && cfg != nil {
		board = cfg.Board
	}
	counts := teamstate.CountByRole(claims, board)
	activeCount, reviewCount, blockedCount := counts.Active, counts.Review, counts.Blocked

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

	// claimRows prints the claims of one row group (a member, the reserve, a
	// member who left): the name and the role on the first line only.
	claimRows := func(name, role string, list []teamstate.Claim) {
		for i, c := range list {
			since := c.ClaimedAt
			if !c.LastActivity.IsZero() {
				since = c.LastActivity.Time()
			}
			sinceStr := textfmt.Duration(time.Since(since))

			memberCol, roleCol := name, role
			if i > 0 {
				memberCol, roleCol = "", ""
			}

			ticketStr := fmt.Sprintf("%s/%s", names.Project(c.Project), c.TicketID)
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n",
				memberCol, roleCol, ticketStr, c.Status, sinceStr)

			// Show MR URL as indented detail line when present.
			if c.MRURL != "" {
				fmt.Fprintf(w, "  \t\t  └ MR: %s\t\t\n", c.MRURL)
			}

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

	known := make(map[string]bool, len(members))
	for _, m := range members {
		known[m.ID] = true
		memberClaims := claimsByMember[m.ID]

		if len(memberClaims) == 0 {
			fmt.Fprintf(w, "  %s\t%s\t%s\t\t\n",
				m.DisplayName, m.Role,
				theme.Subtitle.Render(i18n.T("cmd.team.status.idle")))
			continue
		}
		claimRows(m.DisplayName, m.Role, memberClaims)
	}

	// The tickets nobody holds (the reserve) and those of a member who is no
	// longer in the team are claims all the same: they are listed, not dropped
	// while the counters count them.
	claimRows(i18n.T("v54y.team.status.reserve"), "", claimsByMember[""])
	var gone []string
	for id := range claimsByMember {
		if id != "" && !known[id] {
			gone = append(gone, id)
		}
	}
	sort.Strings(gone)
	for _, id := range gone {
		claimRows(i18n.Tf("v54y.team.status.gone", names.Member(id)), "", claimsByMember[id])
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
		fmt.Fprint(a.IO.ErrOut, i18n.Tf("cmd.team.cannot_sync", theme.WarningStyle.Render(theme.IconWarning), err))
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

	events, err := repo.ListEvents(ctx, project, since)
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.team.listing_events"), err)
	}

	// Filter by member if specified
	member, _ := cmd.Flags().GetString("member")
	if member != "" {
		filtered := events[:0]
		for _, e := range events {
			if e.Involves(member) {
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

	names := readNames(ctx, a, repo)
	for _, e := range events {
		ts := textfmt.Clock(e.Timestamp)
		fmt.Fprintf(a.IO.Out, "  %s %s %s %s\n",
			theme.Subtitle.Render(ts), eventSymbol(e), theme.Bold.Render(names.Member(e.Actor)), e.DescribeAs(names))
	}

	if totalCount > limit {
		fmt.Fprintf(a.IO.Out, "\n  %s %s\n",
			theme.Subtitle.Render(theme.IconInfo),
			i18n.Tf("cmd.team.activity.truncated", limit, totalCount))
	}
	fmt.Fprintln(a.IO.Out)

	return nil
}

// ensureTeamRepo returns a ready Repo of the team ResolveTeam gives, or an
// error if there is no such team or it is not cloned.
func ensureTeamRepo(ctx context.Context, a *app.App) (*teamstate.Repo, error) {
	t, err := ResolveTeam(ctx, a)
	if err != nil {
		return nil, err
	}
	return ensureTeamRepoFor(t)
}

// ensureTeamRepoFor returns the Repo of team t, which must be enabled and cloned.
func ensureTeamRepoFor(t resolvedTeam) (*teamstate.Repo, error) {
	if !t.Enabled {
		return nil, t.notEnabledError()
	}
	repo := teamstate.NewRepo(t.StateRepo, t.StatePath)
	if !repo.IsCloned() {
		return nil, fmt.Errorf(i18n.T("cmd.team.team_state_repository_not_found_in"), t.StatePath, theme.Bold.Render("oh team init"))
	}
	return repo, nil
}

// writeTeamConfig updates hub.toml with team settings using the new [[teams]]
// format (Service.SaveTeam): it changes the team on hub.toml as it is with
// config.Update, then brings the in-memory Config up to date.
func writeTeamConfig(stateRepo, statePath, memberID string) error {
	return teamService(MustApp()).SaveTeam(stateRepo, statePath, memberID, "")
}

// eventSymbol is the symbol of an event (glyph.Events, shared with the TUI),
// painted with the style of its tone.
func eventSymbol(e teamstate.Event) string {
	g := e.Glyph()
	switch g.Tone {
	case glyph.ToneSuccess:
		return theme.SuccessStyle.Render(g.Glyph)
	case glyph.ToneError:
		return theme.ErrorStyle.Render(g.Glyph)
	case glyph.ToneWarning:
		return theme.WarningStyle.Render(g.Glyph)
	}
	return theme.Subtitle.Render(g.Glyph)
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
