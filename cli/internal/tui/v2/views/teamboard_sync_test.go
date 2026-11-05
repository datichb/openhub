package views

import (
	"strings"
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

func mountSyncBoard(t *testing.T, cfg TeamBoardViewConfig) (*TeamBoardView, *syncToastShell, *tview.Application) {
	t.Helper()
	frLocale(t)
	if cfg.RefreshRate == 0 {
		cfg.RefreshRate = time.Hour
	}
	cfg.IsConfigured = func() bool { return true }
	if cfg.RefreshFunc == nil {
		cfg.RefreshFunc = func() []TeamTicket { return syncBoardTickets("a-1") }
	}
	v := NewTeamBoardView(cfg)
	sh := &syncToastShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return v.loaded })
	return v, sh, app
}

// r synchronizes the tracker and says what it did; the pull and the new read
// follow.
func TestTeamBoardRefreshSaysWhatTheSyncDid(t *testing.T) {
	var pulls atomic.Int32
	v, sh, app := mountSyncBoard(t, TeamBoardViewConfig{
		PullRate:        -1,
		SyncFunc:        func() error { pulls.Add(1); return nil },
		TrackerSyncFunc: func() (string, error) { return "Synchro : 2 créées", nil },
	})
	before := pulls.Load()
	onLoop(app, func() { v.HandleKey(runeKey('r')) })
	waitFor(t, app, func() bool { return pulls.Load() > before && !v.actionInProgress.Load() })
	assert.Contains(t, sh.seen(), "Synchro : 2 créées")
	assert.NotContains(t, sh.seen(), "tui.teams.sync_pull_failed")
	onLoop(app, v.Unmount)
}

// A team without tracker: r is the pull alone, with no failure said.
func TestTeamBoardRefreshWithoutTrackerOnlyPulls(t *testing.T) {
	var pulls atomic.Int32
	v, sh, app := mountSyncBoard(t, TeamBoardViewConfig{
		PullRate:        -1,
		SyncFunc:        func() error { pulls.Add(1); return nil },
		TrackerSyncFunc: func() (string, error) { return "", ErrNoTracker },
	})
	before := pulls.Load()
	onLoop(app, func() { v.HandleKey(runeKey('r')) })
	waitFor(t, app, func() bool { return pulls.Load() > before && !v.actionInProgress.Load() })
	for _, toast := range sh.seen() {
		assert.NotContains(t, toast, "Erreur", "no failure toast for a team without tracker")
		assert.NotContains(t, toast, ErrNoTracker.Error())
	}
	onLoop(app, v.Unmount)
}

// A real failure of the sync is said, and the pull still happens.
func TestTeamBoardRefreshSaysTheFailureOfTheSync(t *testing.T) {
	var pulls atomic.Int32
	v, sh, app := mountSyncBoard(t, TeamBoardViewConfig{
		PullRate:        -1,
		SyncFunc:        func() error { pulls.Add(1); return nil },
		TrackerSyncFunc: func() (string, error) { return "", assert.AnError },
	})
	before := pulls.Load()
	onLoop(app, func() { v.HandleKey(runeKey('r')) })
	waitFor(t, app, func() bool { return pulls.Load() > before && !v.actionInProgress.Load() })
	found := false
	for _, toast := range sh.seen() {
		found = found || strings.Contains(toast, assert.AnError.Error())
	}
	assert.True(t, found, "toasts: %v", sh.seen())
	onLoop(app, v.Unmount)
}

// The board pulls by itself, quietly: the changes of the colleagues come to a
// board that stays open, a failing pull is not a toast.
func TestTeamBoardPullsPeriodicallyAndQuietly(t *testing.T) {
	var pulls atomic.Int32
	var reads atomic.Int32
	v, sh, app := mountSyncBoard(t, TeamBoardViewConfig{
		PullRate: 20 * time.Millisecond,
		SyncFunc: func() error {
			if pulls.Add(1) > 1 {
				return assert.AnError
			}
			return nil
		},
		RefreshFunc: func() []TeamTicket { reads.Add(1); return syncBoardTickets("a-1") },
	})
	waitFor(t, app, func() bool { return pulls.Load() >= 4 })
	assert.NotContains(t, sh.seen(), "tui.teams.sync_pull_failed")
	for _, toast := range sh.seen() {
		assert.NotContains(t, toast, assert.AnError.Error())
	}
	onLoop(app, v.Unmount)
	stopped := pulls.Load()
	time.Sleep(100 * time.Millisecond)
	assert.LessOrEqual(t, pulls.Load(), stopped+1, "the timer ends with the view")
}

func TestTeamBoardPullRateDefaults(t *testing.T) {
	assert.Equal(t, time.Minute, (&TeamBoardView{}).pullRate())
	assert.Zero(t, (&TeamBoardView{cfg: TeamBoardViewConfig{PullRate: -1}}).pullRate())
	assert.Equal(t, time.Second, (&TeamBoardView{cfg: TeamBoardViewConfig{PullRate: time.Second}}).pullRate())
}
