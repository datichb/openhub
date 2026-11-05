package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The r key of the board on a team that has no tracker to synchronize is the
// pull alone: ErrNoTracker, not a failure.
func TestBoardSyncWithoutTrackerIsNotAFailure(t *testing.T) {
	t.Run("tracker disabled", func(t *testing.T) {
		a, _ := hubWithTracker(t, disabledLocally())
		_, err := syncTrackerForBoard(a, t.Context())
		assert.ErrorIs(t, err, views.ErrNoTracker)
	})
	t.Run("no hub project linked", func(t *testing.T) {
		a, _ := hubWithTracker(t, config.TrackerLocalConfig{})
		_, err := syncTrackerForBoard(a, t.Context())
		assert.ErrorIs(t, err, views.ErrNoTracker)
	})
	t.Run("no team", func(t *testing.T) {
		a, _ := hubWithTracker(t, config.TrackerLocalConfig{})
		a.Config.Teams = nil
		_, err := syncTrackerForBoard(a, t.Context())
		assert.ErrorIs(t, err, views.ErrNoTracker)
	})
	t.Run("not cloned", func(t *testing.T) {
		a, _ := hubWithTracker(t, config.TrackerLocalConfig{})
		a.Config.Teams[0].StatePath = filepath.Join(t.TempDir(), "nothing")
		_, err := syncTrackerForBoard(a, t.Context())
		assert.ErrorIs(t, err, views.ErrNoTracker)
	})
	t.Run("a missing token is a failure the user can act on", func(t *testing.T) {
		a, _ := hubWithTracker(t, config.TrackerLocalConfig{})
		t.Setenv("GITLAB_TOKEN", "")
		_, err := syncTrackerForBoard(a, t.Context())
		var credErr *views.TrackerCredentialsError
		require.ErrorAs(t, err, &credErr)
	})
}

// The automatic sync of the opening of the board says nothing when it has
// nothing to do (auto_sync off, no tracker) and does not call the tracker.
func TestAutoSyncForTUIIsSilentWhenThereIsNothingToDo(t *testing.T) {
	a, _ := hubWithTracker(t, config.TrackerLocalConfig{})
	summary, err := autoSyncForTUI(a, t.Context())
	require.NoError(t, err, "auto_sync is off")
	assert.Empty(t, summary)

	a, _ = hubWithTracker(t, disabledLocally())
	summary, err = autoSyncForTUI(a, t.Context())
	require.NoError(t, err)
	assert.Empty(t, summary)
}

func TestSyncSummary(t *testing.T) {
	i18ntest.Set(t, "en")
	r := &tracker.SyncResult{ClaimsCreated: 2, ClaimsUpdated: 3, ClaimsReleased: 1,
		Warnings: []tracker.SyncWarning{{Message: "w"}}, Errors: []tracker.SyncError{{ProjectID: "p", Err: assert.AnError}}}
	assert.Equal(t, "Tracker sync: 2 created, 3 updated, 1 released, 2 warning(s)", syncSummary(r))
	assert.True(t, syncChangedSomething(r))
	assert.False(t, syncChangedSomething(&tracker.SyncResult{}))
	assert.Equal(t, i18n.Tf("v54y.sync.summary", 0, 0, 0, 0), syncSummary(&tracker.SyncResult{}))
}
