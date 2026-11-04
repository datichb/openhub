package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

func soloTeamState(t *testing.T) *teamstate.Repo {
	t.Helper()
	repo, err := teamstate.InitSolo(t.Context(), filepath.Join(t.TempDir(), "state"), teamstate.Member{ID: "alice"})
	require.NoError(t, err)
	return repo
}

var editorColumns = []teamstate.BoardColumnConfig{
	{ID: "backlog", Name: "BACKLOG", Role: teamstate.ColumnRoleInitial},
	{ID: "doing", Name: "DOING", Role: teamstate.ColumnRoleActive},
	{ID: "done", Name: "DONE", Role: teamstate.ColumnRoleTerminal},
}

// The editor of the columns writes what it changed on top of the latest
// config.toml: the settings a colleague changed while it was open stay.
func TestSaveBoardColumnsKeepsWhatAColleagueChangedMeanwhile(t *testing.T) {
	repo := soloTeamState(t)
	base, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)

	require.NoError(t, repo.UpdateConfig(t.Context(), func(c *teamstate.TeamConfig) error {
		c.Tracker.Type = "jira"
		c.Tracker.LabelStatusMapping = map[string]string{"Review": "done"}
		return nil
	}))

	saveBoardColumns(t.Context(), repo, base, editorColumns, map[string]string{"Ready": "backlog"}, nil, nil)

	got, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, editorColumns, got.Board.Columns)
	assert.Equal(t, "jira", got.Tracker.Type, "the type the colleague set is not erased by the old copy")
	assert.Equal(t, "backlog", got.Tracker.LabelStatusMapping["Ready"])
	assert.Equal(t, "done", got.Tracker.LabelStatusMapping["Review"], "their mapping stays")
}

// The columns changed by a colleague meanwhile are not overwritten.
func TestSaveBoardColumnsDoesNotOverwriteColumnsChangedMeanwhile(t *testing.T) {
	repo := soloTeamState(t)
	base, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	theirs := []teamstate.BoardColumnConfig{
		{ID: "todo", Name: "TODO", Role: teamstate.ColumnRoleInitial}, {ID: "end", Name: "END", Role: teamstate.ColumnRoleTerminal},
	}
	require.NoError(t, repo.UpdateConfig(t.Context(), func(c *teamstate.TeamConfig) error {
		c.Board.Columns = theirs
		return nil
	}))

	saveBoardColumns(t.Context(), repo, base, editorColumns, nil, nil, func() { t.Fatal("saved: afterSave must not run") })

	got, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, theirs, got.Board.Columns)
	assert.Contains(t, configSaveErrorText(&teamstate.ErrConfigConflict{Keys: []string{"board.columns"}}), "board.columns")
}

// The discovery sets what it settled and leaves the rest of config.toml.
func TestApplyDiscoveryLeavesTheOtherSettings(t *testing.T) {
	repo := soloTeamState(t)
	require.NoError(t, repo.UpdateConfig(t.Context(), func(c *teamstate.TeamConfig) error {
		c.Notification.Channel = "dev"
		c.Takeover.StaleDays = 8
		return nil
	}))
	require.NoError(t, repo.UpdateConfig(t.Context(), func(c *teamstate.TeamConfig) error {
		applyDiscovery(c, discoveryResult{trackerType: "gitlab", trackerURL: "https://git.example", projectID: "a/b", columns: editorColumns, pool: []string{"ready"}})
		return nil
	}))
	got, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "dev", got.Notification.Channel)
	assert.Equal(t, 8, got.Takeover.StaleDays)
	assert.Equal(t, "gitlab", got.Tracker.Type)
	assert.True(t, got.Tracker.Enabled)
	assert.Equal(t, "a/b", got.Tracker.TrackerProject)
	assert.Equal(t, []string{"ready"}, got.Tracker.UnassignedLabels)
}
