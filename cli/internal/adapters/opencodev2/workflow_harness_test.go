//go:build integration

package opencodev2

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// Workflow harness (phase 3, checkpoints): a real oh binary for the
// `workflow` MCP server, an isolated OH_HOME, and an in-process oh daemon
// serving the CheckpointService.

var (
	ohBinOnce sync.Once
	ohBinPath string
	ohBinErr  error
)

// buildOh compiles the oh binary once per test run.
func buildOh(t *testing.T) string {
	t.Helper()
	ohBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "ohbin-")
		if err != nil {
			ohBinErr = err
			return
		}
		ohBinPath = filepath.Join(dir, "oh")
		cmd := exec.Command("go", "build", "-o", ohBinPath, ".")
		cmd.Dir = filepath.Join("..", "..", "..")
		if out, err := cmd.CombinedOutput(); err != nil {
			ohBinErr = &buildError{out: string(out), err: err}
		}
	})
	require.NoError(t, ohBinErr)
	return ohBinPath
}

type buildError struct {
	out string
	err error
}

func (e *buildError) Error() string { return e.err.Error() + "\n" + e.out }

type workflowEnv struct {
	home      string // OH_HOME (short: it holds the daemon socket)
	ohBin     string
	bundles   string
	sessions  domain.SessionStore
	servers   domain.ServerStore
	decisions domain.DecisionStore
	cp        *checkpoint.Service
	client    *daemon.Client
	// env is added to the environment of the servers (shell of the sessions).
	env map[string]string
}

// newWorkflowEnv starts an oh daemon on an isolated OH_HOME, watching the
// servers of adapter a.
func newWorkflowEnv(t *testing.T, a *Adapter) *workflowEnv {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "ohw-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(home) })
	e := &workflowEnv{home: home, ohBin: buildOh(t), bundles: filepath.Join(home, "bundles")}
	st, err := sqlite.Open(filepath.Join(home, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	e.sessions, e.servers = sqlite.NewSessionStore(st), sqlite.NewServerStore(st)
	e.decisions = sqlite.NewDecisionStore(st)
	e.cp = &checkpoint.Service{Sessions: e.sessions, States: sqlite.NewCheckpointStore(st), SessionOutputs: sqlite.NewCheckpointStore(st), BundlesDir: e.bundles, SessionsDir: filepath.Join(home, "sessions")}

	paths := daemon.Paths{Dir: filepath.Join(home, "run")}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- daemon.Run(ctx, daemon.Options{Paths: paths, Version: "test", Servers: e.servers, Sessions: e.sessions,
			Decisions: e.decisions, Checkpoints: e.cp, Tick: 200 * time.Millisecond, IdleAfter: time.Hour, IdleSleep: time.Hour,
			Adapter: func(name string) adapters.ToolAdapter {
				if name == Name {
					return a
				}
				return nil
			}})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	})
	e.client = daemon.NewClient(paths)
	require.Eventually(t, func() bool { _, err := e.client.Health(context.Background()); return err == nil }, 5*time.Second, 20*time.Millisecond)
	return e
}

// saveBundle writes bundle.json so that the CheckpointService finds the
// workflow runtime of the sessions using b.
func (e *workflowEnv) saveBundle(t *testing.T, b sessionspec.BundleSpec) {
	t.Helper()
	require.NotEmpty(t, b.Hash)
	dir := filepath.Join(e.bundles, b.Hash)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	data, err := json.Marshal(b)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bundle.json"), data, 0o644))
}

// workflowBundle is the contract bundle with the workflow MCP server.
func workflowBundle(t *testing.T, wf sessionspec.WorkflowRuntime) sessionspec.BundleSpec {
	t.Helper()
	b := contractBundle(t, t.TempDir())
	b.Hash = "h_" + wf.ID
	b.Workflow = &wf
	b.MCP = []sessionspec.MCPServerDef{sessionspec.WorkflowMCPDef()}
	return b
}

// startServer starts a server for b whose MCP servers run the test oh
// binary against this environment.
func (e *workflowEnv) startServer(t *testing.T, a *Adapter, b sessionspec.BundleSpec, prov sessionspec.ProviderSpec) (adapters.ServerHandle, string) {
	t.Helper()
	a.OhBinary = e.ohBin
	root := t.TempDir()
	project := filepath.Join(root, "proj")
	require.NoError(t, os.MkdirAll(project, 0o755))
	require.NoError(t, exec.Command("git", "init", "-q", project).Run())
	env := map[string]string{"OH_HOME": e.home}
	for k, v := range e.env {
		env[k] = v
	}
	h, err := a.StartServer(context.Background(), adapters.ServerGroup{
		Bundle: b, Provider: prov, DataDir: filepath.Join(root, "data"), WorkDir: project, Env: env,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.StopServer(context.Background(), h) })
	return h, project
}

// startGroup starts a server for b through the daemon's credential proxy
// (the oh plugin reaches the daemon hooks there) and registers it as group
// "g1", watched by the daemon.
func (e *workflowEnv) startGroup(t *testing.T, a *Adapter, b sessionspec.BundleSpec, bedrockToken string) (adapters.ServerHandle, string) {
	t.Helper()
	ctx := context.Background()
	g, err := e.client.IssueGrant(ctx, daemon.GrantRequest{Owner: "g1", Provider: "amazon-bedrock", Region: "eu-west-1",
		Source: domain.CredentialSource{Kind: domain.CredentialBearer}, Secret: bedrockToken})
	require.NoError(t, err)
	h, project := e.startServer(t, a, b, sessionspec.ProviderSpec{ID: "amazon-bedrock", Region: "eu-west-1", BaseURL: g.BaseURL, SessionToken: g.Token})
	require.NoError(t, e.servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: Name, ProjectID: "p1", BundleHash: b.Hash,
		PID: h.PID, URL: h.URL, Password: h.Password, ProxyTokenHash: credproxy.TokenHash(g.Token), Status: domain.ServerReady, CreatedAt: time.Now()}))
	return h, project
}

// waitMCPConnected waits until the server reports the MCP server connected.
func waitMCPConnected(t *testing.T, c *Client, dir, name string) {
	t.Helper()
	last := "not listed"
	require.Eventually(t, func() bool {
		list, err := c.MCP(context.Background(), dir)
		if err != nil {
			return false
		}
		for _, m := range list {
			if m.Name == name {
				last = m.Status.Status
				return last == "connected"
			}
		}
		return false
	}, 30*time.Second, 200*time.Millisecond, "MCP server %s: %s", name, last)
}
