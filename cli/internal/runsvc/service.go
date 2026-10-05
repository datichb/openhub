// Package runsvc is the v5 session launcher (RunService): it groups sessions
// on shared tool servers, wires the credential proxy, enforces the closed
// world, creates sessions through the tool API and opens interactive clients.
//
// It is shared by the CLI and the TUI; it contains no UI code.
package runsvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/filelock"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/termlaunch"
)

// DaemonClient is the subset of the ohd client used by the service.
type DaemonClient interface {
	IssueGrant(ctx context.Context, req daemon.GrantRequest) (daemon.GrantResponse, error)
	RevokeOwner(ctx context.Context, owner string) error
	Usage(ctx context.Context, token string) (daemon.UsageResponse, error)
	Touch(ctx context.Context, group string) error
}

// nativeRefresher is implemented by adapters able to re-discover native agents.
type nativeRefresher interface {
	RefreshNatives(ctx context.Context) []string
}

// Service launches and manages v5 sessions.
type Service struct {
	Adapter    adapters.ToolAdapter
	AdapterVer string
	Servers    domain.ServerStore
	Sessions   domain.SessionStore // optional
	Secrets    provider.SecretStore
	Daemon     func(ctx context.Context) (DaemonClient, error)
	ServersDir string // ~/.oh/servers
	BundlesDir string // ~/.oh/bundles (resume loads the session bundle by hash)
	// OnSessionEnd is called when a session is stopped for good (team
	// session.complete event).
	OnSessionEnd func(ctx context.Context, s domain.Session)
	Executable   string // oh binary used to attach (default os.Executable)
}

// StartRequest describes a session to start.
type StartRequest struct {
	ProjectID       string
	TeamID          string
	ProjectTokenKey string // explicit project credential key (ProjectProviderConfig.TokenKey)
	Location        string // session working directory (project base or worktree)
	Bundle          *bundle.Bundle
	EntryAgent      string // default: bundle entry agent
	Title           string
	Prompt          string
	WorkflowID      string
	Mode            string
	MemberID        *string

	Provider      string          // hub provider name: bedrock | anthropic | openrouter
	ProviderCfg   provider.Config // AWS profile / region
	AllowedModels []string
	MaxTokens     int64

	Attach     sessionspec.AttachPref
	ITermStyle termlaunch.ITermStyle
}

// StartResult is the outcome of StartSession.
type StartResult struct {
	SessionID    string
	GroupKey     string
	Server       *domain.Server
	Reused       bool
	Report       adapters.VisibilityReport
	AttachMethod termlaunch.Method
	AttachErr    error // non-nil when no terminal could be opened (caller: browser/suspend)
}

// ErrServerNotRunning is returned when the server of a session is asleep or stopped.
var ErrServerNotRunning = errors.New("the tool server of this session is not running")

// ErrIsolation is returned when the closed world cannot be guaranteed.
var ErrIsolation = errors.New("session isolation check failed")

// StartSession starts (or joins) the server group of the bundle and creates a session.
func (s *Service) StartSession(ctx context.Context, req StartRequest) (*StartResult, error) {
	if req.Bundle == nil || req.Location == "" {
		return nil, errors.New("runsvc: bundle and location are required")
	}
	spec := req.Bundle.Spec
	entry := req.EntryAgent
	if entry == "" {
		entry = spec.EntryAgent
	}
	if !spec.HasAgent(entry) {
		return nil, fmt.Errorf("runsvc: agent %q is not in the bundle", entry)
	}
	dc, err := s.Daemon(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting oh daemon: %w", err)
	}

	cred, region, err := s.resolveProvider(ctx, &req)
	if err != nil {
		return nil, err
	}
	key := sessionspec.GroupKey{BundleHash: spec.Hash, ProjectID: req.ProjectID, Runtime: sessionspec.RuntimeLocal,
		Config: configFingerprint(req, cred, region)}
	gk := key.String()
	unlock, err := filelock.Lock(filepath.Join(s.ServersDir, gk, "lock"))
	if err != nil {
		return nil, err
	}
	srv, reused, report, err := s.ensureServer(ctx, dc, req, key, gk, cred, region)
	unlock()
	if err != nil {
		return nil, err
	}

	res := &StartResult{GroupKey: gk, Server: srv, Reused: reused, Report: report}
	h := handle(srv)
	sid := sessionspec.NewSessionID()
	ss := sessionspec.SessionSpec{
		SessionID: sid, Title: req.Title, Group: key, ProjectID: req.ProjectID,
		Location: req.Location, EntryAgent: entry, Mode: req.Mode, Prompt: req.Prompt,
		Runtime: sessionspec.RuntimeLocal, Attach: req.Attach,
		Model: entryModel(spec, entry), Provider: sessionspec.ProviderSpec{Region: region},
	}
	// The oh row is written before the tool session exists, so that the
	// daemon tracks the session from its very first event.
	s.persistSession(ctx, req, srv, sid, entry)
	if err := s.Adapter.CreateSession(ctx, h, ss); err != nil {
		s.markFailed(ctx, sid)
		return nil, fmt.Errorf("creating session: %w", err)
	}
	res.SessionID = sid

	if req.Prompt != "" {
		if err := s.Adapter.SendPrompt(ctx, h, sid, req.Prompt); err != nil {
			return res, fmt.Errorf("sending initial prompt: %w", err)
		}
	}
	_ = dc.Touch(ctx, gk)

	if req.Attach != sessionspec.AttachNone && req.Attach != sessionspec.AttachSuspend && req.Attach != sessionspec.AttachBrowser {
		res.AttachMethod, res.AttachErr = s.Attach(ctx, sid, req.Location, attachPref(req.Attach), req.ITermStyle, req.Title)
	}
	return res, nil
}

// entryModel is the session model: the entry agent model, else the bundle default.
func entryModel(spec sessionspec.BundleSpec, entry string) *sessionspec.ModelRef {
	for _, a := range spec.Agents {
		if a.ID == entry && a.Model != nil {
			return a.Model
		}
	}
	return spec.DefaultModel
}

// resolveProvider resolves the credential source and the region of a
// request. The Bedrock region follows the oh config, then the AWS SDK chain
// (AWS_REGION, AWS_DEFAULT_REGION, profile), then us-east-1 with a warning.
func (s *Service) resolveProvider(ctx context.Context, req *StartRequest) (provider.ResolvedCredential, string, error) {
	cred, err := provider.ResolveCredentialSource(ctx, s.Secrets, provider.Name(req.Provider), req.ProjectID, req.TeamID, req.ProjectTokenKey, &req.ProviderCfg)
	if err != nil {
		return cred, "", err
	}
	region := req.ProviderCfg.AWSRegion
	if region == "" && deploy.OpencodeProviderID(req.Provider) == "amazon-bedrock" {
		region = credproxy.AWSRegion(ctx, req.ProviderCfg.AWSProfile)
		if region == "" {
			region = "us-east-1"
			slog.Warn("runsvc: no AWS region configured (oh config, AWS_REGION, profile); using us-east-1")
		}
	}
	return cred, region, nil
}

// configFingerprint identifies the provider settings a server is bound to.
// The secret only contributes through its hash.
func configFingerprint(req StartRequest, cred provider.ResolvedCredential, region string) string {
	h := sha256.New()
	secret := sha256.Sum256([]byte(cred.Secret))
	for _, part := range []string{req.ProjectID, deploy.OpencodeProviderID(req.Provider), region,
		string(cred.Source.Kind), cred.Source.KeychainKey, cred.Source.Profile, hex.EncodeToString(secret[:])} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func attachPref(p sessionspec.AttachPref) termlaunch.Pref {
	switch p {
	case sessionspec.AttachITerm:
		return termlaunch.PrefITerm
	case sessionspec.AttachTerminal:
		return termlaunch.PrefTerminal
	case sessionspec.AttachTmux:
		return termlaunch.PrefTmux
	}
	return termlaunch.PrefAuto
}

// ensureServer returns a healthy server for the group, starting one if needed.
// Must be called with the group lock held.
func (s *Service) ensureServer(ctx context.Context, dc DaemonClient, req StartRequest, key sessionspec.GroupKey, gk string, cred provider.ResolvedCredential, region string) (*domain.Server, bool, adapters.VisibilityReport, error) {
	if srv, err := s.Servers.Get(ctx, gk); err == nil && srv.Status == domain.ServerReady && filelock.ProcessAlive(srv.PID) {
		if _, uerr := dc.Usage(ctx, srv.ProxyToken); uerr == nil {
			rep, aerr := s.Adapter.Attest(ctx, handle(srv), req.Bundle.Spec, req.Location)
			if aerr == nil && rep.OK() {
				return srv, true, rep, nil
			}
			slog.Warn("runsvc: existing server failed attestation, restarting", "group", gk, "error", aerr, "unexpected", rep.Unexpected)
		} else {
			slog.Warn("runsvc: existing server lost its proxy grant, restarting", "group", gk)
		}
		_ = s.Adapter.StopServer(ctx, handle(srv))
		_ = dc.RevokeOwner(ctx, gk)
	}

	srv, rep, err := s.startServer(ctx, dc, req, key, gk, cred, region)
	if err == nil && !rep.OK() {
		if r, ok := s.Adapter.(nativeRefresher); ok {
			slog.Info("runsvc: unexpected agents, re-discovering native agents", "unexpected", rep.Unexpected)
			r.RefreshNatives(ctx)
			_ = s.Adapter.StopServer(ctx, handle(srv))
			_ = dc.RevokeOwner(ctx, gk)
			srv, rep, err = s.startServer(ctx, dc, req, key, gk, cred, region)
		}
	}
	if err != nil {
		return nil, false, rep, err
	}
	if !rep.OK() {
		_ = s.Adapter.StopServer(ctx, handle(srv))
		_ = dc.RevokeOwner(ctx, gk)
		_ = s.Servers.SetStatus(ctx, gk, domain.ServerStopped)
		return nil, false, rep, fmt.Errorf("%w: visible outside the bundle: %v", ErrIsolation, rep.Unexpected)
	}
	return srv, false, rep, nil
}

func (s *Service) startServer(ctx context.Context, dc DaemonClient, req StartRequest, key sessionspec.GroupKey, gk string, cred provider.ResolvedCredential, region string) (*domain.Server, adapters.VisibilityReport, error) {
	ocProvider := deploy.OpencodeProviderID(req.Provider)
	grant, err := dc.IssueGrant(ctx, daemon.GrantRequest{
		Owner: gk, Provider: ocProvider, Region: region, Source: cred.Source, Secret: cred.Secret,
		AllowedModels: req.AllowedModels, MaxTokens: req.MaxTokens,
	})
	if err != nil {
		return nil, adapters.VisibilityReport{}, fmt.Errorf("issuing proxy grant: %w", err)
	}

	dataDir := filepath.Join(s.ServersDir, gk, "data")
	srv := &domain.Server{
		GroupKey: gk, Adapter: s.Adapter.Name(), AdapterVersion: s.AdapterVer, Runtime: string(key.Runtime),
		ProjectID: req.ProjectID, BundleHash: key.BundleHash, DataDir: dataDir, WorkDir: req.Location,
		ProxyToken: grant.Token, Status: domain.ServerStarting, CreatedAt: time.Now(),
	}
	if err := s.Servers.Upsert(ctx, srv); err != nil {
		return nil, adapters.VisibilityReport{}, err
	}
	h, err := s.Adapter.StartServer(ctx, adapters.ServerGroup{
		Key: key, Bundle: req.Bundle.Spec, DataDir: dataDir, WorkDir: req.Location,
		Provider: sessionspec.ProviderSpec{ID: ocProvider, Region: region, BaseURL: grant.BaseURL, SessionToken: grant.Token},
	})
	if err != nil {
		_ = dc.RevokeOwner(ctx, gk)
		_ = s.Servers.SetStatus(ctx, gk, domain.ServerStopped)
		return nil, adapters.VisibilityReport{}, fmt.Errorf("starting tool server: %w", err)
	}
	srv.PID, srv.URL, srv.Password, srv.Status = h.PID, h.URL, h.Password, domain.ServerReady
	srv.Port = portOf(h.URL)
	srv.LastActivityAt = time.Now()
	if err := s.Servers.Upsert(ctx, srv); err != nil {
		return nil, adapters.VisibilityReport{}, err
	}
	rep, err := s.Adapter.Attest(ctx, h, req.Bundle.Spec, req.Location)
	if err != nil {
		return srv, rep, fmt.Errorf("checking isolation: %w", err)
	}
	return srv, rep, nil
}

func (s *Service) persistSession(ctx context.Context, req StartRequest, srv *domain.Server, sid, entry string) {
	if s.Sessions == nil || req.ProjectID == "" {
		return
	}
	ext := sid
	title := req.Title
	sess := &domain.Session{
		ID: sid, ProjectID: req.ProjectID, Status: domain.SessionStatusRunning, Provider: req.Provider,
		LaunchPath: req.Location, MemberID: req.MemberID, Platform: s.Adapter.Name(), ExternalSessionID: &ext,
		PID: srv.PID, Type: domain.SessionTypeInteractive,
		WorkflowID: req.WorkflowID, EntryAgent: entry, BundleHash: srv.BundleHash, GroupKey: srv.GroupKey,
		Runtime: srv.Runtime, Mode: req.Mode, State: domain.RunActive,
	}
	if title != "" {
		sess.Title = &title
	}
	if err := s.Sessions.Create(ctx, sess); err != nil {
		slog.Warn("runsvc: session tracking failed", "session", sid, "error", err)
	}
}

func (s *Service) markFailed(ctx context.Context, sid string) {
	if s.Sessions == nil {
		return
	}
	sess, err := s.Sessions.Get(ctx, sid)
	if err != nil {
		return
	}
	sess.State = domain.RunFailed
	sess.Status = domain.SessionStatusFailed
	_ = s.Sessions.Update(ctx, sess)
}

// Attach opens an interactive client for a session in a new terminal tab or
// window. The command run there is `oh session attach <id> --exec`: no secret
// appears on the command line.
func (s *Service) Attach(ctx context.Context, sessionID, dir string, pref termlaunch.Pref, style termlaunch.ITermStyle, title string) (termlaunch.Method, error) {
	exe := s.Executable
	if exe == "" {
		var err error
		if exe, err = os.Executable(); err != nil {
			return "", err
		}
	}
	m, attempts, err := termlaunch.Launch(ctx, termlaunch.Options{
		Pref: pref, ITermStyle: style, Dir: dir, Title: title,
		Argv: attachArgv(exe, sessionID),
	})
	for _, a := range attempts {
		if a.Err != nil {
			slog.Info("runsvc: terminal method failed", "method", a.Method, "error", a.Err)
		}
	}
	return m, err
}

// attachArgv is the command run in the new terminal. OH_HOME (relocated hub)
// is forwarded because the new terminal does not inherit oh's environment.
func attachArgv(exe, sessionID string) []string {
	argv := []string{exe, "session", "attach", sessionID, "--exec"}
	if home := os.Getenv("OH_HOME"); home != "" {
		argv = append([]string{"/usr/bin/env", "OH_HOME=" + home}, argv...)
	}
	return argv
}

// AttachCommand returns the tool client command and environment for a
// session (used by `oh session attach --exec`).
func (s *Service) AttachCommand(ctx context.Context, sessionID string) (argv, env []string, err error) {
	srv, err := s.serverForSession(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	argv, env = s.Adapter.AttachCommand(handle(srv), sessionID)
	return argv, env, nil
}

// PairURL returns a one-time browser URL for the server of a session.
func (s *Service) PairURL(ctx context.Context, sessionID string, pair func(ctx context.Context, url, password string) (string, error)) (string, error) {
	srv, err := s.serverForSession(ctx, sessionID)
	if err != nil {
		return "", err
	}
	return pair(ctx, srv.URL, srv.Password)
}

func (s *Service) serverForSession(ctx context.Context, sessionID string) (*domain.Server, error) {
	if s.Sessions == nil {
		return nil, errors.New("runsvc: no session store")
	}
	sess, err := s.Sessions.Get(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session %s: %w", sessionID, err)
	}
	if sess.GroupKey == "" {
		return nil, fmt.Errorf("session %s was not started by the v5 launcher", sessionID)
	}
	srv, err := s.Servers.Get(ctx, sess.GroupKey)
	if err != nil {
		return nil, fmt.Errorf("server of session %s: %w", sessionID, err)
	}
	if srv.Status != domain.ServerReady || !filelock.ProcessAlive(srv.PID) {
		return nil, fmt.Errorf("%w (session %s, status %s)", ErrServerNotRunning, sessionID, srv.Status)
	}
	return srv, nil
}

// Session returns a v5 session.
func (s *Service) Session(ctx context.Context, sessionID string) (*domain.Session, error) {
	if s.Sessions == nil {
		return nil, errors.New("runsvc: no session store")
	}
	return s.Sessions.Get(ctx, sessionID)
}

// ResumeSession wakes up the server group of an existing session (after a
// sleep or a machine restart). req carries the provider settings; its bundle
// and location default to the session's.
func (s *Service) ResumeSession(ctx context.Context, sessionID string, req StartRequest) error {
	sess, err := s.Session(ctx, sessionID)
	if err != nil {
		return err
	}
	if sess.GroupKey == "" || sess.BundleHash == "" {
		return fmt.Errorf("session %s was not started by the v5 launcher", sessionID)
	}
	if sess.State == domain.RunStopped {
		return fmt.Errorf("session %s was stopped", sessionID)
	}
	if req.Bundle == nil {
		b, err := bundle.Load(s.BundlesDir, sess.BundleHash)
		if err != nil {
			return fmt.Errorf("loading the session bundle: %w", err)
		}
		req.Bundle = b
	}
	if req.Location == "" {
		req.Location = sess.LaunchPath
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
	key := sessionspec.GroupKey{BundleHash: sess.BundleHash, ProjectID: sess.ProjectID, Runtime: sessionspec.RuntimeLocal,
		Config: configFingerprint(req, cred, region)}
	// The session data lives in its original group: keep it even when the
	// provider settings changed. A sleeping server restarts with the new
	// settings; a running one keeps its own until it sleeps.
	gk := sess.GroupKey
	if key.String() != gk {
		slog.Info("runsvc: provider settings changed since the session started", "session", sessionID)
	}
	unlock, err := filelock.Lock(filepath.Join(s.ServersDir, gk, "lock"))
	if err != nil {
		return err
	}
	srv, _, _, err := s.ensureServer(ctx, dc, req, key, gk, cred, region)
	unlock()
	if err != nil {
		return err
	}
	now := time.Now()
	sess.State, sess.StateChangedAt, sess.PID = domain.RunIdle, &now, srv.PID
	if err := s.Sessions.Update(ctx, sess); err != nil {
		slog.Warn("runsvc: session update failed", "session", sessionID, "error", err)
	}
	_ = dc.Touch(ctx, gk)
	return nil
}

// StopSession stops a session: its agent loop is interrupted and, when no
// other session of the group is still open, the tool server is stopped.
func (s *Service) StopSession(ctx context.Context, sessionID string) error {
	sess, err := s.Session(ctx, sessionID)
	if err != nil {
		return err
	}
	if srv, err := s.Servers.Get(ctx, sess.GroupKey); err == nil && srv.Status == domain.ServerReady && filelock.ProcessAlive(srv.PID) {
		_ = s.Adapter.Control(ctx, handle(srv), sessionID, adapters.ControlOp{Kind: "interrupt"})
		others := 0
		if list, err := s.Sessions.List(ctx, sess.ProjectID); err == nil {
			for _, o := range list {
				if o.ID != sessionID && o.GroupKey == sess.GroupKey && !terminal(o.State) {
					others++
				}
			}
		}
		if others == 0 {
			_ = s.Adapter.StopServer(ctx, handle(srv))
			_ = s.Servers.SetStatus(ctx, sess.GroupKey, domain.ServerStopped)
			if dc, err := s.Daemon(ctx); err == nil {
				_ = dc.RevokeOwner(ctx, sess.GroupKey)
			}
		}
	}
	now := time.Now()
	sess.State, sess.StateChangedAt, sess.Status, sess.EndedAt = domain.RunStopped, &now, domain.SessionStatusCompleted, &now
	if err := s.Sessions.Update(ctx, sess); err != nil {
		return err
	}
	if s.OnSessionEnd != nil {
		s.OnSessionEnd(ctx, *sess)
	}
	return nil
}

func terminal(s domain.RunState) bool {
	return s == domain.RunStopped || s == domain.RunCompleted || s == domain.RunFailed
}

func handle(srv *domain.Server) adapters.ServerHandle {
	return adapters.ServerHandle{URL: srv.URL, Password: srv.Password, PID: srv.PID}
}

func portOf(rawURL string) int {
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0
	}
	port, _ := strconv.Atoi(u.Port())
	return port
}
