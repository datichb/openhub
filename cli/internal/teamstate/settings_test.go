package teamstate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

func TestSettings_GetSetUnset(t *testing.T) {
	i18ntest.Set(t, "en")
	cfg := &TeamConfig{Takeover: TakeoverConfig{StaleDays: 3}}
	set := func(key, value string) { t.Helper(); require.NoError(t, cfg.Set(key, value), key) }

	set("tracker.type", "gitlab")
	set("tracker.enabled", "true")
	set("tracker.auto_plan_unassigned", "yes")
	set("tracker.max_in_progress_per_member", "7")
	set("tracker.unassigned_labels", "a, b ,, c")
	set("tracker.status_mapping.In QA", "validation")
	set("claim.lease_hours", "10")
	set("mcp.gitlab.url", "https://gl.example.com")
	set("mcp.gitlab.enabled", "false")
	set("limits.enforced.session_budget_usd", "2.5")
	set("limits.recommended.models", "eu.*")
	set("notification.destinations.0.type", "slack")
	set("notification.destinations.0.webhook_url", "https://hooks.slack.com/services/T/B/x")
	set("governance.config", "lead")

	assert.Equal(t, "gitlab", cfg.Tracker.Type)
	assert.True(t, cfg.Tracker.Enabled)
	assert.True(t, cfg.Tracker.AutoPlanUnassigned)
	require.NotNil(t, cfg.Tracker.MaxInProgressPerMember)
	assert.Equal(t, 7, *cfg.Tracker.MaxInProgressPerMember)
	assert.Equal(t, []string{"a", "b", "c"}, cfg.Tracker.UnassignedLabels)
	assert.Equal(t, "validation", cfg.Tracker.StatusMapping["In QA"])
	assert.Equal(t, 10, cfg.Claim.LeaseHours)
	assert.Equal(t, "https://gl.example.com", cfg.MCP["gitlab"].URL)
	require.NotNil(t, cfg.MCP["gitlab"].Enabled)
	assert.False(t, *cfg.MCP["gitlab"].Enabled)
	assert.Equal(t, 2.5, cfg.Limits.Enforced.SessionBudgetUSD)
	assert.Equal(t, []string{"eu.*"}, cfg.Limits.Recommended.Models)
	require.Len(t, cfg.Notification.Destinations, 1)
	assert.Equal(t, "lead", cfg.Governance.Config)

	for key, want := range map[string]string{
		"tracker.max_in_progress_per_member": "7", "tracker.unassigned_labels": "a, b, c", "mcp.gitlab.enabled": "false",
		"limits.enforced.session_budget_usd": "2.5", "claim.lease_hours": "10", "tracker.status_mapping.In QA": "validation",
		"tracker.type_enforced_no": "",
	} {
		got, err := cfg.Get(key)
		if key == "tracker.type_enforced_no" {
			assert.Error(t, err)
			continue
		}
		require.NoError(t, err, key)
		assert.Equal(t, want, got, key)
	}
	got, err := cfg.Get("mcp.figma.url")
	require.NoError(t, err)
	assert.Empty(t, got, "an entry that does not exist reads empty")
	got, err = cfg.Get("notification.destinations")
	require.NoError(t, err)
	assert.Equal(t, "1 item(s)", got)

	// unset: the default, the removal of a key, of an item, of an entry
	require.NoError(t, cfg.Unset("claim.lease_hours"))
	assert.Equal(t, DefaultLeaseHours, cfg.Claim.LeaseHours)
	require.NoError(t, cfg.Unset("takeover.stale_days"))
	assert.Equal(t, 3, cfg.Takeover.StaleDays)
	require.NoError(t, cfg.Unset("tracker.max_in_progress_per_member"))
	assert.Nil(t, cfg.Tracker.MaxInProgressPerMember)
	require.NoError(t, cfg.Unset("tracker.status_mapping.In QA"))
	assert.Empty(t, cfg.Tracker.StatusMapping)
	require.NoError(t, cfg.Unset("mcp.gitlab"))
	assert.Empty(t, cfg.MCP)
	require.NoError(t, cfg.Unset("notification.destinations.0"))
	assert.Empty(t, cfg.Notification.Destinations)
	require.NoError(t, cfg.Unset("notification.destinations.4"))
	require.NoError(t, cfg.Unset("models.families.nope"))

	// refusals
	for _, bad := range [][2]string{
		{"tracker.enabled", "maybe"}, {"claim.lease_hours", "ten"}, {"limits.enforced.session_budget_usd", "x"},
		{"tracker", "x"}, {"nope", "x"}, {"tracker.nope", "x"}, {"notification.destinations.3.type", "slack"},
		{"board.columns", "a,b"}, {"mcp.gitlab.nope", "x"},
	} {
		assert.Error(t, cfg.Set(bad[0], bad[1]), bad[0])
	}
}

func TestSettings_ListAndMask(t *testing.T) {
	i18ntest.Set(t, "en")
	cfg := &TeamConfig{}
	require.NoError(t, cfg.Set("notification.webhook_url", "https://chat.example.com/hooks/abcdefgh1234"))
	require.NoError(t, cfg.Set("tracker.type", "jira"))
	byKey := map[string]string{}
	for _, s := range cfg.Settings() {
		byKey[s.Key] = s.Value
	}
	assert.Equal(t, "jira", byKey["tracker.type"])
	assert.Contains(t, byKey, "claim.heartbeat_minutes")
	assert.True(t, IsSecretSetting("notification.webhook_url"))
	assert.True(t, IsSecretSetting("notification.destinations.2.webhook_url"))
	assert.False(t, IsSecretSetting("notification.channel"))
	assert.Equal(t, "https://chat.example.com/…1234", SettingText("notification.webhook_url", byKey["notification.webhook_url"]))
	assert.Equal(t, "dev", SettingText("notification.channel", "dev"))
	assert.Equal(t, "", MaskWebhook(""))
	assert.Equal(t, "••••", MaskWebhook("not a url"))

	set := map[string]bool{"tracker.type": true, "notification.destinations": true}
	assert.True(t, InFile(set, "tracker.type"))
	assert.True(t, InFile(set, "notification.destinations.0.type"))
	assert.False(t, InFile(set, "tracker.enabled"))
}

func TestInferNotifierType(t *testing.T) {
	t.Parallel()
	for url, want := range map[string]string{
		"https://hooks.slack.com/services/T/B/x":         "slack",
		"https://discord.com/api/webhooks/1/abc":         "discord",
		"https://contoso.webhook.office.com/webhookb2/x": "teams",
		"https://chat.example.com/hooks/abc":             "mattermost",
		"":                                               "mattermost",
	} {
		assert.Equal(t, want, InferNotifierType(url), url)
	}
	n := NotificationConfig{MattermostWebhook: "https://hooks.slack.com/services/T/B/x"}
	assert.Equal(t, "slack", n.EffectiveType())
	assert.Equal(t, "https://hooks.slack.com/services/T/B/x", n.EffectiveURL())
	assert.True(t, n.UsesFormerWebhookKey())
	assert.Equal(t, "teams", NotificationConfig{Type: "teams", WebhookURL: "https://x/y"}.EffectiveType())
	assert.Empty(t, NotificationConfig{}.EffectiveType())
}
