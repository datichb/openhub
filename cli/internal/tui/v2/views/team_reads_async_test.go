package views

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/textfmt"
)

// The views that show the team-state read it in load() (after a pull): the
// mount returns at once with « loading », the answer of a read made before an
// unmount is not applied to the next mount, and the view can be mounted again
// (run with -race).

func textOf(loop *tview.Application, tv **tview.TextView) (txt string) {
	onLoop(loop, func() {
		if *tv != nil {
			txt = (*tv).GetText(true)
		}
	})
	return
}

func TestActivityViewLoadAndRemount(t *testing.T) {
	frLocale(t)
	_, tc := soloState(t)
	var calls atomic.Int32
	first, release := make(chan struct{}), make(chan struct{})
	v := NewActivityView(func() TeamResolution { return tc })
	v.read = func(context.Context, activityReader, string) (activityData, error) {
		event := func(ticket string) teamstate.Event {
			return teamstate.Event{Timestamp: time.Now(), Actor: "alice", Type: teamstate.EventClaimTaken, Ticket: ticket}
		}
		if calls.Add(1) == 1 {
			close(first)
			<-release
			return activityData{cloned: true, events: []teamstate.Event{event("ancien-1")}}, nil
		}
		return activityData{cloned: true, events: []teamstate.Event{event("nouveau-2")}}, nil
	}
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)

	onLoop(loop, func() { v.Mount(content, loop) })
	<-first
	assert.Contains(t, textOf(loop, &v.list), "Chargement", "the mount does not wait for the events")

	onLoop(loop, v.Unmount)
	onLoop(loop, func() { v.Mount(tview.NewFlex(), loop) })
	require.Eventually(t, func() bool { return strings.Contains(textOf(loop, &v.list), "nouveau-2") }, 2*time.Second, 10*time.Millisecond)

	close(release)
	time.Sleep(50 * time.Millisecond)
	txt := textOf(loop, &v.list)
	assert.Contains(t, txt, "nouveau-2")
	assert.NotContains(t, txt, "ancien-1", "the answer of the previous mount is dropped")
}

func TestActivityViewReadAfterUnmountIsIgnored(t *testing.T) {
	frLocale(t)
	_, tc := soloState(t)
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	v := NewActivityView(func() TeamResolution { return tc })
	v.read = func(context.Context, activityReader, string) (activityData, error) {
		close(started)
		<-release
		defer close(done)
		return activityData{cloned: true}, nil
	}
	v.SetShell(&mockShell{})
	loop := runApp(t, tview.NewFlex())
	onLoop(loop, func() { v.Mount(tview.NewFlex(), loop) })
	<-started
	onLoop(loop, v.Unmount)
	close(release)
	<-done
	time.Sleep(50 * time.Millisecond)
	assert.Empty(t, textOf(loop, &v.list))
}

func TestTeamStatusViewLoadAndRemount(t *testing.T) {
	frLocale(t)
	_, tc := soloState(t)
	var calls atomic.Int32
	first, release := make(chan struct{}), make(chan struct{})
	v := NewTeamStatusView(func() TeamResolution { return tc })
	v.read = func(context.Context, teamStatusReader) *teamStatusData {
		member := func(id string) []teamstate.Member { return []teamstate.Member{{ID: id}} }
		if calls.Add(1) == 1 {
			close(first)
			<-release
			return &teamStatusData{members: member("ancien")}
		}
		return &teamStatusData{members: member("nouveau")}
	}
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)

	onLoop(loop, func() { v.Mount(content, loop) })
	<-first
	assert.Contains(t, textOf(loop, &v.tv), "Chargement")

	onLoop(loop, v.Unmount)
	onLoop(loop, func() { v.Mount(tview.NewFlex(), loop) })
	require.Eventually(t, func() bool { return strings.Contains(textOf(loop, &v.tv), "@nouveau") }, 2*time.Second, 10*time.Millisecond)

	close(release)
	time.Sleep(50 * time.Millisecond)
	assert.NotContains(t, textOf(loop, &v.tv), "@ancien", "the answer of the previous mount is dropped")
}

// A clone that is missing says so instead of showing an empty status.
func TestTeamStatusViewWithoutClone(t *testing.T) {
	frLocale(t)
	tc := TeamResolution{Enabled: true, TeamID: "t", StatePath: t.TempDir() + "/absent", MemberID: "alice"}
	v := NewTeamStatusView(func() TeamResolution { return tc })
	content := tview.NewFlex()
	v.Mount(content, nil)
	assert.Contains(t, screenText(t, content, 100, 14), "non cloné")
}

func TestTeamEventAgeFallsBackToTheDateAfterAWeek(t *testing.T) {
	frLocale(t)
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	defer textfmt.SetNow(func() time.Time { return now })()
	assert.Equal(t, "", textfmt.AgoOrDate(time.Time{}))
	assert.Equal(t, "il y a 3 min", textfmt.AgoOrDate(now.Add(-3*time.Minute)))
	assert.Contains(t, textfmt.AgoOrDate(now.AddDate(0, 0, -10)), "2026")
}

func TestTakeoverViewLoadAndRemount(t *testing.T) {
	frLocale(t)
	_, tc := soloState(t)
	var calls atomic.Int32
	first, release := make(chan struct{}), make(chan struct{})
	v := NewTakeoverView(func() TeamResolution { return tc })
	v.read = func(context.Context, takeoverReader) (takeoverData, error) {
		brief := func(ticket string) []teamstate.TakeoverMeta {
			return []teamstate.TakeoverMeta{{TicketID: ticket, Project: "web", TransferredFrom: "a", TransferredTo: "b"}}
		}
		if calls.Add(1) == 1 {
			close(first)
			<-release
			return takeoverData{cloned: true, briefs: brief("ancien-1")}, nil
		}
		return takeoverData{cloned: true, briefs: brief("nouveau-2")}, nil
	}
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)
	listed := func() (n int) {
		onLoop(loop, func() {
			if v.list != nil && len(v.briefs) > 0 {
				n = len(v.briefs)
			}
		})
		return
	}

	onLoop(loop, func() { v.Mount(content, loop) })
	<-first
	assert.Zero(t, listed(), "the mount does not wait for the briefs")

	onLoop(loop, v.Unmount)
	onLoop(loop, func() { v.Mount(tview.NewFlex(), loop) })
	require.Eventually(t, func() bool { return listed() == 1 }, 2*time.Second, 10*time.Millisecond)

	close(release)
	time.Sleep(50 * time.Millisecond)
	onLoop(loop, func() {
		assert.Equal(t, "nouveau-2", v.briefs[0].TicketID, "the answer of the previous mount is dropped")
	})
}
