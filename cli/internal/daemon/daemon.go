package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
)

// SecretGetter reads secrets from the host secret store (keychain…).
type SecretGetter interface {
	Get(ctx context.Context, key string) (string, error)
}

// Options configures a daemon.
type Options struct {
	Paths     Paths
	Version   string
	Grants    domain.GrantStore
	Servers   domain.ServerStore
	Secrets   SecretGetter  // nil = no secret store (pending grants wait for clients)
	IdleAfter time.Duration // stop after this long without live servers (default 10m)
	IdleSleep time.Duration // put an idle server group to sleep after this long (default 5m)
	Tick      time.Duration // supervision period (default 15s)
	// OnSessionEnd is called when the daemon stops a session for good (team
	// session.complete event).
	OnSessionEnd func(ctx context.Context, s domain.Session)
	// Sessions is updated by the session watchers (run state, cost, tokens).
	Sessions domain.SessionStore
	// Decisions receives the pending decisions of the tracked sessions (inbox).
	Decisions domain.DecisionStore
	// Notify shows desktop notifications (nil = disabled).
	Notify NotifyFunc
	// NotifyWindow groups the notifications of this period (default 3s).
	NotifyWindow time.Duration
	// ProjectName names a project in notifications (optional).
	ProjectName func(ctx context.Context, projectID string) string
	// SessionsDir (~/.oh/sessions) receives the results snapshot of the
	// sessions of a group before its server sleeps or stops.
	SessionsDir string
	// ServersDir (~/.oh/servers) holds the group locks shared with the oh
	// clients ("" = the daemon does not lock groups).
	ServersDir string
	// Checkpoints is the CheckpointService (workflow MCP backend, checkpoint
	// state). Nil = workflow API unavailable.
	Checkpoints *checkpoint.Service
	// Adapter returns the tool adapter for a server's adapter name (nil = no watcher).
	Adapter func(name string) adapters.ToolAdapter
	// GatewayView returns how the runtime of a server group sees the
	// machine (container path translation), for the Beads gateway.
	GatewayView func(ctx context.Context, group string) (gateway.View, error)
	// BeadsBinary is the real bd run by the gateway ("" = looked up in PATH).
	BeadsBinary string
	// MCPCommand returns the machine command of the oh MCP server name of a
	// server group, served over HTTP to runtimes outside the machine
	// (P4-T08). Nil = MCP gateway unavailable.
	MCPCommand func(ctx context.Context, srv domain.Server, name string) (gateway.MCPCommand, error)
	// SigV4 builds an AWS signer for a profile/region (overridable in tests).
	SigV4 func(ctx context.Context, profile, region string) (credproxy.Auth, error)
}

// Daemon is the running ohd process.
type Daemon struct {
	opts     Options
	proxy    *credproxy.Proxy
	gateway  *gateway.Store
	mcp      *gateway.MCP
	listener net.Listener
	http     *http.Server

	wmu      sync.Mutex
	watchers map[string]*watcher
	feed     *hub
	notes    *notifier

	mu          sync.Mutex
	clients     map[string]client
	verified    map[string]bool // "group#pid" → authenticated as our server
	verifyFails map[string]int  // "group#pid" → consecutive failed checks
	policies    map[string]QuitPolicy
	pending     map[string]domain.ProxyGrant
	lastBusy    time.Time
	stop        chan struct{}
	restored    chan struct{} // closed once the persisted grants are restored
	stopOnce    sync.Once
	sigMu       sync.Mutex
	sigCache    map[string]credproxy.Auth // SigV4 signers by profile/region
	kick        chan struct{}
}

type stateFile struct {
	ProxyPort int `json:"proxy_port"`
	// ListenHosts are additional proxy listen addresses (bridge gateways
	// for containers on Linux), restored at startup.
	ListenHosts []string `json:"listen_hosts,omitempty"`
}

// Run starts the daemon and blocks until it stops (idle timeout, shutdown
// request or ctx cancellation). It returns ErrAlreadyRunning when another
// daemon holds the lock.
func Run(ctx context.Context, opts Options) error {
	if opts.IdleAfter == 0 {
		opts.IdleAfter = 10 * time.Minute
	}
	if opts.Tick == 0 {
		opts.Tick = 15 * time.Second
	}
	if opts.IdleSleep == 0 {
		opts.IdleSleep = 5 * time.Minute
	}
	if opts.SigV4 == nil {
		opts.SigV4 = func(ctx context.Context, profile, region string) (credproxy.Auth, error) {
			return credproxy.NewSigV4FromProfile(ctx, profile, region)
		}
	}
	if err := os.MkdirAll(opts.Paths.Dir, 0o700); err != nil {
		return err
	}
	unlock, err := lockFile(opts.Paths.Lock())
	if err != nil {
		return err
	}
	defer unlock()

	d := &Daemon{opts: opts, proxy: credproxy.New(), pending: map[string]domain.ProxyGrant{}, clients: map[string]client{}, verified: map[string]bool{}, verifyFails: map[string]int{}, policies: map[string]QuitPolicy{}, watchers: map[string]*watcher{}, feed: newHub(), lastBusy: time.Now(), stop: make(chan struct{}), restored: make(chan struct{}), sigCache: map[string]credproxy.Auth{}, kick: make(chan struct{}, 1)}
	d.proxy.Hooks = d.hooksHandler()
	if err := d.startProxy(); err != nil {
		return err
	}
	defer func() {
		cctx, cancel := context.WithTimeout(context.Background(), proxyCloseWait)
		defer cancel()
		_ = d.proxy.Close(cctx)
	}()
	// The grants are restored once the socket answers: loading AWS
	// credentials (SSO, IMDS…) may be slower than the clients' spawn timeout.
	// Until then the proxy and the grant routes wait for the restoration.
	d.proxy.Hold()
	d.startGateway(ctx)

	_ = os.Remove(opts.Paths.Socket())
	l, err := net.Listen("unix", opts.Paths.Socket())
	if err != nil {
		return fmt.Errorf("listening on %s: %w", opts.Paths.Socket(), err)
	}
	_ = os.Chmod(opts.Paths.Socket(), 0o600)
	d.listener = l
	d.http = &http.Server{Handler: d.routes(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := d.http.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("ohd api stopped", "error", err)
		}
	}()
	slog.Info("ohd started", "pid", os.Getpid(), "proxy", d.proxy.URL(), "socket", opts.Paths.Socket())
	defer d.stopWatchers()
	go func() {
		defer close(d.restored)
		defer d.proxy.Release()
		d.restoreGrants(ctx)
	}()
	if opts.Notify != nil {
		d.notes = newNotifier(d, opts.Notify)
		go d.notes.run(ctx)
	}
	// Supervision revokes grants: it must not race with their restoration.
	select {
	case <-d.restored:
	case <-ctx.Done():
		return d.shutdown()
	}
	d.supervise(ctx)

	ticker := time.NewTicker(opts.Tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return d.shutdown()
		case <-d.stop:
			return d.shutdown()
		case <-d.kick:
			d.supervise(ctx)
		case <-ticker.C:
			if d.supervise(ctx) {
				slog.Info("ohd idle, stopping", "idle_after", opts.IdleAfter)
				return d.shutdown()
			}
		}
	}
}

func (d *Daemon) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	d.feed.close() // ends the live streams, so that Shutdown does not wait for them
	if d.mcp != nil {
		d.mcp.Close()
	}
	if d.http != nil {
		_ = d.http.Shutdown(ctx)
	}
	_ = os.Remove(d.opts.Paths.Socket())
	return nil
}

func (d *Daemon) requestStop() { d.stopOnce.Do(func() { close(d.stop) }) }

// wake schedules an immediate supervision pass (e.g. a server was registered).
func (d *Daemon) wake() {
	select {
	case d.kick <- struct{}{}:
	default:
	}
}

// startProxy binds the proxy on the port saved by a previous daemon: running
// tool servers have that URL in their configuration.
func (d *Daemon) startProxy() error {
	var st stateFile
	if data, err := os.ReadFile(d.opts.Paths.State()); err == nil {
		_ = json.Unmarshal(data, &st)
	}
	restored := false
	if st.ProxyPort > 0 {
		if err := d.proxy.Start(fmt.Sprintf("127.0.0.1:%d", st.ProxyPort)); err == nil {
			restored = true
		} else {
			slog.Warn("ohd: previous proxy port unavailable, picking a new one", "port", st.ProxyPort)
		}
	}
	if !restored {
		if err := d.proxy.Start("127.0.0.1:0"); err != nil {
			return err
		}
	}
	for _, h := range st.ListenHosts {
		if _, err := d.proxy.Listen(h); err != nil {
			slog.Warn("ohd: cannot restore proxy listener", "host", h, "error", err)
		}
	}
	if restored {
		return nil
	}
	return d.saveState()
}

// saveState persists the proxy port and additional listen hosts.
func (d *Daemon) saveState() error {
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(d.proxy.URL(), "http://"))
	p, _ := strconv.Atoi(port)
	st := stateFile{ProxyPort: p}
	for h := range d.proxy.Listeners() {
		st.ListenHosts = append(st.ListenHosts, h)
	}
	sort.Strings(st.ListenHosts)
	data, _ := json.Marshal(st)
	return os.WriteFile(d.opts.Paths.State(), data, 0o600)
}

// proxyCloseWait bounds the proxy shutdown (in-flight LLM streams).
const proxyCloseWait = 5 * time.Second

// orphanGrace keeps a recent grant whose server row is not written yet (a
// client is starting the server).
const orphanGrace = 2 * time.Minute

// restoreGrants re-registers persisted grants. Secrets are re-read from the
// secret store (or AWS profiles); grants whose secret is unavailable stay
// pending until a client provides it. Orphan grants (no live server holds
// them) are revoked instead.
func (d *Daemon) restoreGrants(ctx context.Context) {
	if d.opts.Grants == nil {
		return
	}
	grants, err := d.opts.Grants.ListActive(ctx)
	if err != nil {
		slog.Warn("ohd: cannot list grants", "error", err)
		return
	}
	for _, g := range grants {
		if d.orphanGrant(ctx, g) {
			_ = d.opts.Grants.Revoke(ctx, g.Token, time.Now())
			slog.Info("ohd: orphan grant revoked", "owner", g.Owner)
			continue
		}
		secret := ""
		if g.Source.KeychainKey != "" && d.opts.Secrets != nil {
			secret, _ = d.opts.Secrets.Get(ctx, g.Source.KeychainKey)
		}
		if err := d.register(ctx, g, secret); err != nil {
			d.mu.Lock()
			d.pending[g.Token] = g
			d.mu.Unlock()
			slog.Info("ohd: grant pending its secret", "owner", g.Owner, "reason", err)
		}
	}
}

// orphanGrant reports whether no server can hold a persisted grant: its
// group has no server row, is stopped or asleep, or holds another token.
func (d *Daemon) orphanGrant(ctx context.Context, g domain.ProxyGrant) bool {
	if d.opts.Servers == nil || time.Since(g.CreatedAt) < orphanGrace {
		return false
	}
	srv, err := d.opts.Servers.Get(ctx, g.Owner)
	if errors.Is(err, domain.ErrNotFound) {
		return true
	}
	if err != nil {
		return false
	}
	if srv.Status == domain.ServerStopped || srv.Status == domain.ServerSleeping {
		return true
	}
	return srv.ProxyToken != "" && srv.ProxyToken != g.Token
}

// sigV4 returns the AWS signer of a profile/region, built once (the AWS
// config and credentials are loaded once for all the grants using them).
func (d *Daemon) sigV4(ctx context.Context, profile, region string) (credproxy.Auth, error) {
	key := profile + "\x00" + region
	d.sigMu.Lock()
	defer d.sigMu.Unlock()
	if a, ok := d.sigCache[key]; ok {
		return a, nil
	}
	a, err := d.opts.SigV4(ctx, profile, region)
	if err != nil {
		return nil, err
	}
	d.sigCache[key] = a
	return a, nil
}

// register builds the upstream auth for a grant and installs it in the proxy.
func (d *Daemon) register(ctx context.Context, g domain.ProxyGrant, secret string) error {
	var auth credproxy.Auth
	switch g.Source.Kind {
	case domain.CredentialSigV4:
		a, err := d.sigV4(ctx, g.Source.Profile, g.Region)
		if err != nil {
			return err
		}
		auth = a
	case domain.CredentialBearer, domain.CredentialAPIKey:
		if secret == "" {
			return errors.New("secret unavailable")
		}
		auth = credproxy.BearerAuth{Token: secret}
		if g.Provider == credproxy.ProviderAnthropic {
			auth = credproxy.HeaderAuth{Name: "x-api-key", Value: secret}
		}
	default:
		return fmt.Errorf("unknown credential kind %q", g.Source.Kind)
	}
	up, err := upstreamFor(g.Provider, g.Region, auth)
	if err != nil {
		return err
	}
	return d.proxy.IssueWithToken(g.Token, credproxy.Grant{
		SessionID: g.Owner, Provider: g.Provider, Upstream: up,
		AllowedModels: g.AllowedModels, MaxTokens: g.MaxTokens,
	})
}

func upstreamFor(provider, region string, auth credproxy.Auth) (credproxy.Upstream, error) {
	switch provider {
	case credproxy.ProviderBedrock:
		return credproxy.BedrockUpstream(region, auth), nil
	case credproxy.ProviderAnthropic:
		return credproxy.Upstream{BaseURL: "https://api.anthropic.com/v1", Auth: auth}, nil
	case credproxy.ProviderOpenRouter:
		return credproxy.Upstream{BaseURL: "https://openrouter.ai/api/v1", Auth: auth}, nil
	case credproxy.ProviderOpenAI:
		return credproxy.Upstream{BaseURL: "https://api.openai.com/v1", Auth: auth}, nil
	}
	return credproxy.Upstream{}, fmt.Errorf("unsupported provider %q", provider)
}

// supervise marks dead servers stopped, revokes their grants, and reports
// whether the daemon has been idle long enough to exit.
func (d *Daemon) supervise(ctx context.Context) bool {
	live := 0
	var ready []domain.Server
	if d.opts.Servers != nil {
		servers, err := d.opts.Servers.List(ctx)
		if err == nil {
			for _, s := range servers {
				if s.Status == domain.ServerStopped || s.Status == domain.ServerSleeping {
					continue
				}
				if s.Status == domain.ServerStarting && time.Since(s.CreatedAt) < 2*time.Minute {
					live++ // being started by a client: never touch it
					continue
				}
				if s.PID > 0 && processAlive(s.PID) && d.isOurServer(ctx, s) {
					live++
					if s.Status == domain.ServerReady {
						ready = append(ready, s)
					}
					continue
				}
				// Dead, or the PID now belongs to another process (reboot): mark
				// stopped without signalling it, only if the row is unchanged.
				if ok, _ := d.opts.Servers.SetStatusIf(ctx, s.GroupKey, s.PID, s.Status, domain.ServerStopped); ok {
					d.revokeOwner(ctx, s.GroupKey)
				}
			}
		}
	}
	d.syncWatchers(ctx, ready)
	d.applyLifecycle(ctx, ready)
	d.notes.requestScan() // decisions raised by other processes (CLI, TUI)
	d.mu.Lock()
	defer d.mu.Unlock()
	if live > 0 {
		d.lastBusy = time.Now()
		return false
	}
	return time.Since(d.lastBusy) > d.opts.IdleAfter
}

// isOurServer checks that a live PID is really the tool server (authenticated
// API call), so that a reused PID (after a reboot) is never mistaken for it
// nor signalled. Success is cached per (group, pid); a server is declared
// foreign only after several consecutive failures (a busy server may be slow).
// verifyFailLimit is the number of failed checks before a live PID is
// considered foreign.
const verifyFailLimit = 3

func (d *Daemon) isOurServer(ctx context.Context, s domain.Server) bool {
	key := s.GroupKey + "#" + strconv.Itoa(s.PID)
	d.mu.Lock()
	ok := d.verified[key]
	d.mu.Unlock()
	if ok || d.opts.Adapter == nil {
		return true
	}
	ad := d.opts.Adapter(s.Adapter)
	if ad == nil {
		return true
	}
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := ad.ActiveSessions(cctx, adapters.ServerHandle{URL: s.URL, Password: s.Password, PID: s.PID})
	d.mu.Lock()
	defer d.mu.Unlock()
	if err == nil {
		d.verified[key] = true
		delete(d.verifyFails, key)
		return true
	}
	d.verifyFails[key]++
	return d.verifyFails[key] < verifyFailLimit
}

func (d *Daemon) revokeOwner(ctx context.Context, owner string) {
	d.proxy.RevokeSession(owner)
	if d.gateway != nil {
		d.gateway.RevokeOwner(owner)
	}
	if d.mcp != nil {
		d.mcp.StopGroup(owner)
	}
	if d.opts.Grants != nil {
		_ = d.opts.Grants.RevokeOwner(ctx, owner, time.Now())
	}
	d.mu.Lock()
	for t, g := range d.pending {
		if g.Owner == owner {
			delete(d.pending, t)
		}
	}
	d.mu.Unlock()
}

func (d *Daemon) liveServers(ctx context.Context) int {
	if d.opts.Servers == nil {
		return 0
	}
	servers, err := d.opts.Servers.List(ctx)
	if err != nil {
		return 0
	}
	n := 0
	for _, s := range servers {
		if s.Status != domain.ServerStopped && s.Status != domain.ServerSleeping && s.PID > 0 && processAlive(s.PID) {
			n++
		}
	}
	return n
}
