package cmd

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// oh config writes the limit of tickets in progress under its current key;
// 0 is no limit, a negative value is refused.
func TestConfigSetsTheInProgressLimit(t *testing.T) {
	writersHome(t)

	out, _ := runWriters(t, configSetCmd(), "tracker.max_in_progress_per_member", "3")
	assert.Contains(t, out, "tracker.max_in_progress_per_member")
	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg.Tracker.MaxInProgressPerMember)
	assert.Equal(t, 3, *cfg.Tracker.MaxInProgressPerMember)

	runWriters(t, configSetCmd(), "tracker.max_in_progress_per_member", "0")
	cfg, err = config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg.Tracker.MaxInProgressPerMember)
	assert.Zero(t, *cfg.Tracker.MaxInProgressPerMember, "0 is no limit, kept")

	for _, bad := range []string{"-1", "many", "2.5"} {
		c := configSetCmd()
		c.SetOut(&bytes.Buffer{})
		err := c.RunE(c, []string{"tracker.max_in_progress_per_member", bad})
		require.Error(t, err, bad)
		assert.Contains(t, err.Error(), bad)
	}

	runWriters(t, configUnsetCmd(), "tracker.max_in_progress_per_member")
	cfg, err = config.Load()
	require.NoError(t, err)
	assert.Nil(t, cfg.Tracker.MaxInProgressPerMember)
}

// The former key of oh config is accepted as the new one, and announced with
// the key that replaces it; nothing is written under the former name.
func TestConfigFormerAutoPlanKeyWritesTheNewKey(t *testing.T) {
	writersHome(t)
	var notes []config.Notice
	config.SetNoticeHandler(func(n config.Notice) { notes = append(notes, n) })
	config.DeliverNotices()

	runWriters(t, configSetCmd(), "tracker.max_auto_plan_per_member", "7")
	require.Len(t, notes, 1)
	assert.Equal(t, "tracker.max_in_progress_per_member", notes[0].Replacement)

	data, err := os.ReadFile(config.ConfigPath())
	require.NoError(t, err)
	assert.Contains(t, string(data), "max_in_progress_per_member = 7")
	assert.NotContains(t, string(data), "max_auto_plan_per_member")
}

// oh doctor tells a hub.toml that still sets the former key which command
// writes the new one.
func TestDoctorFormerAutoPlanKeyInHubToml(t *testing.T) {
	defer i18n.SetLocale("en")
	i18n.SetLocale("en")
	configtest.Home(t)
	writeHubToml(t, "[tracker]\nmax_auto_plan_per_member = 4\n")
	checks := legacyKeysChecks(context.Background())
	require.NotEmpty(t, checks)
	assert.True(t, checks[0].Warn)
	assert.Contains(t, checks[0].Detail, "tracker.max_auto_plan_per_member → tracker.max_in_progress_per_member")
	assert.Contains(t, checks[0].Detail, "oh config set tracker.max_in_progress_per_member")
}

// oh doctor tells a team whose config.toml still sets the former key how to
// write the new one; a team that does not is left alone.
func TestDoctorFormerAutoPlanKeyInTheTeamConfig(t *testing.T) {
	a, repo := setupBeat(t)
	i18ntest.Set(t, "en")
	ctx := context.Background()

	assert.Empty(t, trackerLimitResults(ctx, a), "nothing set: nothing said")

	require.NoError(t, os.WriteFile(repo.Path()+"/config.toml", []byte("[tracker]\nmax_auto_plan_per_member = 9\n"), 0o644))
	rs := trackerLimitResults(ctx, a)
	require.Len(t, rs, 1)
	assert.Equal(t, doctorIDTrackerLimit, rs[0].ID)
	assert.True(t, rs[0].OK && rs[0].Warn)
	assert.Contains(t, rs[0].Detail, "max_auto_plan_per_member = 9")
	assert.Contains(t, rs[0].Detail, "oh team config")
	assert.Contains(t, rs[0].Detail, "max_in_progress_per_member")

	// Writing the config with the new key clears the finding.
	cfg, err := repo.LoadConfig(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.SaveConfig(ctx, cfg))
	assert.Empty(t, trackerLimitResults(ctx, a))
	got, err := repo.LoadConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, 9, got.Tracker.InProgressLimit())
	assert.False(t, got.Tracker.UsesFormerInProgressKey())
}
