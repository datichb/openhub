package session

import (
	"context"
	"errors"
	"time"

	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
)

// Live follow-up (P3-T10): the oh daemon watches the tool servers and
// publishes feed items and session changes; the CLI and the TUI read them
// here. The service never talks to the tool event stream itself.

// LiveStream opens the daemon stream (daemon.Client.Stream).
type LiveStream func(ctx context.Context, sessionID string) (<-chan daemon.StreamEvent, error)

// ErrNoLive means the live stream is unavailable (oh daemon not running).
var ErrNoLive = errors.New("live follow-up unavailable (oh daemon not running)")

// defaultPollEvery is the refresh period of Subscribe without the daemon.
const defaultPollEvery = 2 * time.Second

// Follow returns the live feed of a session (read-only), starting with its
// recent backlog. The channel closes when ctx ends or the daemon stops.
func (s *Service) Follow(ctx context.Context, sessionID string) (<-chan domain.FeedItem, error) {
	if s.Live == nil {
		return nil, ErrNoLive
	}
	evs, err := s.Live(ctx, sessionID)
	if err != nil {
		if errors.Is(err, daemon.ErrNotRunning) {
			return nil, ErrNoLive
		}
		return nil, err
	}
	out := make(chan domain.FeedItem, 64)
	go func() {
		defer close(out)
		for ev := range evs {
			if ev.Feed == nil || ev.Feed.SessionID != sessionID {
				continue
			}
			select {
			case out <- *ev.Feed:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// Subscribe returns the changes of every session (badge, Sessions view,
// inbox). It uses the daemon stream and falls back to polling the stores
// while the daemon is not running. The channel closes when ctx ends.
func (s *Service) Subscribe(ctx context.Context) <-chan domain.SessionChange {
	out := make(chan domain.SessionChange, 64)
	every := s.PollEvery
	if every <= 0 {
		every = defaultPollEvery
	}
	go func() {
		defer close(out)
		emit := func(c domain.SessionChange) bool {
			select {
			case out <- c:
				return true
			case <-ctx.Done():
				return false
			}
		}
		var last map[string]pollState
		for ctx.Err() == nil {
			if s.Live != nil {
				if evs, err := s.Live(ctx, ""); err == nil {
					last = nil
					for ev := range evs {
						if ev.Change != nil && !emit(*ev.Change) {
							return
						}
					}
					select { // stream ended: reconnect (or poll)
					case <-ctx.Done():
						return
					case <-time.After(time.Second):
					}
					continue
				}
			}
			// Daemon unavailable: poll a few times, then try the stream again.
			for i := 0; i < 3 && ctx.Err() == nil; i++ {
				cur := s.pollState(ctx)
				if last != nil {
					for _, c := range diffPoll(last, cur) {
						if !emit(c) {
							return
						}
					}
				}
				last = cur
				select {
				case <-ctx.Done():
					return
				case <-time.After(every):
				}
			}
		}
	}()
	return out
}

type pollState struct {
	state     domain.RunState
	cost      float64
	decisions int
}

func (s *Service) pollState(ctx context.Context) map[string]pollState {
	out := map[string]pollState{}
	if s.Sessions == nil {
		return out
	}
	list, err := s.Sessions.List(ctx, "")
	if err != nil {
		return out
	}
	for _, sess := range list {
		if sess.GroupKey != "" {
			out[sess.ID] = pollState{state: sess.State, cost: sess.Cost}
		}
	}
	if s.Decisions != nil {
		if open, err := s.Decisions.ListOpen(ctx, domain.DecisionFilter{}); err == nil {
			for _, d := range open {
				p := out[d.SessionID]
				p.decisions++
				out[d.SessionID] = p
			}
		}
	}
	return out
}

func diffPoll(prev, cur map[string]pollState) []domain.SessionChange {
	var out []domain.SessionChange
	now := time.Now()
	for id, c := range cur {
		p, ok := prev[id]
		if !ok || p != c {
			out = append(out, domain.SessionChange{Time: now, SessionID: id, State: c.state, Decisions: !ok || p.decisions != c.decisions})
		}
	}
	for id := range prev {
		if _, ok := cur[id]; !ok {
			out = append(out, domain.SessionChange{Time: now, SessionID: id, Decisions: true})
		}
	}
	return out
}
