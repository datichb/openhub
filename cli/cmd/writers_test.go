package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// The commands write to the writers of the cobra command (OutOrStdout,
// ErrOrStderr), not to os.Stdout and os.Stderr: a test, or a caller that runs
// oh in its own process, reads them. The commands below used to write to
// os.Stdout.

func writersHome(t *testing.T) {
	t.Helper()
	i18ntest.Set(t, "en")
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	require.NoError(t, os.WriteFile(filepath.Join(home, "hub.toml"), []byte("[cli]\nlanguage = \"en\"\nsetup_done = true\n"), 0o600))
	config.Reset()
	t.Cleanup(config.Reset)
}

func runWriters(t *testing.T, c *cobra.Command, args ...string) (string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errOut)
	require.NoError(t, c.ParseFlags(args))
	if c.RunE != nil {
		require.NoError(t, c.RunE(c, c.Flags().Args()))
	} else {
		c.Run(c, c.Flags().Args())
	}
	return out.String(), errOut.String()
}

func TestConfigCommandsWriteToTheCommandOutput(t *testing.T) {
	writersHome(t)

	out, _ := runWriters(t, configPathCmd())
	assert.Equal(t, config.ConfigPath()+"\n", out)

	out, _ = runWriters(t, configGetCmd(), "cli.language")
	assert.Equal(t, "en\n", out)

	out, _ = runWriters(t, configListCmd())
	assert.Contains(t, out, "cli.language")

	out, _ = runWriters(t, configListCmd(), "--keys")
	assert.Contains(t, out, "session.attach\n")

	out, _ = runWriters(t, configListCmd(), "--all")
	assert.Contains(t, out, "session.attach")

	out, _ = runWriters(t, configListCmd(), "--json")
	assertCompact(t, "config list --json", out)

	out, _ = runWriters(t, configListCmd(), "--keys", "--json")
	assertCompact(t, "config list --keys --json", out)

	out, _ = runWriters(t, configListCmd(), "--all", "--json")
	assertCompact(t, "config list --all --json", out)

	c := configSetCmd()
	var buf bytes.Buffer
	c.SetOut(&buf)
	require.NoError(t, c.RunE(c, []string{"session.attach", "browser"}))
	assert.Contains(t, buf.String(), "session.attach")
	assert.Contains(t, buf.String(), "browser")

	buf.Reset()
	c = configUnsetCmd()
	c.SetOut(&buf)
	require.NoError(t, c.RunE(c, []string{"session.attach"}))
	assert.Contains(t, buf.String(), "session.attach")
}

func TestHubModelWritersWriteToTheGivenWriter(t *testing.T) {
	writersHome(t)
	var buf bytes.Buffer
	require.NoError(t, setHubModelDefault(&buf, "sonnet"))
	assert.Contains(t, buf.String(), "models.default")
	assert.Contains(t, buf.String(), "sonnet")

	buf.Reset()
	require.NoError(t, setHubModelFamily(&buf, "quality", "opus"))
	assert.Contains(t, buf.String(), "quality")

	buf.Reset()
	require.NoError(t, setHubModelAgent(&buf, "reviewer", "haiku"))
	assert.Contains(t, buf.String(), "reviewer")

	buf.Reset()
	require.NoError(t, unsetHubModel(&buf, "models.default"))
	assert.Contains(t, buf.String(), "models.default")
}

func TestVersionWritesToTheCommandOutput(t *testing.T) {
	i18ntest.Set(t, "en")
	out, _ := runWriters(t, versionCmd)
	assert.Contains(t, out, "oh ")
	assert.Contains(t, out, "commit")
}

func TestCompletionWritesToTheCommandOutput(t *testing.T) {
	var out bytes.Buffer
	completionCmd.SetOut(&out)
	t.Cleanup(func() { completionCmd.SetOut(nil) })
	require.NoError(t, completionCmd.RunE(completionCmd, []string{"bash"}))
	assert.Contains(t, out.String(), "bash completion")
}

func TestInitAlreadyInitializedAsksOnTheCommandStreams(t *testing.T) {
	writersHome(t)
	c := &cobra.Command{}
	var out, errOut bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errOut)
	c.SetIn(bytes.NewBufferString("n\n"))
	require.NoError(t, runInit(c, nil))
	assert.Contains(t, errOut.String(), "already")
	assert.Empty(t, out.String())
}
