package views

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// The views of the team show the name of a member and of a project, never their
// ID where a name is known; an ID nobody knows stays readable.

func teamNames() NamesFunc {
	return func(context.Context, teamstate.MemberReader) Names {
		return NewNames(map[string]string{"88280c5b": "transparence-sru"}, []teamstate.Member{{ID: "bdatiche", DisplayName: "Benjamin D."}})
	}
}

func TestActivityViewShowsTheNamesOfTheActorAndTheProject(t *testing.T) {
	i18ntest.Set(t, "en")
	_, tc := soloState(t)
	v := NewActivityView(func() TeamResolution { return tc })
	v.SetNames(teamNames())
	v.read = func(context.Context, activityReader, string) (activityData, error) {
		return activityData{cloned: true, events: []teamstate.Event{
			{Timestamp: time.Now(), Actor: "bdatiche", Type: teamstate.EventClaimTaken, Project: "88280c5b", Ticket: "bd-7"},
			{Timestamp: time.Now(), Actor: "ghost", Type: teamstate.EventClaimTaken, Project: "deadbeef", Ticket: "bd-8"},
		}}, nil
	}
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)
	onLoop(loop, func() { v.Mount(content, loop) })
	require.Eventually(t, func() bool { return strings.Contains(textOf(loop, &v.list), "claimed") }, 2*time.Second, 10*time.Millisecond)
	txt := textOf(loop, &v.list)
	assert.Contains(t, txt, "Benjamin D. claimed transparence-sru/bd-7")
	assert.NotContains(t, txt, "bdatiche")
	assert.NotContains(t, txt, "88280c5b")
	assert.Contains(t, txt, "ghost claimed deadbeef/bd-8", "an ID nobody knows stays readable")
}

func TestTeamStatusViewShowsTheDisplayNameOfTheMembers(t *testing.T) {
	i18ntest.Set(t, "en")
	_, tc := soloState(t)
	tc.MemberID = "bdatiche"
	v := NewTeamStatusView(func() TeamResolution { return tc })
	v.SetNames(teamNames())
	v.read = func(context.Context, teamStatusReader) *teamStatusData {
		return &teamStatusData{
			members: []teamstate.Member{{ID: "bdatiche", DisplayName: "Benjamin D."}, {ID: "nameless"}},
			events:  []teamstate.Event{{Timestamp: time.Now(), Actor: "bdatiche", Type: teamstate.EventClaimTaken, Project: "88280c5b", Ticket: "bd-7"}},
		}
	}
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)
	onLoop(loop, func() { v.Mount(content, loop) })
	require.Eventually(t, func() bool { return strings.Contains(textOf(loop, &v.tv), "@Benjamin D.") }, 2*time.Second, 10*time.Millisecond)
	txt := textOf(loop, &v.tv)
	assert.NotContains(t, txt, "@bdatiche", "the list of the members")
	assert.Contains(t, txt, "@nameless", "a member without a display name is their ID")
	assert.Contains(t, txt, "Benjamin D. claimed transparence-sru/bd-7", "the recent activity")
}

func TestTakeoverViewShowsTheProjectAndTheMembersByName(t *testing.T) {
	i18ntest.Set(t, "en")
	_, tc := soloState(t)
	v := NewTakeoverView(func() TeamResolution { return tc })
	v.SetNames(teamNames())
	v.read = func(context.Context, takeoverReader) (takeoverData, error) {
		return takeoverData{cloned: true, briefs: []teamstate.TakeoverMeta{
			{TicketID: "bd-7", Project: "88280c5b", TransferredFrom: "bdatiche", TransferredTo: "gone"},
			{TicketID: "bd-8", Project: "deadbeef", TransferredFrom: "gone", TransferredTo: "bdatiche"},
		}}, nil
	}
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)
	onLoop(loop, func() { v.Mount(content, loop) })
	require.Eventually(t, func() bool {
		var n int
		onLoop(loop, func() { n = len(v.briefs) })
		return n == 2
	}, 2*time.Second, 10*time.Millisecond)
	var main0, sec0, main1, sec1, detail string
	onLoop(loop, func() {
		main0, sec0 = v.list.GetItemText(0)
		main1, sec1 = v.list.GetItemText(1)
		detail = v.page.Detail.GetText(true)
	})
	assert.Contains(t, main0, "bd-7  Benjamin D. → gone")
	assert.Contains(t, sec0, "transparence-sru")
	assert.NotContains(t, main0+sec0+detail, "88280c5b")
	assert.Contains(t, detail, "transparence-sru")
	assert.Contains(t, detail, "Benjamin D.")
	assert.Contains(t, main1, "gone → Benjamin D.")
	assert.Contains(t, sec1, "deadbeef (unknown project)", "an unknown project is marked, its ID stays readable")

	// The IDs are what the actions take.
	onLoop(loop, func() { assert.Equal(t, "88280c5b", v.briefs[0].Project) })
}

// The detail of a project names its team; the ID is only the fallback.
func TestProjectDetailNamesTheTeam(t *testing.T) {
	i18ntest.Set(t, "en")
	v := NewProjectsView(ProjectsViewConfig{})
	v.detail = tview.NewTextView().SetDynamicColors(true)
	teamLine := func(p ProjectItem) string {
		v.showDetail(p)
		for _, l := range strings.Split(v.detail.GetText(true), "\n") {
			if strings.HasPrefix(l, "Team:") {
				return l
			}
		}
		return ""
	}
	assert.Equal(t, "Team: Platform", teamLine(ProjectItem{ID: "a", TeamID: "plat-1", TeamName: "Platform"}))
	assert.Equal(t, "Team: plat-1", teamLine(ProjectItem{ID: "a", TeamID: "plat-1"}), "a team the hub does not list stays readable")
	assert.Equal(t, "Team: none (solo project)", teamLine(ProjectItem{ID: "b"}))
}

// The window of a session of another member names the project.
func TestOtherMemberSessionDetailNamesTheProject(t *testing.T) {
	i18ntest.Set(t, "en")
	s := TeamMemberSession{Member: "Alice", Ticket: "bd-7", Project: "88280c5b", ProjectName: "transparence-sru", Session: "ses_1", Live: true}
	assert.Contains(t, s.detail(), "transparence-sru")
	assert.NotContains(t, s.detail(), "88280c5b")
	s.ProjectName = ""
	assert.Contains(t, s.detail(), "88280c5b", "without a name: the ID")
}
