package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// v5.1 (0-C): the commands that change the user's data without a terminal
// are refused (ExitUsage) unless --yes is given; nothing is modified.

func TestGCWithoutTerminalIsRefusedWithExitUsage(t *testing.T) {
	_, _ = gcTestApp(t)
	terminalIs(t, false)
	stubConfirmForm(t)
	bundle := filepath.Join(ohBundlesDir(), "deadbeef")

	_, err := gcRun(t)
	require.Error(t, err)
	assert.Equal(t, ExitUsage, ExitCode(err))
	assert.ErrorContains(t, err, "--yes")
	assert.DirExists(t, bundle, "nothing removed")

	_, err = gcRun(t, "--yes")
	require.NoError(t, err)
	assert.NoDirExists(t, bundle)
}

func TestGCAsksTheCommonForm(t *testing.T) {
	_, _ = gcTestApp(t)
	terminalIs(t, true)
	bundle := filepath.Join(ohBundlesDir(), "deadbeef")

	fa := stubConfirmForm(t, false)
	out, err := gcRun(t)
	require.NoError(t, err)
	assert.Equal(t, []bool{false}, fa.initial, "no is the default")
	assert.Contains(t, out, "Nothing was removed")
	assert.DirExists(t, bundle)

	stubConfirmForm(t, true)
	_, err = gcRun(t)
	require.NoError(t, err)
	assert.NoDirExists(t, bundle)
}

func TestFixConfigWithoutTerminalIsRefusedWithExitUsage(t *testing.T) {
	i18ntest.Set(t, "en")
	isolatedHub(t)
	broken := []byte("this is = = not toml")
	require.NoError(t, os.WriteFile(config.ConfigPath(), broken, 0o600))
	backup := config.ConfigPath() + ".bak-20260101T000000Z"
	require.NoError(t, os.WriteFile(backup, []byte("[cli]\nlanguage = \"en\"\n"), 0o600))
	prev := startupReport
	startupReport = &StartupReport{ConfigErr: errors.New("broken")}
	t.Cleanup(func() { startupReport = prev })
	terminalIs(t, false)
	stubConfirmForm(t)

	var out bytes.Buffer
	err := fixConfig(quietYesCmd(t, &out))
	require.Error(t, err)
	assert.Equal(t, ExitUsage, ExitCode(err))
	got, rerr := os.ReadFile(config.ConfigPath())
	require.NoError(t, rerr)
	assert.Equal(t, broken, got, "hub.toml left as it was")

	require.NoError(t, fixConfig(quietYesCmd(t, &out, "--yes")))
	got, rerr = os.ReadFile(config.ConfigPath())
	require.NoError(t, rerr)
	assert.Contains(t, string(got), "language")
	assert.NotEqual(t, broken, got)
}

// quietYesCmd is a command with --yes, parsed with args, that writes to out.
func quietYesCmd(t *testing.T, out *bytes.Buffer, args ...string) *cobra.Command {
	t.Helper()
	c := yesCmd(t, args...)
	c.SetOut(out)
	c.SetErr(out)
	c.SetContext(context.Background())
	return c
}

// fakeOrphanLister lists one orphan server of a test and records the stops.
type fakeOrphanLister struct {
	adapters.ToolAdapter
	stopped []int
}

func (f *fakeOrphanLister) ServerProcesses(context.Context) ([]adapters.ServerProcess, error) {
	return []adapters.ServerProcess{{PID: 4242, PPID: 1, WorkDir: filepath.Join(os.TempDir(), "TestSomething123", "work")}}, nil
}

func (f *fakeOrphanLister) StopServerProcess(_ context.Context, pid int) error {
	f.stopped = append(f.stopped, pid)
	return nil
}

func TestFixOrphanServersConfirmation(t *testing.T) {
	i18ntest.Set(t, "en")
	isolatedHub(t)
	lister := &fakeOrphanLister{}
	prevA, prevT := v5Adapter, v5Tool
	v5Adapter, v5Tool = lister, adapters.ToolInfo{DisplayName: "tool"}
	t.Cleanup(func() { v5Adapter, v5Tool = prevA, prevT })
	terminalIs(t, false)
	stubConfirmForm(t)

	var out bytes.Buffer
	err := fixOrphanServers(quietYesCmd(t, &out))
	require.Error(t, err)
	assert.Equal(t, ExitUsage, ExitCode(err))
	assert.Empty(t, lister.stopped, "nothing stopped")

	require.NoError(t, fixOrphanServers(quietYesCmd(t, &out, "--yes")))
	assert.Equal(t, []int{4242}, lister.stopped)
}

func TestRepairHasTheYesFlag(t *testing.T) {
	f := repairCmd.Flags().Lookup("yes")
	require.NotNil(t, f)
	assert.Equal(t, "y", f.Shorthand)
}
