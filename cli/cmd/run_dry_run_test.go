package cmd

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/hubcontent"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// dryRunTool is a tool that cannot do anything: a dry-run must never reach
// the adapter (no server, no session).
type dryRunTool struct{ adapters.ToolAdapter }

func (dryRunTool) Name() string                        { return "faketool" }
func (dryRunTool) Capabilities() adapters.Capabilities { return adapters.Capabilities{} }

// dryRunHub is a hub with the embedded content, a git project and a tool that
// fails the test if it is used.
func dryRunHub(t *testing.T) (*app.App, *domain.Project) {
	t.Helper()
	i18ntest.Set(t, "en")
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	config.Reset()
	t.Cleanup(config.Reset)
	require.NoError(t, hubcontent.Extract(hubcontent.HubContentDir()))
	// Session bundles are sealed read-only: let the temporary folder go.
	t.Cleanup(func() {
		_ = filepath.WalkDir(home, func(path string, d os.DirEntry, _ error) error {
			if d != nil && d.IsDir() {
				_ = os.Chmod(path, 0o700)
			}
			return nil
		})
	})

	s, err := sqlite.Open(filepath.Join(home, "oh.db"))
	require.NoError(t, err)
	prev := store
	store = s
	t.Cleanup(func() { store = prev; s.Close() })
	a := &app.App{Config: &config.Config{}, IO: app.DefaultIOStreams()}
	wireStores(a, s)
	prevApp := application
	application = a
	t.Cleanup(func() { application = prevApp })

	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"}} {
		c := exec.Command("git", args...)
		c.Dir = dir
		out, err := c.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	p := &domain.Project{ID: "demo", Name: "demo", Path: dir, Status: domain.ProjectStatusActive}
	require.NoError(t, a.Projects.Create(context.Background(), p))

	prevDetect := detectV2
	detectV2 = func(context.Context) (adapters.ToolAdapter, adapters.ToolInfo, error) {
		return dryRunTool{}, adapters.ToolInfo{Name: "faketool", DisplayName: "faketool", Version: "2.0.25"}, nil
	}
	v5Once, v5Adapter, v5Tool, v5Err = sync.Once{}, nil, adapters.ToolInfo{}, nil
	prevCreds := launchCredentials
	launchCredentials = func(context.Context, *app.App, *domain.Project, string) error { return nil }
	t.Cleanup(func() {
		detectV2, launchCredentials = prevDetect, prevCreds
		v5Once, v5Adapter, v5Tool, v5Err = sync.Once{}, nil, adapters.ToolInfo{}, nil
	})
	return a, p
}

func dryRunCmd(t *testing.T, p *domain.Project, args ...string) (string, error) {
	t.Helper()
	c := &cobra.Command{Use: "run"}
	addRunFlags(c)
	require.NoError(t, c.Flags().Parse(append([]string{"--dry-run", "-p", p.ID}, args...)))
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&out)
	c.SetContext(context.Background())
	err := runWorkflowCmd(c, c.Flags().Args())
	return out.String(), err
}

func worktreeCount(t *testing.T, dir string) int {
	t.Helper()
	c := exec.Command("git", "worktree", "list", "--porcelain")
	c.Dir = dir
	out, err := c.Output()
	require.NoError(t, err)
	return strings.Count(string(out), "worktree ")
}

// oh run --dry-run prepares the launch and shows it, then stops: no session,
// no worktree, no ticket claim, no files for local shells, nothing asked.
func TestRunDryRunStartsNothing(t *testing.T) {
	a, p := dryRunHub(t)
	withTerminal(t, true) // a terminal must not make it ask anything
	asked := withConfirm(t, true)

	out, err := dryRunCmd(t, p, "spec", "-m", "A feature", "--location", "new")
	require.NoError(t, err)
	assert.Contains(t, out, "Preparing workflow spec")
	assert.Contains(t, out, "worktree", "the planned new worktree is shown")
	assert.Contains(t, out, "Planned pauses", "the pauses of the mode are shown")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(out), "dry-run: nothing was started"), out)
	assert.Zero(t, *asked)

	sessions, err := a.Sessions.List(context.Background(), p.ID)
	require.NoError(t, err)
	assert.Empty(t, sessions)
	assert.Equal(t, 1, worktreeCount(t, p.Path), "no worktree created")
	_, err = os.Stat(ohShimsDir())
	assert.True(t, os.IsNotExist(err), "no bd shim written")
	_, err = os.Stat(filepath.Join(config.HubDir(), "run", "shell"))
	assert.True(t, os.IsNotExist(err), "no shell start-up files written")
}

// The prepared launch carries no claim hook and refuses to start.
func TestRunDryRunPreparedRunCannotStart(t *testing.T) {
	a, p := dryRunHub(t)
	prep, err := prepareWorkflowRun(context.Background(), a, runOptions{Workflow: "libre", Project: p, Inputs: map[string]string{}, DryRun: true}, &bytes.Buffer{})
	require.NoError(t, err)
	assert.Nil(t, prep.svc.OnTicketsStarted, "no ticket claim")
	assert.Nil(t, prep.svc.LocalShellEnv)
	_, err = prep.start(context.Background(), a, nil)
	assert.ErrorContains(t, err, "never starts a session")
}

func TestRunDryRunHeadlessRefusesWaitingCheckpoints(t *testing.T) {
	_, p := dryRunHub(t)
	_, err := dryRunCmd(t, p, "spec", "-m", "x", "--headless")
	require.Error(t, err, "spec pauses in its default mode: it cannot run headless")
	out, err := dryRunCmd(t, p, "libre", "--headless")
	require.NoError(t, err)
	assert.Contains(t, out, "dry-run: nothing was started")
}

func TestRunDryRunRemoteRefused(t *testing.T) {
	_, p := dryRunHub(t)
	_, err := dryRunCmd(t, p, "audit", "--runtime", "remote")
	assert.ErrorContains(t, err, "does not prepare a remote launch")
}
