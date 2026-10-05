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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/domain"
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
	// Sessions is updated by the session watchers (run state, cost, tokens).
	Sessions domain.SessionStore
	// Adapter returns the tool adapter for a server's adapter name (nil = no watcher).
	Adapter func(name string) adapters.ToolAdapter
	// SigV4 builds an AWS signer for a profile/region (overridable in tests).
	SigV4 func(ctx context.Context, profile, region string) (credproxy.Auth, error)
}

// Daemon is the running ohd process.
type Daemon struct {
	opts     Options
	proxy    *credproxy.Proxy
	listener net.Listener
	http     *http.Server

	wmu      sync.Mutex
	watchers map[string]*watcher

	mu       sync.Mutex
	clients  map[string]client
	policies map[string]QuitPolicy
	pending  map[string]domain.ProxyGrant
	lastBusy time.Time
	stop     chan struct{}
	stopOnce sync.Once
	kick     chan struct{}
}

type stateFile struct {
	ProxyPort int `json:"proxy_port"`
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

	d := &Daemon{opts: opts, proxy: credproxy.New(), pending: map[string]domain.ProxyGrant{}, clients: map[string]client{}, policies: map[string]QuitPolicy{}, watchers: map[string]*watcher{}, lastBusy: time.Now(), stop: make(chan struct{}), kick: make(chan struct{}, 1)}
	if err := d.startProxy(); err != nil {
		return err
	}
	defer func() { _ = d.proxy.Close(context.Background()) }()
	d.restoreGrants(ctx)

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
	if st.ProxyPort > 0 {
		if err := d.proxy.Start(fmt.Sprintf("127.0.0.1:%d", st.ProxyPort)); err == nil {
			return nil
		}
		slog.Warn("ohd: previous proxy port unavailable, picking a new one", "port", st.ProxyPort)
	}
	if err := d.proxy.Start("127.0.0.1:0"); err != nil {
		return err
	}
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(d.proxy.URL(), "http://"))
	p, _ := strconv.Atoi(port)
	data, _ := json.Marshal(stateFile{ProxyPort: p})
	return os.WriteFile(d.opts.Paths.State(), data, 0o600)
}

// restoreGrants re-registers persisted grants. Secrets are re-read from the
// secret store (or AWS profiles); grants whose secret is unavailable stay
// pending until a client provides it.
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

// register builds the upstream auth for a grant and installs it in the proxy.
func (d *Daemon) register(ctx context.Context, g domain.ProxyGrant, secret string) error {
	var auth credproxy.Auth
	switch g.Source.Kind {
	case domain.CredentialSigV4:
		a, err := d.opts.SigV4(ctx, g.Source.Profile, g.Region)
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
				if s.PID > 0 && processAlive(s.PID) {
					live++
					if s.Status == domain.ServerReady {
						ready = append(ready, s)
					}
					continue
				}
				if s.Status == domain.ServerStarting && time.Since(s.CreatedAt) < 2*time.Minute {
					live++
					continue
				}
				_ = d.opts.Servers.SetStatus(ctx, s.GroupKey, domain.ServerStopped)
				d.revokeOwner(ctx, s.GroupKey)
			}
		}
	}
	d.syncWatchers(ctx, ready)
	d.applyLifecycle(ctx, ready)
	d.mu.Lock()
	defer d.mu.Unlock()
	if live > 0 {
		d.lastBusy = time.Now()
		return false
	}
	return time.Since(d.lastBusy) > d.opts.IdleAfter
}

func (d *Daemon) revokeOwner(ctx context.Context, owner string) {
	d.proxy.RevokeSession(owner)
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
