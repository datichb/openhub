// Package headlesstrack provides session tracking for headless (batch) LLM runs.
//
// Interactive sessions are tracked by the launcher package. Headless runs
// (brief enrichment, sweep decomposition, etc.) bypass the launcher and were
// previously invisible to the hub's session store, dashboard, and team-state
// events.
//
// This package provides a Track() function that wraps a headless run with
// the same create → execute → enrich → update → emit-event lifecycle used
// by the launcher, adapted for batch mode.
package headlesstrack

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/platform"
)

// Opts configures a tracked headless run.
type Opts struct {
	// Sessions is the session store for persistence. If nil, tracking is skipped.
	Sessions domain.SessionStore
	// PlatformName is the backend identifier (e.g. "opencode").
	PlatformName string
	// ProjectID is the hub project ID.
	ProjectID string
	// ProjectPath is the working directory for the run.
	ProjectPath string
	// Provider is the LLM provider name.
	Provider string
	// Label identifies the use-case (e.g. "brief-enrichment", "sweep-decomposition").
	Label string
	// CorrelationID groups related headless runs (e.g. all runs in a single sweep).
	// If empty, each run is standalone.
	CorrelationID string
	// MemberID is the team member running the session (nil = solo/unknown).
	MemberID string
}

// Track wraps a headless platform call with session tracking.
//
// It creates a session record before the call, enriches it with the result
// after, and persists the update. The headless result (and any error from
// the run) are returned transparently to the caller.
//
// If opts.Sessions is nil, the function simply calls fn() without tracking.
func Track(ctx context.Context, opts Opts, fn func(ctx context.Context) (*platform.HeadlessResult, error)) (*platform.HeadlessResult, error) {
	if opts.Sessions == nil {
		return fn(ctx)
	}

	// 1. Create session record
	session := &domain.Session{
		ID:         uuid.New().String(),
		ProjectID:  opts.ProjectID,
		Status:     domain.SessionStatusRunning,
		Provider:   opts.Provider,
		LaunchPath: opts.ProjectPath,
		Platform:   opts.PlatformName,
		Type:       domain.SessionTypeHeadless,
	}
	if opts.Label != "" {
		session.Label = &opts.Label
	}
	if opts.CorrelationID != "" {
		session.CorrelationID = &opts.CorrelationID
	}
	if opts.MemberID != "" {
		session.MemberID = &opts.MemberID
	}
	if err := opts.Sessions.Create(ctx, session); err != nil {
		slog.Warn("headlesstrack: failed to create session record", "error", err)
		// Continue without tracking — the headless run itself should not fail.
		return fn(ctx)
	}

	// 2. Execute the headless run
	result, runErr := fn(ctx)

	// 3. Enrich and update
	now := time.Now()
	session.EndedAt = &now
	if runErr != nil {
		session.Status = domain.SessionStatusFailed
	} else {
		session.Status = domain.SessionStatusCompleted
	}
	if result != nil {
		session.Model = result.Model
		session.Cost = result.Cost
		session.TokensIn = result.TokensIn
		session.TokensOut = result.TokensOut
		session.TokensReasoning = result.TokensReasoning
	}
	if err := opts.Sessions.Update(ctx, session); err != nil {
		slog.Warn("headlesstrack: failed to update session record", "error", err)
	}

	return result, runErr
}
