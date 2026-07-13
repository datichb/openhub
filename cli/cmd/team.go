package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var teamCmd = &cobra.Command{
	Use:   "team",
	Short: "Gestion de l'équipe et de la collaboration",
	Long: `Commandes pour le travail en équipe : initialisation, statut,
activité, et gestion du wiki partagé.`,
}

var teamInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Active les fonctions d'équipe",
	Long: `Configure la collaboration d'équipe en connectant un repo team-state partagé.

Le repo team-state doit être créé au préalable sur GitLab/GitHub.
Cette commande clone le repo, enregistre le membre courant, et active les
fonctionnalités d'équipe dans le hub.`,
	RunE: runTeamInit,
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

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, common.Title.Render("  Team Setup  "))
	fmt.Fprintln(a.IO.Out)

	var (
		stateRepo          string
		memberID           string
		displayName        string
		gitlabUsername     string
		mattermostUsername string
		role               string
	)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("URL du repo team-state (Git)").
				Description("Ce repo doit exister et être accessible par tous les membres").
				Placeholder("git@gitlab.company.com:team/team-state.git").
				Value(&stateRepo).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("l'URL du repo est requise")
					}
					return nil
				}),
		).Title("Repository d'état partagé"),
		huh.NewGroup(
			huh.NewInput().
				Title("Ton identifiant (clé unique)").
				Description("Sera utilisé comme clé dans members.toml (ex: benjamin, alice)").
				Value(&memberID).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("l'identifiant est requis")
					}
					return nil
				}),
			huh.NewInput().
				Title("Nom d'affichage").
				Value(&displayName),
			huh.NewInput().
				Title("Username GitLab").
				Value(&gitlabUsername),
			huh.NewInput().
				Title("Username Mattermost").
				Value(&mattermostUsername),
			huh.NewSelect[string]().
				Title("Rôle").
				Options(
					huh.NewOption("Lead", "lead"),
					huh.NewOption("Développeur", "dev"),
					huh.NewOption("Reviewer", "reviewer"),
				).
				Value(&role),
		).Title("Ton profil"),
	)

	if err := form.Run(); err != nil {
		return err
	}

	// Determine local path
	statePath := a.Config.Team.StatePath
	if statePath == "" {
		statePath = config.DefaultTeamStatePath()
	}

	// Clone repo
	fmt.Fprintf(a.IO.Out, "\n%s Clonage du repo team-state...\n", common.Subtitle.Render(common.IconArrow))
	repo := teamstate.NewRepo(stateRepo, statePath)
	if err := repo.Clone(ctx); err != nil {
		return fmt.Errorf("cloning team-state: %w", err)
	}
	fmt.Fprintf(a.IO.Out, "%s Repo cloné dans %s\n", common.SuccessStyle.Render(common.IconSuccess), statePath)

	// Initialize directory structure
	if err := repo.InitStructure(ctx); err != nil {
		return fmt.Errorf("initializing structure: %w", err)
	}

	// Add member
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
			fmt.Fprintf(a.IO.Out, "%s Membre %q déjà enregistré\n",
				common.WarningStyle.Render(common.IconWarning), memberID)
		} else {
			return fmt.Errorf("adding member: %w", err)
		}
	} else {
		// Commit and push the new member
		if err := repo.CommitAndPush(ctx, fmt.Sprintf("team: add member %s", memberID), "members.toml"); err != nil {
			return fmt.Errorf("pushing member registration: %w", err)
		}
		fmt.Fprintf(a.IO.Out, "%s Membre %q enregistré\n",
			common.SuccessStyle.Render(common.IconSuccess), memberID)
	}

	// Update hub.toml
	if err := writeTeamConfig(stateRepo, statePath, memberID); err != nil {
		return err
	}
	fmt.Fprintf(a.IO.Out, "%s Configuration hub.toml mise à jour\n",
		common.SuccessStyle.Render(common.IconSuccess))

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "%s Fonctions d'équipe activées !\n",
		common.SuccessStyle.Render(common.IconSuccess))
	fmt.Fprintf(a.IO.Out, "  Utilise %s pour voir l'état de l'équipe.\n",
		common.Bold.Render("oh team status"))

	return nil
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
