package views

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/glyph"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/textfmt"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// TeamMemberSession is a session of another member of the team, as the claims
// of the team-state tell it: who works on which ticket, in which session, since
// when. It is all the team landing knows of it (10 §3): the session itself runs
// on the machine of the member, so the landing only shows it, it never acts on
// it.
type TeamMemberSession struct {
	// Member is the display name of the member as it was typed; MemberID its ID.
	Member, MemberID string
	Ticket, Title    string
	Project          string
	// ProjectName is how the project is shown (its name, or its ID marked as
	// unknown); empty: the ID.
	ProjectName string
	Session     string
	// Status is the status of the claim.
	Status string
	// Since is when the member took the ticket.
	Since time.Time
	// Live: the claim is not expired (its lease runs, or it waits for a
	// review, which never expires).
	Live bool
}

// OtherMemberSessions lists the sessions of the other members from the claims
// of the team-state, the most recent first: the claims held by someone else,
// with a session on them, neither planned nor finished. self is the ID of the
// member who looks; members gives the display names; cfg (nil: the defaults)
// decides which claims are expired at now.
func OtherMemberSessions(claims []teamstate.Claim, members []teamstate.Member, cfg *teamstate.TeamConfig, self string, now time.Time) []TeamMemberSession {
	names := make(map[string]string, len(members))
	for _, m := range members {
		names[m.ID] = m.DisplayName
	}
	var terminals []string
	if cfg != nil {
		terminals = cfg.Board.TerminalStatuses()
	}
	var out []TeamMemberSession
	for i := range claims {
		c := &claims[i]
		if c.ClaimedBy == "" || c.ClaimedBy == self || c.Session == "" ||
			c.Status == teamstate.ClaimStatusPlanned || c.Status == teamstate.ClaimStatusDone || slices.Contains(terminals, c.Status) {
			continue
		}
		name := names[c.ClaimedBy]
		if name == "" {
			name = c.ClaimedBy // no longer in the registry
		}
		out = append(out, TeamMemberSession{
			Member: name, MemberID: c.ClaimedBy, Ticket: c.TicketID, Title: c.Title, Project: c.Project,
			Session: c.Session, Status: c.Status, Since: c.ClaimedAt, Live: !c.IsExpired(now, cfg),
		})
	}
	slices.SortStableFunc(out, func(a, b TeamMemberSession) int { return b.Since.Compare(a.Since) })
	return out
}

// icon is the glyph of the session in the list: running (the claim is alive)
// or at rest.
func (s TeamMemberSession) icon() string {
	if s.Live {
		return glyph.Active
	}
	return glyph.Inactive
}

// label is the line of the list: « Alice  bd-50 ».
func (s TeamMemberSession) label() string { return s.Member + "  " + s.Ticket }

// desc is the second column: the session and since when.
func (s TeamMemberSession) desc() string {
	parts := []string{s.Session}
	if age := textfmt.Age(s.Since); age != "" {
		parts = append(parts, i18n.Tf("tui.tm.others.since", age))
	}
	return strings.Join(parts, " · ")
}

// detail is the text of the read only window of a session.
func (s TeamMemberSession) detail() string {
	var b strings.Builder
	field := func(label, value string) {
		if value != "" {
			fmt.Fprintf(&b, "%s %s\n", theme.Paint(theme.TextSecondaryHex, label), tview.Escape(value))
		}
	}
	field(i18n.T("tui.tm.others.field.ticket"), s.Ticket)
	field(i18n.T("tui.tm.others.field.title"), s.Title)
	project := s.ProjectName
	if project == "" {
		project = s.Project
	}
	field(i18n.T("tui.tm.others.field.project"), project)
	field(i18n.T("tui.tm.others.field.session"), s.Session)
	field(i18n.T("tui.tm.others.field.status"), s.Status)
	if !s.Since.IsZero() {
		field(i18n.T("tui.tm.others.field.since"), textfmt.DateTime(s.Since)+" ("+textfmt.Ago(s.Since)+")")
	}
	if !s.Live {
		b.WriteString("\n" + theme.Paint(theme.WarningHex, theme.SymWarning+" "+i18n.T("tui.tm.others.expired")) + "\n")
	}
	b.WriteString("\n" + theme.Paint(theme.TextMutedHex, i18n.T("tui.tm.others.read_only")) + "\n")
	return b.String()
}
