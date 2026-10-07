package runsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Session environment (S7, P3-T12): variables of the shell commands run by
// one session, distinct from the server process environment shared by the
// group. Static variables (StartRequest.SessionEnv, never secrets) are kept
// in ~/.oh/sessions/<id>/env.json for resumes; dynamic ones (gateway tokens…)
// come from the Service.SessionEnv hook, called again at each resume and
// never persisted. The tool may hold the environment in memory only
// (constat on opencode 2.0.20), so it is applied again whenever the server restarts.

// EnvSessionID is always set to the oh session ID.
const EnvSessionID = "OH_SESSION_ID"

// SessionEnvRequest describes the session whose environment is built.
type SessionEnvRequest struct {
	SessionID string
	GroupKey  string
	ProjectID string
	Location  string
	Resume    bool // false at creation, true when the server was restarted
	// Runtime of the server group ("" or local, container…).
	Runtime    sessionspec.RuntimeKind
	WorkflowID string
	// BeadsAllow is the workflow Beads allow-list (nil = not declared).
	BeadsAllow []string
	// GatewayURL is the base URL of the oh gateways seen from the runtime of
	// the group ("" in the local runtime).
	GatewayURL string
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
		_ = os.Remove(filepath.Join(dir, "beads.json"))
	}
}

// saveBeadsAllow keeps the Beads allow-list of a session for resumes
// (nothing when not declared; an empty list is kept as such).
func (s *Service) saveBeadsAllow(id string, allow []string) error {
	dir := s.sessionDir(id)
	if dir == "" || allow == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(allow)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "beads.json"), data, 0o600)
}

func (s *Service) loadBeadsAllow(id string) ([]string, error) {
	dir := s.sessionDir(id)
	if dir == "" {
		return nil, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, "beads.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var allow []string
	if err := json.Unmarshal(data, &allow); err != nil {
		return nil, fmt.Errorf("reading session Beads allow-list: %w", err)
	}
	if allow == nil {
		allow = []string{}
	}
	return allow, nil
}

// machineShellKeys are the machine variables a local session shell needs
// (tools on the PATH such as bd, git, language toolchains): opencode 2.0.20
// replaces the whole shell environment with the session environment, so
// without them `bd` and the user's tools are not found. Secrets never are.
var machineShellKeys = []string{"PATH", "HOME", "USER", "LOGNAME", "SHELL", "LANG", "LC_ALL", "LC_CTYPE",
	"TMPDIR", "TERM", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "GOPATH", "GOROOT", "VOLTA_HOME", "NVM_DIR", "BUN_INSTALL", "CARGO_HOME", "RUSTUP_HOME", "PYENV_ROOT"}

// withMachineShellEnv adds, for a local session, the machine variables of
// machineShellKeys to the static variables (which win), and puts the
// directories of first at the head of the PATH.
func withMachineShellEnv(kind sessionspec.RuntimeKind, static map[string]string, first []string) map[string]string {
	if kind != "" && kind != sessionspec.RuntimeLocal {
		return static
	}
	env := map[string]string{}
	for _, k := range machineShellKeys {
		if v, ok := os.LookupEnv(k); ok && v != "" {
			env[k] = v
		}
	}
	maps.Copy(env, static)
	if len(first) > 0 {
		path := strings.Join(first, string(os.PathListSeparator))
		if env["PATH"] != "" {
			path += string(os.PathListSeparator) + env["PATH"]
		}
		env["PATH"] = path
	}
	return env
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
	allow, err := s.loadBeadsAllow(sess.ID)
	if err != nil {
		return err
	}
	env, err := s.buildSessionEnv(ctx, static, SessionEnvRequest{
		SessionID: sess.ID, GroupKey: sess.GroupKey, ProjectID: sess.ProjectID, Location: sess.LaunchPath, Resume: true,
		Runtime: sessionspec.RuntimeKind(sess.Runtime), WorkflowID: sess.WorkflowID, BeadsAllow: allow,
		GatewayURL: s.loadGatewayURL(sess.GroupKey),
	})
	if err != nil {
		return err
	}
	return setter.SetSessionEnv(ctx, handle(srv), sess.ID, env)
}

// The session shell does not inherit the server process environment once a
// session environment is set (opencode 2.0.20): the gateway URL of the group
// is kept next to the group data and handed to the SessionEnv hook.
const gatewayURLFile = "gateway_url"

// saveGatewayURL records the gateway URL of a group (next to the proxy, as
// seen by its server: from inside the runtime, or on the machine).
func (s *Service) saveGatewayURL(gk, proxyURL string) error {
	if s.ServersDir == "" {
		return nil
	}
	p := filepath.Join(s.ServersDir, gk, gatewayURLFile)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(gatewayURL(proxyURL)), 0o600)
}

// saveProxyURL records the proxy URL a group's server is started with (as
// seen by the server): the daemon puts the group to sleep when it no longer
// listens on that port (daemon.ProxyURLPath).
func (s *Service) saveProxyURL(gk, proxyURL string) error {
	if s.ServersDir == "" {
		return nil
	}
	p := daemon.ProxyURLPath(s.ServersDir, gk)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(proxyURL), 0o600)
}

func (s *Service) loadGatewayURL(gk string) string {
	if s.ServersDir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(s.ServersDir, gk, gatewayURLFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// gatewayURL is the base URL of the oh gateways next to a proxy provider
// URL (same listener).
func gatewayURL(providerURL string) string {
	u, err := url.Parse(providerURL)
	if err != nil {
		return ""
	}
	u.Path, u.RawPath, u.RawQuery = "/"+beadswire.Prefix, "", ""
	return u.String()
}
