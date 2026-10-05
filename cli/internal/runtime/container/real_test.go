//go:build container

package container

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
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
				img, err := rt.EnsureImage(context.Background(), g)
				if err != nil {
					t.Fatal(err)
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
