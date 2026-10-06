package opencodev2

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsolatedConfigHome(t *testing.T) {
	user := t.TempDir()
	for _, d := range []string{"opencode", "git", "gh"} {
		require.NoError(t, os.MkdirAll(filepath.Join(user, d), 0o755))
	}
	t.Setenv("XDG_CONFIG_HOME", user)
	dir := filepath.Join(t.TempDir(), "config-home")

	got, err := isolatedConfigHome(dir)
	require.NoError(t, err)
	assert.Equal(t, dir, got)
	_, err = os.Lstat(filepath.Join(dir, "opencode"))
	assert.True(t, os.IsNotExist(err), "the user opencode config is hidden")
	target, err := os.Readlink(filepath.Join(dir, "git"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(user, "git"), target, "other tools keep their config")

	// Rebuilt at each start: a removed entry disappears, a new one appears.
	require.NoError(t, os.RemoveAll(filepath.Join(user, "gh")))
	require.NoError(t, os.MkdirAll(filepath.Join(user, "npm"), 0o755))
	_, err = isolatedConfigHome(dir)
	require.NoError(t, err)
	_, err = os.Lstat(filepath.Join(dir, "gh"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Readlink(filepath.Join(dir, "npm"))
	assert.NoError(t, err)

	// No user config home: an empty directory.
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(user, "missing"))
	empty := filepath.Join(t.TempDir(), "c")
	_, err = isolatedConfigHome(empty)
	require.NoError(t, err)
	entries, _ := os.ReadDir(empty)
	assert.Empty(t, entries)
}
