package remote

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/remote/gitlab"
)

// ErrUnknownTarget is returned when a remote session names a target that is
// no longer configured.
var ErrUnknownTarget = errors.New("the remote target of this session is not configured any more")

// TrackResult is the state of a remote session after a check.
type TrackResult struct {
	SessionID string
	Ref       domain.RemoteRef
	Changed   bool
}

// pipelineStatus maps a GitLab pipeline status to the remote status.
func pipelineStatus(p string) domain.RemoteStatus {
	switch p {
	case "running":
		return domain.RemoteRunning
	case "success":
		return domain.RemoteReady
	case "failed", "canceled", "skipped":
		return domain.RemoteFailed
	}
	return domain.RemoteSent // created, pending, preparing, waiting_for_resource, scheduled, manual
}

// Pending reports whether the pipeline of a reference may still change.
func Pending(r domain.RemoteRef) bool {
	return r.Status == domain.RemoteSent || r.Status == domain.RemoteRunning
}

// ToFetch reports whether a remote session waits to be fetched (« À récupérer »).
func ToFetch(r domain.RemoteRef) bool {
	return r.Status == domain.RemoteReady || r.Status == domain.RemoteFailed
}

// ToResolve reports whether a fetched session still has a Beads journal to replay.
func ToResolve(r domain.RemoteRef) bool { return r.Status == domain.RemoteFetched }

func (s *Service) clientFor(ctx context.Context, name string) (*gitlab.Client, *config.RemoteTarget, error) {
	if s.Target == nil {
		return nil, nil, ErrUnknownTarget
	}
	t, ok := s.Target(name)
	if !ok || t == nil {
		return nil, nil, fmt.Errorf("%w: %s", ErrUnknownTarget, name)
	}
	token := ""
	if s.Secrets != nil {
		token, _ = s.Secrets.Get(ctx, t.TokenKeyOrDefault())
	}
	if token == "" {
		return nil, nil, ErrNoToken
	}
	return s.client(*t, token), t, nil
}

// Track checks the pipelines of the remote sessions still running (P5-T15):
// the reference and the session state follow the pipeline, and Notify is
// called when a session becomes ready to fetch (or failed).
func (s *Service) Track(ctx context.Context) ([]TrackResult, error) {
	if s.Remote == nil {
		return nil, errors.New("remote: no remote store")
	}
	refs, err := s.Remote.ListRemote(ctx)
	if err != nil {
		return nil, err
	}
	var out []TrackResult
	var errs []error
	for sid, ref := range refs {
		if !Pending(ref) {
			continue
		}
		res, err := s.trackOne(ctx, sid, ref)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", sid, err))
			continue
		}
		out = append(out, *res)
	}
	return out, errors.Join(errs...)
}

// TrackSession checks one remote session (whatever its status).
func (s *Service) TrackSession(ctx context.Context, sid string) (*TrackResult, error) {
	ref, err := s.Remote.GetRemoteRef(ctx, sid)
	if err != nil {
		return nil, err
	}
	if ref == nil {
		return nil, ErrNotRemote
	}
	if !Pending(*ref) {
		return &TrackResult{SessionID: sid, Ref: *ref}, nil
	}
	return s.trackOne(ctx, sid, *ref)
}

// ErrNotRemote is returned for a session that did not run remotely.
var ErrNotRemote = errors.New("this session did not run remotely")

func (s *Service) trackOne(ctx context.Context, sid string, ref domain.RemoteRef) (*TrackResult, error) {
	c, _, err := s.clientFor(ctx, ref.Target)
	if err != nil {
		return nil, err
	}
	pl, err := c.Pipeline(ctx, fmt.Sprint(ref.RunnerID), ref.Pipeline)
	if err != nil {
		return nil, err
	}
	st := pipelineStatus(pl.Status)
	res := &TrackResult{SessionID: sid, Ref: ref}
	if st == ref.Status {
		return res, nil
	}
	now := s.now().UTC()
	ref.Status, ref.UpdatedAt = st, now
	if st == domain.RemoteFailed {
		ref.Error = "pipeline " + pl.Status
	}
	if err := s.Remote.SetRemoteRef(ctx, sid, ref); err != nil {
		return nil, err
	}
	res.Ref, res.Changed = ref, true
	if sess, err := s.Sessions.Get(ctx, sid); err == nil {
		switch st {
		case domain.RemoteReady:
			sess.State = domain.RunCompleted
		case domain.RemoteFailed:
			sess.State = domain.RunFailed
		default:
			sess.State = domain.RunActive
		}
		t := time.Time(now)
		sess.StateChangedAt = &t
		if err := s.Sessions.Update(ctx, sess); err != nil {
			slog.Warn("remote: session state not updated", "session", sid, "err", err)
		}
		if ToFetch(ref) && s.Notify != nil {
			title := sid
			if sess.Title != nil {
				title = *sess.Title
			}
			s.Notify(title, string(st))
		}
	}
	return res, nil
}
