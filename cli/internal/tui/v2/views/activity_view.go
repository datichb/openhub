package views

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
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
func (v *ActivityView) Title() string { return i18n.T("tui.team.activity") }

// StatusHints returns keybinding hints.
func (v *ActivityView) StatusHints() string {
	return fmt.Sprintf("t %s · w %s · 0 %s · r %s · Esc %s", i18n.T("tui.hints.today"), i18n.T("tui.hints.week"), i18n.T("tui.hints.all"), i18n.T("tui.hints.refresh"), i18n.T("tui.hints.back"))
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
	case '0':
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
		v.list.SetText("  " + i18n.T("tui.activity.team_not_configured"))
		return
	}

	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)

	if !repo.IsCloned() {
		v.list.SetText("  " + i18n.T("tui.activity.repo_not_cloned"))
		return
	}

	// Async pull — updates the view on completion (or after > 1s toast).
	syncAsync(v.app, repo, v.shell, func(_ error) {
		v.renderEvents(repo)
	})
}

func (v *ActivityView) renderEvents(repo teamstate.TeamStateWriter) {
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
		v.list.SetText(fmt.Sprintf("  %s", i18n.Tf("tui.activity.error", err.Error())))
		return
	}

	if len(events) == 0 {
		filterLabel := v.filterLabel()
		v.list.SetText(fmt.Sprintf("  %s", i18n.Tf("tui.activity.no_activity", filterLabel)))
		return
	}

	// Build display
	var text string
	filterLabel := v.filterLabel()
	text += fmt.Sprintf("  %s%s%s  %s\n\n",
		theme.ColorTag(theme.AccentHex), i18n.T("tui.activity.title_header"), theme.TagColor, i18n.Tf("tui.activity.filter_label", filterLabel))

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
		return i18n.T("tui.activity.filter_today")
	case "week":
		return i18n.T("tui.activity.filter_week")
	case "all":
		return i18n.T("tui.activity.filter_all")
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
		return i18n.Tf("tui.activity.event_claim_taken", e.Ticket)
	case teamstate.EventClaimReleased:
		return i18n.Tf("tui.activity.event_claim_released", e.Ticket)
	case teamstate.EventClaimTransferred:
		to := ""
		if e.Data != nil {
			if t, ok := e.Data["to"].(string); ok {
				to = t
			}
		}
		return i18n.Tf("tui.activity.event_claim_transferred", e.Ticket, to)
	case teamstate.EventReviewReady:
		return i18n.Tf("tui.activity.event_review_ready", e.Project)
	case teamstate.EventSessionComplete:
		return i18n.Tf("tui.activity.event_session_complete", e.Project)
	case teamstate.EventAuditFinding:
		return i18n.Tf("tui.activity.event_audit_finding", e.Project)
	case teamstate.EventClaimConflict:
		return i18n.Tf("tui.activity.event_claim_conflict", e.Ticket)
	default:
		return e.Type
	}
}
