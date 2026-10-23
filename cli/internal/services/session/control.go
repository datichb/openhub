package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Instructions and control (P3-T09): short instructions from oh (S6), and
// interrupt / model switch / compaction (S10). Fork creates a session: it is
// done by the RunService (runsvc.ForkSession).

// SendOptions tunes Send.
type SendOptions struct {
	// Synthetic sends a message from oh rather than a user prompt (S6).
	Synthetic bool
	// Delivery: steer (next step boundary, default of the tool) or queue
	// (after the running loop).
	Delivery adapters.Delivery
	// ReportEnd is set by the callers that speak for a user (TUI, CLI): the
	// tool only admits the message, so when the session turns out to be over
	// as the send returns, Send says it (EndedError.During) instead of
	// letting the user believe the message was taken.
	ReportEnd bool
}

// EndedError is returned for a session that is over: the tool server may
// still run, but nothing it is told will be taken into account.
type EndedError struct {
	SessionID string
	State     domain.RunState
	// Send: the operation was a message to the session.
	Send bool
	// During: the session was open when the message was sent and over right
	// after: the tool admitted the message, which may never be read.
	During bool
}

func (e *EndedError) Error() string {
	if e.During {
		return fmt.Sprintf("session %s ended while the message was sent (%s): it may not have been taken into account", e.SessionID, e.State)
	}
	return fmt.Sprintf("session %s is %s", e.SessionID, e.State)
}

// Is makes errors.Is(err, ErrSessionEnded) true.
func (e *EndedError) Is(target error) bool { return target == ErrSessionEnded }

// ErrSessionEnded matches an *EndedError.
var ErrSessionEnded = errors.New("the session is over")

// Send sends an instruction to a session.
func (s *Service) Send(ctx context.Context, sessionID, text string, o SendOptions) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("empty instruction")
	}
	kind := adapters.ControlPrompt
	if o.Synthetic {
		kind = adapters.ControlSynthetic
	}
	err := s.control(ctx, sessionID, adapters.ControlOp{Kind: kind, Text: text, Delivery: o.Delivery})
	var ended *EndedError
	if errors.As(err, &ended) {
		ended.Send = true
	}
	if err != nil || !o.ReportEnd || s.Sessions == nil {
		return err
	}
	if sess, gerr := s.Sessions.Get(ctx, sessionID); gerr == nil && isTerminal(sess.State) {
		return &EndedError{SessionID: sessionID, State: sess.State, Send: true, During: true}
	}
	return nil
}

// Interrupt stops the running agent loop of a session (the session stays open).
func (s *Service) Interrupt(ctx context.Context, sessionID string) error {
	return s.control(ctx, sessionID, adapters.ControlOp{Kind: adapters.ControlInterrupt})
}

// Compact compacts the history of a session.
func (s *Service) Compact(ctx context.Context, sessionID string) error {
	return s.control(ctx, sessionID, adapters.ControlOp{Kind: adapters.ControlCompact})
}

// SwitchModel changes the model of the next turns ("provider/model[#variant]").
func (s *Service) SwitchModel(ctx context.Context, sessionID, model string) error {
	ref := sessionspec.ParseModelRef(strings.TrimSpace(model))
	if ref.Provider == "" || ref.Model == "" {
		return fmt.Errorf("invalid model %q (provider/model expected)", model)
	}
	if err := s.control(ctx, sessionID, adapters.ControlOp{Kind: adapters.ControlSwitchModel, Model: &ref}); err != nil {
		return err
	}
	if s.Sessions != nil {
		if sess, err := s.Sessions.Get(ctx, sessionID); err == nil {
			sess.Model = ref.String()
			_ = s.Sessions.Update(ctx, sess)
		}
	}
	return nil
}

func (s *Service) control(ctx context.Context, sessionID string, op adapters.ControlOp) error {
	if s.Adapter == nil {
		return ErrNotSupported
	}
	h, err := s.handle(ctx, sessionID)
	if err != nil {
		return err
	}
	return s.Adapter.Control(ctx, h, sessionID, op)
}

// handle returns the running tool server of a session.
func (s *Service) handle(ctx context.Context, sessionID string) (adapters.ServerHandle, error) {
	if s.Sessions == nil || s.Servers == nil {
		return adapters.ServerHandle{}, errors.New("session: stores not configured")
	}
	sess, err := s.Sessions.Get(ctx, sessionID)
	if err != nil {
		return adapters.ServerHandle{}, fmt.Errorf("session %s: %w", sessionID, err)
	}
	if sess.GroupKey == "" {
		return adapters.ServerHandle{}, fmt.Errorf("session %s was not started by the v5 launcher", sessionID)
	}
	if isTerminal(sess.State) {
		return adapters.ServerHandle{}, &EndedError{SessionID: sessionID, State: sess.State}
	}
	srv, err := s.Servers.Get(ctx, sess.GroupKey)
	if err != nil || srv.Status != domain.ServerReady || (s.Alive != nil && !s.Alive(srv.PID)) {
		return adapters.ServerHandle{}, fmt.Errorf("%w (session %s)", ErrServerNotRunning, sessionID)
	}
	return adapters.ServerHandle{URL: srv.URL, Password: srv.Password, PID: srv.PID}, nil
}
