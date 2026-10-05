// Package session is the SessionService (03 §2.4): read model of the v5
// sessions and of their pending decisions (inbox), decisions, instructions
// and live follow-up. Shared by the CLI and the TUI; no UI code. Real-time
// tracking stays in the oh daemon, which feeds the stores read here.
package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
)

// Service is the SessionService.
type Service struct {
	Sessions  domain.SessionStore
	Decisions domain.DecisionStore
	Servers   domain.ServerStore
	Adapter   adapters.ToolAdapter
	// BundlesDir locates session bundles (strict isolation of a session).
	BundlesDir string
	// SessionsDir keeps per-session results snapshots (~/.oh/sessions).
	SessionsDir string
	// Resolvers answer decision kinds owned by other services (checkpoint).
	Resolvers map[domain.DecisionKind]Resolver
	// Live opens the oh daemon live stream (nil = polling only).
	Live LiveStream
	// PollEvery is the refresh period of Subscribe without the daemon (default 2s).
	PollEvery time.Duration
	// Alive reports whether a server PID is alive (nil = trust the store).
	Alive func(pid int) bool
	Now   func() time.Time
}

// InboxItem is an open decision with its session (nil when unknown).
type InboxItem struct {
	Decision domain.Decision
	Session  *domain.Session
}

// Inbox returns the open decisions matching f, oldest first (I3).
func (s *Service) Inbox(ctx context.Context, f domain.DecisionFilter) ([]InboxItem, error) {
	if s.Decisions == nil {
		return nil, errors.New("session: no decision store")
	}
	open, err := s.Decisions.ListOpen(ctx, f)
	if err != nil {
		return nil, err
	}
	cache := map[string]*domain.Session{}
	out := make([]InboxItem, 0, len(open))
	for _, d := range open {
		sess, ok := cache[d.SessionID]
		if !ok && s.Sessions != nil {
			if got, err := s.Sessions.Get(ctx, d.SessionID); err == nil {
				sess = got
			}
			cache[d.SessionID] = sess
		}
		if sess != nil && isTerminal(sess.State) {
			continue // closed by its stop; never shown
		}
		out = append(out, InboxItem{Decision: d, Session: sess})
	}
	return out, nil
}

// Raise records a decision raised by oh itself (checkpoint, budget…). The ID
// defaults to domain.DecisionID(kind, session, ToolRef); the group key is
// taken from the session. Raising an existing decision refreshes its payload.
func (s *Service) Raise(ctx context.Context, d *domain.Decision) error {
	if s.Decisions == nil {
		return errors.New("session: no decision store")
	}
	if d.SessionID == "" || d.Kind == "" {
		return errors.New("session: decision needs a session and a kind")
	}
	if d.ID == "" {
		d.ID = domain.DecisionID(d.Kind, d.SessionID, d.ToolRef)
	}
	if d.GroupKey == "" && s.Sessions != nil {
		sess, err := s.Sessions.Get(ctx, d.SessionID)
		if err != nil {
			return fmt.Errorf("session %s: %w", d.SessionID, err)
		}
		d.GroupKey = sess.GroupKey
	}
	return s.Decisions.Upsert(ctx, d)
}

func isTerminal(st domain.RunState) bool {
	return st == domain.RunStopped || st == domain.RunCompleted || st == domain.RunFailed
}
