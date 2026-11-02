package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

// launchCommand is a copy of base with its flags, whose outputs are captured.
func launchCommand(t *testing.T, base *cobra.Command) (c *cobra.Command, out, errOut *bytes.Buffer) {
	t.Helper()
	c = &cobra.Command{Use: base.Use}
	base.Flags().VisitAll(func(f *pflag.Flag) { c.Flags().String(f.Name, "", "") })
	out, errOut = &bytes.Buffer{}, &bytes.Buffer{}
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetContext(context.Background())
	return c, out, errOut
}

// withoutTerminal makes canLaunchTUI false, with an app over the given projects.
func withoutTerminal(t *testing.T, cfg *config.Config, projects ...domain.Project) *app.App {
	t.Helper()
	i18ntest.Set(t, "en")
	common.SetNoTUI(true)
	t.Cleanup(func() { common.SetNoTUI(true) })
	a := &app.App{Config: cfg, Projects: &mockProjectStore{projects: projects}, IO: &app.IOStreams{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}}
	orig := application
	application = a
	t.Cleanup(func() { application = orig })
	return a
}

// withFakeBd puts a bd in the PATH (shell script: not on Windows, where ./cmd
// is not tested).
func withFakeBd(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bd"), []byte("#!/bin/sh\n"), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestBoardProject(t *testing.T) {
	root := t.TempDir()
	alpha := domain.Project{ID: "alpha", Name: "alpha", Path: filepath.Join(root, "alpha")}
	beta := domain.Project{ID: "beta", Name: "beta", Path: filepath.Join(root, "beta")}
	for _, p := range []domain.Project{alpha, beta} {
		require.NoError(t, os.MkdirAll(p.Path, 0o755))
	}
	ctx := context.Background()
	i18ntest.Set(t, "en")

	got, err := boardProject(ctx, &mockProjectStore{projects: []domain.Project{alpha, beta}}, "beta")
	require.NoError(t, err)
	assert.Equal(t, "beta", got, "--project wins")

	t.Chdir(beta.Path)
	got, err = boardProject(ctx, &mockProjectStore{projects: []domain.Project{alpha, beta}}, "")
	require.NoError(t, err)
	assert.Equal(t, "beta", got, "the project of the current directory")

	t.Chdir(root)
	got, err = boardProject(ctx, &mockProjectStore{projects: []domain.Project{alpha}}, "")
	require.NoError(t, err)
	assert.Equal(t, "alpha", got, "the only project")

	_, err = boardProject(ctx, &mockProjectStore{projects: []domain.Project{alpha, beta}}, "")
	require.Error(t, err)
	assert.Equal(t, ExitUsage, ExitCode(err))
	assert.Contains(t, err.Error(), "--project")
}

// Without a terminal there is no board to show: exit code 2 and what works
// without one. With no bd, nothing can be listed: the hint, as before.
func TestBoardWithoutTerminal(t *testing.T) {
	withoutTerminal(t, &config.Config{})

	t.Run("bd missing", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		c, out, _ := launchCommand(t, boardCmd)
		require.NoError(t, runBoard(c, nil))
		assert.Contains(t, out.String(), "brew install beads")
	})

	t.Run("bd present", func(t *testing.T) {
		withFakeBd(t)
		c, out, _ := launchCommand(t, boardCmd)
		err := runBoard(c, nil)
		require.Error(t, err)
		assert.Equal(t, ExitUsage, ExitCode(err))
		assert.Contains(t, localizeError(err), "needs a terminal")
		assert.Contains(t, localizeError(err), "oh beads list")
		assert.Empty(t, out.String())
	})
}

func TestTeamBoardWithoutTerminal(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state")
	require.NoError(t, os.MkdirAll(filepath.Join(statePath, ".git"), 0o755))

	t.Run("no team", func(t *testing.T) {
		withoutTerminal(t, &config.Config{})
		c, _, _ := launchCommand(t, teamBoardCmd)
		err := runTeamBoard(c, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "oh team init")
	})

	t.Run("team set up", func(t *testing.T) {
		withoutTerminal(t, &config.Config{Teams: []config.TeamConfig{{ID: "acme", Enabled: true, StateRepo: "git@example.com:acme/state.git", StatePath: statePath}}})
		c, _, _ := launchCommand(t, teamBoardCmd)
		err := runTeamBoard(c, nil)
		require.Error(t, err)
		assert.Equal(t, ExitUsage, ExitCode(err))
		assert.Contains(t, localizeError(err), "needs a terminal")
	})
}

// `oh dashboard` needs a terminal.
func TestDashboardWithoutTerminal(t *testing.T) {
	withoutTerminal(t, &config.Config{})
	c, _, _ := launchCommand(t, dashboardCmd)
	err := runDashboard(c, nil)
	require.Error(t, err)
	assert.Equal(t, ExitUsage, ExitCode(err))
	assert.Contains(t, localizeError(err), "Metrics view")
}

// The options of the commands: --watch is kept for the scripts, out of the help.
func TestBoardOptions(t *testing.T) {
	for _, c := range []*cobra.Command{boardCmd, teamBoardCmd} {
		w := c.Flags().Lookup("watch")
		require.NotNil(t, w, c.CommandPath())
		assert.True(t, w.Hidden, c.CommandPath()+" --watch has no effect: the board refreshes on its own")
	}
	assert.NotNil(t, boardCmd.Flags().Lookup("project"))
}
