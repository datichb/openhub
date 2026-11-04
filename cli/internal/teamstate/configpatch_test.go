package teamstate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/limits"
)

func boolPtr(b bool) *bool { return &b }

func TestConfigPatch_KeepsTheSettingsOfBothSides(t *testing.T) {
	t.Parallel()
	base := &TeamConfig{Tracker: TrackerConfig{Type: "gitlab"}, MCP: map[string]SharedMCPConfig{"gitlab": {URL: "https://a"}}}
	mine := CloneConfig(base)
	mine.Tracker.Type = "jira"
	mine.MCP["gitlab"] = SharedMCPConfig{URL: "https://a", Enabled: boolPtr(true)}
	mine.Models.Families = map[string]string{"opus": "x"}

	theirs := CloneConfig(base)
	theirs.Notification.Channel = "dev"
	theirs.MCP["gitlab"] = SharedMCPConfig{URL: "https://b"}

	p := DiffConfig(base, mine)
	assert.Equal(t, []string{"mcp.gitlab.enabled", "models.families.opus", "tracker.type"}, p.Keys())
	require.NoError(t, p.Apply(theirs))
	assert.Equal(t, "jira", theirs.Tracker.Type)
	assert.Equal(t, "dev", theirs.Notification.Channel, "the setting of the colleague stays")
	assert.Equal(t, "https://b", theirs.MCP["gitlab"].URL, "the URL of the colleague stays")
	require.NotNil(t, theirs.MCP["gitlab"].Enabled)
	assert.True(t, *theirs.MCP["gitlab"].Enabled)
	assert.Equal(t, "x", theirs.Models.Families["opus"])
}

func TestConfigPatch_RefusesToOverwriteASettingChangedMeanwhile(t *testing.T) {
	t.Parallel()
	base := &TeamConfig{Tracker: TrackerConfig{Type: "gitlab", TrackerProject: "a/b"}}
	mine := CloneConfig(base)
	mine.Tracker.Type = "jira"
	mine.Tracker.TrackerProject = "c/d"

	theirs := CloneConfig(base)
	theirs.Tracker.Type = "gitlab-ee"
	theirs.Tracker.TrackerProject = "c/d" // same value as mine: not a conflict
	before := CloneConfig(theirs)

	err := DiffConfig(base, mine).Apply(theirs)
	c, ok := IsConfigConflict(err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, []string{"tracker.type"}, c.Keys)
	assert.Equal(t, before, theirs, "nothing is applied when a setting conflicts")
}

func TestConfigPatch_MapEntriesAndLists(t *testing.T) {
	t.Parallel()
	base := &TeamConfig{
		Tracker: TrackerConfig{LabelStatusMapping: map[string]string{"A": "done", "B": "review"}},
		Board:   BoardConfig{Columns: []BoardColumnConfig{{ID: "a", Name: "A"}}},
	}
	mine := CloneConfig(base)
	delete(mine.Tracker.LabelStatusMapping, "A")
	mine.Tracker.LabelStatusMapping["C"] = "done"
	mine.Board.Columns = append(mine.Board.Columns, BoardColumnConfig{ID: "b", Name: "B"})

	theirs := CloneConfig(base)
	theirs.Tracker.LabelStatusMapping["D"] = "blocked"
	require.NoError(t, DiffConfig(base, mine).Apply(theirs))
	assert.Equal(t, map[string]string{"B": "review", "C": "done", "D": "blocked"}, theirs.Tracker.LabelStatusMapping)
	assert.Len(t, theirs.Board.Columns, 2)

	// a column list changed on both sides is one setting: conflict
	other := CloneConfig(base)
	other.Board.Columns = []BoardColumnConfig{{ID: "z", Name: "Z"}}
	_, ok := IsConfigConflict(DiffConfig(base, mine).Apply(other))
	assert.True(t, ok)

	// the entry deleted by a colleague while it was edited here
	gone := CloneConfig(base)
	delete(gone.Tracker.LabelStatusMapping, "B")
	edit := CloneConfig(base)
	edit.Tracker.LabelStatusMapping["B"] = "blocked"
	c, ok := IsConfigConflict(DiffConfig(base, edit).Apply(gone))
	require.True(t, ok)
	assert.Equal(t, []string{"tracker.label_status_mapping.B"}, c.Keys)
}

func TestConfigPatch_EmptyAndNilEquivalents(t *testing.T) {
	t.Parallel()
	base := &TeamConfig{Models: TeamModelsConfig{Families: map[string]string{}}}
	edited := &TeamConfig{}
	assert.True(t, DiffConfig(base, edited).Empty(), "an empty map is an absent one")
	edited.Limits = &limits.TeamLimits{}
	assert.True(t, DiffConfig(base, edited).Empty(), "an empty limits table is no limits")
	edited.Limits = &limits.TeamLimits{Enforced: limits.Limits{MaxActiveSessions: 3}}
	p := DiffConfig(base, edited)
	assert.Equal(t, []string{"limits.enforced.max_active_sessions"}, p.Keys())
	cfg := &TeamConfig{}
	require.NoError(t, p.Apply(cfg))
	require.NotNil(t, cfg.Limits)
	assert.Equal(t, 3, cfg.Limits.Enforced.MaxActiveSessions)
}

func TestConfigPatch_ScalarPointers(t *testing.T) {
	t.Parallel()
	base := &TeamConfig{}
	mine := CloneConfig(base)
	mine.Tracker.TypeEnforced = boolPtr(false)
	theirs := CloneConfig(base)
	require.NoError(t, DiffConfig(base, mine).Apply(theirs))
	require.NotNil(t, theirs.Tracker.TypeEnforced)
	assert.False(t, *theirs.Tracker.TypeEnforced, "an explicit false is not an absent setting")
	assert.NotSame(t, mine.Tracker.TypeEnforced, theirs.Tracker.TypeEnforced)
}

// twoClones is a team-state seen by two members: the same bare remote, two
// clones.
func twoClones(t *testing.T) (a, b *Repo) {
	t.Helper()
	a, bare := setupGitTestRepo(t)
	b = NewRepo(bare, filepath.Join(t.TempDir(), "other"))
	require.NoError(t, b.Clone(context.Background()))
	return a, b
}

// Two members edit different settings from the same copy: both are kept.
func TestUpdateConfigFrom_ConcurrentEditsOfDifferentKeysAreBothKept(t *testing.T) {
	t.Parallel()
	a, b := twoClones(t)
	ctx := context.Background()
	base, err := a.LoadConfig(ctx)
	require.NoError(t, err)
	baseB, err := b.LoadConfig(ctx)
	require.NoError(t, err)

	editA := CloneConfig(base)
	editA.Tracker.Type = "jira"
	require.NoError(t, a.UpdateConfigFrom(ctx, base, editA))

	editB := CloneConfig(baseB)
	editB.Notification.Channel = "dev-team"
	require.NoError(t, b.UpdateConfigFrom(ctx, baseB, editB), "B's copy is older than the remote")

	got, err := b.LoadConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, "jira", got.Tracker.Type, "the edit of A is not overwritten by the old copy of B")
	assert.Equal(t, "dev-team", got.Notification.Channel)
	require.NoError(t, a.Pull(ctx))
	got, err = a.LoadConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, "dev-team", got.Notification.Channel)
	assert.Equal(t, "jira", got.Tracker.Type)
}

// The same setting changed by both: the second is refused and nothing is written.
func TestUpdateConfigFrom_SameKeyIsAConflictAndNothingIsOverwritten(t *testing.T) {
	t.Parallel()
	a, b := twoClones(t)
	ctx := context.Background()
	baseA, err := a.LoadConfig(ctx)
	require.NoError(t, err)
	baseB, err := b.LoadConfig(ctx)
	require.NoError(t, err)

	editA := CloneConfig(baseA)
	editA.Tracker.Type = "jira"
	editA.Tracker.TrackerProject = "ABC"
	require.NoError(t, a.UpdateConfigFrom(ctx, baseA, editA))

	editB := CloneConfig(baseB)
	editB.Tracker.Type = "gitlab"
	editB.Notification.Channel = "dev"
	err = b.UpdateConfigFrom(ctx, baseB, editB)
	c, ok := IsConfigConflict(err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, []string{"tracker.type"}, c.Keys)

	got, err := b.LoadConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, "jira", got.Tracker.Type)
	assert.Empty(t, got.Notification.Channel, "a refused change is written for no key")
	assert.Equal(t, gittest.Run(t, a.Path(), "rev-parse", "HEAD"), gittest.Run(t, b.Path(), "rev-parse", "HEAD"))
	assert.Empty(t, gittest.Run(t, b.Path(), "status", "--porcelain"))
}

// A push refused: the patch runs again on the version of the remote.
func TestUpdateConfig_RefusedPushReplaysThePatchOnTheRemoteVersion(t *testing.T) {
	t.Parallel()
	a, b := twoClones(t)
	ctx := context.Background()
	a.beforePush = func() {
		a.beforePush = nil
		require.NoError(t, b.UpdateConfig(ctx, func(c *TeamConfig) error {
			c.Notification.Channel = "from-b"
			return nil
		}))
	}
	var seen []string
	require.NoError(t, a.UpdateConfig(ctx, func(c *TeamConfig) error {
		seen = append(seen, c.Notification.Channel)
		c.Tracker.Type = "jira"
		return nil
	}))
	assert.Equal(t, []string{"", "from-b"}, seen, "the second run starts from what the colleague pushed")
	got, err := a.LoadConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, "from-b", got.Notification.Channel)
	assert.Equal(t, "jira", got.Tracker.Type)
}

// A refused push followed by a patch that now conflicts: refused, not replayed over it.
func TestUpdateConfigFrom_ConflictAppearingOnTheReplayIsRefused(t *testing.T) {
	t.Parallel()
	a, b := twoClones(t)
	ctx := context.Background()
	base, err := a.LoadConfig(ctx)
	require.NoError(t, err)
	a.beforePush = func() {
		a.beforePush = nil
		require.NoError(t, b.UpdateConfig(ctx, func(c *TeamConfig) error {
			c.Tracker.Type = "gitlab-ee"
			return nil
		}))
	}
	edit := CloneConfig(base)
	edit.Tracker.Type = "jira"
	err = a.UpdateConfigFrom(ctx, base, edit)
	_, ok := IsConfigConflict(err)
	require.True(t, ok, "%v", err)
	got, err := a.LoadConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, "gitlab-ee", got.Tracker.Type)
}

// An unreadable config.toml is not an empty configuration: nothing is written over it.
func TestUpdateConfig_RefusesAnUnreadableConfig(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	path := filepath.Join(repo.Path(), "config.toml")
	bad := []byte("this is {{{{ invalid toml")
	require.NoError(t, os.WriteFile(path, bad, 0o644))
	require.NoError(t, repo.CommitAndPush(ctx, "bad", "config.toml"))
	head := gittest.Run(t, repo.Path(), "rev-parse", "HEAD")

	err := repo.UpdateConfig(ctx, func(c *TeamConfig) error { c.Tracker.Type = "jira"; return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing config.toml")
	got, rerr := os.ReadFile(path)
	require.NoError(t, rerr)
	assert.Equal(t, string(bad), string(got))
	assert.Equal(t, head, gittest.Run(t, repo.Path(), "rev-parse", "HEAD"))
}

// The strict validation of the mappings stays, on the resulting config.
func TestUpdateConfig_ValidatesTheResult(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	err := repo.UpdateConfig(ctx, func(c *TeamConfig) error {
		c.Tracker.LabelStatusMapping = map[string]string{"Ready": "wip"}
		return nil
	})
	require.Error(t, err)
	err = repo.UpdateConfig(ctx, func(c *TeamConfig) error {
		c.Claim.LeaseHours = 9999
		return nil
	})
	require.Error(t, err)
	got, lerr := repo.LoadConfig(ctx)
	require.NoError(t, lerr)
	assert.Empty(t, got.Tracker.LabelStatusMapping)
	assert.NotEqual(t, 9999, got.Claim.LeaseHours)

	boom := errors.New("boom")
	assert.ErrorIs(t, repo.UpdateConfig(ctx, func(*TeamConfig) error { return boom }), boom)
}
