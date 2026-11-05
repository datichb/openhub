package views

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncToastShell records the toasts a view shows.
type syncToastShell struct {
	mockShell
	mu     sync.Mutex
	toasts []string
}

func (s *syncToastShell) ShowToastMsg(msg string, _ bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toasts = append(s.toasts, msg)
}

func (s *syncToastShell) seen() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.toasts...)
}

func syncBoardTickets(id string) []TeamTicket {
	return []TeamTicket{{ID: id, Title: "T " + id, Status: "in_progress", Assignee: "alice"}}
}

// The board synchronizes the tracker by itself when it opens, off the loop, and
// reads its tickets again when the sync did something; a sync that did nothing
// says nothing, a failure is told once.
func TestTeamBoardSynchronizesTheTrackerWhenItOpens(t *testing.T) {
	frLocale(t)
	var reads, syncs atomic.Int32
	sh := &syncToastShell{}
	v := NewTeamBoardView(TeamBoardViewConfig{
		RefreshRate:  time.Hour,
		IsConfigured: func() bool { return true },
		RefreshFunc: func() []TeamTicket {
			if reads.Add(1) == 1 {
				return nil
			}
			return syncBoardTickets("neuf-1")
		},
		SyncFunc: func() error { return nil },
		OpenSyncFunc: func() (string, error) {
			syncs.Add(1)
			return "Synchro : 1 créée", nil
		},
	})
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)

	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return len(v.allTickets) == 1 })
	assert.EqualValues(t, 1, syncs.Load(), "once per opening")
	assert.Contains(t, sh.seen(), "Synchro : 1 créée")
	onLoop(app, v.Unmount)
}

func TestTeamBoardOpenSyncSaysNothingWhenThereIsNothing(t *testing.T) {
	frLocale(t)
	var syncs atomic.Int32
	sh := &syncToastShell{}
	v := NewTeamBoardView(TeamBoardViewConfig{
		RefreshRate:  time.Hour,
		IsConfigured: func() bool { return true },
		RefreshFunc:  func() []TeamTicket { return syncBoardTickets("a-1") },
		SyncFunc:     func() error { return nil },
		OpenSyncFunc: func() (string, error) { syncs.Add(1); return "", nil },
	})
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)

	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return syncs.Load() == 1 })
	time.Sleep(50 * time.Millisecond)
	assert.Empty(t, sh.seen())
	onLoop(app, v.Unmount)
}

func TestTeamBoardOpenSyncFailureIsToldOnce(t *testing.T) {
	frLocale(t)
	sh := &syncToastShell{}
	var syncs atomic.Int32
	v := NewTeamBoardView(TeamBoardViewConfig{
		RefreshRate:  time.Hour,
		IsConfigured: func() bool { return true },
		RefreshFunc:  func() []TeamTicket { return syncBoardTickets("a-1") },
		SyncFunc:     func() error { return nil },
		OpenSyncFunc: func() (string, error) { syncs.Add(1); return "", assert.AnError },
	})
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)

	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return len(sh.seen()) == 1 })
	onLoop(app, func() { v.refreshIfIdle() })
	time.Sleep(50 * time.Millisecond)
	require.Len(t, sh.seen(), 1)
	assert.EqualValues(t, 1, syncs.Load())
	assert.Contains(t, sh.seen()[0], assert.AnError.Error())
	onLoop(app, v.Unmount)
}
