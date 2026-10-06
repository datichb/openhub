package session

import (
	"context"
	"errors"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// AbsenceRecap summarizes what happened to the v5 sessions while oh was
// closed (I7, P3-T21).
type AbsenceRecap struct {
	Changed       int     // sessions whose state changed
	Waiting       int     // …now waiting for a decision
	Sleeping      int     // …now asleep
	Finished      int     // …now stopped, completed or failed
	Cost          float64 // cost of the changed sessions
	NewDecisions  int     // decisions raised since
	OpenDecisions int     // decisions still waiting now
}

// Empty reports whether nothing happened.
func (r AbsenceRecap) Empty() bool { return r.Changed == 0 && r.NewDecisions == 0 }

// Recap returns what changed since t.
func (s *Service) Recap(ctx context.Context, since time.Time) (AbsenceRecap, error) {
	var r AbsenceRecap
	if s.Sessions == nil {
		return r, errors.New("session: no session store")
	}
	all, err := s.Sessions.List(ctx, "")
	if err != nil {
		return r, err
	}
	for _, sess := range all {
		if sess.GroupKey == "" || sess.StateChangedAt == nil || !sess.StateChangedAt.After(since) {
			continue
		}
		r.Changed++
		r.Cost += sess.Cost
		switch {
		case sess.State == domain.RunWaiting:
			r.Waiting++
		case sess.State == domain.RunSleeping:
			r.Sleeping++
		case isTerminal(sess.State):
			r.Finished++
		}
	}
	if s.Decisions != nil {
		if list, err := s.Decisions.ListSince(ctx, since); err == nil {
			for _, d := range list {
				if d.CreatedAt.After(since) {
					r.NewDecisions++
				}
			}
		}
		if open, err := s.Decisions.ListOpen(ctx, domain.DecisionFilter{}); err == nil {
			r.OpenDecisions = len(open)
		}
	}
	return r, nil
}
