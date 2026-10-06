//go:build container

package runsvc

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway"
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/mcp/protocol"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/runtime/container"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// TestMain runs the test binary as a stdio MCP server standing for an oh
// MCP server on the machine (MCP gateway).
func TestMain(m *testing.M) {
	if os.Getenv("OH_RUNSVC_TEST_MCP") == "1" {
		s := protocol.NewServer("oh-test", "1")
		s.RegisterTool(protocol.Tool{Name: "ping", Description: "ping", InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
			func(context.Context, json.RawMessage) (*protocol.ToolResult, error) {
				return &protocol.ToolResult{Content: []protocol.ContentBlock{{Type: "text", Text: "pong"}}}, nil
			})
		_ = s.Serve()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// TestRealBeadsGateway runs bd from the shell of a container session (no
// LLM call): reads go through the Beads gateway to the real bd of the
// machine, commands outside beads.allow are refused, and the container
// environment holds no secret.
func TestRealBeadsGateway(t *testing.T) {
	for _, bin := range []string{"opencode", "bd"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s binary not found", bin)
		}
	}
	for _, k := range []container.EngineKind{container.EngineColima, container.EnginePodman} {
		t.Run(string(k), func(t *testing.T) { realBeadsGateway(t, k) })
	}
}

func realBeadsGateway(t *testing.T, engine container.EngineKind) {
	ctx := context.Background()
	rt := container.New(container.Options{Engine: engine, CacheDir: t.TempDir()})
	if av, _ := rt.Available(ctx); !av.OK {
		t.Skipf("%s unavailable: %s", engine, av.Message())
	}
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".cache", "oh-container-tests")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(base, "gw-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	short, err := os.MkdirTemp("/tmp", "ohg-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(short) })

	project := filepath.Join(root, "proj")
	sh := func(dir string, name string, args ...string) string {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
		return string(out)
	}
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	sh(project, "git", "init", "-q")
	sh(project, "bd", "init", "--quiet", "--prefix", "gw")
	var issue struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(sh(project, "bd", "create", "created on the machine", "--json")), &issue); err != nil || issue.ID == "" {
		t.Fatalf("bd create on the machine: %v", err)
	}

	st, err := sqlite.Open(filepath.Join(short, "oh.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if _, err := st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1', 'p1', ?)`, project); err != nil {
		t.Fatal(err)
	}
	servers, sessions := sqlite.NewServerStore(st), sqlite.NewSessionStore(st)

	a := opencodev2.New("", filepath.Join(home, "Library", "Caches", "oh-container-tests"))
	if _, err := a.Detect(ctx); err != nil {
		t.Fatal(err)
	}
	secrets := realSecrets{"openhub.provider.bedrock.token": "real-secret-must-not-leak"}
	paths := daemon.Paths{Dir: filepath.Join(short, "run")}
	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- daemon.Run(dctx, daemon.Options{Paths: paths, Version: "test", Grants: sqlite.NewGrantStore(st), Servers: servers,
			Secrets: secrets, Tick: time.Second, Sessions: sessions,
			Adapter: func(string) adapters.ToolAdapter { return a },
			MCPCommand: func(_ context.Context, srv domain.Server, name string) (gateway.MCPCommand, error) {
				exe, err := os.Executable()
				return gateway.MCPCommand{Argv: []string{exe}, Dir: srv.WorkDir,
					Env: map[string]string{"OH_RUNSVC_TEST_MCP": "1", "OH_TEST_SERVICE_TOKEN": "service-secret-must-not-leak"}}, err
			},
			GatewayView: func(ctx context.Context, group string) (gateway.View, error) {
				srv, err := servers.Get(ctx, group)
				if err != nil {
					return gateway.View{}, err
				}
				spec, err := container.LoadSpec(srv.DataDir)
				if err != nil {
					return gateway.View{}, err
				}
				return gateway.View{Paths: spec.Paths, Locations: spec.Locations}, nil
			}})
	}()
	t.Cleanup(func() { cancel(); <-done })
	dc := daemon.NewClient(paths)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := dc.Health(ctx); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("daemon not ready")
		}
		time.Sleep(20 * time.Millisecond)
	}

	model := sessionspec.ParseModelRef("amazon-bedrock/eu.anthropic.claude-haiku-4-5-20251001-v1:0")
	bdir := filepath.Join(root, "bundle")
	if err := os.MkdirAll(filepath.Join(bdir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	b := &bundle.Bundle{Dir: bdir, Spec: sessionspec.BundleSpec{
		Hash: "gatewaybundle01", Root: bdir, EntryAgent: "lead",
		Agents:    []sessionspec.AgentDef{{ID: "lead", Description: "lead", Mode: "primary", Body: "You are LEAD."}},
		SkillsDir: filepath.Join(bdir, "skills"), MaxDepth: 1, DefaultModel: &model,
		MCP: []sessionspec.MCPServerDef{{Name: "team", Type: "local", Command: []string{"/usr/local/bin/oh", "mcp", "serve", "team"}}},
	}}
	svc := &Service{
		Adapter: a, AdapterVer: a.Ver, Servers: servers, Sessions: sessions, SessionsDir: filepath.Join(root, "sessions"),
		Secrets: secrets, ServersDir: filepath.Join(root, "servers"), Executable: "/usr/local/bin/oh",
		Daemon:   func(context.Context) (DaemonClient, error) { return dc, nil },
		Runtimes: map[sessionspec.RuntimeKind]ohruntime.Runtime{sessionspec.RuntimeContainer: rt},
		SessionEnv: func(ctx context.Context, r SessionEnvRequest) (map[string]string, error) {
			if r.Runtime != sessionspec.RuntimeContainer {
				return nil, nil
			}
			g, err := dc.IssueGatewayGrant(ctx, daemon.GatewayGrantRequest{SessionID: r.SessionID, GroupKey: r.GroupKey,
				ProjectID: r.ProjectID, Location: r.Location, BeadsAllow: r.BeadsAllow, GatewayURL: r.GatewayURL})
			return map[string]string{beadswire.EnvURL: r.GatewayURL, beadswire.EnvToken: g.Token}, err
		},
	}
	var img string
	t.Cleanup(func() {
		if list, err := svc.Servers.List(context.Background()); err == nil {
			for _, s := range list {
				svc.stopServer(context.Background(), &s)
			}
		}
		e, _ := rt.Engine(context.Background())
		if img != "" {
			_, _ = container.ExecRunner{}.Run(context.Background(), e.CLI, e.Command("rmi", img)[1:]...)
		}
	})

	res, err := svc.StartSession(ctx, StartRequest{
		ProjectID: "p1", Location: project, Bundle: b, Title: "gateway", Provider: "bedrock",
		ProviderCfg: providerCfg(), Attach: sessionspec.AttachNone, Runtime: sessionspec.RuntimeContainer,
		BeadsAllow: []string{"show", "list"}, Progress: func(l string) { t.Log(l) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec, err := container.LoadSpec(res.Server.DataDir); err == nil {
		img = spec.Image
	}

	c := opencodev2.NewClient(res.Server.URL, res.Server.Password)
	shell := func(command string) string {
		t.Helper()
		cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		evs, _, err := c.Events(cctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.Shell(cctx, res.SessionID, command); err != nil {
			t.Fatal(err)
		}
		for ev := range evs {
			if ev.Type != "session.shell.ended" || ev.SessionID() != res.SessionID {
				continue
			}
			var d struct {
				Output struct {
					Output string `json:"output"`
				} `json:"output"`
			}
			if err := json.Unmarshal(ev.Data, &d); err != nil {
				t.Fatal(err)
			}
			return d.Output.Output
		}
		t.Fatalf("no output for %q", command)
		return ""
	}

	if out := shell("bd show " + issue.ID + " --json; echo EXIT=$?"); !strings.Contains(out, "created on the machine") || !strings.Contains(out, "EXIT=0") {
		t.Fatalf("bd show through the gateway:\n%s", out)
	}
	if out := shell("cd /tmp && bd list; echo EXIT=$?"); !strings.Contains(out, issue.ID) || !strings.Contains(out, "EXIT=0") {
		t.Fatalf("bd list outside the project directory (runs in the session location):\n%s", out)
	}
	out := shell("bd create 'from the container'; echo EXIT=$?")
	if !strings.Contains(out, "EXIT=1") || !strings.Contains(out, "create") {
		t.Fatalf("bd create must be refused:\n%s", out)
	}
	if list := sh(project, "bd", "list"); strings.Contains(list, "from the container") {
		t.Fatalf("refused command reached Beads:\n%s", list)
	}
	if out := shell("bd --db /tmp/x.db list; echo EXIT=$?"); !strings.Contains(out, "EXIT=1") {
		t.Fatalf("--db must be refused:\n%s", out)
	}
	// The oh MCP server runs on the machine, reached over HTTP (P4-T08).
	var mcpStatus string
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(500 * time.Millisecond) {
		list, err := c.MCP(ctx, "/work/proj")
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range list {
			if m.Name == "team" {
				mcpStatus = m.Status.Status
			}
		}
		if mcpStatus == "connected" {
			break
		}
	}
	if mcpStatus != "connected" {
		t.Fatalf("oh MCP server through the gateway: status %q", mcpStatus)
	}
	spec, err := container.LoadSpec(res.Server.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	e, _ := rt.Engine(ctx)
	procEnv, err := container.ExecRunner{}.Run(ctx, e.CLI, e.Command("exec", spec.Name, "env")[1:]...)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(procEnv), "service-secret-must-not-leak") || strings.Contains(string(procEnv), "mcp\",\"serve") ||
		!strings.Contains(string(procEnv), "/oh/v1/hooks/mcp/team") || !strings.Contains(string(procEnv), "{env:AWS_BEARER_TOKEN_BEDROCK}") {
		t.Fatalf("server environment of the container (MCP declared remote, no service secret):\n%s", procEnv)
	}
	env := shell("env")
	if strings.Contains(env, "real-secret-must-not-leak") {
		t.Fatalf("secret in the session environment:\n%s", env)
	}
	if !strings.Contains(env, beadswire.EnvToken+"="+beadswire.TokenPrefix) || !strings.Contains(env, beadswire.EnvURL+"=http://") {
		t.Fatalf("gateway variables missing:\n%s", env)
	}

	if err := svc.StopSession(ctx, res.SessionID); err != nil {
		t.Fatal(err)
	}
}
