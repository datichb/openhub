package runsvc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/filelock"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// ErrNoPorter is returned when the tool adapter cannot import sessions.
var ErrNoPorter = errors.New("the tool adapter cannot import sessions")

// AdoptSession brings a session that ran elsewhere (remote run, phase 5)
// into the local server group of its bundle: the server is started (or
// joined) for req.Location, the transcripts are imported (parents first; a
// session already there is kept), then the session becomes a local,
// resumable one (state idle) with its environment applied.
func (s *Service) AdoptSession(ctx context.Context, sessionID string, transcripts [][]byte, req StartRequest) error {
	if req.Location == "" {
		return errors.New("runsvc: the adopted session has no location")
	}
	req.Runtime = sessionspec.RuntimeLocal
	_, err := s.adopt(ctx, sessionID, transcripts, req)
	return err
}

// adopt imports the transcripts of a session into the server group of
// req (runtime, bundle, provider) at req.Location, then records the
// session there (state idle) with its environment applied.
func (s *Service) adopt(ctx context.Context, sessionID string, transcripts [][]byte, req StartRequest) (*StartResult, error) {
	porter, ok := s.Adapter.(adapters.SessionPorter)
	if !ok {
		return nil, ErrNoPorter
	}
	sess, err := s.Session(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if req.Bundle == nil {
		b, err := bundle.Load(s.BundlesDir, sess.BundleHash)
		if err != nil {
			return nil, fmt.Errorf("loading the session bundle: %w", err)
		}
		req.Bundle = b
	}
	req.ProjectID = sess.ProjectID
	dc, err := s.Daemon(ctx)
	if err != nil {
		return nil, err
	}
	cred, region, err := s.resolveProvider(ctx, &req)
	if err != nil {
		return nil, err
	}
	kind := runtimeKind(req.Runtime)
	rt, err := s.runtime(kind)
	if err != nil {
		return nil, err
	}
	req.Runtime = kind
	key := sessionspec.GroupKey{BundleHash: req.Bundle.Spec.Hash, ProjectID: req.ProjectID, Runtime: kind,
		Config: configFingerprint(req, ToolProviderID(s.Adapter, req.Provider), cred, region)}
	j, err := s.joinGroup(ctx, dc, req, &key, cred, region, rt)
	if err != nil {
		return nil, err
	}
	gk, srv, pg := j.gk, j.srv, j.pg
	h := handle(srv)
	for _, tr := range transcripts {
		if _, err := porter.ImportSession(ctx, h, tr, innerPath(pg, req.Location)); err != nil && !errors.Is(err, adapters.ErrSessionExists) {
			return nil, fmt.Errorf("importing the session: %w", err)
		}
	}
	now := time.Now()
	sess.Runtime, sess.GroupKey, sess.BundleHash = string(kind), gk, req.Bundle.Spec.Hash
	sess.LaunchPath, sess.PID, sess.Platform = req.Location, srv.PID, s.Adapter.Name()
	sess.State, sess.StateChangedAt = domain.RunIdle, &now
	if err := s.Sessions.Update(ctx, sess); err != nil {
		return nil, err
	}
	if err := s.reapplySessionEnv(ctx, srv, sess); err != nil {
		return nil, fmt.Errorf("applying the session environment: %w", err)
	}
	// p6-debts: the imported sessions come without their session rules
	// (checkpoint locks); until the daemon sets them, the oh plugin refuses
	// what they lock.
	if s.RefreshRules != nil {
		if err := s.RefreshRules(ctx, sess.ID); err != nil {
			slog.Warn("runsvc: session rules of the imported session not applied", "session", sess.ID, "error", err)
		}
	}
	_ = dc.Touch(ctx, gk)
	return &StartResult{SessionID: sess.ID, GroupKey: gk, Server: srv}, nil
}

// TransferError is a transfer that failed: the session was left as it was
// (runtime, group, conversation). The caller may fall back to
// OnExistingKeep (new session continuing the work, with a brief).
type TransferError struct {
	SessionID string
	Err       error
}

func (e *TransferError) Error() string {
	return fmt.Sprintf("transferring session %s: %v", e.SessionID, e.Err)
}

func (e *TransferError) Unwrap() error { return e.Err }

// TransferSession moves a session, with its conversation, to the server
// group of req (runtime local or container, bundle, provider) and applies
// the model of req's entry agent. The source server must run (it is woken
// up); the conversation is exported from it and imported into the target
// group at the session location (as the target sees it), then the source
// server is stopped when no other open session uses it. On any failure
// before the switch the session stays where it was (*TransferError).
// req.Bundle may be another bundle than the session's (the caller warns:
// WarnTransferBundle).
func (s *Service) TransferSession(ctx context.Context, sessionID string, req StartRequest) (*StartResult, error) {
	fail := func(err error) (*StartResult, error) { return nil, &TransferError{SessionID: sessionID, Err: err} }
	porter, ok := s.Adapter.(adapters.SessionPorter)
	if !ok {
		return fail(ErrNoPorter)
	}
	sess, err := s.Session(ctx, sessionID)
	if err != nil {
		return fail(err)
	}
	switch {
	case sess.LaunchPath == "":
		return fail(errors.New("the session has no location"))
	case sess.Runtime == string(sessionspec.RuntimeRemote):
		return fail(errors.New("a remote session is fetched with oh session fetch"))
	case sess.State == domain.RunStopped || sess.State == domain.RunFailed:
		return fail(fmt.Errorf("the session is %s", sess.State))
	}
	src, err := s.serverForSession(ctx, sessionID)
	if errors.Is(err, ErrServerNotRunning) {
		if werr := s.wake(ctx, sess, req); werr != nil {
			return fail(fmt.Errorf("waking the session up: %w", werr))
		}
		src, err = s.serverForSession(ctx, sessionID)
	}
	if err != nil {
		return fail(err)
	}
	_ = s.Adapter.Control(ctx, handle(src), sessionID, adapters.ControlOp{Kind: adapters.ControlInterrupt})
	s.snapshotResults(ctx, src, sessionID)
	tr, err := porter.ExportSession(ctx, handle(src), sessionID)
	if err != nil {
		return fail(fmt.Errorf("exporting the session: %w", err))
	}
	from := *sess
	req.Location = sess.LaunchPath
	res, err := s.adopt(ctx, sessionID, [][]byte{tr}, req)
	if err != nil {
		// The row is written last by adopt: put it back as it was.
		if cur, gerr := s.Session(ctx, sessionID); gerr == nil && cur.GroupKey != from.GroupKey {
			_ = s.Sessions.Update(ctx, &from)
		}
		return fail(err)
	}
	if res.GroupKey != from.GroupKey {
		s.releaseServer(ctx, src, from)
	}
	if m := plannedModel(req); m != "" && m != sess.Model {
		ref := sessionspec.ParseModelRef(m)
		if err := s.Adapter.Control(ctx, handle(res.Server), sessionID, adapters.ControlOp{Kind: adapters.ControlSwitchModel, Model: &ref}); err != nil {
			slog.Warn("runsvc: model of the transferred session not switched", "session", sessionID, "error", err)
		} else if cur, err := s.Session(ctx, sessionID); err == nil {
			cur.Model = m
			_ = s.Sessions.Update(ctx, cur)
		}
	}
	s.setResumeContext(ctx, res.Server, sessionID)
	res.Existing = OnExistingTransfer
	return res, nil
}

// releaseServer stops the server a session left when no other open
// session of its group remains.
func (s *Service) releaseServer(ctx context.Context, srv *domain.Server, left domain.Session) {
	if list, err := s.Sessions.List(ctx, left.ProjectID); err == nil {
		for _, o := range list {
			if o.ID != left.ID && o.GroupKey == left.GroupKey && !terminal(o.State) {
				return
			}
		}
	}
	s.stopServer(ctx, srv)
	_ = s.Servers.SetStatus(ctx, left.GroupKey, domain.ServerStopped)
	if dc, err := s.Daemon(ctx); err == nil {
		revokeOwner(ctx, dc, left.GroupKey)
	}
}

// wake restarts the server group of a sleeping session (Service.WakeSession,
// else ResumeSession with the provider settings of req).
func (s *Service) wake(ctx context.Context, sess *domain.Session, req StartRequest) error {
	if s.WakeSession != nil {
		return s.WakeSession(ctx, sess.ID)
	}
	r := StartRequest{ProjectID: sess.ProjectID, TeamID: req.TeamID, ProjectTokenKey: req.ProjectTokenKey,
		Provider: req.Provider, ProviderCfg: req.ProviderCfg, IsolateUserConfig: req.IsolateUserConfig, EnvAllow: req.EnvAllow}
	if sess.Provider != "" {
		r.Provider = sess.Provider
	}
	return s.ResumeSession(ctx, sess.ID, r)
}

// continueExisting continues an existing session instead of starting a new
// one (OnExistingResume, OnExistingTransfer) and opens its client as the
// launch would.
func (s *Service) continueExisting(ctx context.Context, base StartRequest, e ExistingSession, action OnExisting) (*StartResult, error) {
	var res *StartResult
	switch action {
	case OnExistingTransfer:
		r, err := s.TransferSession(ctx, e.ID, base)
		if err != nil {
			return nil, err
		}
		res = r
	default:
		sess, err := s.Session(ctx, e.ID)
		if err != nil {
			return nil, err
		}
		if srv, err := s.Servers.Get(ctx, sess.GroupKey); err != nil || srv.Status != domain.ServerReady || !filelock.ProcessAlive(srv.PID) {
			if err := s.wake(ctx, sess, base); err != nil {
				return nil, fmt.Errorf("waking session %s up: %w", e.ID, err)
			}
		}
		res = &StartResult{SessionID: e.ID, GroupKey: sess.GroupKey, Existing: OnExistingResume}
		if srv, err := s.Servers.Get(ctx, sess.GroupKey); err == nil {
			res.Server = srv
		}
	}
	if base.Attach != sessionspec.AttachNone && base.Attach != sessionspec.AttachSuspend && base.Attach != sessionspec.AttachBrowser {
		res.AttachMethod, res.AttachErr = s.Attach(ctx, res.SessionID, e.LaunchPath, attachPref(base.Attach), base.ITermStyle, e.Title)
	}
	return res, nil
}
