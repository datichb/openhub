package opencodev2

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/semver"
)

// DiscoverNatives starts a throw-away server with an empty configuration and
// returns every selectable (non-hidden) agent it exposes. These are the agents
// opencode ships plus any agent defined in the user's global config: all of
// them must be disabled in oh sessions (closed world).
func DiscoverNatives(ctx context.Context, binary string) ([]string, error) {
	tmp, err := os.MkdirTemp("", "oh-natives-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	work := filepath.Join(tmp, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		return nil, err
	}
	srv, err := StartServer(ctx, ServerOptions{
		Binary:        binary,
		WorkDir:       work,
		DataDir:       filepath.Join(tmp, "data"),
		ConfigContent: "{}",
		ReadyTimeout:  30 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("starting discovery server: %w", err)
	}
	defer func() { _ = srv.Stop(context.Background(), 3*time.Second) }()

	agents, err := srv.Client.Agents(ctx, work)
	if err != nil {
		return nil, fmt.Errorf("listing agents: %w", err)
	}
	var natives []string
	for _, a := range agents {
		if !a.Hidden && !SystemAgents[a.ID] {
			natives = append(natives, a.ID)
		}
	}
	sort.Strings(natives)
	return natives, nil
}

// nativesCache is persisted per opencode version.
type nativesCache struct {
	Version   string    `json:"version"`
	Natives   []string  `json:"natives"`
	CheckedAt time.Time `json:"checked_at"`
}

// NativesCachePath returns the cache file for an opencode version.
func NativesCachePath(cacheDir, version string) string {
	return filepath.Join(cacheDir, "opencode-natives-"+sanitizeVersion(version)+".json")
}

// LoadNatives returns cached natives for version, or discovers and caches them.
// On any discovery failure it falls back to DefaultNatives.
func LoadNatives(ctx context.Context, binary, version, cacheDir string, refresh bool) []string {
	path := NativesCachePath(cacheDir, version)
	if !refresh {
		if data, err := os.ReadFile(path); err == nil {
			var c nativesCache
			if json.Unmarshal(data, &c) == nil && c.Version == version && len(c.Natives) > 0 {
				return c.Natives
			}
		}
	}
	natives, err := DiscoverNatives(ctx, binary)
	if err != nil || len(natives) == 0 {
		return append([]string(nil), DefaultNatives...)
	}
	natives = union(natives, DefaultNatives)
	if err := os.MkdirAll(cacheDir, 0o700); err == nil {
		data, _ := json.MarshalIndent(nativesCache{Version: version, Natives: natives, CheckedAt: time.Now()}, "", "  ")
		_ = os.WriteFile(path, data, 0o600)
	}
	return natives
}

func union(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range append(append([]string(nil), a...), b...) {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func sanitizeVersion(v string) string {
	return strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(v)
}

// Version returns the version of the opencode binary ("2.0.20").
func Version(ctx context.Context, binary string) (string, error) {
	out, err := exec.CommandContext(ctx, binary, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("running %s --version: %w", binary, err)
	}
	return semver.FromOutput(string(out)), nil
}
