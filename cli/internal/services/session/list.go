package session

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// Read model of the v5 sessions (CLI list, TUI Sessions view and badge).

// View is a v5 session with its project and open decisions.
type View struct {
	Session     domain.Session
	ProjectName string
	Decisions   []domain.Decision // open, oldest first
}

// Open reports whether the session is not finished.
func (v View) Open() bool { return !isTerminal(v.Session.State) }

// ListFilter selects sessions.
type ListFilter struct {
	ProjectID string
	All       bool // include finished sessions
}

// List returns the v5 sessions, most recent first.
func (s *Service) List(ctx context.Context, f ListFilter) ([]View, error) {
	if s.Sessions == nil {
		return nil, errors.New("session: no session store")
	}
	all, err := s.Sessions.List(ctx, f.ProjectID)
	if err != nil {
		return nil, err
	}
	byID := map[string][]domain.Decision{}
	if s.Decisions != nil {
		if open, err := s.Decisions.ListOpen(ctx, domain.DecisionFilter{}); err == nil {
			for _, d := range open {
				byID[d.SessionID] = append(byID[d.SessionID], d)
			}
		}
	}
	names := map[string]string{}
	out := make([]View, 0, len(all))
	for _, sess := range all {
		if sess.GroupKey == "" {
			continue // legacy (pre-v5) session
		}
		if !f.All && isTerminal(sess.State) {
			continue
		}
		v := View{Session: sess, Decisions: byID[sess.ID]}
		if isTerminal(sess.State) {
			v.Decisions = nil
		}
		if s.Projects != nil && sess.ProjectID != "" {
			name, ok := names[sess.ProjectID]
			if !ok {
				if p, err := s.Projects.Get(ctx, sess.ProjectID); err == nil {
					name = p.Name
				}
				names[sess.ProjectID] = name
			}
			v.ProjectName = name
		}
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Session.StartedAt.After(out[j].Session.StartedAt) })
	return out, nil
}

// Counts summarizes open sessions for the badge: live sessions (● N: not
// asleep nor finished) and open decisions (⏸ M).
type Counts struct {
	Running   int
	Decisions int
}

// Count returns the badge counts (projectID "" = all projects).
func (s *Service) Count(ctx context.Context, projectID string) (Counts, error) {
	views, err := s.List(ctx, ListFilter{ProjectID: projectID})
	if err != nil {
		return Counts{}, err
	}
	var c Counts
	for _, v := range views {
		if v.Session.State != domain.RunSleeping {
			c.Running++
		}
		c.Decisions += len(v.Decisions)
	}
	return c, nil
}

// ErrAmbiguous is returned when a session reference matches several sessions.
var ErrAmbiguous = errors.New("ambiguous session reference")

// Resolve finds a v5 session by ID or unique ID prefix (with or without
// the "ses_" prefix).
func (s *Service) Resolve(ctx context.Context, ref string) (*domain.Session, error) {
	if s.Sessions == nil {
		return nil, errors.New("session: no session store")
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, errors.New("empty session reference")
	}
	if sess, err := s.Sessions.Get(ctx, ref); err == nil {
		return sess, nil
	}
	all, err := s.Sessions.List(ctx, "")
	if err != nil {
		return nil, err
	}
	var found []domain.Session
	for _, sess := range all {
		if sess.GroupKey == "" {
			continue
		}
		if strings.HasPrefix(sess.ID, ref) || strings.HasPrefix(sess.ID, "ses_"+ref) {
			found = append(found, sess)
		}
	}
	switch len(found) {
	case 0:
		return nil, fmt.Errorf("session %s: %w", ref, domain.ErrNotFound)
	case 1:
		return &found[0], nil
	}
	return nil, fmt.Errorf("%w: %s (%d sessions)", ErrAmbiguous, ref, len(found))
}

// ErrNoDecision is returned when a session has no open decision of the kind.
var ErrNoDecision = errors.New("no open decision of this kind")

// ErrSeveralDecisions is returned when a session has several open decisions
// of the kind: the caller must name one.
type ErrSeveralDecisions struct {
	Decisions []domain.Decision
}

func (e *ErrSeveralDecisions) Error() string {
	return fmt.Sprintf("%d open decisions: choose one", len(e.Decisions))
}

// Pick returns the open decision to answer: ref may be a decision ID, or a
// session reference whose only open decision of one of the kinds is taken.
func (s *Service) Pick(ctx context.Context, ref string, kinds ...domain.DecisionKind) (*domain.Decision, error) {
	if s.Decisions == nil {
		return nil, errors.New("session: no decision store")
	}
	if d, err := s.Decisions.Get(ctx, ref); err == nil {
		return d, nil
	}
	sess, err := s.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	open, err := s.Decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: sess.ID})
	if err != nil {
		return nil, err
	}
	var match []domain.Decision
	for _, d := range open {
		for _, k := range kinds {
			if d.Kind == k {
				match = append(match, d)
			}
		}
	}
	switch len(match) {
	case 0:
		if d := s.recentlyResolved(ctx, sess.ID, kinds); d != nil {
			return nil, &ResolvedError{By: d.ResolvedBy}
		}
		return nil, ErrNoDecision
	case 1:
		return &match[0], nil
	}
	return nil, &ErrSeveralDecisions{Decisions: match}
}

// recentResolution is how long a resolved decision explains that a session
// has none open ("already decided in the tool", A30).
const recentResolution = time.Hour

// recentlyResolved returns the last decision of the kinds of a session
// resolved in the last hour (nil when none).
func (s *Service) recentlyResolved(ctx context.Context, sessionID string, kinds []domain.DecisionKind) *domain.Decision {
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	list, err := s.Decisions.ListSince(ctx, now().Add(-recentResolution))
	if err != nil {
		return nil
	}
	var last *domain.Decision
	for i := range list {
		d := &list[i]
		if d.SessionID != sessionID || d.ResolvedAt == nil || !slices.Contains(kinds, d.Kind) {
			continue
		}
		if last == nil || d.ResolvedAt.After(*last.ResolvedAt) {
			last = d
		}
	}
	return last
}
