package views

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ActivityView displays the team activity stream.
type ActivityView struct {
	app         *tview.Application
	resolveTeam ResolveTeamFunc
	shell       ShellAccess
	list        *tview.TextView
	filter      string // "today", "week", "all"
}

var _ View = (*ActivityView)(nil)

// NewActivityView creates a new team activity view.
// resolveTeam is called on every refresh to obtain the effective team config.
func NewActivityView(resolveTeam ResolveTeamFunc) *ActivityView {
	return &ActivityView{resolveTeam: resolveTeam, filter: "week"}
}

// SetShell provides the shell reference for toast notifications.
func (v *ActivityView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *ActivityView) ID() string { return "team.activity" }

// Title returns the display title.
func (v *ActivityView) Title() string { return "Activité" }

// StatusHints returns keybinding hints.
func (v *ActivityView) StatusHints() string {
	return "t aujourd'hui · w semaine · a tout · r refresh · Esc retour"
}

// Mount builds the activity stream.
func (v *ActivityView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.list = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	v.list.SetBackgroundColor(theme.BgPanel)
	v.list.SetBorderPadding(1, 0, 2, 2)

	v.refresh()
	content.AddItem(v.list, 0, 1, true)
}

// Unmount cleans up resources.
func (v *ActivityView) Unmount() {
	v.app = nil
	v.list = nil
}

// HandleKey processes activity view key events.
func (v *ActivityView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 't':
		v.filter = "today"
		v.refresh()
		return nil
	case 'w':
		v.filter = "week"
		v.refresh()
		return nil
	case 'a':
		v.filter = "all"
		v.refresh()
		return nil
	case 'r':
		v.refresh()
		return nil
	}
	return event
}

func (v *ActivityView) refresh() {
	if v.list == nil {
		return
	}

	tc := v.resolveTeam()
	if !tc.Enabled {
		v.list.SetText("  Équipe non configurée pour ce projet. Utilisez 'team configure' dans l'omnibar.")
		return
	}

	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)

	if !repo.IsCloned() {
		v.list.SetText("  Team state non cloné. Lancez 'oh team init'.")
		return
	}

	// Async pull — updates the view on completion (or after > 1s toast).
	syncAsync(v.app, repo, v.shell, func(_ error) {
		v.renderEvents(repo)
	})
}

func (v *ActivityView) renderEvents(repo *teamstate.Repo) {
	if v.list == nil {
		return
	}

	// Determine time filter
	var since time.Time
	switch v.filter {
	case "today":
		now := time.Now()
		since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "week":
		since = time.Now().AddDate(0, 0, -7)
	case "all":
		since = time.Time{} // epoch
	}

	// Fetch events for all projects
	events, err := repo.ListEvents("", since)
	if err != nil {
		v.list.SetText(fmt.Sprintf("  Erreur: %s", err.Error()))
		return
	}

	if len(events) == 0 {
		filterLabel := v.filterLabel()
		v.list.SetText(fmt.Sprintf("  Aucune activité (%s)", filterLabel))
		return
	}

	// Build display
	var text string
	filterLabel := v.filterLabel()
	text += fmt.Sprintf("  %s%s%s  Filtre: %s\n\n",
		theme.ColorTag(theme.AccentHex), "Activité équipe", theme.TagColor, filterLabel)

	for i := len(events) - 1; i >= 0; i-- { // newest first
		e := events[i]
		ts := e.Timestamp.Local().Format("15:04")
		icon := eventIcon(e.Type)
		actor := e.Actor
		desc := formatEventDescription(e)

		text += fmt.Sprintf("  %s%s%s  %s %s%s%s %s\n",
			theme.ColorTag(theme.TextMutedHex), ts, theme.TagColor,
			icon,
			theme.ColorTag(theme.TextSecondaryHex), actor, theme.TagColor,
			desc,
		)
	}

	v.list.SetText(text)
}

func (v *ActivityView) filterLabel() string {
	switch v.filter {
	case "today":
		return "aujourd'hui"
	case "week":
		return "7 derniers jours"
	case "all":
		return "tout"
	}
	return v.filter
}

func eventIcon(eventType string) string {
	switch eventType {
	case teamstate.EventClaimTaken:
		return "[green]●[-]"
	case teamstate.EventClaimReleased:
		return "[yellow]○[-]"
	case teamstate.EventClaimTransferred:
		return "[blue]◆[-]"
	case teamstate.EventReviewReady:
		return "[purple]◆[-]"
	case teamstate.EventSessionComplete:
		return "[green]✓[-]"
	case teamstate.EventAuditFinding:
		return "[red]![-]"
	case teamstate.EventClaimConflict:
		return "[red]✗[-]"
	default:
		return "[white]·[-]"
	}
}

func formatEventDescription(e teamstate.Event) string {
	switch e.Type {
	case teamstate.EventClaimTaken:
		return fmt.Sprintf("a claim %q", e.Ticket)
	case teamstate.EventClaimReleased:
		return fmt.Sprintf("a libéré %q", e.Ticket)
	case teamstate.EventClaimTransferred:
		to := ""
		if e.Data != nil {
			if t, ok := e.Data["to"].(string); ok {
				to = t
			}
		}
		return fmt.Sprintf("a transféré %q → %s", e.Ticket, to)
	case teamstate.EventReviewReady:
		return fmt.Sprintf("review prête sur %s", e.Project)
	case teamstate.EventSessionComplete:
		return fmt.Sprintf("session terminée sur %s", e.Project)
	case teamstate.EventAuditFinding:
		return fmt.Sprintf("finding audit sur %s", e.Project)
	case teamstate.EventClaimConflict:
		return fmt.Sprintf("conflit de claim sur %q", e.Ticket)
	default:
		return e.Type
	}
}
