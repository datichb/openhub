package teamstate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

const oldConfig = `[parallel]
max_sessions = 5

[notification]
enabled = true
mattermost_webhook = "https://hooks.slack.com/services/T0/B0/secret"

[takeover]
stale_days = 99

[tracker]
type = "gitlab"
type_enforced = true
max_auto_plan_per_member = 4
sync_interval_minutes = 500
future_key = 1
`

func issueKeys(issues []ConfigIssue) []string {
	var keys []string
	for _, i := range issues {
		keys = append(keys, i.Key)
	}
	return keys
}

func TestConfigIssues_ReportsWhatIsOutdatedOrWrong(t *testing.T) {
	i18ntest.Set(t, "en")
	issues := configIssues([]byte(oldConfig))
	assert.ElementsMatch(t, []string{
		"parallel", "tracker.type_enforced", "notification.mattermost_webhook", "tracker.max_auto_plan_per_member",
		"takeover.stale_days", "tracker.sync_interval_minutes", "tracker.future_key",
	}, issueKeys(issues))
	fixable := FixableIssues(issues)
	assert.Len(t, fixable, len(issues)-1, "everything but the unknown key is repaired")
	for _, i := range issues {
		assert.NotEmpty(t, i.Detail)
		assert.NotContains(t, i.Detail, "secret", "a webhook is never in a finding")
	}

	assert.Empty(t, configIssues([]byte("[tracker]\ntype = \"jira\"\n")))
	bad := configIssues([]byte("not toml {{"))
	require.Len(t, bad, 1)
	assert.Equal(t, IssueUnreadable, bad[0].Kind)

	board := configIssues([]byte("[[board.columns]]\nid = \"a\"\nname = \"A\"\n"))
	require.Len(t, board, 1)
	assert.Equal(t, IssueBoard, board[0].Kind)

	notif := func(s string) []string { return issueKeys(configIssues([]byte(s))) }
	assert.Equal(t, []string{"notification.enabled"}, notif("[notification]\nenabled = true\n"), "on, nowhere to send")
	assert.Equal(t, []string{"notification.enabled"}, notif("[notification]\ntype = \"slack\"\nwebhook_url = \"https://h/x\"\n"), "a webhook that is never used")
	assert.Equal(t, []string{"notification.type"}, notif("[notification]\nenabled = true\ntype = \"irc\"\nwebhook_url = \"https://h/x\"\n"))
	assert.Equal(t, []string{"notification.destinations[0]"}, notif("[notification]\nenabled = true\n[[notification.destinations]]\ntype = \"slack\"\n"))
}

func TestRepairConfig_MigratesAndClamps(t *testing.T) {
	i18ntest.Set(t, "en")
	repo, _ := setupGitTestRepo(t)
	addMembers(t, repo)
	ctx := WithActor(context.Background(), "lea")
	path := filepath.Join(repo.Path(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(oldConfig), 0o644))
	require.NoError(t, repo.CommitAndPush(ctx, "old", "config.toml"))

	repaired, err := repo.RepairConfig(ctx)
	require.NoError(t, err)
	assert.Len(t, repaired, 6)

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(raw)
	assert.NotContains(t, text, "parallel")
	assert.NotContains(t, text, "type_enforced")
	assert.NotContains(t, text, "mattermost_webhook")
	assert.NotContains(t, text, "max_auto_plan_per_member")
	assert.Contains(t, text, "future_key", "an unknown key is left for the team")
	got, err := repo.LoadConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, "https://hooks.slack.com/services/T0/B0/secret", got.Notification.WebhookURL)
	assert.Equal(t, "slack", got.Notification.Type)
	assert.True(t, got.Notification.Enabled)
	assert.Equal(t, 30, got.Takeover.StaleDays)
	assert.Equal(t, MaxSyncIntervalMinute, got.Tracker.SyncIntervalMinutes)
	require.NotNil(t, got.Tracker.MaxInProgressPerMember)
	assert.Equal(t, 4, *got.Tracker.MaxInProgressPerMember)
	assert.Equal(t, []string{"tracker.future_key"}, UnknownKeys(raw))

	events, err := repo.ListEvents(ctx, TeamEventsProject, time.Time{})
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "lea", events[0].Actor)
	assert.NotContains(t, events[0].dataKeys(), "secret")

	again, err := repo.RepairConfig(ctx)
	require.NoError(t, err)
	assert.Empty(t, again, "nothing left to repair, nothing written")
}

func TestAnonymousURL(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"https://gitlab.example.com/org/team.git":         "https://gitlab.example.com/org/team.git",
		"https://ben:tok@gitlab.example.com/org/team.git": "https://gitlab.example.com/org/team.git",
		"git@gitlab.example.com:org/team.git":             "https://gitlab.example.com/org/team.git",
		"ssh://git@gitlab.example.com/org/team":           "https://gitlab.example.com/org/team.git",
	} {
		got, ok := anonymousURL(in)
		assert.True(t, ok, in)
		assert.Equal(t, want, got, in)
	}
	_, ok := anonymousURL("/srv/git/team.git")
	assert.False(t, ok)
}

// A repository that answers without credentials is public; one that asks for them is not.
func TestProbePublic(t *testing.T) {
	_, bare := setupGitTestRepo(t)
	open := gittest.ServeHTTP(t, bare, "open", "", "")
	public, err := ProbePublic(t.Context(), open.URL)
	require.NoError(t, err)
	assert.True(t, public)

	private := gittest.ServeHTTP(t, bare, "private", "ben", "s3cret")
	public, err = ProbePublic(t.Context(), private.URL)
	require.NoError(t, err)
	assert.False(t, public)
	assert.Zero(t, private.Accepted()+private.Refused(), "no credential is ever sent")

	_, err = ProbePublic(t.Context(), "/srv/git/team.git")
	assert.Error(t, err)
}
