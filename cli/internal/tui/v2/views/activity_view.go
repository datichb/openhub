package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/glyph"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/textfmt"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// The filters of the activity, one key each.
var (
	activityTodayKey = keys.Own("activity.today", 't', "tui.hints.today")
	activityWeekKey  = keys.Own("activity.week", 's', "tui.hints.week")
	activityAllKey   = keys.Own("activity.all", '0', "tui.hints.all")
)

// ActivityView displays the team activity stream.
type ActivityView struct {
	app         *tview.Application
	resolveTeam ResolveTeamFunc
	shell       ShellAccess
	list        *tview.TextView
	filter      string   // "today", "week", "all"
	loads       loadSlot // the reads of the events: only the latest is applied

	// read replaces the read of the events (tests).
	read func(repo activityReader, filter string) (activityData, error)
}

// activityData is what a read of the activity gives back.
type activityData struct {
	cloned bool
	events []teamstate.Event
}

// activityReader is what the view reads of the team-state: whether the clone
// is there, and the events.
type activityReader interface {
	teamstate.RepoInfo
	teamstate.EventReader
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
	return keys.Hints(activityTodayKey.Hint(), activityWeekKey.Hint(), activityAllKey.Hint(), keys.Refresh.Hint(), keys.Back.Hint())
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
	v.loads.Invalidate() // drop the read in flight
	v.app = nil
	v.list = nil
}

// HandleKey processes activity view key events.
func (v *ActivityView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch {
	case activityTodayKey.Matches(event):
		v.filter = "today"
	case activityWeekKey.Matches(event):
		v.filter = "week"
	case activityAllKey.Matches(event):
		v.filter = "all"
	case keys.Refresh.Matches(event):
	default:
		return event
	}
	v.refresh()
	return nil
}

// refresh pulls the team-state, then reads the events off the event loop.
func (v *ActivityView) refresh() {
	if v.list == nil {
		return
	}

	tc := v.resolveTeam()
	if !tc.Enabled {
		v.loads.Invalidate()
		v.list.SetText("  " + stateText(stateEmpty, i18n.T("tui.activity.team_not_configured")))
		return
	}
	v.list.SetText("  " + stateText(stateLoading, ""))

	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	filter, read := v.filter, v.read
	if read == nil {
		read = readActivity
	}
	fetch := func(context.Context) (activityData, error) { return read(repo, filter) }
	// Pull first (the toast of a slow pull, the warning of a failed one).
	syncLoad(v.app, shellContext(v.shell), &v.loads, repo, v.shell, fetch, func(d activityData, err error) {
		if v.list != nil {
			v.render(filter, d, err)
		}
	})
}

// readActivity reads the events of the period of filter. It reads files: call
// it off the event loop.
func readActivity(repo activityReader, filter string) (activityData, error) {
	if !repo.IsCloned() {
		return activityData{}, nil
	}
	events, err := repo.ListEvents("", activitySince(filter, time.Now()))
	return activityData{cloned: true, events: events}, err
}

// activitySince is the start of the period of a filter.
func activitySince(filter string, now time.Time) time.Time {
	switch filter {
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "week":
		return now.AddDate(0, 0, -7)
	}
	return time.Time{} // all
}

func (v *ActivityView) render(filter string, d activityData, err error) {
	switch {
	case err != nil:
		v.list.SetText("  " + stateText(stateError, err.Error()))
		return
	case !d.cloned:
		v.list.SetText("  " + stateText(stateEmpty, i18n.T("tui.activity.repo_not_cloned")))
		return
	case len(d.events) == 0:
		v.list.SetText("  " + stateText(stateEmpty, i18n.Tf("tui.activity.no_activity", v.labelOf(filter))))
		return
	}

	var b strings.Builder
	fmt.Fprintf(&b, "  %s  %s\n\n",
		theme.Paint(theme.ActiveMode.PrimaryHex, i18n.T("tui.activity.title_header")),
		i18n.Tf("tui.activity.filter_label", v.labelOf(filter)))
	for i := len(d.events) - 1; i >= 0; i-- { // newest first
		e := d.events[i]
		fmt.Fprintf(&b, "  %s  %s %s %s\n",
			theme.Paint(theme.TextMutedHex, textfmt.Clock(e.Timestamp)),
			eventIcon(e.Type),
			theme.Paint(theme.TextSecondaryHex, tview.Escape(e.Actor)),
			tview.Escape(formatEventDescription(e)),
		)
	}
	v.list.SetText(b.String())
}

func (v *ActivityView) labelOf(filter string) string {
	switch filter {
	case "today":
		return i18n.T("tui.activity.filter_today")
	case "week":
		return i18n.T("tui.activity.filter_week")
	case "all":
		return i18n.T("tui.activity.filter_all")
	}
	return filter
}

// eventToneHex is the color of the tone of an event (glyph.Events).
var eventToneHex = map[glyph.Tone]string{
	glyph.ToneSuccess: theme.SuccessHex,
	glyph.ToneError:   theme.ErrorHex,
	glyph.ToneWarning: theme.WarningHex,
	glyph.ToneInfo:    theme.InfoHex,
	glyph.ToneMuted:   theme.TextMutedHex,
}

// eventIcon is the symbol of an event type, painted with the color of its
// tone: the table is glyph.Events, shared with oh team activity.
func eventIcon(eventType string) string {
	g := glyph.ForEvent(eventType)
	return tagged(eventToneHex[g.Tone], g.Glyph)
}

// formatEventDescription is the phrase of an event after its actor (Activity
// and Team Status views): Event.Describe, escaped for tview by the callers.
func formatEventDescription(e teamstate.Event) string { return e.Describe() }
