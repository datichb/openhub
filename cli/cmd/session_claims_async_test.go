package cmd

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// endedSessionApp is an app with a team project and a claim held by ses_1.
func endedSessionApp(t *testing.T) (*app.App, *teamstate.Repo, *domain.Project, domain.Session) {
	t.Helper()
	a, repo, _ := setupClaimTestApp(t)
	proj := teamProject(claimsProject)
	a.Projects = &mockProjectStore{projects: []domain.Project{proj}}
	claimsCreate(t, repo, "pt-open", "testuser", "ses_1")
	prev := ticketFactsOf
	ticketFactsOf = func(string) ticketLookup { return lookup(map[string]ticketFacts{"pt-open": {}}) }
	t.Cleanup(func() { ticketFactsOf = prev })
	return a, repo, &proj, domain.Session{ID: "ses_1", ProjectID: claimsProject, Tickets: []string{"pt-open"}}
}

func swapClaimFlushes(t *testing.T, tr *taskTracker) {
	t.Helper()
	prev := claimFlushes
	claimFlushes = tr
	t.Cleanup(func() { claimFlushes = prev })
}

// v5.1 (B3b), risk (c) of B3a: in the daemon the end of a session used to
// queue its decisions from a goroutine; one that dies (or a daemon that stops)
// before it runs lost them. They are queued before sessionClaimsEnd returns.
func TestSessionClaimsEndQueuesBeforeTheBackgroundTask(t *testing.T) {
	a, repo, proj, s := endedSessionApp(t)
	swapClaimFlushes(t, &taskTracker{spawn: func(func()) {}}) // the task never starts

	sessionClaimsEnd(context.Background(), a, proj, s, true)

	ops, err := repo.ClaimQueue()
	require.NoError(t, err)
	require.Len(t, ops, 1, "queued although the background task never ran")
	assert.Equal(t, teamstate.ClaimOpRelease, ops[0].Kind)
	assert.Equal(t, "ses_1", ops[0].Session)

	// The next beat of the daemon applies it.
	b := beaterOf(a, time.Now())
	require.NoError(t, b.beat(context.Background()))
	claimsGone(t, repo, "pt-open")
}

// The background flush is tracked: it ends the claims, and the wait of the
// in-process daemon sees it.
func TestSessionClaimsEndFlushesInTheBackground(t *testing.T) {
	a, repo, proj, s := endedSessionApp(t)
	swapClaimFlushes(t, &taskTracker{spawn: func(fn func()) { go fn() }})

	sessionClaimsEnd(context.Background(), a, proj, s, true)
	require.True(t, claimFlushes.wait(30*time.Second))
	claimsGone(t, repo, "pt-open")
	ops, err := repo.ClaimQueue()
	require.NoError(t, err)
	assert.Empty(t, ops)
}

func TestTaskTrackerWait(t *testing.T) {
	var wg sync.WaitGroup
	tr := &taskTracker{spawn: func(fn func()) { wg.Add(1); go func() { defer wg.Done(); fn() }() }}
	assert.True(t, tr.wait(time.Millisecond), "nothing running")

	release := make(chan struct{})
	tr.run(func() { <-release })
	assert.False(t, tr.wait(20*time.Millisecond), "still running: the wait is bounded")
	go func() { time.Sleep(20 * time.Millisecond); close(release) }()
	assert.True(t, tr.wait(5*time.Second))

	wg.Wait()
}

// A panic (recovered and logged by safego.Go) does not leave the counter up.
func TestTaskTrackerSurvivesAPanic(t *testing.T) {
	tr := &taskTracker{spawn: safego.Go}
	tr.run(func() { panic("boom") })
	assert.True(t, tr.wait(5*time.Second))
}

// Windows: the daemon lives in the oh process; when oh quits, stopInProcessDaemon
// gives the flush of a session that has just ended the time to finish, within
// a bound.
func TestStopInProcessDaemonWaitsForTheClaimFlushes(t *testing.T) {
	var wg sync.WaitGroup
	tr := &taskTracker{spawn: func(fn func()) { wg.Add(1); go func() { defer wg.Done(); fn() }() }}
	swapClaimFlushes(t, tr)
	prev := claimFlushShutdownWait
	claimFlushShutdownWait = 5 * time.Second
	t.Cleanup(func() { claimFlushShutdownWait = prev })

	var finished atomic.Bool
	tr.run(func() { time.Sleep(150 * time.Millisecond); finished.Store(true) })

	done := make(chan error, 1)
	done <- nil
	inProcess.mu.Lock()
	inProcess.cancel, inProcess.done = func() {}, done
	inProcess.mu.Unlock()
	stopInProcessDaemon()
	assert.True(t, finished.Load(), "the flush had the time to finish")
	assert.False(t, inProcessDaemonRunning())

	// A flush that does not finish does not hold oh for longer than the bound.
	claimFlushShutdownWait = 100 * time.Millisecond
	stuck := make(chan struct{})
	tr.run(func() { <-stuck })
	start := time.Now()
	waitClaimFlushes()
	assert.Less(t, time.Since(start), 3*time.Second)
	close(stuck)
	wg.Wait()
}
