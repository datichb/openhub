package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/textfmt"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// TeamStatusView displays team status (who works on what) and activity.
type TeamStatusView struct {
	app         *tview.Application
	resolveTeam ResolveTeamFunc
	shell       ShellAccess
	tv          *tview.TextView
	loads       loadSlot // the reads of the team-state: only the latest is applied

	// read replaces the read of the team-state (tests).
	read func(repo teamstate.TeamStateReader) *teamStatusData
}

// teamStatusData is what a read of the team-state gives back.
type teamStatusData struct {
	members    []teamstate.Member
	claims     []teamstate.Claim
	events     []teamstate.Event
	membersErr error
	claimsErr  error
	eventsErr  error
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
func (v *TeamStatusView) StatusHints() string {
	return keys.Hints(keys.Refresh.Hint(), keys.Back.Hint())
}

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
	v.loads.Invalidate() // drop the read in flight
	v.app = nil
	v.tv = nil
}

// HandleKey processes key events.
func (v *TeamStatusView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if keys.Refresh.Matches(event) {
		v.syncAndRender()
		return nil
	}
	return event
}

// syncAndRender pulls the team-state, reads it off the event loop, then
// draws the status.
func (v *TeamStatusView) syncAndRender() {
	if v.tv == nil {
		return
	}
	tc := v.resolveTeam()
	if !tc.Enabled {
		v.loads.Invalidate()
		v.render(tc, nil)
		return
	}
	setState(v.tv, stateLoading, "")
	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	read := v.read
	if read == nil {
		read = readTeamStatus
	}
	fetch := func(context.Context) (*teamStatusData, error) { return read(repo), nil }
	syncLoad(v.app, shellContext(v.shell), &v.loads, repo, v.shell, fetch, func(d *teamStatusData, _ error) {
		if v.tv != nil {
			v.render(tc, d)
		}
	})
}

// readTeamStatus reads the members, the claims and the last events; nil when
// the clone of the team-state is missing. It reads files: call it off the
// event loop.
func readTeamStatus(repo teamstate.TeamStateReader) *teamStatusData {
	if !repo.IsCloned() {
		return nil
	}
	var d teamStatusData
	d.members, d.membersErr = repo.ListMembers()
	d.claims, d.claimsErr = repo.ListClaims("")
	d.events, d.eventsErr = repo.ListEventsLimited("", 5)
	return &d
}

// render draws the status; d is nil when the clone of the team-state is
// missing.
func (v *TeamStatusView) render(tc TeamResolution, d *teamStatusData) {
	bold, plain := theme.StyleTag(theme.AttrBold), theme.TagAttrs
	secondary := func(s string) string { return theme.Paint(theme.TextSecondaryHex, s) }

	var sb strings.Builder
	fmt.Fprintf(&sb, "\n  %s%s%s\n\n", bold, i18n.T("tui.teamstatus.title"), plain)

	if !tc.Enabled {
		fmt.Fprintf(&sb, "  %s\n\n", secondary(i18n.T("tui.teamstatus.team_not_configured")))
		fmt.Fprintf(&sb, "  %s\n", secondary(i18n.T("tui.teamstatus.configure_hint")))
		v.tv.SetText(sb.String())
		return
	}

	fmt.Fprintf(&sb, "  %s    %s\n", secondary(i18n.T("tui.teamstatus.repo_label")), tview.Escape(tc.StateRepo))
	fmt.Fprintf(&sb, "  %s  %s\n\n", secondary(i18n.T("tui.teamstatus.member_label")), tview.Escape(tc.MemberID))

	if d == nil {
		fmt.Fprintf(&sb, "  %s\n", stateText(stateEmpty, i18n.T("tui.teamstatus.repo_not_cloned")))
		v.tv.SetText(sb.String())
		return
	}

	// --- Members with assignments ---
	if d.membersErr != nil || d.claimsErr != nil {
		if v.shell != nil {
			err := d.membersErr
			if err == nil {
				err = d.claimsErr
			}
			v.shell.ShowToastMsg(i18n.Tf("tui.teamstatus.error_loading", err.Error()), false)
		}
	}

	// Build member → claims map
	memberClaims := make(map[string][]teamstate.Claim)
	for _, c := range d.claims {
		memberClaims[c.ClaimedBy] = append(memberClaims[c.ClaimedBy], c)
	}

	if len(d.members) > 0 {
		fmt.Fprintf(&sb, "  %s%s%s\n\n", bold, i18n.Tf("tui.teamstatus.members_header", len(d.members)), plain)
		for _, m := range d.members {
			myClaims := memberClaims[m.ID]
			countKey := "tui.teamstatus.ticket_count_plural"
			if len(myClaims) == 1 {
				countKey = "tui.teamstatus.ticket_count"
			}

			// Highlight current user
			nameColor := theme.TextSecondaryHex
			if m.ID == tc.MemberID {
				nameColor = theme.ActiveMode.PrimaryHex
			}

			fmt.Fprintf(&sb, "  %s %s",
				theme.Paint(nameColor, fmt.Sprintf("@%-12s", tview.Escape(m.ID))),
				secondary(fmt.Sprintf("%-14s", i18n.Tf(countKey, len(myClaims)))))

			if len(myClaims) > 0 {
				var parts []string
				for _, c := range myClaims {
					parts = append(parts, tview.Escape(fmt.Sprintf("%s (%s)", c.TicketID, c.Status)))
				}
				sb.WriteString(" " + strings.Join(parts, ", "))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// --- Status summary ---
	statusCounts := make(map[string]int)
	for _, c := range d.claims {
		statusCounts[c.Status]++
	}
	if len(d.claims) > 0 {
		fmt.Fprintf(&sb, "  %s%s%s\n\n  ", bold, i18n.T("tui.teamstatus.summary_header"), plain)
		statuses := []struct {
			key   string
			label string
		}{
			{teamstate.ClaimStatusPlanned, i18n.T("tui.teamstatus.status_planned")},
			{teamstate.ClaimStatusInProgress, i18n.T("tui.teamstatus.status_in_progress")},
			{teamstate.ClaimStatusReview, i18n.T("tui.teamstatus.status_review")},
			{teamstate.ClaimStatusValidation, i18n.T("tui.teamstatus.status_validation")},
			{teamstate.ClaimStatusBlocked, i18n.T("tui.teamstatus.status_blocked")},
			{teamstate.ClaimStatusDone, i18n.T("tui.teamstatus.status_done")},
		}
		var parts []string
		for _, s := range statuses {
			parts = append(parts, fmt.Sprintf("%s %d %s", theme.IconDot, statusCounts[s.key], s.label))
		}
		sb.WriteString(strings.Join(parts, "  "))
		sb.WriteString("\n\n")
	}

	// --- Recent activity ---
	if d.eventsErr != nil && v.shell != nil {
		v.shell.ShowToastMsg(i18n.Tf("tui.teamstatus.error_events", d.eventsErr.Error()), false)
	}
	fmt.Fprintf(&sb, "  %s%s%s\n\n", bold, i18n.T("tui.teamstatus.recent_activity_header"), plain)
	if len(d.events) == 0 {
		fmt.Fprintf(&sb, "  %s\n", stateText(stateEmpty, i18n.T("tui.teamstatus.no_recent_activity")))
	} else {
		for _, e := range d.events {
			fmt.Fprintf(&sb, "  %s %s %s %s\n",
				secondary(fmt.Sprintf("%-12s", textfmt.AgoOrDate(e.Timestamp))), eventIcon(e.Type),
				tview.Escape(e.Actor), tview.Escape(formatEventDescription(e)))
		}
	}

	v.tv.SetText(sb.String())
}
