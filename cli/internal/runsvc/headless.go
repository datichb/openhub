package runsvc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
)

// Headless runs (`oh run --headless`, brief-enrich, `v5Platform.RunHeadless`):
// a session without interactive client, awaited until its turn is over.

// HeadlessResult is the outcome of a headless session.
type HeadlessResult struct {
	SessionID string
	Text      string // what the assistant wrote
	Result    adapters.SessionResult
}

// ErrHeadlessUnsupported is returned when the tool adapter cannot wait for
// a turn.
var ErrHeadlessUnsupported = errors.New("the tool adapter cannot run sessions without interface")

// ErrDecisionPending is returned when a headless session waits for a
// decision (permission, question, checkpoint): nobody can answer it.
var ErrDecisionPending = errors.New("the session waits for a decision")

// decisionPoll is how often a headless wait checks for pending decisions.
var decisionPoll = 3 * time.Second

// AwaitTurn waits until the turn of a session is over and returns its text
// and results. A decision raised meanwhile ends the wait with
// ErrDecisionPending (the session is left as is: `oh session approve`).
func (s *Service) AwaitTurn(ctx context.Context, sessionID string) (*HeadlessResult, error) {
	w, ok := s.Adapter.(adapters.TurnWaiter)
	if !ok {
		return nil, ErrHeadlessUnsupported
	}
	// A queued session has not started its turn yet (restrictions).
	if err := s.waitDequeued(ctx, sessionID); err != nil {
		return nil, fmt.Errorf("waiting for a free session slot: %w", err)
	}
	srv, err := s.serverForSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	h := handle(srv)
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()
	pending := make(chan adapters.PendingDecision, 1)
	poll := decisionPoll
	go func() {
		t := time.NewTicker(poll)
		defer t.Stop()
		for {
			select {
			case <-wctx.Done():
				return
			case <-t.C:
				if list, err := s.Adapter.Pending(wctx, h, sessionID); err == nil && len(list) > 0 {
					pending <- list[0]
					cancel()
					return
				}
			}
		}
	}()
	err = w.WaitIdle(wctx, h, sessionID)
	select {
	case d := <-pending:
		return nil, fmt.Errorf("%w (%s)", ErrDecisionPending, d.Kind)
	default:
	}
	if err != nil {
		return nil, fmt.Errorf("waiting for the session turn: %w", err)
	}
	text, err := w.AssistantText(ctx, h, sessionID)
	if err != nil {
		return nil, err
	}
	out := &HeadlessResult{SessionID: sessionID, Text: text}
	if r, err := s.Adapter.Results(ctx, h, sessionID); err == nil {
		out.Result = r
	}
	return out, nil
}
