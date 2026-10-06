//go:build container

package container

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Real engines: `make test-container` (Colima and/or Podman running).
func TestRealDetect(t *testing.T) {
	for _, k := range []EngineKind{EngineColima, EnginePodman, EngineDocker} {
		t.Run(string(k), func(t *testing.T) {
			rt := New(Options{Engine: k})
			av, err := rt.Available(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if !av.OK {
				t.Skipf("%s unavailable: %s", k, av.Message())
			}
			e, _ := rt.Engine(context.Background())
			t.Logf("%s %s via %v, host %s, details %v", e.Kind, e.Version, e.Command(), e.Host, e.Details)
			if e.Version == "" || rt.HostAddress() == "" {
				t.Fatalf("incomplete engine: %+v", e)
			}
		})
	}
}

func realTool(t *testing.T) *opencodev2.LinuxTool {
	t.Helper()
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	return &opencodev2.LinuxTool{Ver: "2.0.20", CacheDir: filepath.Join(cache, "oh-container-tests")}
}

// TestRealImage builds a glibc and a musl project image with the real
// opencode Linux binary and checks the oh layer.
func TestRealImage(t *testing.T) {
	for _, k := range []EngineKind{EngineColima, EnginePodman} {
		for _, base := range []struct{ name, from, libc string }{
			{"debian", "debian:bookworm-slim", "glibc"},
			{"alpine", "alpine:3.20", "musl"},
		} {
			t.Run(string(k)+"/"+base.name, func(t *testing.T) {
				rt := New(Options{Engine: k, CacheDir: t.TempDir()})
				if av, _ := rt.Available(context.Background()); !av.OK {
					t.Skipf("%s unavailable: %s", k, av.Message())
				}
				dir := t.TempDir()
				df := "FROM " + base.from + "\n"
				if base.libc == "musl" {
					df += "RUN apk add --no-cache libstdc++ libgcc ripgrep git\n"
				}
				if err := os.WriteFile(filepath.Join(dir, "Dockerfile.dev"), []byte(df), 0o644); err != nil {
					t.Fatal(err)
				}
				g := ohruntime.Group{ProjectID: "oh-real-" + base.name, ProjectDir: dir, Tool: realTool(t),
					Progress: func(l string) { t.Log(l) }}
				if est, err := rt.Estimate(context.Background(), g); err != nil || est.Ready {
					t.Fatalf("estimate before the build: %+v %v", est, err)
				}
				img, err := rt.EnsureImage(context.Background(), g)
				if err != nil {
					t.Fatal(err)
				}
				if est, err := rt.Estimate(context.Background(), g); err != nil || !est.Ready || !sameRef(est.Image, img.Ref) {
					t.Fatalf("estimate after the build: %+v %v (image %s)", est, err, img.Ref)
				}
				if img.Libc != base.libc {
					t.Fatalf("libc = %s, want %s", img.Libc, base.libc)
				}
				e, _ := rt.Engine(context.Background())
				t.Cleanup(func() {
					_, _ = ExecRunner{}.Run(context.Background(), e.CLI, e.Command("rmi", img.Ref, img.BaseRef)[1:]...)
				})
				out, err := ExecRunner{}.Run(context.Background(), e.CLI, e.Command("run", "--rm", img.Ref, "opencode", "--version")[1:]...)
				if err != nil || !strings.Contains(string(out), "2.0.20") {
					t.Fatalf("opencode --version: %s %v", out, err)
				}
				_, err = ExecRunner{}.Run(context.Background(), e.CLI, e.Command("run", "--rm", img.Ref, "bd", "show", "x")[1:]...)
				if err == nil || !strings.Contains(err.Error(), "Beads") {
					t.Fatalf("fake bd: %v", err)
				}
				again, err := rt.EnsureImage(context.Background(), g)
				if err != nil || again.Built || again.Ref != img.Ref {
					t.Fatalf("cache: %+v %v", again, err)
				}
			})
		}
	}
}

// homeTemp is a temporary directory under $HOME: Colima does not share
// $TMPDIR with its VM.
func homeTemp(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(home, ".cache", "oh-container-tests")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(base, "run-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return clean(dir)
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestRealMountsAndNetwork checks, in a real container: worktree and git
// metadata, file ownership on the machine, read-only bundle, no secret in the
// environment, machine loopback reachable through the host address.
func TestRealMountsAndNetwork(t *testing.T) {
	for _, k := range []EngineKind{EngineColima, EnginePodman} {
		t.Run(string(k), func(t *testing.T) {
			rt := New(Options{Engine: k, CacheDir: t.TempDir()})
			if av, _ := rt.Available(context.Background()); !av.OK {
				t.Skipf("%s unavailable: %s", k, av.Message())
			}
			root := homeTemp(t)
			project := filepath.Join(root, "app")
			worktree := filepath.Join(root, "app-feat")
			bundle := filepath.Join(root, "bundle")
			data := filepath.Join(root, "servers", "g", "data")
			for _, d := range []string{project, bundle, data} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(project, "Dockerfile.dev"), []byte("FROM debian:bookworm-slim\nRUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates && rm -rf /var/lib/apt/lists/*\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			gitRun(t, project, "init", "-q", "-b", "main")
			gitRun(t, project, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "init")
			gitRun(t, project, "worktree", "add", "-q", "-b", "feat", worktree)
			_ = os.WriteFile(filepath.Join(bundle, "agent.md"), []byte("x"), 0o644)

			l, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("oh-host")) })}
			go func() { _ = srv.Serve(l) }()
			t.Cleanup(func() { _ = srv.Close() })
			port := l.Addr().(*net.TCPAddr).Port

			g := ohruntime.Group{ProjectID: "oh-real-mounts", ProjectDir: project, Locations: []string{worktree},
				BundleDir: bundle, DataDir: data, Tool: realTool(t),
				Key: sessionspec.GroupKey{BundleHash: "real", ProjectID: "oh-real-mounts", Runtime: sessionspec.RuntimeContainer}}
			pg, err := rt.Prepare(context.Background(), g)
			if err != nil {
				t.Fatal(err)
			}
			spec := pg.Spec.(*Spec)
			t.Cleanup(func() {
				e, _ := rt.Engine(context.Background())
				_, _ = ExecRunner{}.Run(context.Background(), e.CLI, e.Command("rmi", spec.Image)[1:]...)
				_, _ = ExecRunner{}.Run(context.Background(), e.CLI, e.Command("volume", "rm", "-f", "oh-oh-real-mounts-home")[1:]...)
			})
			inWT, _ := pg.Paths.ToInner(worktree)
			script := strings.Join([]string{
				"set -e",
				"git status --short --branch | head -1",
				"echo hello > created.txt",
				"if touch " + InnerBundle + "/w 2>/dev/null; then echo BUNDLE-WRITABLE; fi",
				"env | grep -E 'AWS_|SECRET|TOKEN' || echo no-secret",
				"exec 3<>/dev/tcp/" + pg.HostAddress + "/" + strconv.Itoa(port) + "; printf 'GET / HTTP/1.0\\r\\n\\r\\n' >&3; tail -1 <&3; echo",
			}, "\n")
			t.Setenv("AWS_SECRET_ACCESS_KEY", "must-not-leak")
			cmd, err := rt.Command(context.Background(), pg, ohruntime.Proc{Argv: []string{"bash", "-c", script}, Dir: inWT})
			if err != nil {
				t.Fatal(err)
			}
			out, err := cmd.CombinedOutput()
			t.Logf("%s", out)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"## feat", "no-secret", "oh-host"} {
				if !strings.Contains(string(out), want) {
					t.Errorf("missing %q", want)
				}
			}
			if strings.Contains(string(out), "BUNDLE-WRITABLE") {
				t.Error("bundle is writable")
			}
			st, err := os.Stat(filepath.Join(worktree, "created.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if uid := st.Sys().(*syscall.Stat_t).Uid; int(uid) != os.Getuid() {
				t.Errorf("created file uid = %d, want %d", uid, os.Getuid())
			}
		})
	}
}
