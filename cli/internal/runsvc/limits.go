package runsvc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/limits"
)

// ErrDailyBudget is returned when the daily budget of a new session's
// restrictions is already spent.
var ErrDailyBudget = errors.New("the daily budget is spent")

// DailyBudgetError details ErrDailyBudget.
type DailyBudgetError struct {
	Spent, Allowance float64
	Scope            string
}

func (e *DailyBudgetError) Error() string {
	return fmt.Sprintf("%s ($%.2f of $%.2f, %s)", ErrDailyBudget, e.Spent, e.Allowance, e.Scope)
}

// Is makes errors.Is(err, ErrDailyBudget) true.
func (e *DailyBudgetError) Is(target error) bool { return target == ErrDailyBudget }

// checkDailyBudget refuses a new session when its daily budget is spent
// (the running sessions get a budget decision from the daemon).
func (s *Service) checkDailyBudget(ctx context.Context, l limits.Resolved) error {
	if l.DailyBudgetUSD <= 0 || s.Usage == nil {
		return nil
	}
	day, scope := domain.UsageDay(time.Now()), l.DailyScope()
	spent, err := s.Usage.DayCost(ctx, day, limits.ScopeProject(scope))
	if err != nil {
		slog.Warn("runsvc: daily budget not checked", "error", err)
		return nil //nolint:nilerr // soft cap: an unreadable ledger never blocks a launch
	}
	extra, _ := s.Usage.BudgetExtra(ctx, scope, day)
	if allowance := l.DailyBudgetUSD + extra; spent >= allowance {
		return &DailyBudgetError{Spent: spent, Allowance: allowance, Scope: scope}
	}
	return nil
}

// applyLimits sets the proxy model allow-list from the restrictions (an
// explicit AllowedModels wins).
func applyLimits(req *StartRequest) {
	if len(req.AllowedModels) == 0 && len(req.Limits.Models) > 0 {
		req.AllowedModels = append([]string(nil), req.Limits.Models...)
	}
}

// queueSlot decides whether the first prompt of a new session must wait
// (I6): too many sessions working in the scope of the maximum, sessions
// already queued there (first come, first served), or the tool servers
// above the memory cap. It returns the queue entry (nil = start now) and
// the number of sessions queued before it.
func (s *Service) queueSlot(ctx context.Context, dc DaemonClient, req StartRequest) (queued *limits.Queued, ahead int) {
	l := req.Limits
	if (l.MaxActiveSessions == 0 && l.MemoryMB == 0) || s.Sessions == nil || s.SessionsDir == "" {
		return nil, 0
	}
	q := limits.Queued{Prompt: req.Prompt, Priority: limits.PriorityInteractive, QueuedAt: time.Now(),
		Scope: l.ActiveScope(), MaxActive: l.MaxActiveSessions, MemoryMB: l.MemoryMB}
	if req.Headless {
		q.Priority = limits.PriorityHeadless
	}
	list, err := s.Sessions.List(ctx, limits.ScopeProject(q.Scope))
	if err != nil {
		slog.Warn("runsvc: restrictions not checked (sessions unreadable)", "error", err)
		return nil, 0
	}
	working := 0
	for _, o := range list {
		switch o.State {
		case domain.RunActive:
			working++
		case domain.RunQueued:
			if p, ok, _ := limits.LoadQueued(s.SessionsDir, o.ID); ok && !q.Less(p) {
				ahead++
			}
		}
	}
	full := l.MaxActiveSessions > 0 && working >= l.MaxActiveSessions
	if !full && l.MemoryMB > 0 {
		if h, err := dc.Health(ctx); err == nil && h.MemoryMB >= l.MemoryMB {
			full = true
		}
	}
	if !full && ahead == 0 {
		return nil, 0
	}
	return &q, ahead
}

// waitDequeued waits until a queued session got its first prompt (the
// daemon sends it when a slot frees up).
func (s *Service) waitDequeued(ctx context.Context, sessionID string) error {
	if s.Sessions == nil {
		return nil
	}
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		sess, err := s.Sessions.Get(ctx, sessionID)
		if err != nil || sess.State != domain.RunQueued {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}
