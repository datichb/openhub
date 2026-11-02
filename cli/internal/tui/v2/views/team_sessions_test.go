package views

import (
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

// 10 §3: the sessions of the other members, read from the claims of the
// team-state (member, ticket, session, since when). Read only.
func TestOtherMemberSessions(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) time.Time { return now.Add(-d) }
	future := teamstate.InstantOf(now.Add(time.Hour))
	past := teamstate.InstantOf(now.Add(-time.Hour))
	members := []teamstate.Member{{ID: "alice", DisplayName: "Alice Martin"}, {ID: "ben", DisplayName: "ben"}, {ID: "me", DisplayName: "Moi"}}
	claims := []teamstate.Claim{
		{TicketID: "bd-50", Project: "api", ClaimedBy: "alice", Status: teamstate.ClaimStatusInProgress, Session: "ses_a", ClaimedAt: at(26 * time.Hour), LeaseUntil: future, Title: "Fix login"},
		{TicketID: "bd-51", Project: "web", ClaimedBy: "ben", Status: teamstate.ClaimStatusReview, Session: "ses_b", ClaimedAt: at(2 * time.Hour), LeaseUntil: past},
		{TicketID: "bd-52", Project: "api", ClaimedBy: "alice", Status: teamstate.ClaimStatusInProgress, Session: "ses_c", ClaimedAt: at(3 * time.Hour), LeaseUntil: past},
		{TicketID: "bd-53", Project: "api", ClaimedBy: "me", Status: teamstate.ClaimStatusInProgress, Session: "ses_me", ClaimedAt: at(time.Hour)},
		{TicketID: "bd-54", Project: "api", ClaimedBy: "ben", Status: teamstate.ClaimStatusInProgress, ClaimedAt: at(time.Hour)},
		{TicketID: "bd-55", Project: "api", ClaimedBy: "ben", Status: teamstate.ClaimStatusDone, Session: "ses_d", ClaimedAt: at(time.Hour)},
		{TicketID: "bd-56", Project: "api", ClaimedBy: "ben", Status: teamstate.ClaimStatusPlanned, Session: "ses_e", ClaimedAt: at(time.Hour)},
		{TicketID: "bd-57", Project: "api", Status: teamstate.ClaimStatusInProgress, Session: "ses_f", ClaimedAt: at(time.Hour)},
	}
	got := OtherMemberSessions(claims, members, &teamstate.TeamConfig{}, "me", now)

	require.Len(t, got, 3, "the sessions of the others only: not mine, not without a session, not finished or planned, not of the pool")
	assert.Equal(t, []string{"bd-51", "bd-52", "bd-50"}, []string{got[0].Ticket, got[1].Ticket, got[2].Ticket}, "the most recent first")
	assert.Equal(t, "Alice Martin", got[2].Member, "the display name as it was typed")
	assert.Equal(t, "ses_a", got[2].Session)
	assert.Equal(t, "Fix login", got[2].Title)
	assert.True(t, got[2].Live, "lease still valid")
	assert.True(t, got[0].Live, "a claim in review does not expire")
	assert.False(t, got[1].Live, "lease past: nobody renews it")
	assert.Equal(t, at(26*time.Hour), got[2].Since)
	assert.Equal(t, "ben", got[0].Member, "without display name... here it is the one typed")
}

func TestOtherMemberSessionsFallBackOnTheMemberID(t *testing.T) {
	now := time.Now()
	got := OtherMemberSessions([]teamstate.Claim{{TicketID: "t", Project: "p", ClaimedBy: "gone", Status: teamstate.ClaimStatusInProgress, Session: "s", ClaimedAt: now}},
		nil, nil, "me", now)
	require.Len(t, got, 1)
	assert.Equal(t, "gone", got[0].Member)
}

// The section is read only: it lists the sessions, Enter shows the detail and
// nothing writes.
func TestTeamLandingListsTheSessionsOfTheOthersReadOnly(t *testing.T) {
	frLocale(t)
	since := time.Now().Add(-26 * time.Hour)
	v := NewTeamModeView(TeamModeConfig{
		TeamStats: func() TeamModeStats {
			return TeamModeStats{MemberCount: 3, ActiveCount: 2, Others: []TeamMemberSession{
				{Member: "Alice Martin", MemberID: "alice", Ticket: "bd-50", Title: "Fix login", Project: "api", Session: "ses_a", Since: since, Live: true},
				{Member: "ben", MemberID: "ben", Ticket: "bd-51", Project: "web", Session: "ses_b", Since: since},
			}}
		},
	})
	sh := &sessionDetailShell{mockShell: mockShell{team: &ActiveTeam{ID: "t1", Name: "core"}}}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	eventually(t, app, func() bool { return len(v.others) == 2 })

	var labels, descs []string
	var others int
	onLoop(app, func() {
		in := false
		for _, it := range v.items {
			if it.Icon == "─" {
				in = it.SectionID == "others"
				continue
			}
			if in {
				others++
				labels = append(labels, it.Label)
				descs = append(descs, it.Desc)
			}
		}
	})
	require.Equal(t, 2, others)
	assert.Equal(t, "Alice Martin  bd-50", labels[0])
	assert.Contains(t, descs[0], "ses_a")
	assert.Contains(t, descs[0], "1 j", "since when")

	// Enter: a read only window with a single way out, nothing written.
	var idx int
	onLoop(app, func() {
		for i, it := range v.items {
			if it.Label == "Alice Martin  bd-50" {
				idx = i
			}
		}
		v.executeItem(idx)
	})
	assert.Contains(t, sh.title, "Alice Martin")
	assert.Contains(t, sh.body, "bd-50")
	assert.Contains(t, sh.body, "Fix login")
	assert.Contains(t, sh.body, "ses_a")
	require.Len(t, sh.actions, 1, "no action of writing: close only")
	assert.Equal(t, "[Fermer]", sh.actions[0].Label)
}

type sessionDetailShell struct {
	mockShell
	title, body string
	actions     []ModalAction
}

func (s *sessionDetailShell) ShowScrollableModal(title, body string, actions []ModalAction) {
	s.title, s.body, s.actions = title, body, actions
}
