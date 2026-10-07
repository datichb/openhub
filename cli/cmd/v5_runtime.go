package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/buildinfo"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/filelock"
	"github.com/datichb/openhub/cli/internal/runsvc"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/runtime/container"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/keychain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// v5 runtime wiring: tool adapter, oh daemon, RunService.

func ohRunDir() string      { return filepath.Join(config.HubDir(), "run") }
func ohServersDir() string  { return filepath.Join(config.HubDir(), "servers") }
func ohSessionsDir() string { return filepath.Join(config.HubDir(), "sessions") }
func ohCacheDir() string    { return filepath.Join(config.HubDir(), "cache") }

// ensureDaemon returns a client to ohd, starting it when needed. The client
// holds the issuing capability (token routes, M12).
func ensureDaemon(ctx context.Context) (*daemon.Client, daemon.Health, error) {
	capability, _, err := daemonCapability(ctx)
	if err != nil {
		return nil, daemon.Health{}, err
	}
	return daemon.Ensure(ctx, daemon.Paths{Dir: ohRunDir()}, daemon.EnsureOptions{Version: buildinfo.Version, Capability: capability,
		InProcess: startInProcessDaemon})
}

// daemonCapability returns the issuing capability shared with the daemon:
// in the OS keychain, else in a 0600 file under ~/.oh/run. The encrypted
// file store is not used (it would prompt for a passphrase in the daemon).
func daemonCapability(ctx context.Context) (string, daemon.CapabilitySource, error) {
	return daemon.LoadCapability(ctx, capabilityStore(), daemon.Paths{Dir: ohRunDir()})
}

// capabilityStore is the OS keychain when items can be stored there
// without a system dialog (nil otherwise: 0600 file).
func capabilityStore() daemon.CapabilityStore {
	if keychain.Probe() != nil || !keychain.HasDefault() {
		return nil
	}
	return keychain.New(config.HubDir())
}

// detectV2Adapter returns the opencode V2 adapter, or an error when the
// installed opencode is not a V2 release.
func detectV2Adapter(ctx context.Context) (*opencodev2.Adapter, error) {
	a := opencodev2.New("", ohCacheDir())
	if _, err := a.Detect(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

// sessionEndHook emits the team session.complete event of a v5 session (the
// session row is closed by the RunService). async=false waits for the
// write (short-lived CLI commands).
func sessionEndHook(a *app.App, async bool) func(context.Context, domain.Session) {
	return func(ctx context.Context, s domain.Session) {
		if a == nil || a.Projects == nil || s.ProjectID == "" {
			return
		}
		proj, err := a.Projects.Get(ctx, s.ProjectID)
		if err != nil {
			return
		}
		resolved := config.ResolveTeamForProject(a.Config, proj)
		if !resolved.Enabled || resolved.MemberID == "" || resolved.StateRepo == "" {
			return
		}
		repo := teamstate.NewRepo(resolved.StateRepo, resolved.StatePath)
		if !repo.IsCloned() {
			return
		}
		duration := 0.0
		if s.EndedAt != nil {
			duration = s.EndedAt.Sub(s.StartedAt).Seconds()
		}
		event := teamstate.NewSessionCompleteEvent(resolved.MemberID, s.ProjectID, map[string]interface{}{
			"session_id": s.ID,
			"duration_s": duration,
			"tokens_in":  s.TokensIn,
			"tokens_out": s.TokensOut,
			"provider":   s.Provider,
			"model":      s.Model,
			"status":     string(s.Status),
		})
		if s.Title != nil && *s.Title != "" {
			event.Data["summary"] = *s.Title
		}
		if async {
			repo.AppendEventAsync(event)
			return
		}
		wctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := repo.AppendEvent(wctx, event); err != nil {
			slog.Warn("team session.complete event failed", "session", s.ID, "error", err)
		}
	}
}

// v5Runtimes are the non-local execution environments, configured by the
// Settings › Exécution (`[execution]` of hub.toml): engine, image cache,
// pinned tool version.
func v5Runtimes(a *app.App) map[sessionspec.RuntimeKind]ohruntime.Runtime {
	ex := executionConfig(a)
	var rt ohruntime.Runtime = v5ContainerRuntime(a)
	if v5Adapter != nil {
		rt = pinRuntime(rt, ex.OpencodeVersion, v5Adapter.Ver)
	}
	return map[sessionspec.RuntimeKind]ohruntime.Runtime{sessionspec.RuntimeContainer: rt}
}

// executionConfig returns the Settings › Exécution of the app.
func executionConfig(a *app.App) config.ExecutionConfig {
	if a != nil && a.Config != nil {
		return a.Config.Execution
	}
	return config.ExecutionConfig{}
}

// v5ContainerRuntime is the container runtime configured by the Settings
// (engine, image cache), without the pinned version check (Doctor).
func v5ContainerRuntime(a *app.App) *container.Runtime {
	ex := executionConfig(a)
	engine, _ := container.ParseEngine(ex.Engine)
	return container.New(container.Options{Engine: engine, KeepImages: ex.Images(),
		CacheDir: filepath.Join(ohCacheDir(), "container")})
}

// newRunService wires the RunService for the current app.
func newRunService(ctx context.Context, a *app.App) (*runsvc.Service, error) {
	if store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if !v5Available(ctx) {
		return nil, fmt.Errorf("opencode V2 is required for v5 sessions: %v", v5Err)
	}
	ad := v5Adapter
	shellPath, err := localShellPath()
	if err != nil {
		return nil, err
	}
	return &runsvc.Service{
		Adapter:        ad,
		AdapterVer:     ad.Ver,
		Servers:        sqlite.NewServerStore(store),
		Sessions:       a.Sessions,
		Secrets:        a.Secrets,
		ServersDir:     ohServersDir(),
		BundlesDir:     ohBundlesDir(),
		SessionsDir:    ohSessionsDir(),
		Decisions:      sqlite.NewDecisionStore(store),
		Usage:          sqlite.NewUsageStore(store),
		OnSessionEnd:   sessionEndHook(a, false),
		Runtimes:       v5Runtimes(a),
		LocalShellPath: shellPath,
		SessionEnv: gatewaySessionEnv(func(ctx context.Context) (gatewayGranter, error) {
			c, _, err := ensureDaemon(ctx)
			return c, err
		}),
		Daemon: func(ctx context.Context) (runsvc.DaemonClient, error) {
			c, _, err := ensureDaemon(ctx)
			if err != nil {
				return nil, err
			}
			return c, nil
		},
	}, nil
}

// newSessionService wires the SessionService (inbox, decisions, instructions,
// results, live follow-up). Without opencode V2, only the stored state is
// available.
func newSessionService(ctx context.Context, a *app.App) (*sessionsvc.Service, error) {
	if store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	dc := daemon.NewClient(daemon.Paths{Dir: ohRunDir()})
	svc := &sessionsvc.Service{
		Sessions:    a.Sessions,
		Projects:    a.Projects,
		Decisions:   sqlite.NewDecisionStore(store),
		Servers:     sqlite.NewServerStore(store),
		BundlesDir:  ohBundlesDir(),
		SessionsDir: ohSessionsDir(),
		Alive:       filelock.ProcessAlive,
		Live:        dc.Stream,
	}
	if v5Available(ctx) {
		svc.Adapter = v5Adapter
		ensureDaemonForLiveServers(ctx, svc.Servers)
	}
	svc.UseCheckpoints(newCheckpointService(a), dc.WorkflowRefresh)
	svc.Resolvers[domain.DecisionBudget] = sessionsvc.BudgetResolver(sqlite.NewUsageStore(store), func(ctx context.Context, id string) error {
		rs, err := newRunService(ctx, a)
		if err != nil {
			return err
		}
		return rs.StopSession(ctx, id)
	})
	return svc, nil
}

// newCheckpointService wires the CheckpointService (workflow state of the
// sessions; the daemon feeds it, the CLI and the TUI answer its decisions).
func newCheckpointService(a *app.App) *checkpoint.Service {
	cs := sqlite.NewCheckpointStore(store)
	return &checkpoint.Service{Sessions: a.Sessions, States: cs, SessionOutputs: cs, BundlesDir: ohBundlesDir(), SessionsDir: ohSessionsDir()}
}

// ensureDaemonForLiveServers starts the oh daemon when a tool server runs
// without it (daemon stopped or crashed): sessions are only tracked, and
// their decisions recorded, while the daemon runs.
func ensureDaemonForLiveServers(ctx context.Context, servers domain.ServerStore) {
	if _, err := daemon.NewClient(daemon.Paths{Dir: ohRunDir()}).Health(ctx); err == nil {
		return
	}
	list, err := servers.List(ctx)
	if err != nil {
		return
	}
	for _, s := range list {
		if s.Status == domain.ServerReady && filelock.ProcessAlive(s.PID) {
			if _, _, err := ensureDaemon(ctx); err != nil {
				slog.Debug("oh daemon not started", "error", err)
			}
			return
		}
	}
}
