package cmd

import (
	"context"
	"fmt"
	"github.com/datichb/openhub/cli/internal/services/fanout"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	"github.com/datichb/openhub/cli/internal/services/report"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/services/wiki"
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
	var gcCfg config.GCConfig
	if c, err := config.Load(); err == nil && c != nil {
		gcCfg = c.GC
	}
	return daemon.Ensure(ctx, daemon.Paths{Dir: ohRunDir()}, daemon.EnsureOptions{Version: buildinfo.Version, Capability: capability,
		InProcess: startInProcessDaemon, LogMaxBytes: gcCfg.LogMaxBytes(), LogKeep: gcCfg.Logs()})
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

// sessionEndHook emits the team session.complete event of a v5 session and
// ends the team claims of its tickets (the session row is closed by the
// RunService). async=false waits for the
// write (short-lived CLI commands).
func sessionEndHook(a *app.App, async bool) func(context.Context, domain.Session) {
	return func(ctx context.Context, s domain.Session) {
		if a == nil || a.Projects == nil || s.ProjectID == "" {
			return
		}
		// The daemon writes the team-state in the background: it waits
		// longer for the lock of the clone (N15).
		ctx = teamstate.WithLockWait(ctx, teamstate.BackgroundLockWait)
		proj, err := a.Projects.Get(ctx, s.ProjectID)
		if err != nil {
			return
		}
		sessionClaimsEnd(ctx, a, proj, s, async)
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
		wctx, cancel := context.WithTimeout(teamstate.WithLockWait(context.Background(), teamstate.BackgroundLockWait), teamstate.BackgroundLockWait+30*time.Second)
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
		rt = pinRuntime(rt, ex.ToolVersion, v5Tool.Version)
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
// (engine, Colima profile, image cache), without the pinned version check (Doctor).
func v5ContainerRuntime(a *app.App) *container.Runtime {
	ex := executionConfig(a)
	engine, _ := container.ParseEngine(ex.Engine)
	profile := ex.ColimaProfile
	if profile == "" {
		profile = os.Getenv("COLIMA_PROFILE")
	}
	return container.New(container.Options{Engine: engine, ColimaProfile: profile, KeepImages: ex.Images(),
		CacheDir: filepath.Join(ohCacheDir(), "container")})
}

// newRunService wires the RunService for the current app.
func newRunService(ctx context.Context, a *app.App) (*runsvc.Service, error) {
	if store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if !v5Available(ctx) {
		return nil, v2Unsupported(v5Err)
	}
	ad := v5Adapter
	shellPath, err := localShellPath()
	if err != nil {
		return nil, err
	}
	shellEnv, err := localShellStartup(shellPath)
	if err != nil {
		return nil, err
	}
	svc := &runsvc.Service{
		Adapter:        ad,
		AdapterVer:     v5Tool.Version,
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
		LocalShellEnv:  shellEnv,
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
	}
	svc.WakeSession = func(ctx context.Context, id string) error { return resumeV5Session(ctx, a, svc, id) }
	return svc, nil
}

// newSessionService wires the SessionService (inbox, decisions, instructions,
// results, live follow-up). Without a supported tool, only the stored state is
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
		Reports:     newReportService(),
		Wiki:        newWikiService(),
		Alive:       filelock.ProcessAlive,
		Live:        dc.Stream,
	}
	if v5Available(ctx) {
		svc.Adapter = v5Adapter
		ensureDaemonForLiveServers(ctx, svc.Servers)
	}
	svc.UseCheckpoints(newCheckpointService(a), func(ctx context.Context, id string) error {
		capability, _, err := daemonCapability(ctx) // privileged route (N1)
		if err != nil {
			return err
		}
		return dc.WithCapability(capability).WorkflowRefresh(ctx, id)
	})
	svc.OhDecided = dc.SessionDecided
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

// newWikiService wires the WikiService (proposals of the wiki MCP server,
// LD-J).
func newWikiService() *wiki.Service {
	return &wiki.Service{Store: sqlite.NewWikiProposalStore(store), SessionsDir: ohSessionsDir()}
}

// daemonCheckpoints is the CheckpointService of the daemon: at the commit
// checkpoint, oh applies the wiki proposals accepted on the card (LD-J).
func daemonCheckpoints(a *app.App, wk *wiki.Service) *checkpoint.Service {
	cp := newCheckpointService(a)
	cp.CommitPassed = wk.CommitPassed
	// R6: at the merge checkpoint of a fan-out, oh merges the integration
	// branch and closes the tickets merged.
	in := &fanout.Integrator{Sessions: sqlite.NewSessionStore(store), Record: func(ctx context.Context, id string, tickets []string) {
		if _, err := cp.Record(ctx, id, checkpoint.RecordClosed, tickets...); err != nil {
			slog.Warn("fan-out: closed tickets not recorded", "session", id, "error", err)
		}
	}}
	cp.Integrate = in.Integrate
	return cp
}

// envWorkflowReports set to "off" makes the daemon keep no report
// (workflow_report answers "not recorded": evaluation of the fallback).
const envWorkflowReports = "OH_WORKFLOW_REPORTS"

// newReportService wires the ReportService (reports of workflow_report).
func newReportService() *report.Service {
	return &report.Service{Store: sqlite.NewReportStore(store), SessionsDir: ohSessionsDir()}
}

// daemonReports is the ReportService of the daemon (nil when turned off).
func daemonReports() *report.Service {
	if strings.EqualFold(os.Getenv(envWorkflowReports), "off") {
		return nil
	}
	return newReportService()
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
