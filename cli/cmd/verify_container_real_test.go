//go:build container && !windows

package cmd

import (
	"context"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// TestRealContainerVerify (fanout-container, `go test -tags container`):
// the verification of a container session as the daemon runs it, in a
// one-off container of the image of the project, on the location of the
// session (here an integration worktree), then removed.
func TestRealContainerVerify(t *testing.T) {
	ctx := context.Background()
	u, err := user.Current()
	if err != nil {
		t.Skip(err)
	}
	t.Setenv("HOME", u.HomeDir) // the engine (Colima) and the folders it shares
	base := filepath.Join(u.HomeDir, ".cache", "oh-container-tests")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(base, "verify-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	t.Setenv("OH_HOME", filepath.Join(root, ".oh"))
	if !v5Available(ctx) {
		t.Skip("no supported tool installed")
	}
	a := &app.App{Config: &config.Config{}}
	rt := v5ContainerRuntime(a)
	if av, _ := rt.Available(ctx); !av.OK {
		t.Skipf("container engine unavailable: %s", av.Message())
	}
	t.Cleanup(func() {
		e, _ := rt.Engine(context.Background())
		list, _ := exec.Command(e.CLI, e.Command("images", "--format", "{{.Repository}}:{{.Tag}}")[1:]...).Output()
		for _, ref := range strings.Fields(string(list)) {
			if strings.Contains(ref, "/oh-real-verify:") {
				_ = exec.Command(e.CLI, e.Command("rmi", ref)[1:]...).Run()
			}
		}
	})

	project := filepath.Join(root, "app")
	intg := filepath.Join(root, "app-oh-fanout-real-integration")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "Dockerfile.dev"), []byte("FROM debian:bookworm-slim\nRUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates && rm -rf /var/lib/apt/lists/*\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "."}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "init"},
		{"worktree", "add", "-q", "-b", "oh/fanout/real/integration", intg}} {
		if out, err := exec.Command("git", append([]string{"-C", project}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(intg, "merged.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := sqlite.Open(filepath.Join(root, "oh.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if _, err := st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('oh-real-verify', 'app', ?)`, project); err != nil {
		t.Fatal(err)
	}
	a.Projects = sqlite.NewProjectStore(st)

	sess := &domain.Session{ID: "ses_real", ProjectID: "oh-real-verify", Runtime: "container", LaunchPath: intg}
	run, done, err := containerVerify(a)(ctx, sess)
	if err != nil {
		t.Fatal(err)
	}
	out := &checkpoint.TailBuffer{Max: 4096}
	t.Setenv("AWS_SECRET_ACCESS_KEY", "must-not-leak")
	exit, err := run(ctx, intg, []string{"bash", "-c", "test -f merged.txt && git rev-parse --abbrev-ref HEAD && (env | grep -E 'AWS_|SECRET' || echo no-secret)"},
		[]string{"CI=true"}, out)
	t.Logf("%s", out.Bytes())
	if err != nil || exit != 0 {
		t.Fatalf("exit %d, %v", exit, err)
	}
	if s := string(out.Bytes()); !strings.Contains(s, "oh/fanout/real/integration") || !strings.Contains(s, "no-secret") {
		t.Fatalf("unexpected output: %s", s)
	}
	exit, err = run(ctx, filepath.Join(root, "elsewhere"), []string{"true"}, nil, out)
	if err != nil || exit != -1 {
		t.Fatalf("a folder not mounted: exit %d, %v", exit, err)
	}
	done()
	if entries, _ := os.ReadDir(ohServersDir()); len(entries) != 0 {
		t.Errorf("left in the servers folder: %v", entries)
	}
}
