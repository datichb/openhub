package runsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
)

// Session environment (S7, P3-T12): variables of the shell commands run by
// one session, distinct from the server process environment shared by the
// group. Static variables (StartRequest.SessionEnv, never secrets) are kept
// in ~/.oh/sessions/<id>/env.json for resumes; dynamic ones (gateway tokens…)
// come from the Service.SessionEnv hook, called again at each resume and
// never persisted. The tool may hold the environment in memory only
// (opencode V2), so it is applied again whenever the server restarts.

// EnvSessionID is always set to the oh session ID.
const EnvSessionID = "OH_SESSION_ID"

// SessionEnvRequest describes the session whose environment is built.
type SessionEnvRequest struct {
	SessionID string
	GroupKey  string
	ProjectID string
	Location  string
	Resume    bool // false at creation, true when the server was restarted
}

// SessionEnvFunc returns dynamic per-session variables. They override the
// static ones; an error aborts the start (or the resume).
type SessionEnvFunc func(ctx context.Context, r SessionEnvRequest) (map[string]string, error)

func (s *Service) sessionDir(id string) string {
	if s.SessionsDir == "" {
		return ""
	}
	return filepath.Join(s.SessionsDir, id)
}

// saveStaticEnv persists the static variables of a session (owner-only).
func (s *Service) saveStaticEnv(id string, env map[string]string) error {
	dir := s.sessionDir(id)
	if dir == "" || len(env) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "env.json"), data, 0o600)
}

func (s *Service) loadStaticEnv(id string) (map[string]string, error) {
	dir := s.sessionDir(id)
	if dir == "" {
		return nil, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, "env.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var env map[string]string
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("reading session environment: %w", err)
	}
	return env, nil
}

func (s *Service) removeStaticEnv(id string) {
	if dir := s.sessionDir(id); dir != "" {
		_ = os.Remove(filepath.Join(dir, "env.json"))
	}
}

// buildSessionEnv merges the static variables, the hook variables and
// OH_SESSION_ID (always last: it cannot be overridden).
func (s *Service) buildSessionEnv(ctx context.Context, static map[string]string, r SessionEnvRequest) (map[string]string, error) {
	env := maps.Clone(static)
	if env == nil {
		env = map[string]string{}
	}
	if s.SessionEnv != nil {
		dyn, err := s.SessionEnv(ctx, r)
		if err != nil {
			return nil, fmt.Errorf("session environment: %w", err)
		}
		maps.Copy(env, dyn)
	}
	env[EnvSessionID] = r.SessionID
	return env, nil
}

// reapplyGroupEnv applies again the environment of every open session of a
// group whose server was just (re)started.
func (s *Service) reapplyGroupEnv(ctx context.Context, srv *domain.Server, except string) error {
	if s.Sessions == nil {
		return nil
	}
	list, err := s.Sessions.List(ctx, srv.ProjectID)
	if err != nil {
		return err
	}
	var errs []error
	for i := range list {
		sess := &list[i]
		if sess.ID == except || sess.GroupKey != srv.GroupKey || terminal(sess.State) {
			continue
		}
		if err := s.reapplySessionEnv(ctx, srv, sess); err != nil {
			errs = append(errs, fmt.Errorf("session %s: %w", sess.ID, err))
		}
	}
	return errors.Join(errs...)
}

// reapplySessionEnv applies the environment of an existing session again
// (after its server restarted).
func (s *Service) reapplySessionEnv(ctx context.Context, srv *domain.Server, sess *domain.Session) error {
	setter, ok := s.Adapter.(adapters.SessionEnvSetter)
	if !ok {
		return nil
	}
	static, err := s.loadStaticEnv(sess.ID)
	if err != nil {
		return err
	}
	env, err := s.buildSessionEnv(ctx, static, SessionEnvRequest{
		SessionID: sess.ID, GroupKey: sess.GroupKey, ProjectID: sess.ProjectID, Location: sess.LaunchPath, Resume: true,
	})
	if err != nil {
		return err
	}
	return setter.SetSessionEnv(ctx, handle(srv), sess.ID, env)
}
