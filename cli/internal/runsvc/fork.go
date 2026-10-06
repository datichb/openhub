package runsvc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionresults"
)

// ErrForkUnsupported is returned when the tool cannot fork sessions.
var ErrForkUnsupported = errors.New("the session tool cannot fork sessions")

// ForkSession creates a new session with a copy of the history of another
// one (S9), on the same server group. The new oh session copies the origin
// settings and environment; the parent link is not stored yet (migration
// v34, track F) — the title says where it comes from.
func (s *Service) ForkSession(ctx context.Context, sessionID string) (string, error) {
	forker, ok := s.Adapter.(adapters.Forker)
	if !ok {
		return "", ErrForkUnsupported
	}
	parent, err := s.Session(ctx, sessionID)
	if err != nil {
		return "", err
	}
	srv, err := s.serverForSession(ctx, sessionID)
	if err != nil {
		return "", err
	}
	id, err := forker.Fork(ctx, handle(srv), sessionID)
	if err != nil {
		return "", fmt.Errorf("forking session: %w", err)
	}
	title := "fork · " + sessionID
	if parent.Title != nil && *parent.Title != "" {
		title = "fork · " + *parent.Title
	}
	ext := id
	child := &domain.Session{
		ID: id, ProjectID: parent.ProjectID, Status: domain.SessionStatusRunning, Provider: parent.Provider, Model: parent.Model,
		LaunchPath: parent.LaunchPath, MemberID: parent.MemberID, Platform: parent.Platform, ExternalSessionID: &ext,
		PID: srv.PID, Type: domain.SessionTypeInteractive, Title: &title,
		WorkflowID: parent.WorkflowID, EntryAgent: parent.EntryAgent, BundleHash: parent.BundleHash, GroupKey: parent.GroupKey,
		Runtime: parent.Runtime, Mode: parent.Mode, State: domain.RunIdle,
	}
	now := time.Now()
	child.StateChangedAt = &now
	if s.Sessions != nil {
		if err := s.Sessions.Create(ctx, child); err != nil {
			return id, fmt.Errorf("tracking the forked session: %w", err)
		}
	}
	static, err := s.loadStaticEnv(sessionID)
	if err == nil {
		err = s.saveStaticEnv(id, static)
	}
	if err == nil {
		var allow []string
		if allow, err = s.loadBeadsAllow(sessionID); err == nil {
			err = s.saveBeadsAllow(id, allow)
		}
	}
	if err == nil {
		err = s.reapplySessionEnv(ctx, srv, child)
	}
	if err != nil {
		slog.Warn("runsvc: environment of the forked session not applied", "session", id, "error", err)
	}
	return id, nil
}

// snapshotResults saves the results of a session while its server runs.
func (s *Service) snapshotResults(ctx context.Context, srv *domain.Server, sessionID string) {
	if s.SessionsDir == "" {
		return
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res, err := s.Adapter.Results(cctx, handle(srv), sessionID)
	if err != nil {
		slog.Debug("runsvc: results not saved", "session", sessionID, "error", err)
		return
	}
	_ = sessionresults.Save(s.SessionsDir, res, time.Now())
}
