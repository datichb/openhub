//go:build container && !windows

package container

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/services/fanout"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// TestRealFanoutVerification (fanout-container, `make test-container`): the
// layout of a fan-out in a real container. oh creates the integration and
// child worktrees on the machine; a child commits in its container (as its
// agent would); oh merges it into the integration branch on the machine;
// the verification of cp-merge runs in a fresh container where the
// integration worktree is mounted, and leaves nothing behind.
func TestRealFanoutVerification(t *testing.T) {
	for _, k := range []EngineKind{EngineColima, EnginePodman} {
		t.Run(string(k), func(t *testing.T) { realFanoutVerification(t, k) })
	}
}

func realFanoutVerification(t *testing.T, k EngineKind) {
	ctx := context.Background()
	rt := New(Options{Engine: k, CacheDir: t.TempDir()})
	if av, _ := rt.Available(ctx); !av.OK {
		t.Skipf("%s unavailable: %s", k, av.Message())
	}
	root := homeTemp(t)
	project := filepath.Join(root, "app")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "Dockerfile.dev"), []byte("FROM debian:bookworm-slim\nRUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates && rm -rf /var/lib/apt/lists/*\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, project, "init", "-q", "-b", "main")
	gittest.Run(t, project, "add", "Dockerfile.dev")
	gittest.Run(t, project, "commit", "-q", "-m", "init")
	base, err := fanout.Head(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	intg, err := fanout.AddWorktree(ctx, project, fanout.IntegrationBranch("ses_real"), base)
	if err != nil {
		t.Fatal(err)
	}
	childBranch := fanout.ChildBranch("ses_real", "ev-1")
	child, err := fanout.AddWorktree(ctx, project, childBranch, base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(intg)
		_ = os.RemoveAll(child)
	})
	tool := realTool(t)
	group := func(id string, loc string) *ohruntime.Prepared {
		if err := os.MkdirAll(filepath.Join(root, "servers", id, "data"), 0o700); err != nil {
			t.Fatal(err)
		}
		pg, err := rt.Prepare(ctx, ohruntime.Group{GroupID: id, ProjectID: "oh-real-fanout", ProjectDir: project, Locations: []string{loc},
			DataDir: filepath.Join(root, "servers", id, "data"), Tool: tool,
			Key: sessionspec.GroupKey{ProjectID: "oh-real-fanout", Runtime: sessionspec.RuntimeContainer}})
		if err != nil {
			t.Fatal(err)
		}
		return pg
	}

	// The child commits in its container.
	cpg := group("real-fanout-child", child)
	inChild, _ := cpg.Paths.ToInner(child)
	var out bytes.Buffer
	exit, err := rt.Exec(ctx, cpg, ohruntime.Proc{Argv: []string{"bash", "-c",
		"echo done > child.txt && git add child.txt && git -c user.email=t@t -c user.name=t commit -q -m ev-1 && git rev-parse --abbrev-ref HEAD"}, Dir: inChild}, &out)
	t.Logf("child: %s", out.String())
	if err != nil || exit != 0 {
		t.Fatalf("commit in the container of the child: exit %d, %v", exit, err)
	}
	if !strings.Contains(out.String(), childBranch) {
		t.Fatalf("the child worked on another branch: %s", out.String())
	}

	// oh merges it on the machine.
	if n, err := fanout.CommitsAhead(ctx, intg, base, childBranch); err != nil || n != 1 {
		t.Fatalf("commits of the child seen by oh: %d %v", n, err)
	}
	if _, err := fanout.Merge(ctx, intg, childBranch, "Merge ev-1"); err != nil {
		t.Fatal(err)
	}

	// The verification of cp-merge, in a fresh container on the
	// integration worktree.
	t.Setenv("AWS_SECRET_ACCESS_KEY", "must-not-leak")
	vpg := group("verify-real-fanout", intg)
	spec := vpg.Spec.(*Spec)
	inIntg, ok := vpg.Paths.ToInner(intg)
	if !ok {
		t.Fatal("integration worktree not mounted")
	}
	out.Reset()
	exit, err = rt.Exec(ctx, vpg, ohruntime.Proc{Argv: []string{"bash", "-c",
		"test -f child.txt && git log --oneline -1 && (env | grep -E 'AWS_|SECRET|TOKEN' || echo no-secret)"}, Dir: inIntg, Env: map[string]string{"CI": "true"}}, &out)
	t.Logf("verify: %s", out.String())
	if err != nil || exit != 0 {
		t.Fatalf("verification on the integration branch: exit %d, %v", exit, err)
	}
	if !strings.Contains(out.String(), "Merge ev-1") || !strings.Contains(out.String(), "no-secret") {
		t.Fatalf("unexpected verification output: %s", out.String())
	}
	out.Reset()
	if exit, err = rt.Exec(ctx, vpg, ohruntime.Proc{Argv: []string{"bash", "-c", "exit 4"}, Dir: inIntg}, &out); err != nil || exit != 4 {
		t.Fatalf("failing verification: exit %d, %v", exit, err)
	}

	e, _ := rt.Engine(ctx)
	ps, _ := ExecRunner{}.Run(ctx, e.CLI, e.Command("ps", "-a", "--format", "{{.Names}}")[1:]...)
	if strings.Contains(string(ps), spec.Name) {
		t.Errorf("container %s left behind", spec.Name)
	}
	vols, _ := ExecRunner{}.Run(ctx, e.CLI, e.Command("volume", "ls", "--format", "{{.Name}}")[1:]...)
	for _, m := range spec.Mounts {
		if m.Volume && m.Target == InnerHome && strings.Contains(string(vols), m.Source) {
			t.Errorf("HOME volume %s left behind", m.Source)
		}
	}
	t.Cleanup(func() {
		list, _ := ExecRunner{}.Run(context.Background(), e.CLI, e.Command("images", "--format", "{{.Repository}}:{{.Tag}}")[1:]...)
		for _, ref := range strings.Fields(string(list)) {
			if strings.Contains(ref, "/oh-real-fanout:") {
				_, _ = ExecRunner{}.Run(context.Background(), e.CLI, e.Command("rmi", ref)[1:]...)
			}
		}
	})
}
