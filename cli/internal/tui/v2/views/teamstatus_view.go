package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// TeamStatusView displays team status (who works on what) and activity.
type TeamStatusView struct {
	app         *tview.Application
	resolveTeam ResolveTeamFunc
	shell       ShellAccess
	tv          *tview.TextView
}

var _ View = (*TeamStatusView)(nil)

// NewTeamStatusView creates a new team status view.
// resolveTeam is called on every render to obtain the effective team config
// for the currently active project (per-project override → hub fallback).
func NewTeamStatusView(resolveTeam ResolveTeamFunc) *TeamStatusView {
	return &TeamStatusView{resolveTeam: resolveTeam}
}

// SetShell provides the shell reference for toast notifications.
func (v *TeamStatusView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *TeamStatusView) ID() string { return "team.status" }

// Title returns the display title.
func (v *TeamStatusView) Title() string { return i18n.T("tui.team.status") }

// StatusHints returns keybinding hints.
func (v *TeamStatusView) StatusHints() string { return "r refresh · Esc retour" }

// Mount builds the team status display.
func (v *TeamStatusView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.tv = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	v.tv.SetBackgroundColor(theme.BgPanel)
	v.tv.SetBorderPadding(1, 0, 2, 2)

	v.syncAndRender()
	content.AddItem(v.tv, 0, 1, true)
}

// Unmount cleans up resources.
func (v *TeamStatusView) Unmount() {
	v.app = nil
	v.tv = nil
}

// HandleKey processes key events.
func (v *TeamStatusView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Rune() == 'r' {
		v.syncAndRender()
		return nil
	}
	return event
}

// syncAndRender performs an async pull then re-renders the status view.
func (v *TeamStatusView) syncAndRender() {
	tc := v.resolveTeam()
	if !tc.Enabled {
		v.render(tc, nil)
		return
	}
	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	if !repo.IsCloned() {
		v.render(tc, nil)
		return
	}
	syncAsync(v.app, repo, v.shell, func(_ error) {
		v.render(tc, repo)
	})
}

func (v *TeamStatusView) render(tc TeamResolution, repo *teamstate.Repo) {
	if v.tv == nil {
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n  [::b]Statut de l'équipe%s\n\n", theme.TagReset))

	if !tc.Enabled {
		sb.WriteString(fmt.Sprintf("  %sÉquipe non configurée pour ce projet.%s\n\n",
			theme.ColorTag(theme.TextSecondaryHex), theme.TagColor))
		sb.WriteString(fmt.Sprintf("  %sUtilisez 'team configure' dans l'omnibar pour configurer.%s\n",
			theme.ColorTag(theme.TextSecondaryHex), theme.TagColor))
		v.tv.SetText(sb.String())
		return
	}

	sb.WriteString(fmt.Sprintf("  %sRepo :%s    %s\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, tc.StateRepo))
	sb.WriteString(fmt.Sprintf("  %sMembre :%s  %s\n\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, tc.MemberID))

	if repo == nil {
		sb.WriteString(fmt.Sprintf("  %sRepo non cloné. Exécutez 'oh team init'.%s\n",
			theme.ColorTag(theme.TextSecondaryHex), theme.TagColor))
		v.tv.SetText(sb.String())
		return
	}

	// --- Members with assignments ---
	members, membersErr := repo.ListMembers()
	claims, claimsErr := repo.ListClaims("")

	if membersErr != nil || claimsErr != nil {
		if v.shell != nil {
			errMsg := ""
			if membersErr != nil {
				errMsg = membersErr.Error()
			} else {
				errMsg = claimsErr.Error()
			}
			v.shell.ShowToastMsg("Erreur chargement: "+errMsg, false)
		}
	}

	// Build member → claims map
	memberClaims := make(map[string][]teamstate.Claim)
	for _, c := range claims {
		memberClaims[c.ClaimedBy] = append(memberClaims[c.ClaimedBy], c)
	}

	if len(members) > 0 {
		sb.WriteString(fmt.Sprintf("  [::b]─── Membres (%d) ───%s\n\n", len(members), theme.TagReset))
		for _, m := range members {
			myClaims := memberClaims[m.ID]
			countStr := fmt.Sprintf("[%d ticket", len(myClaims))
			if len(myClaims) != 1 {
				countStr += "s"
			}
			countStr += "]"

			// Highlight current user
			nameColor := theme.ColorTag(theme.TextSecondaryHex)
			if m.ID == tc.MemberID {
				nameColor = theme.ColorTag(theme.AccentHex)
			}

			sb.WriteString(fmt.Sprintf("  %s@%-12s%s %s%-14s%s",
				nameColor, m.ID, theme.TagColor,
				theme.ColorTag(theme.TextSecondaryHex), countStr, theme.TagColor))

			if len(myClaims) > 0 {
				var parts []string
				for _, c := range myClaims {
					parts = append(parts, fmt.Sprintf("%s (%s)", c.TicketID, c.Status))
				}
				sb.WriteString(" " + strings.Join(parts, ", "))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// --- Status summary ---
	statusCounts := make(map[string]int)
	for _, c := range claims {
		statusCounts[c.Status]++
	}
	if len(claims) > 0 {
		sb.WriteString(fmt.Sprintf("  [::b]─── Résumé ───%s\n\n  ", theme.TagReset))
		statuses := []struct {
			key   string
			label string
		}{
			{teamstate.ClaimStatusPlanned, "planned"},
			{teamstate.ClaimStatusInProgress, "in progress"},
			{teamstate.ClaimStatusReview, "review"},
			{teamstate.ClaimStatusBlocked, "blocked"},
			{teamstate.ClaimStatusDone, "done"},
		}
		var parts []string
		for _, s := range statuses {
			count := statusCounts[s.key]
			parts = append(parts, fmt.Sprintf("● %d %s", count, s.label))
		}
		sb.WriteString(strings.Join(parts, "  "))
		sb.WriteString("\n\n")
	}

	// --- Recent activity ---
	events, eventsErr := repo.ListEventsLimited("", 5)
	if eventsErr != nil && v.shell != nil {
		v.shell.ShowToastMsg("Erreur événements: "+eventsErr.Error(), false)
	}
	sb.WriteString(fmt.Sprintf("  [::b]─── Activité récente ───%s\n\n", theme.TagReset))
	if len(events) == 0 {
		sb.WriteString(fmt.Sprintf("  %sAucune activité récente.%s\n",
			theme.ColorTag(theme.TextSecondaryHex), theme.TagColor))
	} else {
		for _, e := range events {
			ago := formatTimeAgo(e.Timestamp)
			sb.WriteString(fmt.Sprintf("  %s%-12s%s %s %s %s\n",
				theme.ColorTag(theme.TextSecondaryHex), ago, theme.TagColor,
				e.Actor, formatEventType(e.Type), e.Ticket))
		}
	}

	v.tv.SetText(sb.String())
}

// formatTimeAgo returns a human-readable relative time string.
func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "à l'instant"
	case d < time.Hour:
		return fmt.Sprintf("il y a %dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("il y a %dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "hier"
		}
		return fmt.Sprintf("il y a %dj", days)
	default:
		return t.Format("02 Jan")
	}
}

// formatEventType returns a verb for the event type.
func formatEventType(eventType string) string {
	switch eventType {
	case teamstate.EventClaimTaken:
		return "a pris"
	case teamstate.EventClaimReleased:
		return "a libéré"
	case teamstate.EventClaimTransferred:
		return "a transféré"
	case teamstate.EventSessionComplete:
		return "a terminé"
	case teamstate.EventReviewReady:
		return "review prête"
	case teamstate.EventAuditFinding:
		return "audit sur"
	default:
		return eventType
	}
}
