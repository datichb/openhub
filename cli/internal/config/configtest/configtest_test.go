package configtest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/testenv"
)

func TestMain(m *testing.M) {
	finish := testenv.Isolate()
	os.Exit(finish(m.Run()))
}

func TestHome(t *testing.T) {
	t.Setenv("OH_HOME", filepath.Join(t.TempDir(), "elsewhere"))
	var firstHome string

	t.Run("hub in the temporary home", func(t *testing.T) {
		home := configtest.Home(t)
		firstHome = home
		assert.Equal(t, home, os.Getenv("HOME"), "HOME is the returned directory")
		assert.Equal(t, filepath.Join(home, ".oh"), config.HubDir(), "the hub is under that home, whatever OH_HOME was")

		require.NoError(t, os.MkdirAll(config.HubDir(), 0o755), "creating the hub")
		require.NoError(t, os.WriteFile(config.ConfigPath(), []byte("[cli]\nlanguage = \"fr\"\n"), 0o644), "writing hub.toml")
		cfg, err := config.Load()
		require.NoError(t, err, "loading the hub")
		assert.Equal(t, "fr", cfg.CLI.Language, "the config is read from the temporary hub")
	})

	t.Run("nothing leaks to the next test", func(t *testing.T) {
		home := configtest.Home(t)
		assert.NotEqual(t, firstHome, home, "each call has its own directory")
		cfg, err := config.Load()
		require.NoError(t, err, "loading the new hub")
		assert.NotEqual(t, "fr", cfg.CLI.Language, "the cache of the previous test is gone")
	})
}
