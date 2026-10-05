//go:build container

package runsvc

import (
	"context"
	"net/url"
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
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/runtime/container"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

type realSecrets map[string]string

func (m realSecrets) Get(_ context.Context, k string) (string, error) {
	if v, ok := m[k]; ok {
		return v, nil
	}
	return "", domain.ErrNotFound
}

// TestRealContainerSession runs the whole v5 launch in a container (no LLM
// call): image, opencode serve in the container, closed-world attestation,
// session creation through the API from the machine, no secret in the
// container environment, credential proxy reachable from the container.
func TestRealContainerSession(t *testing.T) {
	if _, err := exec.LookPath("opencode"); err != nil {
		t.Skip("opencode binary not found")
	}
	for _, k := range []container.EngineKind{container.EngineColima, container.EnginePodman} {
		t.Run(string(k), func(t *testing.T) { realContainerSession(t, k) })
	}
}

func realContainerSession(t *testing.T, engine container.EngineKind) {
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
	root, err := os.MkdirTemp(base, "run-") // shared with the VM
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	short, err := os.MkdirTemp("/tmp", "ohc-") // daemon socket (short path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(short) })

	st, err := sqlite.Open(filepath.Join(short, "oh.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	project := filepath.Join(root, "proj")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "-q", project).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if _, err := st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1', 'p1', ?)`, project); err != nil {
		t.Fatal(err)
	}

	a := opencodev2.New("", filepath.Join(home, "Library", "Caches", "oh-container-tests"))
	if _, err := a.Detect(ctx); err != nil {
		t.Fatal(err)
	}
	secrets := realSecrets{"openhub.provider.bedrock.token": "real-secret-must-not-leak"}
	paths := daemon.Paths{Dir: filepath.Join(short, "run")}
	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- daemon.Run(dctx, daemon.Options{Paths: paths, Version: "test", Grants: sqlite.NewGrantStore(st), Servers: sqlite.NewServerStore(st),
			Secrets: secrets, Tick: time.Second, Sessions: sqlite.NewSessionStore(st),
			Adapter: func(string) adapters.ToolAdapter { return a }})
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

	skills := filepath.Join(root, "bundle", "skills")
	if err := os.MkdirAll(filepath.Join(skills, "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(skills, "alpha", "SKILL.md"), []byte("---\nname: alpha\ndescription: skill alpha\n---\nbody\n"), 0o644)
	model := sessionspec.ParseModelRef("amazon-bedrock/eu.anthropic.claude-haiku-4-5-20251001-v1:0")
	b := &bundle.Bundle{Dir: filepath.Join(root, "bundle"), Spec: sessionspec.BundleSpec{
		Hash: "containerbundle01", Root: filepath.Join(root, "bundle"), EntryAgent: "lead",
		Agents:    []sessionspec.AgentDef{{ID: "lead", Description: "lead", Mode: "primary", Body: "You are LEAD."}},
		Skills:    []sessionspec.SkillDef{{ID: "alpha", Dir: filepath.Join(skills, "alpha")}},
		SkillsDir: skills, MaxDepth: 1, DefaultModel: &model,
	}}

	svc := &Service{
		Adapter: a, AdapterVer: a.Ver, Servers: sqlite.NewServerStore(st), Sessions: sqlite.NewSessionStore(st),
		Secrets: secrets, ServersDir: filepath.Join(root, "servers"), Executable: "/usr/local/bin/oh",
		Daemon:   func(context.Context) (DaemonClient, error) { return dc, nil },
		Runtimes: map[sessionspec.RuntimeKind]ohruntime.Runtime{sessionspec.RuntimeContainer: rt},
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
		ProjectID: "p1", Location: project, Bundle: b, Title: "container", Provider: "bedrock",
		ProviderCfg: providerCfg(), Attach: sessionspec.AttachNone, Runtime: sessionspec.RuntimeContainer,
		Progress: func(l string) { t.Log(l) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Report.OK() {
		t.Fatalf("closed world broken: %v", res.Report.Unexpected)
	}
	if !strings.HasPrefix(res.Server.URL, "http://127.0.0.1:") {
		t.Fatalf("server URL %s", res.Server.URL)
	}
	spec, err := container.LoadSpec(res.Server.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	img = spec.Image

	// The machine client talks to the server in the container.
	c := opencodev2.NewClient(res.Server.URL, res.Server.Password)
	sess, err := c.GetSession(ctx, res.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Agent != "lead" {
		t.Fatalf("session agent = %q", sess.Agent)
	}

	e, _ := rt.Engine(ctx)
	run := func(script string) string {
		out, err := container.ExecRunner{}.Run(ctx, e.CLI, e.Command("exec", spec.Name, "bash", "-c", script)[1:]...)
		if err != nil {
			t.Fatalf("%s: %v", script, err)
		}
		return string(out)
	}
	env := run("env")
	if strings.Contains(env, "real-secret-must-not-leak") || strings.Contains(env, "AWS_SECRET") {
		t.Fatalf("secret in the container environment:\n%s", env)
	}
	if !strings.Contains(env, "AWS_BEARER_TOKEN_BEDROCK=ohs_") {
		t.Fatalf("proxy session token missing:\n%s", env)
	}
	if v := run("opencode --version"); !strings.Contains(v, a.Ver) {
		t.Fatalf("opencode in the image = %s, adapter %s", v, a.Ver)
	}
	// The credential proxy answers from inside (unknown token → 401).
	proxyHost := e.Host
	pu, err := url.Parse(mustHealth(t, dc).ProxyURL)
	if err != nil {
		t.Fatal(err)
	}
	port := pu.Port()
	reply := run("exec 3<>/dev/tcp/" + proxyHost + "/" + port + "; printf 'GET /amazon-bedrock/x HTTP/1.0\\r\\nAuthorization: Bearer nope\\r\\n\\r\\n' >&3; head -1 <&3")
	if !strings.Contains(reply, "401") {
		t.Fatalf("proxy from the container: %q", reply)
	}

	if err := svc.StopSession(ctx, res.SessionID); err != nil {
		t.Fatal(err)
	}
	out, _ := (container.ExecRunner{}).Run(ctx, e.CLI, e.Command("ps", "-a", "--filter", "name="+spec.Name, "--format", "{{.Names}}")[1:]...)
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("container still present after stop: %s", out)
	}
}

func mustHealth(t *testing.T, dc *daemon.Client) daemon.Health {
	h, err := dc.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return h
}
