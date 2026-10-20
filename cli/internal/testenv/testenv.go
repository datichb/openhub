// Package testenv isolates a test binary from the real environment of the
// user: HOME and the XDG directories point into a temporary folder, and the
// variables that could lead to a real hub (OH_HOME, OH_TEST_*…) are removed.
// OH_HOME is left unset rather than fixed: the hub is then ~/.oh of the
// temporary HOME, which is what the tests that set HOME themselves expect
// (and what they find in CI); the tests that want another hub set OH_HOME.
// The global git configuration (GIT_CONFIG_GLOBAL) is the one of
// gittest.Defaults, not the user's: an identity and main as the initial
// branch, so that no test depends on the machine (a CI runner has neither).
// It is used by the TestMain of the packages whose tests, directly or through
// the code they exercise, read these paths.
//
//	func TestMain(m *testing.M) {
//		finish := testenv.Isolate()
//		os.Exit(finish(m.Run()))
//	}
//
// A watchdog checks, once the tests ran, that hub.toml and oh.db of the real
// hub have not changed: a test that writes there fails its package.
package testenv

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

// marker is set in the environment of the isolated test binary. A helper
// process that re-executes the test binary (and inherits the environment)
// finds it and stays as isolated as its parent, keeping the variables the
// parent passed to it.
const marker = "OH_TESTENV"

// keep lists the prefixes of the OH_ variables that stay: the controls of
// the tests that call a real model (build tags e2e and integration, never
// run by default) and their credentials.
var keep = []string{"OH_E2E_", "OH_EVAL_"}

// watched are the files of a hub that a test must never write.
var watched = []string{"hub.toml", "oh.db"}

var original []string

// OriginalEnv is the environment of the test binary before Isolate, for the
// rare test that reads something of the real environment (credentials of a
// tool, for the tests that call a real model). Read only: never write there.
func OriginalEnv() []string {
	if original == nil {
		return os.Environ()
	}
	return append([]string(nil), original...)
}

// Isolate isolates the process and returns the function to call with the
// exit code of the tests: it checks the real hub, removes the temporary
// folder and returns the code to give to os.Exit. When the folder cannot be
// created, the process exits: tests must not run without isolation.
func Isolate() (finish func(code int) int) {
	if os.Getenv(marker) != "" {
		return func(code int) int { return code }
	}
	original = os.Environ()
	hubs := realHubs()
	before := snapshot(hubs)
	pins := toolPins()

	dir, err := os.MkdirTemp("", "ohti-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "testenv: cannot isolate the tests:", err)
		os.Exit(1)
	}
	home := filepath.Join(dir, "home")
	env := map[string]string{
		marker:            dir,
		"HOME":            home,
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"XDG_DATA_HOME":   filepath.Join(home, ".local", "share"),
		"XDG_CACHE_HOME":  filepath.Join(home, ".cache"),
		"XDG_STATE_HOME":  filepath.Join(home, ".local", "state"),
	}
	for _, d := range []string{home, env["XDG_CONFIG_HOME"], env["XDG_DATA_HOME"], env["XDG_CACHE_HOME"], env["XDG_STATE_HOME"]} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			_ = os.RemoveAll(dir)
			fmt.Fprintln(os.Stderr, "testenv: cannot isolate the tests:", err)
			os.Exit(1)
		}
	}
	// The git configuration of the tests, not the one of the user: no test
	// depends on the identity or the defaults of the machine (a CI runner has
	// none).
	path := filepath.Join(dir, "gitconfig")
	if os.WriteFile(path, []byte(gittest.Defaults()), 0o600) == nil {
		env["GIT_CONFIG_GLOBAL"] = path
	}
	for k, v := range pins {
		env[k] = v
	}
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); removed(k) {
			_ = os.Unsetenv(k)
		}
	}
	for k, v := range env {
		_ = os.Setenv(k, v)
	}

	return func(code int) int {
		defer os.RemoveAll(dir)
		if msg := changes(before, snapshot(hubs)); msg != "" {
			fmt.Fprintln(os.Stderr, "testenv: FAIL: the tests changed the real hub:\n"+msg+
				"(a test writes outside its temporary folder; if oh was in use meanwhile on that hub, run again)")
			if code == 0 {
				return 1
			}
		}
		return code
	}
}

// removed reports whether Isolate removes the variable: the OH_ namespace
// (OH_TEST_*, OH_HOME, OH_PASSPHRASE, the session and gateway variables…)
// except the controls of the model tests.
func removed(k string) bool {
	if !strings.HasPrefix(k, "OH_") || k == marker {
		return false
	}
	for _, p := range keep {
		if strings.HasPrefix(k, p) {
			return false
		}
	}
	return true
}

// realHubs are the places of the real hub, as the tests would find them
// without isolation: OH_HOME if set, and ~/.oh.
func realHubs() []string {
	var dirs []string
	if d := os.Getenv("OH_HOME"); d != "" {
		dirs = append(dirs, d)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dirs = append(dirs, filepath.Join(home, ".oh"))
	}
	var files []string
	seen := map[string]bool{}
	for _, d := range dirs {
		for _, f := range watched {
			p := filepath.Clean(filepath.Join(d, f))
			if !seen[p] {
				seen[p] = true
				files = append(files, p)
			}
		}
	}
	return files
}

type state struct {
	exists bool
	size   int64
	mtime  time.Time
}

// snapshot reads (read only, nothing is created) the state of the files.
func snapshot(files []string) map[string]state {
	out := make(map[string]state, len(files))
	for _, f := range files {
		if fi, err := os.Lstat(f); err == nil {
			out[f] = state{true, fi.Size(), fi.ModTime()}
		} else {
			out[f] = state{}
		}
	}
	return out
}

// changes describes the files whose existence, size or modification time
// differ; empty when nothing changed.
func changes(before, after map[string]state) string {
	var b strings.Builder
	for f, was := range before {
		now := after[f]
		switch {
		case was.exists && !now.exists:
			fmt.Fprintf(&b, "  %s: removed\n", f)
		case !was.exists && now.exists:
			fmt.Fprintf(&b, "  %s: created\n", f)
		case was.exists && (was.size != now.size || !was.mtime.Equal(now.mtime)):
			fmt.Fprintf(&b, "  %s: modified (%d → %d bytes, %s → %s)\n", f, was.size, now.size,
				was.mtime.Format(time.RFC3339Nano), now.mtime.Format(time.RFC3339Nano))
		}
	}
	return b.String()
}

// toolPins fixes where the Go toolchain keeps its caches, as the user has
// them: the tests that run `go build` do not start over from an empty cache
// nor lose the downloaded modules once HOME moved.
func toolPins() map[string]string {
	pins := map[string]string{}
	var names []string
	for _, k := range []string{"GOCACHE", "GOMODCACHE", "GOPATH", "GOENV"} {
		if os.Getenv(k) == "" {
			names = append(names, k)
		}
	}
	if len(names) == 0 {
		return pins
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return pins
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, goBin, append([]string{"env"}, names...)...).Output()
	if err != nil {
		return pins
	}
	values := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(values) != len(names) {
		return pins
	}
	for i, k := range names {
		if values[i] != "" {
			pins[k] = values[i]
		}
	}
	return pins
}
