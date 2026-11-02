package views

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
)

// The boards read their tickets (and Beads, the beads summary…) in load():
// the mount returns at once, the answer of a read made before an unmount is
// not applied to the next mount, and the view can be mounted again (run with
// -race).

func boardTicket(id string) BoardTicket {
	return BoardTicket{ID: id, Title: "Ticket " + id, Status: "planned", Priority: "high", Type: "task"}
}

func TestBoardViewChecksBeadsOffTheLoop(t *testing.T) {
	frLocale(t)
	var calls atomic.Int32
	first, release := make(chan struct{}), make(chan struct{})
	v := NewBoardView(BoardViewConfig{
		CheckInitialized: func() bool {
			if calls.Add(1) == 1 {
				close(first)
				<-release
				return true // the answer of the first mount
			}
			return false
		},
		OnInitBeads: func() {},
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := runApp(t, content)

	onLoop(app, func() { v.Mount(content, app) })
	<-first
	onLoop(app, func() {
		assert.Equal(t, 1, content.GetItemCount(), "a placeholder holds the place of the board")
		assert.Nil(t, v.columnCards, "no board before the answer")
	})

	onLoop(app, v.Unmount)
	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return !v.initialized })
	onLoop(app, func() { assert.Contains(t, v.StatusHints(), "i initialiser beads") })

	close(release)
	time.Sleep(50 * time.Millisecond)
	onLoop(app, func() {
		assert.False(t, v.initialized, "the answer of the previous mount is dropped")
		assert.Nil(t, v.columnCards)
	})
}

func TestBoardViewReloadsOffTheLoop(t *testing.T) {
	frLocale(t)
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	v := NewBoardView(BoardViewConfig{
		RefreshRate: time.Hour,
		RefreshFunc: func() []BoardTicket {
			if calls.Add(1) == 1 {
				close(started)
				<-release
				return []BoardTicket{boardTicket("ancien-1")}
			}
			return []BoardTicket{boardTicket("nouveau-2")}
		},
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := runApp(t, content)

	onLoop(app, func() { v.Mount(content, app) }) // the first read blocks: the mount does not
	<-started
	onLoop(app, func() { v.HandleKey(runeKey('r')) })
	waitFor(t, app, func() bool { _, ok := v.ticketsByID["nouveau-2"]; return ok })

	close(release)
	time.Sleep(50 * time.Millisecond)
	onLoop(app, func() {
		assert.Contains(t, v.ticketsByID, "nouveau-2")
		assert.NotContains(t, v.ticketsByID, "ancien-1", "a slow read does not overwrite a newer one")
		v.Unmount()
	})
}

func TestTeamBoardViewReloadsOffTheLoop(t *testing.T) {
	frLocale(t)
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	ticket := func(id string) []TeamTicket {
		return []TeamTicket{{ID: id, Title: "T " + id, Status: "in_progress", Assignee: "alice"}}
	}
	v := NewTeamBoardView(TeamBoardViewConfig{
		RefreshRate:  time.Hour,
		IsConfigured: func() bool { return true },
		RefreshFunc: func() []TeamTicket {
			if calls.Add(1) == 1 {
				close(started)
				<-release
				return ticket("ancien-1")
			}
			return ticket("nouveau-2")
		},
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := runApp(t, content)

	onLoop(app, func() { v.Mount(content, app) })
	<-started
	// A manual refresh while the first read is slow: it takes over.
	onLoop(app, func() { v.HandleKey(runeKey('r')) })
	waitFor(t, app, func() bool { return len(v.allTickets) == 1 && v.allTickets[0].ID == "nouveau-2" })
	waitFor(t, app, func() bool { return !v.actionInProgress.Load() })

	close(release)
	time.Sleep(50 * time.Millisecond)
	onLoop(app, func() {
		assert.Equal(t, "nouveau-2", v.allTickets[0].ID, "a slow read does not overwrite a newer one")
	})

	// Unmounted during a refresh: the flag that guards the actions is free for
	// the next mount.
	onLoop(app, func() { v.actionInProgress.Store(true); v.Unmount() })
	onLoop(app, func() { v.Mount(tview.NewFlex(), app) })
	onLoop(app, func() { assert.False(t, v.actionInProgress.Load()); v.Unmount() })
}

func TestTeamBoardViewBeadsSummaryOffTheLoop(t *testing.T) {
	frLocale(t)
	release := make(chan struct{})
	v := NewTeamBoardView(TeamBoardViewConfig{
		IsConfigured: func() bool { return true },
		Tickets:      []TeamTicket{{ID: "gitlab-1", Title: "Un", Status: "in_progress"}},
		BeadsSummaryFunc: func() map[string]BeadsSummary {
			<-release
			return map[string]BeadsSummary{"gitlab-1": {Done: 1, Total: 3}}
		},
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) }) // blocked summary: the mount does not wait
	onLoop(app, func() { assert.Nil(t, v.getBeadsSummary()) })
	close(release)
	waitFor(t, app, func() bool { return v.getBeadsSummary() != nil })
	onLoop(app, func() {
		card := v.buildTeamCard(v.allTickets[0])
		assert.Contains(t, card.MainText, "1/3", "the badge is drawn with the summary")
		v.Unmount()
	})
}
