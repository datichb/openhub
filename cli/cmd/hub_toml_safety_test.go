package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
)

// oh init never rewrites an existing hub.toml from an empty config: an invalid
// file stays byte for byte as it was, and the message gives its path and the
// command that repairs it.
func TestInitDoesNotOverwriteAnInvalidHubToml(t *testing.T) {
	configtest.Home(t)
	t.Cleanup(func() { application = nil })
	broken := []byte("[cli\nlanguage = \"fr\"\nsetup_done = true\n[llm]\ndefault_provider = \"bedrock\"\n")
	seedHubToml(t, string(broken))
	before := dirListing(t, config.HubDir())

	a, err := startInitApp()

	require.Error(t, err)
	assert.Nil(t, a)
	assert.Contains(t, err.Error(), config.ConfigPath())
	assert.Contains(t, err.Error(), "oh doctor --fix")
	got, rerr := os.ReadFile(config.ConfigPath())
	require.NoError(t, rerr)
	assert.Equal(t, broken, got, "the original bytes are untouched")
	assert.Equal(t, before, dirListing(t, config.HubDir()), "nothing was created or removed next to it")
}

func dirListing(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// oh doctor --fix replaces hub.toml from a backup: the Config the views hold
// takes the restored file in place, and the broken file is kept.
func TestRepairConfigUpdatesTheSharedConfigInPlace(t *testing.T) {
	configtest.Home(t)
	origReport, origApp := startupReport, application
	t.Cleanup(func() { startupReport, application = origReport, origApp })

	seedHubToml(t, "name = \"restored\"\n")
	require.NoError(t, os.Rename(config.ConfigPath(), config.ConfigPath()+".bak-20260101T000000.000000000Z"))
	seedHubToml(t, "[cli\nbroken")

	held := &config.Config{Name: "held by the views"}
	application = &app.App{Config: held}
	startupReport = &StartupReport{ConfigErr: errors.New("hub.toml does not load")}

	var out bytes.Buffer
	require.NoError(t, repairConfig(&out, alwaysConfirm))

	assert.Nil(t, startupReport.ConfigErr)
	assert.Same(t, held, application.Config, "the pointer the views hold is kept")
	assert.Equal(t, "restored", held.Name)
	kept, _ := filepath.Glob(config.ConfigPath() + ".broken-*")
	require.Len(t, kept, 1)
	data, err := os.ReadFile(kept[0])
	require.NoError(t, err)
	assert.Equal(t, "[cli\nbroken", string(data), "the broken file is kept")
}
