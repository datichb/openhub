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
	"github.com/datichb/openhub/cli/internal/gateway"
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// TestLocalSessionBeadsHooksAndShell (piste T, T2; no LLM call): a real
// server, the oh daemon and the real bd, on a project initialized by
// `bd init` with its git hooks.
//   - A16: the user's ~/.zshenv puts another bd first on the PATH; oh's
//     shell start-up keeps the fake bd first all the same;
//   - A19: `git commit` from the session shell runs the hooks of Beads
//     (`bd hooks run …` through the fake bd and the gateway), although the
//     workflow does not allow `hooks`: the commit goes through.
func TestLocalSessionBeadsHooksAndShell(t *testing.T) {
	for _, bin := range []string{"opencode", "bd", "zsh"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s binary not found", bin)
		}
	}
	zsh, _ := exec.LookPath("zsh")
	ctx := context.Background()
	root, err := os.MkdirTemp("/tmp", "ohlh-") // short path for the daemon socket
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

	// The user's start-up file puts another bd first (A16).
	home, other := filepath.Join(root, "home"), filepath.Join(root, "other")
	for _, d := range []string{home, other} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(other, "bd"), []byte("#!/bin/sh\necho OTHER-BD\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".zshenv"), []byte("export PATH=\""+other+":$PATH\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", zsh)

	project := filepath.Join(root, "proj")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	sh(project, "git", "init", "-q")
	sh(project, "git", "config", "user.name", "t")
	sh(project, "git", "config", "user.email", "t@t")
	sh(project, "bd", "init", "--quiet", "--prefix", "lh")
	if hp := strings.TrimSpace(sh(project, "git", "config", "core.hooksPath")); !strings.HasSuffix(hp, ".beads/hooks") {
		t.Skipf("bd init did not install its git hooks (core.hooksPath = %q)", hp)
	}
	sh(project, "git", "add", ".")
	sh(project, "git", "commit", "--allow-empty", "-qm", "init")
	t.Setenv("HOME", home) // read by the session shell (machine variables)

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

	startup, err := gateway.ShellStartup(filepath.Join(root, "run", "shell"), []string{shims}, os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := gateway.CheckShellBD(ctx, LocalSessionEnv(nil, nil, []string{shims}), filepath.Join(shims, "bd")); err == nil {
		t.Fatalf("without oh's start-up, the user's .zshenv must win (A16 reproduced), got %s", got)
	}
	model := sessionspec.ParseModelRef("amazon-bedrock/eu.anthropic.claude-haiku-4-5-20251001-v1:0")
	bdir := filepath.Join(root, "bundle")
	if err := os.MkdirAll(filepath.Join(bdir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	b := &bundle.Bundle{Dir: bdir, Spec: sessionspec.BundleSpec{
		Hash: "localhooks01", Root: bdir, EntryAgent: "lead",
		Agents:    []sessionspec.AgentDef{{ID: "lead", Description: "lead", Mode: "primary", Body: "You are LEAD."}},
		SkillsDir: filepath.Join(bdir, "skills"), MaxDepth: 1, DefaultModel: &model,
	}}
	svc := &Service{
		Adapter: a, AdapterVer: a.Ver, Servers: servers, Sessions: sessions, SessionsDir: filepath.Join(root, "sessions"),
		Secrets: secrets, ServersDir: filepath.Join(root, "servers"), Executable: "/usr/local/bin/oh",
		Daemon:         func(context.Context) (DaemonClient, error) { return dc, nil },
		LocalShellPath: []string{shims},
		LocalShellEnv:  startup,
		SessionEnv: func(ctx context.Context, r SessionEnvRequest) (map[string]string, error) {
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
		ProjectID: "p1", Location: project, Bundle: b, Title: "local hooks", Provider: "bedrock",
		ProviderCfg: providerCfg(), Attach: sessionspec.AttachNone, BeadsAllow: []string{"show", "list"},
	})
	if err != nil {
		t.Fatal(err)
	}

	c := opencodev2.NewClient(res.Server.URL, res.Server.Password)
	shell := func(command string) string {
		t.Helper()
		cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
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

	if out := shell("command -v bd"); !strings.Contains(out, filepath.Join(shims, "bd")) {
		t.Fatalf("A16: the fake bd must come first despite ~/.zshenv:\n%s", out)
	}
	if out := shell("echo change > feature.txt && git add feature.txt && git commit -m 'feat: change'; echo EXIT=$?"); !strings.Contains(out, "EXIT=0") {
		t.Fatalf("A19: git commit with the Beads hooks must go through:\n%s", out)
	}
	if log := sh(project, "git", "log", "--format=%s"); !strings.Contains(log, "feat: change") {
		t.Fatalf("no commit:\n%s", log)
	}
	if out := shell("bd hooks run pre-commit; echo EXIT=$?"); !strings.Contains(out, "EXIT=1") {
		t.Fatalf("bd hooks outside a git hook stays refused by beads.allow:\n%s", out)
	}
	if err := svc.StopSession(ctx, res.SessionID); err != nil {
		t.Fatal(err)
	}
}
