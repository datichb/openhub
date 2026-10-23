package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// hubWithTracker is a hub whose team core has a cloned team-state with a
// GitLab tracker (the token comes from GITLAB_TOKEN, the URL is unreachable:
// the tests that must not reach it assert on the error).
func hubWithTracker(t *testing.T, localTracker config.TrackerLocalConfig) (*app.App, *bytes.Buffer) {
	t.Helper()
	configtest.Home(t)
	t.Setenv("GITLAB_TOKEN", "glpat-test")
	t.Setenv("OH_GITLAB_URL", "http://127.0.0.1:1") // the server GITLAB_TOKEN is valid for
	t.Setenv("GITLAB_URL", "")

	statePath := filepath.Join(t.TempDir(), "state")
	_, err := teamstate.InitSolo(context.Background(), statePath, teamstate.Member{ID: "alice"})
	require.NoError(t, err)
	toml := "[tracker]\ntype = \"gitlab\"\nenabled = true\ntracker_url = \"http://127.0.0.1:1\"\ntracker_project = \"acme/app\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(statePath, "config.toml"), []byte(toml), 0o644))

	cfg := &config.Config{
		Teams:   []config.TeamConfig{{ID: "core", Enabled: true, StateRepo: "git@h:acme/core.git", StatePath: statePath, MemberID: "alice"}},
		Tracker: localTracker,
	}
	out := &bytes.Buffer{}
	io := app.DefaultIOStreams()
	io.Out, io.ErrOut = out, out
	a := &app.App{Config: cfg, IO: io}
	application = a
	teamSyncTrackerCmd.SetContext(t.Context())
	t.Cleanup(func() { application = nil })
	return a, out
}

func disabledLocally() config.TrackerLocalConfig {
	off := false
	return config.TrackerLocalConfig{Enabled: &off}
}

// The sync honors [tracker] enabled = false everywhere: the command line
// prints a notice, the screen refuses with a message, the automatic engine
// skips. (The screen used to sync anyway.)
func TestTrackerDisabledLocallyIsHonoredByCLITUIAndAutoEngine(t *testing.T) {
	a, out := hubWithTracker(t, disabledLocally())

	t.Run("screen", func(t *testing.T) {
		res, err := runSyncTrackerForTUI(a, t.Context())
		assert.Nil(t, res)
		require.Error(t, err)
		assert.Equal(t, i18n.T("tui.team.sync.tracker_disabled"), err.Error())
	})
	t.Run("command line", func(t *testing.T) {
		out.Reset()
		require.NoError(t, runSyncTracker(teamSyncTrackerCmd, nil), "a notice, not a failure")
		assert.Contains(t, out.String(), "enabled = false")
	})
	t.Run("automatic engine", func(t *testing.T) {
		assert.Nil(t, resolveTrackerEngine(t.Context(), a))
	})
}

// The same typed error is a message on the command line and another one on
// the screen.
func TestSyncTrackerErrorsAreTranslated(t *testing.T) {
	t.Run("no team", func(t *testing.T) {
		a, _ := hubWithTracker(t, config.TrackerLocalConfig{})
		a.Config.Teams = nil
		err := runSyncTracker(teamSyncTrackerCmd, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "oh team init")
		_, tuiErr := runSyncTrackerForTUI(a, t.Context())
		assert.Equal(t, i18n.T("tui.team.sync.team_not_configured"), tuiErr.Error())
	})
	t.Run("not cloned", func(t *testing.T) {
		a, _ := hubWithTracker(t, config.TrackerLocalConfig{})
		a.Config.Teams[0].StatePath = filepath.Join(t.TempDir(), "nothing")
		err := runSyncTracker(teamSyncTrackerCmd, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "oh team init")
		_, tuiErr := runSyncTrackerForTUI(a, t.Context())
		assert.Equal(t, i18n.T("tui.team.sync.not_cloned"), tuiErr.Error())
	})
	t.Run("no hub project linked to the tracker", func(t *testing.T) {
		a, out := hubWithTracker(t, config.TrackerLocalConfig{})
		require.NoError(t, runSyncTracker(teamSyncTrackerCmd, nil), "a notice on stderr, not a failure")
		assert.NotEmpty(t, out.String())
		_, tuiErr := runSyncTrackerForTUI(a, t.Context())
		assert.Equal(t, i18n.T("tui.team.sync.no_projects"), tuiErr.Error())
	})
}

func TestSyncTrackerTUICredentialsErrorIsTyped(t *testing.T) {
	a, _ := hubWithTracker(t, config.TrackerLocalConfig{})
	t.Setenv("GITLAB_TOKEN", "")

	_, err := runSyncTrackerForTUI(a, t.Context())
	var credErr *views.TrackerCredentialsError
	require.ErrorAs(t, err, &credErr)
	assert.Equal(t, i18n.Tf("tui.team.sync.token_missing_server", "127.0.0.1:1"), credErr.Msg, "the screen names the server")
	assert.True(t, tracker.IsMissingToken(credErr.Err), "the detail (sources, declaration) is kept for the modal")

	// the command line prints the whole detail, without the former generic hint
	err = runSyncTracker(teamSyncTrackerCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "127.0.0.1:1")
	assert.Contains(t, err.Error(), "GITLAB_TOKEN")
	assert.Contains(t, err.Error(), "oh config set tracker.tracker_token_url")
}
