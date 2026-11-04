package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// anonymisedTeamConfig is the shape of a real GitLab team config (four labels
// mapped to 'todo', four unassigned_labels, an empty [board]), anonymised.
func anonymisedTeamConfig(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "internal", "teamstate", "testdata", "gitlab_team_todo.toml"))
	require.NoError(t, err)
	return string(data)
}

func mappingsSetup(t *testing.T, config string) (*teamstate.Repo, string) {
	t.Helper()
	_, repo := setupBeat(t)
	i18ntest.Set(t, "en")
	path := filepath.Join(repo.Path(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(config), 0o644))
	return repo, path
}

// oh doctor names the clone and each value that is not a status, and says what
// repairs it; a clone without such a value is left alone.
func TestDoctorTrackerMappingsNamesTheValues(t *testing.T) {
	_, repo := setupBeat(t)
	i18ntest.Set(t, "en")
	ctx := context.Background()
	assert.Empty(t, trackerMappingsDoctorChecks(ctx), "nothing set: nothing said")

	require.NoError(t, os.WriteFile(filepath.Join(repo.Path(), "config.toml"), []byte(anonymisedTeamConfig(t)), 0o644))
	rs := trackerMappingsDoctorChecks(ctx)
	require.Len(t, rs, 1)
	r := rs[0]
	assert.Equal(t, doctorIDTrackerMappings, r.ID)
	assert.True(t, r.OK && r.Warn, "a warning, not a failure")
	assert.Contains(t, r.Detail, filepath.Join(repo.Path(), "config.toml"))
	assert.Contains(t, r.Detail, "4 tracker mapping(s)")
	for _, label := range []string{"Todo", "Design to do", "Ready to dev", "US to do"} {
		assert.Contains(t, r.Detail, `"`+label+`" = 'todo'`, label)
	}
	assert.Contains(t, r.Detail, "'planned'")
	assert.Contains(t, r.Detail, "oh doctor --fix")
	assert.NotContains(t, r.Detail, "correct them by hand", "all can be repaired")
	assert.NotNil(t, trackerMappingsFix(r), "the Doctor view offers the repair")
}

func TestDoctorTrackerMappingsSaysWhatNeedsAHand(t *testing.T) {
	mappingsSetup(t, "[tracker]\n[tracker.label_status_mapping]\nOdd = 'wip'\n")
	rs := trackerMappingsDoctorChecks(context.Background())
	require.Len(t, rs, 1)
	assert.Equal(t, doctorIDTrackerMappingsManual, rs[0].ID)
	assert.Contains(t, rs[0].Detail, `"Odd" = 'wip'`)
	assert.Contains(t, rs[0].Detail, "correct them by hand")
	assert.NotContains(t, rs[0].Detail, "oh doctor --fix")
	assert.Nil(t, trackerMappingsFix(rs[0]), "nothing to repair")
}

// oh doctor --fix writes the values as the status they stand for, after a
// confirmation, and the check is green afterwards.
func TestDoctorFixTrackerMappings(t *testing.T) {
	repo, path := mappingsSetup(t, anonymisedTeamConfig(t))
	ctx := context.Background()

	var out bytes.Buffer
	asked := 0
	refuse := func(string) (bool, error) { asked++; return false, nil }
	require.NoError(t, repairTrackerMappings(ctx, &out, refuse))
	assert.Equal(t, 1, asked)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "'todo'", "refused: nothing written")
	assert.Contains(t, out.String(), "'todo' → 'planned'")

	out.Reset()
	require.NoError(t, repairTrackerMappings(ctx, &out, alwaysConfirm))
	raw, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "'todo'")
	assert.Contains(t, string(raw), "Todo = 'planned'")
	assert.Contains(t, string(raw), "'Dev doing' = 'in_progress'", "the other mappings are kept")
	assert.Contains(t, out.String(), "4 mapping(s) rewritten")
	assert.Empty(t, trackerMappingsDoctorChecks(ctx), "green afterwards")

	cfg, err := repo.LoadConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"Design to do", "Ready to dev", "Todo", "US to do"}, cfg.Tracker.UnassignedLabels, "the rest of the config is kept")

	asked = 0
	require.NoError(t, repairTrackerMappings(ctx, &out, refuse))
	assert.Zero(t, asked, "nothing to repair: nothing asked")
}

// A value that stands for nothing is named by the repair, and left alone.
func TestDoctorFixTrackerMappingsLeavesWhatStandsForNothing(t *testing.T) {
	_, path := mappingsSetup(t, "[tracker]\n[tracker.label_status_mapping]\nReady = 'todo'\nOdd = 'wip'\n")
	var out bytes.Buffer
	require.NoError(t, repairTrackerMappings(context.Background(), &out, alwaysConfirm))
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "Ready = 'planned'")
	assert.Contains(t, string(raw), "Odd = 'wip'")
	assert.Contains(t, out.String(), `"Odd" = 'wip'`)
}
