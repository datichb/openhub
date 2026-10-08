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
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/filelock"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/limits"
	"github.com/datichb/openhub/cli/internal/provider"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/sessionctx"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/termlaunch"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// DaemonClient is the subset of the ohd client used by the service.
type DaemonClient interface {
	IssueGrant(ctx context.Context, req daemon.GrantRequest) (daemon.GrantResponse, error)
	RevokeOwner(ctx context.Context, owner string) error
	Usage(ctx context.Context, token string) (daemon.UsageResponse, error)
	Touch(ctx context.Context, group string) error
	// ProxyListen makes the proxy also listen on host (containers on Linux).
	ProxyListen(ctx context.Context, host string) (string, error)
	// Health reports the daemon state (memory used by the tool servers).
	Health(ctx context.Context) (daemon.Health, error)
}

// containerTooler is implemented by adapters able to install their tool in
// container images.
type containerTooler interface {
	ContainerTool() ohruntime.Tool
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
	// SessionsDir (~/.oh/sessions) keeps per-session files (static environment).
	SessionsDir string
	// SessionEnv returns dynamic per-session variables (S7), e.g. a gateway
	// token minted for the session. Optional.
	SessionEnv SessionEnvFunc
	// OnTicketsStarted is called when a session of a run started on Beads
	// tickets (any workflow with a ticket input): the team claims are taken
	// or move to their work status (planned → in progress, QB2). It returns
	// notes for the user (StartResult.Notes). Nil = nothing.
	OnTicketsStarted func(ctx context.Context, projectID string, tickets []string) []string
	// LocalShellPath lists directories put first on the PATH of local
	// session shells: the fake bd, so that Beads goes through the gateway
	// and its beads.allow in every runtime (QB1).
	LocalShellPath []string
	// Decisions, when set, closes the pending decisions of stopped sessions.
	Decisions domain.DecisionStore
	// Usage is the usage ledger: a new session is refused when the daily
	// budget of its restrictions is spent (nil = not checked).
	Usage domain.UsageStore
	// OnSessionEnd is called when a session is stopped for good (team
	// session.complete event).
	OnSessionEnd func(ctx context.Context, s domain.Session)
	Executable   string // oh binary used to attach (default os.Executable)
	// Runtimes are the non-local execution environments (container…).
	Runtimes map[sessionspec.RuntimeKind]ohruntime.Runtime
	// LaunchLocksDir holds the launch locks (default <ServersDir>/../run/launch).
	LaunchLocksDir string
	// Git replaces the git operations of the launcher (tests).
	Git Git
}

// StartRequest describes a session to start.
type StartRequest struct {
	// SessionID is the session id to use ("" = new). Set by the oh runner:
	// a remote session keeps the id chosen on the machine (export/import).
	SessionID       string
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

	// Workflow launch records (oh run, launch form).
	WorkflowLayer   string
	WorkflowVersion int
	WorkflowRisk    string
	LocationKind    string // base | worktree
	ParentSessionID string
	// Tickets are the tickets the session works on (kept on the session).
	Tickets []string
	// Headless marks a session run without interactive client.
	Headless bool

	Provider      string          // hub provider name: bedrock | anthropic | openrouter
	ProviderCfg   provider.Config // AWS profile / region
	AllowedModels []string
	MaxTokens     int64

	Attach     sessionspec.AttachPref
	ITermStyle termlaunch.ITermStyle

	// Runtime is where the server group runs ("" = local).
	Runtime sessionspec.RuntimeKind
	// Container settings (runtime container).
	ProjectDir string            // project base directory (dev Dockerfile); default Location
	Dockerfile string            // "" = detected
	BuildArgs  map[string]string // dev image build arguments
	Volumes    []string          // cache volumes
	Progress   func(line string) // preparation output (image build)
	// IsolateUserConfig hides the user tool configuration from local
	// servers (strict isolation setting, part of the group key).
	IsolateUserConfig bool

	// SessionEnv holds static, non-secret variables of the session shell
	// (persisted for resumes). Secrets go through Service.SessionEnv.
	SessionEnv map[string]string

	// BeadsAllow is the workflow `beads.allow` list, enforced by the Beads
	// gateway in every runtime (nil = no `beads:` block: read-only default;
	// empty = no bd command). Kept for resumes.
	BeadsAllow []string

	// Limits are the resolved restrictions of the session (I6; zero = none).
	// Their model allow-list applies to the group's proxy grant; budgets and
	// the queue are enforced by the daemon. Kept for resumes.
	Limits limits.Resolved
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
	// Notes are messages for the user about the launch (team claims).
	Notes []string
	// Stashed is the stash commit holding the changes of the directory put
	// aside before the session started (DirtyStash).
	Stashed string
	// Queued: the first prompt waits for a free slot (restrictions); Ahead
	// is the number of sessions queued before it.
	Queued bool
	Ahead  int
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
	if err := s.checkDailyBudget(ctx, req.Limits); err != nil {
		return nil, err
	}
	dc, err := s.Daemon(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting oh daemon: %w", err)
	}
	applyLimits(&req)

	cred, region, err := s.resolveProvider(ctx, &req)
	if err != nil {
		return nil, err
	}
	kind := runtimeKind(req.Runtime)
	rt, err := s.runtime(kind)
	if err != nil {
		return nil, err
	}
	key := sessionspec.GroupKey{BundleHash: spec.Hash, ProjectID: req.ProjectID, Runtime: kind,
		Config: configFingerprint(req, ToolProviderID(s.Adapter, req.Provider), cred, region)}
	var (
		gk     string
		srv    *domain.Server
		reused bool
		report adapters.VisibilityReport
		pg     *ohruntime.Prepared
	)
	// A container group whose mounts do not cover the location is restarted
	// when idle; when busy, the next slot (sibling group) is used.
	for ; ; key.Slot++ {
		if key.Slot > maxSlots {
			return nil, fmt.Errorf("runsvc: no container group can host %s (all %d slots busy)", req.Location, maxSlots)
		}
		gk = key.String()
		unlock, err := s.lockGroup(ctx, gk)
		if err != nil {
			return nil, err
		}
		srv, reused, report, pg, err = s.ensureServer(ctx, dc, req, key, gk, cred, region, rt, !s.groupBusy(ctx, req.ProjectID, gk))
		unlock()
		if errors.Is(err, errNotMounted) {
			continue
		}
		if err != nil {
			return nil, err
		}
		break
	}

	res := &StartResult{GroupKey: gk, Server: srv, Reused: reused, Report: report}
	h := handle(srv)
	if !reused {
		if err := s.reapplyGroupEnv(ctx, srv, ""); err != nil {
			slog.Warn("runsvc: session environment not restored", "group", gk, "error", err)
		}
	}
	sid := req.SessionID
	if sid == "" {
		sid = sessionspec.NewSessionID()
	}
	static := withMachineShellEnv(kind, req.SessionEnv, s.LocalShellPath)
	env, err := s.buildSessionEnv(ctx, static, SessionEnvRequest{SessionID: sid, GroupKey: gk, ProjectID: req.ProjectID, Location: req.Location,
		Runtime: kind, WorkflowID: req.WorkflowID, BeadsAllow: req.BeadsAllow, GatewayURL: s.loadGatewayURL(gk)})
	if err != nil {
		return nil, err
	}
	if req.Mode == "" && spec.Workflow != nil {
		req.Mode = spec.Workflow.DefaultMode
	}
	ss := sessionspec.SessionSpec{
		SessionID: sid, Title: req.Title, Group: key, ProjectID: req.ProjectID,
		Location: innerPath(pg, req.Location), EntryAgent: entry, Mode: req.Mode, Prompt: req.Prompt,
		Runtime: kind, Attach: req.Attach,
		Model: entryModel(spec, entry), Provider: sessionspec.ProviderSpec{Region: region},
		SessionEnv: env,
		// Checkpoints and agent locks of the workflow (P3-T02/T04); the
		// daemon replaces them as the session moves on.
		SessionRules: bundle.SessionRules(spec.Workflow, req.Mode, nil),
	}
	if err := s.saveStaticEnv(sid, static); err != nil {
		return nil, fmt.Errorf("saving session environment: %w", err)
	}
	if err := s.saveBeadsAllow(sid, req.BeadsAllow); err != nil {
		return nil, fmt.Errorf("saving session Beads allow-list: %w", err)
	}
	if s.SessionsDir != "" {
		if err := limits.Save(s.SessionsDir, sid, req.Limits); err != nil {
			return nil, fmt.Errorf("saving session restrictions: %w", err)
		}
	}
	var queued *limits.Queued
	if req.Prompt != "" {
		queued, res.Ahead = s.queueSlot(ctx, dc, req)
	}
	// The oh row is written before the tool session exists, so that the
	// daemon tracks the session from its very first event.
	state := domain.RunActive
	if queued != nil {
		state = domain.RunQueued
	}
	s.persistSession(ctx, req, srv, sid, entry, state)
	if err := s.Adapter.CreateSession(ctx, h, ss); err != nil {
		s.markFailed(ctx, sid)
		return nil, fmt.Errorf("creating session: %w", err)
	}
	res.SessionID = sid
	s.setInitialContext(ctx, h, sid, spec.Workflow, req.Mode)

	switch {
	case queued != nil:
		// The daemon sends the prompt when a slot frees up.
		if err := limits.SaveQueued(s.SessionsDir, sid, *queued); err != nil {
			return res, fmt.Errorf("queueing the session: %w", err)
		}
		res.Queued = true
	case req.Prompt != "":
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

// groupLockWait bounds the wait for the lock of a server group (another
// client starting it, possibly building a container image).
const groupLockWait = 15 * time.Minute

// lockGroup takes the lock of a server group (shared with the daemon,
// daemon.GroupLockPath), until ctx is done or groupLockWait expires.
func (s *Service) lockGroup(ctx context.Context, gk string) (func(), error) {
	lctx, cancel := context.WithTimeout(ctx, groupLockWait)
	defer cancel()
	return filelock.LockContext(lctx, daemon.GroupLockPath(s.ServersDir, gk))
}

// maxSlots bounds the sibling container groups of one configuration.
const maxSlots = 8

// errNotMounted means a running container group does not see the location
// and cannot be restarted (busy).
var errNotMounted = errors.New("location not mounted in the running container group")

func runtimeKind(k sessionspec.RuntimeKind) sessionspec.RuntimeKind {
	if k == "" {
		return sessionspec.RuntimeLocal
	}
	return k
}

// runtime returns the execution environment of a kind (nil = local).
func (s *Service) runtime(k sessionspec.RuntimeKind) (ohruntime.Runtime, error) {
	if k == sessionspec.RuntimeLocal {
		return nil, nil
	}
	if rt := s.Runtimes[k]; rt != nil {
		return rt, nil
	}
	return nil, fmt.Errorf("runsvc: runtime %q is not available", k)
}

// groupBusy reports whether a session of the group is working or waiting
// for a decision (its server must not be restarted).
func (s *Service) groupBusy(ctx context.Context, projectID, gk string) bool {
	if s.Sessions == nil {
		return false
	}
	list, err := s.Sessions.List(ctx, projectID)
	if err != nil {
		return true
	}
	for _, o := range list {
		if o.GroupKey == gk && (o.State == domain.RunActive || o.State == domain.RunWaiting || o.State == domain.RunPreparing || o.State == domain.RunQueued) {
			return true
		}
	}
	return false
}

// innerPath is a machine path as seen by the server of a prepared group.
func innerPath(pg *ohruntime.Prepared, p string) string {
	if pg == nil {
		return p
	}
	if in, ok := pg.Paths.ToInner(p); ok {
		return in
	}
	return p
}

// runtimeGroup describes a group to its runtime.
func (s *Service) runtimeGroup(req StartRequest, key sessionspec.GroupKey, gk string) ohruntime.Group {
	g := ohruntime.Group{
		Key: key, GroupID: gk, ProjectID: req.ProjectID, ProjectDir: req.ProjectDir,
		Locations: []string{req.Location}, DataDir: filepath.Join(s.ServersDir, gk, "data"),
		Dockerfile: req.Dockerfile, BuildArgs: req.BuildArgs, Volumes: req.Volumes, Progress: req.Progress,
	}
	if g.ProjectDir == "" {
		g.ProjectDir = req.Location
	}
	if req.Bundle != nil {
		g.BundleDir = req.Bundle.Spec.Root
		if g.BundleDir == "" {
			g.BundleDir = req.Bundle.Dir
		}
	}
	if t, ok := s.Adapter.(containerTooler); ok {
		g.Tool = t.ContainerTool()
	}
	return g
}

// stopServer stops a tool server and removes its runtime environment.
func (s *Service) stopServer(ctx context.Context, srv *domain.Server) {
	_ = s.Adapter.StopServer(ctx, handle(srv))
	rt, err := s.runtime(runtimeKind(sessionspec.RuntimeKind(srv.Runtime)))
	if err != nil || rt == nil {
		return
	}
	if pg, err := rt.Load(ctx, ohruntime.Group{GroupID: srv.GroupKey, DataDir: srv.DataDir}); err == nil {
		_ = rt.Teardown(ctx, pg)
	}
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
	if region == "" && provider.Name(req.Provider) == provider.Bedrock {
		region = credproxy.AWSRegion(ctx, req.ProviderCfg.AWSProfile)
		if region == "" {
			region = "us-east-1"
			slog.Warn("runsvc: no AWS region configured (oh config, AWS_REGION, profile); using us-east-1")
		}
	}
	return cred, region, nil
}

// ToolProviderID is the provider id of the tool for a hub provider (the
// adapter's, or the hub name itself).
func ToolProviderID(ad adapters.ToolAdapter, hubProvider string) string {
	if m, ok := ad.(adapters.ProviderMapper); ok {
		return m.ProviderID(hubProvider)
	}
	return hubProvider
}

// configFingerprint identifies the provider settings a server is bound to.
// The secret only contributes through its hash.
func configFingerprint(req StartRequest, toolProvider string, cred provider.ResolvedCredential, region string) string {
	h := sha256.New()
	secret := sha256.Sum256([]byte(cred.Secret))
	parts := []string{req.ProjectID, toolProvider, region,
		string(cred.Source.Kind), cred.Source.KeychainKey, cred.Source.Profile, hex.EncodeToString(secret[:])}
	if len(req.AllowedModels) > 0 {
		// The allow-list is enforced on the group's proxy grant: another
		// list needs another server. (Absent = fingerprint unchanged.)
		parts = append(parts, "models="+strings.Join(req.AllowedModels, ","))
	}
	for _, part := range parts {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	if req.IsolateUserConfig {
		h.Write([]byte("isolate-user-config")) // only when set: other keys unchanged
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
// With a runtime, a running group must see the location: otherwise it is
// restarted with the location added when restart is allowed, else
// errNotMounted is returned. Must be called with the group lock held.
func (s *Service) ensureServer(ctx context.Context, dc DaemonClient, req StartRequest, key sessionspec.GroupKey, gk string, cred provider.ResolvedCredential, region string, rt ohruntime.Runtime, restart bool) (*domain.Server, bool, adapters.VisibilityReport, *ohruntime.Prepared, error) {
	g := s.runtimeGroup(req, key, gk)
	var prev *ohruntime.Prepared
	if rt != nil {
		prev, _ = rt.Load(ctx, g)
	}
	if srv, err := s.Servers.Get(ctx, gk); err == nil && srv.Status == domain.ServerReady && filelock.ProcessAlive(srv.PID) {
		switch _, uerr := dc.Usage(ctx, srv.ProxyTokenHash); {
		case uerr != nil:
			slog.Warn("runsvc: existing server lost its proxy grant, restarting", "group", gk)
		case rt != nil && (prev == nil || !prev.Paths.Covers(req.Location)):
			if !restart {
				return nil, false, adapters.VisibilityReport{}, nil, errNotMounted
			}
			slog.Info("runsvc: restarting the container group to mount a new location", "group", gk, "location", req.Location)
		default:
			rep, aerr := s.Adapter.Attest(ctx, handle(srv), req.Bundle.Spec, innerPath(prev, req.Location))
			if aerr == nil && rep.OK() {
				return srv, true, rep, prev, nil
			}
			slog.Warn("runsvc: existing server failed attestation, restarting", "group", gk, "error", aerr, "unexpected", rep.Unexpected)
		}
		s.stopServer(ctx, srv)
		_ = dc.RevokeOwner(ctx, gk)
	}
	if prev != nil {
		// Keep the locations of the group's other sessions (resumable).
		for _, l := range prev.Group.Locations {
			if st, err := os.Stat(l); err == nil && st.IsDir() && l != req.Location {
				g.Locations = append(g.Locations, l)
			}
		}
	}

	srv, rep, pg, err := s.startServer(ctx, dc, req, key, gk, cred, region, rt, g)
	if err == nil && !rep.OK() {
		if r, ok := s.Adapter.(nativeRefresher); ok {
			slog.Info("runsvc: unexpected agents, re-discovering native agents", "unexpected", rep.Unexpected)
			r.RefreshNatives(ctx)
			s.stopServer(ctx, srv)
			_ = dc.RevokeOwner(ctx, gk)
			srv, rep, pg, err = s.startServer(ctx, dc, req, key, gk, cred, region, rt, g)
		}
	}
	if err != nil {
		return nil, false, rep, nil, err
	}
	if !rep.OK() {
		s.stopServer(ctx, srv)
		_ = dc.RevokeOwner(ctx, gk)
		_ = s.Servers.SetStatus(ctx, gk, domain.ServerStopped)
		return nil, false, rep, nil, fmt.Errorf("%w: visible outside the bundle: %v", ErrIsolation, rep.Unexpected)
	}
	return srv, false, rep, pg, nil
}

func (s *Service) startServer(ctx context.Context, dc DaemonClient, req StartRequest, key sessionspec.GroupKey, gk string, cred provider.ResolvedCredential, region string, rt ohruntime.Runtime, g ohruntime.Group) (*domain.Server, adapters.VisibilityReport, *ohruntime.Prepared, error) {
	var pg *ohruntime.Prepared
	if rt != nil {
		if g.Tool == nil {
			return nil, adapters.VisibilityReport{}, nil, fmt.Errorf("runsvc: adapter %s cannot run in a container", s.Adapter.Name())
		}
		// Building an image may take longer than the daemon's idle timeout
		// (no live server yet): keep it busy, and take a fresh client after.
		stopBusy := keepDaemonBusy(ctx, dc, gk)
		var err error
		pg, err = rt.Prepare(ctx, g)
		stopBusy()
		if err != nil {
			return nil, adapters.VisibilityReport{}, nil, fmt.Errorf("preparing the %s runtime: %w", rt.Kind(), err)
		}
		if fresh, derr := s.Daemon(ctx); derr == nil {
			dc = fresh
		}
	}
	ocProvider := ToolProviderID(s.Adapter, req.Provider)
	grant, err := dc.IssueGrant(ctx, daemon.GrantRequest{
		Owner: gk, Provider: ocProvider, Region: region, Source: cred.Source, Secret: cred.Secret,
		AllowedModels: req.AllowedModels, MaxTokens: req.MaxTokens,
	})
	if err != nil {
		return nil, adapters.VisibilityReport{}, nil, fmt.Errorf("issuing proxy grant: %w", err)
	}
	baseURL := grant.BaseURL
	if pg != nil {
		if baseURL, err = s.proxyURLFor(ctx, dc, pg, grant.BaseURL); err != nil {
			_ = dc.RevokeOwner(ctx, gk)
			return nil, adapters.VisibilityReport{}, nil, err
		}
	}
	if err := s.saveGatewayURL(gk, baseURL); err != nil {
		_ = dc.RevokeOwner(ctx, gk)
		return nil, adapters.VisibilityReport{}, nil, err
	}
	if err := s.saveProxyURL(gk, baseURL); err != nil {
		_ = dc.RevokeOwner(ctx, gk)
		return nil, adapters.VisibilityReport{}, nil, err
	}

	dataDir := filepath.Join(s.ServersDir, gk, "data")
	srv := &domain.Server{
		GroupKey: gk, Adapter: s.Adapter.Name(), AdapterVersion: s.AdapterVer, Runtime: string(key.Runtime),
		ProjectID: req.ProjectID, BundleHash: key.BundleHash, DataDir: dataDir, WorkDir: req.Location,
		ProxyTokenHash: credproxy.TokenHash(grant.Token), Status: domain.ServerStarting, CreatedAt: time.Now(),
	}
	if err := s.Servers.Upsert(ctx, srv); err != nil {
		_ = dc.RevokeOwner(ctx, gk)
		return nil, adapters.VisibilityReport{}, nil, err
	}
	h, err := s.Adapter.StartServer(ctx, adapters.ServerGroup{
		Key: key, Bundle: req.Bundle.Spec, DataDir: dataDir, WorkDir: req.Location,
		Provider: sessionspec.ProviderSpec{ID: ocProvider, Region: region, BaseURL: baseURL, SessionToken: grant.Token},
		Runtime:  rt, Prepared: pg, IsolateUserConfig: req.IsolateUserConfig,
	})
	if err != nil {
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		_ = dc.RevokeOwner(cctx, gk)
		_ = s.Servers.SetStatus(cctx, gk, domain.ServerStopped)
		if rt != nil {
			_ = rt.Teardown(cctx, pg)
		}
		return nil, adapters.VisibilityReport{}, nil, fmt.Errorf("starting tool server: %w", err)
	}
	srv.PID, srv.URL, srv.Password, srv.Status = h.PID, h.URL, h.Password, domain.ServerReady
	srv.Port = portOf(h.URL)
	srv.LastActivityAt = time.Now()
	if err := s.Servers.Upsert(ctx, srv); err != nil {
		s.abandonServer(ctx, dc, srv)
		return nil, adapters.VisibilityReport{}, nil, err
	}
	rep, err := s.Adapter.Attest(ctx, h, req.Bundle.Spec, innerPath(pg, req.Location))
	if err != nil {
		s.abandonServer(ctx, dc, srv)
		return nil, rep, nil, fmt.Errorf("checking isolation: %w", err)
	}
	return srv, rep, pg, nil
}

// abandonServer stops a started server whose setup failed afterwards: its
// process, runtime environment and proxy grant must not outlive the error.
// The context may be cancelled (Ctrl+C): cleanup gets its own.
func (s *Service) abandonServer(ctx context.Context, dc DaemonClient, srv *domain.Server) {
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	s.stopServer(cctx, srv)
	_ = dc.RevokeOwner(cctx, srv.GroupKey)
	_ = s.Servers.SetStatus(cctx, srv.GroupKey, domain.ServerStopped)
}

// proxyURLFor rewrites the proxy base URL for a runtime: the host becomes
// the machine address seen from inside; when the machine loopback is not
// reachable (Linux), the daemon also listens on the runtime's host IP.
func (s *Service) proxyURLFor(ctx context.Context, dc DaemonClient, pg *ohruntime.Prepared, base string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("proxy URL %q: %w", base, err)
	}
	port := u.Port()
	if pg.ListenHost != "" {
		l, err := dc.ProxyListen(ctx, pg.ListenHost)
		if err != nil {
			return "", fmt.Errorf("making the credential proxy reachable from the %s runtime: %w", pg.Group.Key.Runtime, err)
		}
		lu, err := url.Parse(l)
		if err != nil {
			return "", err
		}
		port = lu.Port()
	}
	if pg.HostAddress == "" {
		return "", errors.New("runsvc: the runtime has no machine address")
	}
	u.Host = net.JoinHostPort(pg.HostAddress, port)
	return u.String(), nil
}

func (s *Service) persistSession(ctx context.Context, req StartRequest, srv *domain.Server, sid, entry string, state domain.RunState) {
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
		Runtime: srv.Runtime, Mode: req.Mode, State: state,
		WorkflowLayer: req.WorkflowLayer, WorkflowVersion: req.WorkflowVersion, WorkflowRisk: req.WorkflowRisk,
		Location: req.LocationKind, ParentSessionID: req.ParentSessionID,
		StartRef: worktree.StartRef(req.Location), Tickets: req.Tickets,
	}
	if title != "" {
		sess.Title = &title
	}
	if req.Headless {
		sess.Type = domain.SessionTypeHeadless
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
	s.removeStaticEnv(sid)
}

// closeDecisions resolves the open decisions of a stopped session.
func (s *Service) closeDecisions(ctx context.Context, sessionID string) {
	if s.Decisions == nil {
		return
	}
	open, err := s.Decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: sessionID})
	if err != nil {
		return
	}
	now := time.Now()
	for _, d := range open {
		_, _ = s.Decisions.Resolve(ctx, d.ID, domain.ResolvedByGone, nil, now)
	}
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
		Argv: attachArgv(exe, sessionID), Env: attachEnv(),
	})
	for _, a := range attempts {
		if a.Err != nil {
			slog.Info("runsvc: terminal method failed", "method", a.Method, "error", a.Err)
		}
	}
	return m, err
}

// attachArgv is the command run in the new terminal.
func attachArgv(exe, sessionID string) []string {
	return []string{exe, "session", "attach", sessionID, "--exec"}
}

// attachEnv is the environment of that command: OH_HOME (relocated hub) is
// forwarded because the new terminal does not inherit oh's environment.
func attachEnv() map[string]string {
	if home := os.Getenv("OH_HOME"); home != "" {
		return map[string]string{"OH_HOME": home}
	}
	return nil
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
	if s.SessionsDir != "" {
		if l, err := limits.Load(s.SessionsDir, sessionID); err == nil {
			req.Limits = l
		}
	}
	applyLimits(&req)
	dc, err := s.Daemon(ctx)
	if err != nil {
		return err
	}
	cred, region, err := s.resolveProvider(ctx, &req)
	if err != nil {
		return err
	}
	kind := runtimeKind(sessionspec.RuntimeKind(sess.Runtime))
	rt, err := s.runtime(kind)
	if err != nil {
		return err
	}
	req.Runtime = kind
	key := sessionspec.GroupKey{BundleHash: sess.BundleHash, ProjectID: sess.ProjectID, Runtime: kind,
		Config: configFingerprint(req, ToolProviderID(s.Adapter, req.Provider), cred, region)}
	// The session data lives in its original group: keep it even when the
	// provider settings changed. A sleeping server restarts with the new
	// settings; a running one keeps its own until it sleeps.
	gk := sess.GroupKey
	if key.String() != gk {
		slog.Info("runsvc: provider settings changed since the session started", "session", sessionID)
	}
	unlock, err := s.lockGroup(ctx, gk)
	if err != nil {
		return err
	}
	srv, reused, _, _, err := s.ensureServer(ctx, dc, req, key, gk, cred, region, rt, !s.groupBusy(ctx, sess.ProjectID, gk))
	unlock()
	if err != nil {
		return err
	}
	if !reused {
		if err := s.reapplyGroupEnv(ctx, srv, sess.ID); err != nil {
			slog.Warn("runsvc: session environment not restored", "group", gk, "error", err)
		}
		if err := s.reapplySessionEnv(ctx, srv, sess); err != nil {
			return fmt.Errorf("restoring the session environment: %w", err)
		}
	}
	now := time.Now()
	sess.State, sess.StateChangedAt, sess.PID = domain.RunIdle, &now, srv.PID
	if err := s.Sessions.Update(ctx, sess); err != nil {
		slog.Warn("runsvc: session update failed", "session", sessionID, "error", err)
	}
	_ = dc.Touch(ctx, gk)
	if !reused {
		s.setResumeContext(ctx, srv, sess.ID)
	}
	return nil
}

// setInitialContext writes the session state before the first prompt (A29):
// an entry set before the first turn is part of it, while one written during
// the turn is announced at its next step and makes the agent run one more.
// A workflow without checkpoint has no oh.checkpoints entry.
func (s *Service) setInitialContext(ctx context.Context, h adapters.ServerHandle, sessionID string, wf *sessionspec.WorkflowRuntime, mode string) {
	c, ok := sessionctx.InitialCheckpoints(wf, mode, i18n.Locale())
	if !ok {
		return
	}
	w := &sessionctx.Writer{Adapter: s.Adapter, Dir: s.SessionsDir}
	if _, err := w.Set(ctx, h, sessionID, sessionctx.Entry{Key: sessionctx.KeyCheckpoints, Value: c.Value()}); err != nil {
		slog.Warn("runsvc: session state not set", "session", sessionID, "error", err)
	}
}

// setResumeContext tells the entry agent that its server restarted (S8,
// QB8): a session context entry, cleared by the daemon after the next step;
// a synthetic message without the capability.
func (s *Service) setResumeContext(ctx context.Context, srv *domain.Server, sessionID string) {
	w := &sessionctx.Writer{Adapter: s.Adapter, Dir: s.SessionsDir}
	text := i18n.T("cmd.session.resume.context")
	if _, err := w.Set(ctx, handle(srv), sessionID, sessionctx.Entry{Key: sessionctx.KeyResume,
		Value: map[string]any{"text": text}, Fallback: text}); err != nil {
		slog.Warn("runsvc: resume instruction not set", "session", sessionID, "error", err)
	}
}

// StopSession stops a session: its agent loop is interrupted and, when no
// other session of the group is still open, the tool server is stopped.
func (s *Service) StopSession(ctx context.Context, sessionID string) error {
	sess, err := s.Session(ctx, sessionID)
	if err != nil {
		return err
	}
	if srv, err := s.Servers.Get(ctx, sess.GroupKey); err == nil && srv.Status == domain.ServerReady && filelock.ProcessAlive(srv.PID) {
		_ = s.Adapter.Control(ctx, handle(srv), sessionID, adapters.ControlOp{Kind: adapters.ControlInterrupt})
		s.snapshotResults(ctx, srv, sessionID)
		others := 0
		if list, err := s.Sessions.List(ctx, sess.ProjectID); err == nil {
			for _, o := range list {
				if o.ID != sessionID && o.GroupKey == sess.GroupKey && !terminal(o.State) {
					others++
				}
			}
		}
		if others == 0 {
			s.stopServer(ctx, srv)
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
	s.removeStaticEnv(sessionID)
	s.closeDecisions(ctx, sessionID)
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
