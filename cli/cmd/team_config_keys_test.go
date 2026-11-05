package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// runTeamConfigKey runs oh team config <sub> through the pre-run of oh team (--team is its flag).
func runTeamConfigKey(t *testing.T, sub *cobra.Command, args ...string) error {
	t.Helper()
	c := &cobra.Command{Use: sub.Name(), RunE: sub.RunE}
	sub.Flags().VisitAll(func(fl *pflag.Flag) { _ = fl.Value.Set(fl.DefValue); fl.Changed = false })
	c.Flags().AddFlagSet(sub.Flags())
	addTeamFlag(c)
	c.SetContext(t.Context())
	require.NoError(t, c.ParseFlags(args))
	if err := teamPreRunE(c, nil); err != nil {
		return err
	}
	return sub.RunE(c, c.Flags().Args())
}

func TestTeamConfigGetSetUnsetShow(t *testing.T) {
	i18ntest.Set(t, "en")
	configtest.Home(t)
	a, repo, out := setupClaimTestApp(t)
	require.NoError(t, config.Save(a.Config))

	require.NoError(t, runTeamConfigKey(t, teamConfigSetCmd, "claim.lease_hours", "12"))
	assert.Contains(t, out.String(), "claim.lease_hours")
	got, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 12, got.Claim.LeaseHours)

	out.Reset()
	require.NoError(t, runTeamConfigKey(t, teamConfigGetCmd, "claim.lease_hours"))
	assert.Equal(t, "12\n", out.String())

	// typed: a bound, a type and an unknown key are refused with the reason, nothing is written
	err = runTeamConfigKey(t, teamConfigSetCmd, "takeover.stale_days", "99")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "between 1 and 30")
	err = runTeamConfigKey(t, teamConfigSetCmd, "notification.enabled", "maybe")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "true or false")
	err = runTeamConfigKey(t, teamConfigSetCmd, "tracker.nope", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tracker.nope")

	// notifications: enabled and the webhook, shown masked
	out.Reset()
	require.NoError(t, runTeamConfigKey(t, teamConfigSetCmd, "notification.enabled", "true"))
	require.NoError(t, runTeamConfigKey(t, teamConfigSetCmd, "notification.type", "slack"))
	require.NoError(t, runTeamConfigKey(t, teamConfigSetCmd, "notification.webhook_url", "https://hooks.slack.com/services/T0/B0/verysecretvalue"))
	assert.NotContains(t, out.String(), "verysecretvalue", "the webhook is never echoed")
	out.Reset()
	require.NoError(t, runTeamConfigKey(t, teamConfigGetCmd, "notification.webhook_url"))
	assert.NotContains(t, out.String(), "verysecretvalue")
	assert.Contains(t, out.String(), "hooks.slack.com")
	got, err = repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "https://hooks.slack.com/services/T0/B0/verysecretvalue", got.Notification.WebhookURL)
	assert.True(t, got.Notification.Enabled)

	// a list item by index, a list of strings, a map entry
	require.NoError(t, runTeamConfigKey(t, teamConfigSetCmd, "notification.destinations.0.type", "discord"))
	require.NoError(t, runTeamConfigKey(t, teamConfigSetCmd, "notification.destinations.0.webhook_url", "https://discord.com/api/webhooks/1/abc"))
	require.NoError(t, runTeamConfigKey(t, teamConfigSetCmd, "tracker.unassigned_labels", "bug, ready"))
	require.NoError(t, runTeamConfigKey(t, teamConfigSetCmd, "models.families.opus", "claude-opus"))
	got, err = repo.LoadConfig(t.Context())
	require.NoError(t, err)
	require.Len(t, got.Notification.Destinations, 1)
	assert.Equal(t, "discord", got.Notification.Destinations[0].Type)
	assert.Equal(t, []string{"bug", "ready"}, got.Tracker.UnassignedLabels)
	assert.Equal(t, "claude-opus", got.Models.Families["opus"])

	// unset: the default comes back and the key leaves the file
	require.NoError(t, runTeamConfigKey(t, teamConfigUnsetCmd, "claim.lease_hours"))
	require.NoError(t, runTeamConfigKey(t, teamConfigUnsetCmd, "models.families.opus"))
	require.NoError(t, runTeamConfigKey(t, teamConfigUnsetCmd, "notification.destinations.0"))
	got, err = repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 24, got.Claim.LeaseHours)
	assert.Empty(t, got.Models.Families)
	assert.Empty(t, got.Notification.Destinations)
	raw, err := os.ReadFile(filepath.Join(repo.Path(), "config.toml"))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "lease_hours")

	// show --origin: default, team, me
	require.NoError(t, config.Update(func(c *config.Config) error {
		on := true
		c.Tracker.AutoSync = &on
		return nil
	}))
	a.Config.Tracker.AutoSync = boolPtr(true)
	out.Reset()
	require.NoError(t, runTeamConfigKey(t, teamConfigSetCmd, "tracker.type", "jira"))
	out.Reset()
	require.NoError(t, runTeamConfigKey(t, teamConfigShowCmd, "--origin"))
	text := out.String()
	assert.Contains(t, text, "tracker.type = jira  ")
	assert.Contains(t, text, "[team]")
	assert.Contains(t, text, "claim.lease_hours = 24")
	assert.Contains(t, text, "[default]")
	assert.Contains(t, text, "tracker.auto_sync = false")
	assert.Contains(t, text, "[me, hub.toml]")
	assert.NotContains(t, text, "verysecretvalue")

	// the activity keeps a trace of each publication
	events, err := repo.ListEvents(t.Context(), "_team", time.Time{})
	require.NoError(t, err)
	assert.NotEmpty(t, events)
	for _, e := range events {
		assert.Equal(t, "testuser", e.Actor)
	}
}

// oh team config status shows the notifications (the webhook masked) and what is out of date.
func TestTeamConfigStatusShowsNotificationsAndIssues(t *testing.T) {
	i18ntest.Set(t, "en")
	configtest.Home(t)
	a, repo, out := setupClaimTestApp(t)
	require.NoError(t, config.Save(a.Config))
	require.NoError(t, os.WriteFile(filepath.Join(repo.Path(), "config.toml"),
		[]byte("[notification]\nenabled = true\nmattermost_webhook = \"https://chat.example.com/hooks/verysecretvalue\"\n[parallel]\nmax_sessions = 5\n"), 0o644))
	require.NoError(t, runTeamConfigKey(t, teamConfigStatusCmd))
	text := out.String()
	assert.Contains(t, text, "Team notifications")
	assert.Contains(t, text, "mattermost · https://chat.example.com/…alue")
	assert.NotContains(t, text, "verysecretvalue")
	assert.Contains(t, text, "[parallel]")
	assert.Contains(t, text, "oh doctor --fix")
}
