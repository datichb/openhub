package session

import (
	"context"
	"errors"
	"time"

	"github.com/datichb/openhub/cli/internal/daemon/api"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/safego"
)

// Live follow-up: the oh daemon watches the tool servers and
// publishes feed items and session changes; the CLI and the TUI read them
// here. The service never talks to the tool event stream itself.

// LiveStream opens the daemon stream (client.Client.Stream).
type LiveStream func(ctx context.Context, sessionID string) (<-chan api.StreamEvent, error)

// ErrNoLive means the live stream is unavailable (oh daemon not running).
var ErrNoLive = errors.New("live follow-up unavailable (oh daemon not running)")

// defaultReconnectEvery is the pause before Subscribe tries the daemon again.
const defaultReconnectEvery = time.Second

// Follow returns the live feed of a session (read-only), starting with its
// recent backlog. The channel closes when ctx ends or the daemon stops. The
// client resumes a dropped connection without losing or repeating an item; a
// resync event (items lost past the daemon window) has no feed item to show
// and is skipped, the backlog that follows brings back what is still known.
func (s *Service) Follow(ctx context.Context, sessionID string) (<-chan domain.FeedItem, error) {
	if s.Live == nil {
		return nil, ErrNoLive
	}
	evs, err := s.Live(ctx, sessionID)
	if err != nil {
		if errors.Is(err, api.ErrNotRunning) {
			return nil, ErrNoLive
		}
		return nil, err
	}
	out := make(chan domain.FeedItem, 64)
	safego.Go(func() {
		defer close(out)
		for ev := range evs {
			if ev.Feed == nil || ev.Feed.SessionID != sessionID {
				continue
			}
			it := *ev.Feed
			if it.Kind == domain.FeedUsage {
				it.Cost = s.totalCost(ctx, sessionID, it.Cost)
			}
			select {
			case out <- it:
			case <-ctx.Done():
				return
			}
		}
	})
	return out, nil
}

// totalCost is the cost of an oh session for the live feed: the tool
// reports its root session, the session row holds the total with the
// subagent sessions.
func (s *Service) totalCost(ctx context.Context, sessionID string, root float64) float64 {
	if s.Sessions == nil {
		return root
	}
	if sess, err := s.Sessions.Get(ctx, sessionID); err == nil && sess.Cost > root {
		return sess.Cost
	}
	return root
}

// Subscribe returns the changes of every session (badge, Sessions view,
// inbox) from the daemon stream. The stream numbers its entries and resumes
// by itself after a dropped connection (daemon/client), so nothing is
// polled: when the daemon is not running, the subscription waits and tries
// again. Whenever entries may have been missed (a resync from the daemon, a
// stream opened again after a gap), it emits a change with an empty SessionID,
// meaning "reload everything". The channel closes when ctx ends.
func (s *Service) Subscribe(ctx context.Context) <-chan domain.SessionChange {
	out := make(chan domain.SessionChange, 64)
	every := s.ReconnectEvery
	if every <= 0 {
		every = defaultReconnectEvery
	}
	safego.Go(func() {
		defer close(out)
		attempts := 0 // survives a restart after a panic: that is a gap too
		// A panic (the stream, the stores) restarts the loop after a pause,
		// instead of closing the channel on views that would wait for ever.
		safego.Supervise(ctx, "sessions subscription", func(ctx context.Context) {
			emit := func(c domain.SessionChange) bool {
				select {
				case out <- c:
					return true
				case <-ctx.Done():
					return false
				}
			}
			reload := func() bool { return emit(domain.SessionChange{Time: time.Now(), Decisions: true}) }
			for ctx.Err() == nil && s.Live != nil {
				attempts++
				if evs, err := s.Live(ctx, ""); err == nil {
					if attempts > 1 && !reload() {
						return
					}
					for ev := range evs {
						switch {
						case ev.Change != nil:
							if !emit(*ev.Change) {
								return
							}
						case ev.Resync:
							if !reload() {
								return
							}
						}
					}
				}
				select { // the stream ended or the daemon is not running: try again
				case <-ctx.Done():
					return
				case <-time.After(every):
				}
			}
			<-ctx.Done() // no live stream: nothing to follow
		})
	})
	return out
}
