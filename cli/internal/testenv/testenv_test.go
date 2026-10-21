package testenv

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

func TestMain(m *testing.M) {
	finish := Isolate()
	os.Exit(finish(m.Run()))
}

func TestIsolated(t *testing.T) {
	dir := os.Getenv(marker)
	require.NotEmpty(t, dir)
	for _, k := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"} {
		assert.True(t, strings.HasPrefix(os.Getenv(k), dir+string(filepath.Separator)), "%s = %q", k, os.Getenv(k))
	}
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, os.Getenv("HOME"), home)
	_, set := os.LookupEnv("OH_HOME")
	assert.False(t, set, "OH_HOME is removed: the hub is ~/.oh of the temporary HOME")
}

// The tests get the git configuration of the tests, not the user's: an
// identity and main as the initial branch, whatever the machine.
func TestGitConfig(t *testing.T) {
	path := os.Getenv("GIT_CONFIG_GLOBAL")
	require.True(t, strings.HasPrefix(path, os.Getenv(marker)+string(filepath.Separator)), path)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, gittest.Defaults(), string(data))
}

func TestRemoved(t *testing.T) {
	for _, k := range []string{"OH_HOME", "OH_TEST_ROOT", "OH_TEST_PROJECT", "OH_PASSPHRASE", "OH_SESSION_ID", "OH_GATEWAY_URL"} {
		assert.True(t, removed(k), k)
	}
	for _, k := range []string{"HOME", "PATH", "OHX", "OH_E2E_BEDROCK_TOKEN", "OH_EVAL_MODELS", marker} {
		assert.False(t, removed(k), k)
	}
}

func TestChanges(t *testing.T) {
	f := filepath.Join(t.TempDir(), "hub.toml")
	before := snapshot([]string{f})
	assert.Empty(t, changes(before, snapshot([]string{f})))

	require.NoError(t, os.WriteFile(f, []byte("a"), 0o600))
	assert.Contains(t, changes(before, snapshot([]string{f})), "created")

	before = snapshot([]string{f})
	require.NoError(t, os.WriteFile(f, []byte("ab"), 0o600))
	assert.Contains(t, changes(before, snapshot([]string{f})), "modified")

	before = snapshot([]string{f})
	later := time.Now().Add(time.Hour)
	require.NoError(t, os.Chtimes(f, later, later))
	assert.Contains(t, changes(before, snapshot([]string{f})), "modified")

	before = snapshot([]string{f})
	require.NoError(t, os.Remove(f))
	assert.Contains(t, changes(before, snapshot([]string{f})), "removed")
}

func TestRealHubs(t *testing.T) {
	t.Setenv("HOME", "/h")
	t.Setenv("OH_HOME", "/x/oh")
	assert.Equal(t, []string{"/x/oh/hub.toml", "/x/oh/oh.db", "/h/.oh/hub.toml", "/h/.oh/oh.db"}, realHubs())
	t.Setenv("OH_HOME", "/h/.oh/")
	assert.Equal(t, []string{"/h/.oh/hub.toml", "/h/.oh/oh.db"}, realHubs())
}

// TestHelperIsolate is run by TestWatchdog in a test binary of its own,
// started as the user's shell would (OH_HOME and OH_TEST_* exported).
func TestHelperIsolate(t *testing.T) {
	real := os.Getenv("TESTENV_HELPER_REAL")
	if real == "" {
		t.Skip("helper of TestWatchdog")
	}
	assert.Empty(t, os.Getenv("OH_TEST_ROOT"))
	assert.Empty(t, os.Getenv("OH_HOME"))
	hub := filepath.Join(os.Getenv("HOME"), ".oh")
	assert.True(t, strings.HasPrefix(hub, os.Getenv(marker)))
	require.NoError(t, os.MkdirAll(hub, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hub, "hub.toml"), []byte("isolated"), 0o600))
	if os.Getenv("TESTENV_HELPER_WRITE") == "1" {
		require.NoError(t, os.WriteFile(filepath.Join(real, "hub.toml"), []byte("overwritten"), 0o600))
	}
}

func TestWatchdog(t *testing.T) {
	for _, write := range []bool{false, true} {
		real := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(real, "hub.toml"), []byte("witness"), 0o600))
		c := exec.Command(os.Args[0], "-test.run=^TestHelperIsolate$", "-test.count=1")
		home := t.TempDir()
		// The Go tool variables are set so that Isolate does not run `go env`
		// in the helper: the go command leaves a detached process writing its
		// telemetry under HOME, which races with the removal of the folder.
		c.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "OH_HOME=" + real, "OH_TEST_ROOT=/x",
			"GOCACHE=" + filepath.Join(home, "gocache"), "GOMODCACHE=" + filepath.Join(home, "gomod"),
			"GOPATH=" + filepath.Join(home, "gopath"), "GOENV=off",
			"TESTENV_HELPER_REAL=" + real}
		if write {
			c.Env = append(c.Env, "TESTENV_HELPER_WRITE=1")
		}
		out, err := c.CombinedOutput()
		got, rerr := os.ReadFile(filepath.Join(real, "hub.toml"))
		require.NoError(t, rerr)
		if write {
			require.Error(t, err, string(out))
			assert.Contains(t, string(out), "changed the real hub")
			assert.Equal(t, "overwritten", string(got))
		} else {
			require.NoError(t, err, string(out))
			assert.Equal(t, "witness", string(got))
		}
	}
}

func TestFakeSecrets(t *testing.T) {
	secrets := FakeSecrets()
	assert.Len(t, secrets, len(FakeSecretNames))
	for a, va := range secrets {
		for b, vb := range secrets {
			if a != b {
				assert.NotContains(t, va, vb, "%s holds the value of %s", a, b)
			}
		}
	}

	set := SetFakeSecrets(t)
	assert.Equal(t, secrets, set)
	assert.Equal(t, secrets["GH_TOKEN"], os.Getenv("GH_TOKEN"))
}

func TestLeaks(t *testing.T) {
	secrets := FakeSecrets()
	assert.Empty(t, Leaks(secrets, []string{"PATH=/bin", "LANG=fr_FR.UTF-8", "GH_TOKEN=gh"}))
	assert.Empty(t, LeaksInMap(secrets, map[string]string{"A": "b"}))
	assert.Empty(t, LeaksIn(secrets, "docker run -e HOME=/h"))

	got := Leaks(secrets, []string{
		"PATH=/bin",
		"RENAMED=" + secrets["GEMINI_API_KEY"],
		`SERVER_CONFIG_CONTENT={"key":"` + secrets["GH_TOKEN"] + `"}`,
	})
	assert.Equal(t, []string{
		"RENAMED holds the value of GEMINI_API_KEY",
		"SERVER_CONFIG_CONTENT holds the value of GH_TOKEN",
	}, got)
	for _, l := range got {
		assert.NotContains(t, l, "oh-fake-secret", "the report never shows a value")
	}
	assert.Len(t, LeaksInMap(secrets, map[string]string{"X": secrets["OPENAI_API_KEY"]}), 1)
	assert.Len(t, LeaksIn(secrets, "--token "+secrets["GITLAB_TOKEN"]), 1)
}
