package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gc"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// gcTestApp is a hub with an old stopped session: its bundle and its folder
// can go.
func gcTestApp(t *testing.T) (*app.App, string) {
	t.Helper()
	useLocale(t, "en")
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	config.Reset()
	t.Cleanup(config.Reset)
	s, err := sqlite.Open(filepath.Join(home, "oh.db"))
	require.NoError(t, err)
	prev := store
	store = s
	t.Cleanup(func() { store = prev; s.Close() })
	a := &app.App{Config: &config.Config{}, IO: app.DefaultIOStreams()}
	a.Config.GC.Enabled = true
	wireStores(a, s)
	application = a
	t.Cleanup(func() { application = nil })

	_, err = s.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p', 'p', '/p')`)
	require.NoError(t, err)
	ended := time.Now().Add(-60 * 24 * time.Hour)
	require.NoError(t, a.Sessions.Create(context.Background(), &domain.Session{ID: "ses_old", ProjectID: "p", State: domain.RunStopped,
		StartedAt: ended.Add(-time.Hour), EndedAt: &ended, BundleHash: "deadbeef", Status: domain.SessionStatusCompleted}))
	old := time.Now().Add(-60 * 24 * time.Hour)
	for _, d := range []string{filepath.Join(ohBundlesDir(), "deadbeef"), filepath.Join(ohSessionsDir(), "ses_old")} {
		require.NoError(t, os.MkdirAll(d, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(d, "f"), bytes.Repeat([]byte("x"), 2048), 0o600))
		require.NoError(t, os.Chtimes(d, old, old))
	}
	return a, home
}

func gcRun(t *testing.T, args ...string) (string, error) {
	t.Helper()
	c := &cobra.Command{}
	addGCFlags(c)
	require.NoError(t, c.ParseFlags(args))
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&out)
	c.SetContext(context.Background())
	err := runGC(c, nil)
	return out.String(), err
}

// N16: oh gc lists, refuses without a terminal nor --yes, then removes.
func TestOhGC(t *testing.T) {
	_, _ = gcTestApp(t)
	withTerminal(t, false)
	bundle := filepath.Join(ohBundlesDir(), "deadbeef")

	out, err := gcRun(t, "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out, "Unused session bundles: 1")
	assert.Contains(t, out, "Folders of stopped sessions")
	assert.Contains(t, out, "Dry run")
	assert.DirExists(t, bundle)

	_, err = gcRun(t)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes")
	assert.DirExists(t, bundle)

	out, err = gcRun(t, "--yes")
	require.NoError(t, err)
	assert.Contains(t, out, "removed")
	assert.NoDirExists(t, bundle)
	last, ok := readGCLast()
	require.True(t, ok)
	assert.True(t, last.Armed, "the daemon may clean up on its own now")

	out, err = gcRun(t, "--yes")
	require.NoError(t, err)
	assert.Contains(t, out, "Nothing to clean up")

	_, err = gcRun(t, "--only", "nope")
	assert.ErrorContains(t, err, "unknown category")
	_, err = gcRun(t, "--older-than", "x")
	assert.ErrorContains(t, err, "invalid duration")
}

// N16: the first pass of the daemon only simulates (doctor names what can
// be reclaimed); once oh gc ran, the daemon cleans up; never twice within
// [gc] every_hours.
func TestDaemonGC(t *testing.T) {
	a, _ := gcTestApp(t)
	bundle := filepath.Join(ohBundlesDir(), "deadbeef")
	run := daemonGC(a)

	run(context.Background())
	assert.DirExists(t, bundle, "first pass: simulation")
	last, ok := readGCLast()
	require.True(t, ok)
	assert.True(t, last.DryRun)
	assert.Positive(t, last.Bytes)
	check := gcDoctorCheck(context.Background())
	require.Len(t, check, 1)
	assert.True(t, check[0].Warn)
	assert.Contains(t, check[0].Detail, "run oh gc")

	run(context.Background())
	assert.DirExists(t, bundle, "not again within every_hours")

	writeGCLast(gcLast{At: time.Now().Add(-48 * time.Hour), Armed: true})
	run(context.Background())
	assert.NoDirExists(t, bundle, "armed: the daemon cleans up")
	last, _ = readGCLast()
	assert.False(t, last.DryRun)
	assert.True(t, last.Armed)
	assert.False(t, gcDoctorCheck(context.Background())[0].Warn)

	a.Config.GC.Enabled = false
	writeGCLast(gcLast{At: time.Now().Add(-48 * time.Hour), Armed: true})
	require.NoError(t, os.MkdirAll(bundle, 0o755))
	old := time.Now().Add(-60 * 24 * time.Hour)
	require.NoError(t, os.Chtimes(bundle, old, old))
	run(context.Background())
	assert.DirExists(t, bundle, "[gc] enabled = false")
}

func TestParseAge(t *testing.T) {
	for in, want := range map[string]time.Duration{"": 0, "30d": 30 * 24 * time.Hour, "12h": 12 * time.Hour, "90m": 90 * time.Minute} {
		got, err := parseAge(in)
		require.NoError(t, err)
		assert.Equal(t, want, got, in)
	}
	for _, bad := range []string{"x", "-1d", "3w"} {
		_, err := parseAge(bad)
		assert.Error(t, err, bad)
	}
	kinds, err := parseKinds([]string{"bundles", " history"})
	require.NoError(t, err)
	assert.Equal(t, []gc.Kind{gc.KindBundles, gc.KindHistory}, kinds)
}
