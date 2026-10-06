package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
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
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// v5 runtime wiring: tool adapter, oh daemon, RunService.

func ohRunDir() string      { return filepath.Join(config.HubDir(), "run") }
func ohServersDir() string  { return filepath.Join(config.HubDir(), "servers") }
func ohSessionsDir() string { return filepath.Join(config.HubDir(), "sessions") }
func ohCacheDir() string    { return filepath.Join(config.HubDir(), "cache") }

// ensureDaemon returns a client to ohd, starting it when needed.
func ensureDaemon(ctx context.Context) (*daemon.Client, daemon.Health, error) {
	return daemon.Ensure(ctx, daemon.Paths{Dir: ohRunDir()}, daemon.EnsureOptions{Version: buildinfo.Version})
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
// legacy launcher emits it after the tool exits). async=false waits for the
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

// v5Runtimes are the non-local execution environments. The container
// runtime is not exposed in the CLI/TUI yet (phase 4, lot 4.C); its engine
// can be forced with OH_CONTAINER_ENGINE (auto|colima|podman|docker).
func v5Runtimes() map[sessionspec.RuntimeKind]ohruntime.Runtime {
	engine, _ := container.ParseEngine(os.Getenv("OH_CONTAINER_ENGINE"))
	return map[sessionspec.RuntimeKind]ohruntime.Runtime{
		sessionspec.RuntimeContainer: container.New(container.Options{Engine: engine, CacheDir: filepath.Join(ohCacheDir(), "container")}),
	}
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
	return &runsvc.Service{
		Adapter:      ad,
		AdapterVer:   ad.Ver,
		Servers:      sqlite.NewServerStore(store),
		Sessions:     a.Sessions,
		Secrets:      a.Secrets,
		ServersDir:   ohServersDir(),
		BundlesDir:   ohBundlesDir(),
		SessionsDir:  ohSessionsDir(),
		Decisions:    sqlite.NewDecisionStore(store),
		OnSessionEnd: sessionEndHook(a, false),
		Runtimes:     v5Runtimes(),
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
	return svc, nil
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
