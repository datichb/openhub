package teamstate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func addMembers(t *testing.T, r *Repo) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, r.AddMember(ctx, Member{ID: "lea", DisplayName: "Léa", Role: "lead"}))
	require.NoError(t, r.AddMember(ctx, Member{ID: "dan", DisplayName: "Dan", Role: "dev"}))
}

func TestChangeNeedsLead(t *testing.T) {
	t.Parallel()
	for key, want := range map[string]bool{
		"tracker.push_labels_enforced": true, "mcp.gitlab.url_enforced": true, "mcp.gitlab.enabled_enforced": true,
		"limits.enforced.max_budget": true, "limits.enforced": true, "governance.config": true, "governance.publish": true,
		"tracker.push_labels": false, "limits.recommended.max_budget": false, "notification.webhook_url": false, "board.columns": false,
	} {
		assert.Equal(t, want, ChangeNeedsLead(key), key)
	}
}

// The enforced settings are for the leads: a dev is refused with the keys, a lead is not, and
// nothing is written for the refused one.
func TestUpdateConfig_EnforcedSettingsAreReservedToLeads(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	addMembers(t, repo)
	yes := true
	enforce := func(c *TeamConfig) error { c.Tracker.PushLabelsEnforced = &yes; return nil }

	err := repo.UpdateConfig(WithActor(context.Background(), "dan"), enforce)
	nl, ok := IsNotLead(err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, []string{"tracker.push_labels_enforced"}, nl.Keys)
	assert.Equal(t, "dan", nl.Member)
	got, lerr := repo.LoadConfig(context.Background())
	require.NoError(t, lerr)
	assert.False(t, got.Tracker.IsPushLabelsEnforced())

	_, ok = IsNotLead(repo.UpdateConfig(context.Background(), enforce))
	assert.True(t, ok, "no member is a refusal too")

	require.NoError(t, repo.UpdateConfig(WithActor(context.Background(), "lea"), enforce))
	got, lerr = repo.LoadConfig(context.Background())
	require.NoError(t, lerr)
	assert.True(t, got.Tracker.IsPushLabelsEnforced())

	// a setting that binds nobody is free to every member
	require.NoError(t, repo.UpdateConfig(WithActor(context.Background(), "dan"), func(c *TeamConfig) error {
		c.Notification.Channel = "dev"
		return nil
	}))
}

func TestUpdateConfig_GovernanceAnyMemberAndTeamsWithoutLead(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	yes := true
	enforce := func(c *TeamConfig) error { c.Tracker.PushLabelsEnforced = &yes; return nil }

	// nobody has the role lead: the team is not locked out of its settings
	require.NoError(t, repo.AddMember(context.Background(), Member{ID: "dan", DisplayName: "Dan", Role: "dev"}))
	require.NoError(t, repo.UpdateConfig(WithActor(context.Background(), "dan"), enforce))

	require.NoError(t, repo.AddMember(context.Background(), Member{ID: "lea", DisplayName: "Léa", Role: "lead"}))
	// the policy is itself reserved to the leads
	_, ok := IsNotLead(repo.UpdateConfig(WithActor(context.Background(), "dan"), func(c *TeamConfig) error {
		c.Governance.Config = GovernanceConfigAnyMember
		return nil
	}))
	assert.True(t, ok)
	require.NoError(t, repo.UpdateConfig(WithActor(context.Background(), "lea"), func(c *TeamConfig) error {
		c.Governance.Config = GovernanceConfigAnyMember
		return nil
	}))
	require.NoError(t, repo.UpdateConfig(WithActor(context.Background(), "dan"), func(c *TeamConfig) error {
		c.Limits = nil
		c.Tracker.PushLabelsEnforced = nil
		return nil
	}))
}

// Every publication leaves a config_changed event: who, which keys, never the values.
func TestUpdateConfig_LeavesAConfigChangedEvent(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	addMembers(t, repo)
	ctx := WithActor(context.Background(), "dan")
	require.NoError(t, repo.UpdateConfig(ctx, func(c *TeamConfig) error {
		c.Notification.WebhookURL = "https://chat.example.com/hooks/very-secret-token"
		c.Tracker.Type = "jira"
		return nil
	}))
	events, err := repo.ListEvents(context.Background(), TeamEventsProject, time.Time{})
	require.NoError(t, err)
	require.Len(t, events, 1)
	e := events[0]
	assert.Equal(t, EventConfigChanged, e.Type)
	assert.Equal(t, "dan", e.Actor)
	assert.ElementsMatch(t, []any{"notification.webhook_url", "tracker.type"}, e.Data["keys"])
	assert.Equal(t, "notification.webhook_url, tracker.type", e.dataKeys())
	raw, _ := os.ReadFile(filepath.Join(repo.Path(), "projects", TeamEventsProject, "events", e.Timestamp.Format("2006-01")+".jsonl"))
	assert.NotContains(t, string(raw), "very-secret-token")

	// a patch that changes nothing is neither written nor announced
	require.NoError(t, repo.UpdateConfig(ctx, func(c *TeamConfig) error { c.Tracker.Type = "jira"; return nil }))
	events, err = repo.ListEvents(context.Background(), TeamEventsProject, time.Time{})
	require.NoError(t, err)
	assert.Len(t, events, 1)
}

// The sections a change touches are checked, the others are not: a mapping that is wrong
// somewhere does not block an unrelated edit.
func TestUpdateConfig_ValidatesOnlyWhatItTouches(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	bad := `[tracker]
type = "gitlab"
[tracker.label_status_mapping]
"Ready" = "wip"
`
	require.NoError(t, os.WriteFile(filepath.Join(repo.Path(), "config.toml"), []byte(bad), 0o644))
	require.NoError(t, repo.CommitAndPush(ctx, "bad mapping", "config.toml"))

	require.NoError(t, repo.UpdateConfig(ctx, func(c *TeamConfig) error { c.Notification.Channel = "dev"; return nil }),
		"the channel has nothing to do with the mapping")
	require.Error(t, repo.UpdateConfig(ctx, func(c *TeamConfig) error {
		c.Tracker.LabelStatusMapping["Other"] = "nope"
		return nil
	}))
	require.Error(t, repo.UpdateConfig(ctx, func(c *TeamConfig) error { c.Takeover.StaleDays = 99; return nil }))
	require.Error(t, repo.UpdateConfig(ctx, func(c *TeamConfig) error { c.Notification.Type = "irc"; return nil }))
	require.Error(t, repo.UpdateConfig(ctx, func(c *TeamConfig) error {
		c.Board.Columns = []BoardColumnConfig{{ID: "a", Name: "A"}}
		return nil
	}))
}

// The keys oh does not know survive a write, a default is not written, and [parallel] is no more.
func TestConfigFile_KeepsUnknownKeysAndLeavesOutDefaults(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	src := `[parallel]
max_sessions = 5

[takeover]
stale_days = 3
future = "x"

[tracker]
type = "gitlab"
type_enforced = true
auto_sync = true
max_unassigned_issues = 20
brand_new = 7

[mcp.gitlab]
url = "https://gl.example.com"
note = "kept"
`
	path := filepath.Join(repo.Path(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(src), 0o644))
	require.NoError(t, repo.CommitAndPush(ctx, "seed", "config.toml"))

	require.NoError(t, repo.UpdateConfig(ctx, func(c *TeamConfig) error { c.Tracker.Type = "jira"; return nil }))
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(raw)
	for _, kept := range []string{"max_sessions", "future = 'x'", "type_enforced", "brand_new", "note = 'kept'", "type = 'jira'", "auto_sync = true"} {
		assert.Contains(t, text, kept)
	}
	for _, gone := range []string{"stale_days", "max_unassigned_issues", "lease_hours", "heartbeat_minutes", "done_retention_days", "bot_name"} {
		assert.NotContains(t, text, gone, "a default is not written")
	}
	assert.NotContains(t, text, "[board]", "an empty table is not written")
	assert.ElementsMatch(t, []string{"parallel", "takeover.future", "tracker.type_enforced", "tracker.brand_new", "mcp.gitlab.note"}, UnknownKeys(raw))

	got, err := repo.LoadConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, got.Takeover.StaleDays)
	assert.Equal(t, DefaultHeartbeatMinutes, got.Claim.HeartbeatMinutes)
	assert.Equal(t, 5, got.Tracker.SyncIntervalMinutes)
}

// A heartbeat that is off (0) stays written: absent means the default.
func TestConfigFile_HeartbeatOffIsWritten(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	require.NoError(t, repo.UpdateConfig(ctx, func(c *TeamConfig) error { c.Claim.HeartbeatMinutes = 0; return nil }))
	got, err := repo.LoadConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, got.Claim.HeartbeatMinutes)
	require.NoError(t, repo.UpdateConfig(ctx, func(c *TeamConfig) error { c.Claim.HeartbeatMinutes = DefaultHeartbeatMinutes; return nil }))
	raw, err := os.ReadFile(filepath.Join(repo.Path(), "config.toml"))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "heartbeat_minutes")
}

func TestRepoCanEnforce(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	assert.True(t, repo.CanEnforce(ctx, "anyone"), "a team with no member is not locked")
	addMembers(t, repo)
	assert.True(t, repo.CanEnforce(ctx, "lea"))
	assert.False(t, repo.CanEnforce(ctx, "dan"))
	assert.False(t, repo.CanEnforce(ctx, ""))
	require.NoError(t, repo.UpdateConfig(WithActor(ctx, "lea"), func(c *TeamConfig) error { c.Governance.Config = GovernanceConfigAnyMember; return nil }))
	assert.True(t, repo.CanEnforce(ctx, "dan"))
}
