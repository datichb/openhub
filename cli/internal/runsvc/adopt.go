package runsvc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
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
	porter, ok := s.Adapter.(adapters.SessionPorter)
	if !ok {
		return ErrNoPorter
	}
	sess, err := s.Session(ctx, sessionID)
	if err != nil {
		return err
	}
	if req.Bundle == nil {
		b, err := bundle.Load(s.BundlesDir, sess.BundleHash)
		if err != nil {
			return fmt.Errorf("loading the session bundle: %w", err)
		}
		req.Bundle = b
	}
	if req.Location == "" {
		return errors.New("runsvc: the adopted session has no location")
	}
	req.ProjectID = sess.ProjectID
	dc, err := s.Daemon(ctx)
	if err != nil {
		return err
	}
	cred, region, err := s.resolveProvider(ctx, &req)
	if err != nil {
		return err
	}
	req.Runtime = sessionspec.RuntimeLocal
	key := sessionspec.GroupKey{BundleHash: req.Bundle.Spec.Hash, ProjectID: req.ProjectID, Runtime: sessionspec.RuntimeLocal,
		Config: configFingerprint(req, cred, region)}
	gk := key.String()
	unlock, err := s.lockGroup(ctx, gk)
	if err != nil {
		return err
	}
	srv, _, _, _, err := s.ensureServer(ctx, dc, req, key, gk, cred, region, nil, !s.groupBusy(ctx, req.ProjectID, gk))
	unlock()
	if err != nil {
		return err
	}
	h := handle(srv)
	for _, tr := range transcripts {
		if _, err := porter.ImportSession(ctx, h, tr, req.Location); err != nil && !errors.Is(err, adapters.ErrSessionExists) {
			return fmt.Errorf("importing the session: %w", err)
		}
	}
	now := time.Now()
	sess.Runtime, sess.GroupKey, sess.BundleHash = string(sessionspec.RuntimeLocal), gk, req.Bundle.Spec.Hash
	sess.LaunchPath, sess.PID, sess.Platform = req.Location, srv.PID, s.Adapter.Name()
	sess.State, sess.StateChangedAt = domain.RunIdle, &now
	if err := s.Sessions.Update(ctx, sess); err != nil {
		return err
	}
	if err := s.reapplySessionEnv(ctx, srv, sess); err != nil {
		return fmt.Errorf("applying the session environment: %w", err)
	}
	_ = dc.Touch(ctx, gk)
	return nil
}
