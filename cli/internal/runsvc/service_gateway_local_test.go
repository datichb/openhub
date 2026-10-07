//go:build integration

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
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// TestLocalBeadsGateway (QB1) runs bd from the shell of a local session (no
// LLM call): the fake bd first on its PATH sends the commands to the Beads
// gateway of the daemon, which applies beads.allow and runs the real bd of
// the machine, as in a container.
func TestLocalBeadsGateway(t *testing.T) {
	for _, bin := range []string{"opencode", "bd"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s binary not found", bin)
		}
	}
	ctx := context.Background()
	root, err := os.MkdirTemp("/tmp", "ohlg-") // short path for the daemon socket
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	sh := func(dir, name string, args ...string) string {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
		return string(out)
	}
	shims := filepath.Join(root, "bin")
	sh("../..", "go", "build", "-o", filepath.Join(shims, "bd"), "./cmd/oh-bd")

	project := filepath.Join(root, "proj")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	sh(project, "git", "init", "-q")
	sh(project, "bd", "init", "--quiet", "--prefix", "lg")
	var issue struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(sh(project, "bd", "create", "created on the machine", "--json")), &issue); err != nil || issue.ID == "" {
		t.Fatalf("bd create on the machine: %v", err)
	}

	st, err := sqlite.Open(filepath.Join(root, "oh.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if _, err := st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1', 'p1', ?)`, project); err != nil {
		t.Fatal(err)
	}
	servers, sessions := sqlite.NewServerStore(st), sqlite.NewSessionStore(st)
	a := opencodev2.New("", filepath.Join(root, "cache"))
	if _, err := a.Detect(ctx); err != nil {
		t.Fatal(err)
	}
	secrets := mapSecrets{"openhub.provider.bedrock.token": "fake-key"}
	paths := daemon.Paths{Dir: filepath.Join(root, "run")}
	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	realBD, _ := exec.LookPath("bd")
	go func() {
		done <- daemon.Run(dctx, daemon.Options{Paths: paths, Version: "test", Grants: sqlite.NewGrantStore(st), Servers: servers,
			Secrets: secrets, Tick: time.Second, Sessions: sessions, SessionsDir: filepath.Join(root, "sessions"),
			BeadsBinary: realBD, Adapter: func(string) adapters.ToolAdapter { return a }})
	}()
	t.Cleanup(func() { cancel(); <-done })
	dc := daemon.NewClient(paths)
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if _, err := dc.Health(ctx); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("daemon not ready")
		}
	}

	model := sessionspec.ParseModelRef("amazon-bedrock/eu.anthropic.claude-haiku-4-5-20251001-v1:0")
	bdir := filepath.Join(root, "bundle")
	if err := os.MkdirAll(filepath.Join(bdir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	b := &bundle.Bundle{Dir: bdir, Spec: sessionspec.BundleSpec{
		Hash: "localgateway01", Root: bdir, EntryAgent: "lead",
		Agents:    []sessionspec.AgentDef{{ID: "lead", Description: "lead", Mode: "primary", Body: "You are LEAD."}},
		SkillsDir: filepath.Join(bdir, "skills"), MaxDepth: 1, DefaultModel: &model,
	}}
	svc := &Service{
		Adapter: a, AdapterVer: a.Ver, Servers: servers, Sessions: sessions, SessionsDir: filepath.Join(root, "sessions"),
		Secrets: secrets, ServersDir: filepath.Join(root, "servers"), Executable: "/usr/local/bin/oh",
		Daemon:         func(context.Context) (DaemonClient, error) { return dc, nil },
		LocalShellPath: []string{shims},
		SessionEnv: func(ctx context.Context, r SessionEnvRequest) (map[string]string, error) {
			if r.GatewayURL == "" {
				t.Errorf("local group without gateway address")
				return nil, nil
			}
			g, err := dc.IssueGatewayGrant(ctx, daemon.GatewayGrantRequest{SessionID: r.SessionID, GroupKey: r.GroupKey,
				ProjectID: r.ProjectID, Location: r.Location, BeadsAllow: r.BeadsAllow, GatewayURL: r.GatewayURL})
			return map[string]string{beadswire.EnvURL: r.GatewayURL, beadswire.EnvToken: g.Token}, err
		},
	}
	t.Cleanup(func() {
		if list, err := svc.Servers.List(context.Background()); err == nil {
			for _, s := range list {
				svc.stopServer(context.Background(), &s)
			}
		}
	})
	res, err := svc.StartSession(ctx, StartRequest{
		ProjectID: "p1", Location: project, Bundle: b, Title: "local gateway", Provider: "bedrock",
		ProviderCfg: providerCfg(), Attach: sessionspec.AttachNone, BeadsAllow: []string{"show", "list"},
	})
	if err != nil {
		t.Fatal(err)
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

	if out := shell("command -v bd"); !strings.Contains(out, shims) {
		t.Fatalf("the fake bd must come first on the PATH:\n%s", out)
	}
	if out := shell("bd show " + issue.ID + " --json; echo EXIT=$?"); !strings.Contains(out, "created on the machine") || !strings.Contains(out, "EXIT=0") {
		t.Fatalf("bd show through the gateway:\n%s", out)
	}
	if out := shell("bd create 'from the session'; echo EXIT=$?"); !strings.Contains(out, "EXIT=1") || !strings.Contains(out, "create") {
		t.Fatalf("bd create must be refused (beads.allow):\n%s", out)
	}
	if list := sh(project, "bd", "list"); strings.Contains(list, "from the session") {
		t.Fatalf("refused command reached Beads:\n%s", list)
	}
	if err := svc.StopSession(ctx, res.SessionID); err != nil {
		t.Fatal(err)
	}
}
