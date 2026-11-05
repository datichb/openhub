package teamstate

import (
	"context"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/safego"
)

// An event written to the activity of a team is also an announcement: the
// webhook of the team, the person it is for. teamstate does not know how to
// reach them (notify imports teamstate); the program installs a sink, and every
// event that a transaction committed is handed to it once, whoever wrote it
// (claims, reviews, wiki, configuration, the daemon…). A sink is called after
// the commit, outside the transaction, and never blocks the writer.

// EventSource is the team-state an event comes from, as a sink sees it: its
// configuration (the destinations), its members (names and handles) and where
// it lives.
type EventSource interface {
	RepoInfo
	ConfigReader
	MemberReader
}

// EventSink receives the events of a committed transaction.
type EventSink func(ctx context.Context, src EventSource, e Event)

var (
	sinkMu sync.RWMutex
	sink   EventSink
	// sinks counts the deliveries in flight: a short-lived command waits for
	// them before it exits (WaitEventSinks).
	sinks sync.WaitGroup
)

// SetEventSink installs the sink of the program (nil: none).
func SetEventSink(s EventSink) {
	sinkMu.Lock()
	sink = s
	sinkMu.Unlock()
}

// sinkTimeout bounds one delivery of the events of a transaction.
const sinkTimeout = 45 * time.Second

// WaitEventSinks waits, at most for d, for the deliveries in flight. It
// reports false when some are still running.
func WaitEventSinks(d time.Duration) bool {
	done := make(chan struct{})
	safego.Go(func() { sinks.Wait(); close(done) })
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

// queueEvent keeps an event written by the current transaction.
func (r *Repo) queueEvent(e Event) {
	r.evMu.Lock()
	r.evQueue = append(r.evQueue, e)
	r.evMu.Unlock()
}

// resetEvents drops the events of an attempt that did not commit.
func (r *Repo) resetEvents() {
	r.evMu.Lock()
	r.evQueue = nil
	r.evMu.Unlock()
}

// emitEvents hands the events of the committed transaction to the sink.
func (r *Repo) emitEvents() {
	r.evMu.Lock()
	events := r.evQueue
	r.evQueue = nil
	r.evMu.Unlock()
	sinkMu.RLock()
	s := sink
	sinkMu.RUnlock()
	if s == nil || len(events) == 0 {
		return
	}
	sinks.Add(1)
	safego.Go(func() {
		defer sinks.Done()
		ctx, cancel := context.WithTimeout(context.Background(), sinkTimeout)
		defer cancel()
		for _, e := range events {
			s(ctx, r, e)
		}
	})
}
