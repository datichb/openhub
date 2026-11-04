package teamstate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

func customBoard(columns ...BoardColumnConfig) BoardConfig { return BoardConfig{Columns: columns} }

// customWithoutTodo is a board with its own columns and no "todo" column.
func customWithoutTodo() BoardConfig {
	return customBoard(
		BoardColumnConfig{ID: "backlog", Name: "BACKLOG", Role: ColumnRoleInitial},
		BoardColumnConfig{ID: "doing", Name: "DOING", Role: ColumnRoleActive},
		BoardColumnConfig{ID: "done", Name: "DONE", Role: ColumnRoleTerminal},
	)
}

func TestResolveStatus(t *testing.T) {
	withTodo := DefaultBoardConfig()
	tests := []struct {
		name     string
		board    BoardConfig
		value    string
		want     string
		migrated bool
		ok       bool
	}{
		{"a status of the default board", BoardConfig{}, "in_progress", "in_progress", false, true},
		{"planned is always valid", BoardConfig{}, "planned", "planned", false, true},
		{"todo is the initial status of the default board", BoardConfig{}, "todo", "planned", true, true},
		{"todo in capitals", BoardConfig{}, " TODO ", "planned", true, true},
		{"a status in capitals", BoardConfig{}, "In_Progress", "in_progress", true, true},
		{"nothing known", BoardConfig{}, "doing", "", false, false},
		{"empty", BoardConfig{}, "", "", false, false},
		{"a declared todo column is a status", withTodo, "todo", "todo", false, true},
		{"todo without a todo column is the initial column", customWithoutTodo(), "todo", "backlog", true, true},
		{"the ID of a column", customWithoutTodo(), "doing", "doing", false, true},
		{"planned stays valid with columns", customWithoutTodo(), "planned", "planned", false, true},
		{"a built-in status that is not a column", customWithoutTodo(), "review", "", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, migrated, ok := tt.board.ResolveStatus(tt.value)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.migrated, migrated, "migrated")
			assert.Equal(t, tt.ok, ok, "ok")
		})
	}
}

func TestStatusRankFollowsTheColumnsOfTheBoard(t *testing.T) {
	def := BoardConfig{}
	assert.Equal(t, []string{"planned", "in_progress", "review", "validation", "done", "blocked"}, def.OrderedStatuses())
	assert.Less(t, def.StatusRank("planned"), def.StatusRank("in_progress"))
	assert.Less(t, def.StatusRank("validation"), def.StatusRank("blocked"))
	assert.Equal(t, 6, def.StatusRank("nowhere"), "a status the board lacks comes last")

	custom := customWithoutTodo()
	assert.Equal(t, []string{"backlog", "doing", "done"}, custom.OrderedStatuses())
	assert.Equal(t, custom.StatusRank("backlog"), custom.StatusRank("planned"), "planned is the initial column")
}

func TestNormalizeMappings(t *testing.T) {
	tc := TrackerConfig{
		LabelStatusMapping: map[string]string{"Ready": "todo", "Doing": "in_progress", "Odd": "wip"},
		StatusMapping:      map[string]string{"To Do": "todo"},
	}
	issues := tc.NormalizeMappings(BoardConfig{})
	require.Len(t, issues, 3)
	assert.Equal(t, MappingIssue{Table: MappingTableLabelStatus, Key: "Odd", Value: "wip"}, issues[0], "sorted: label table first, by key")
	assert.False(t, issues[0].Migrated())
	assert.Equal(t, MappingIssue{Table: MappingTableLabelStatus, Key: "Ready", Value: "todo", Status: "planned"}, issues[1])
	assert.Equal(t, MappingIssue{Table: MappingTableStatus, Key: "To Do", Value: "todo", Status: "planned"}, issues[2])

	assert.Equal(t, "planned", tc.LabelStatusMapping["Ready"], "migrated in place")
	assert.Equal(t, "planned", tc.StatusMapping["To Do"])
	assert.Equal(t, "wip", tc.LabelStatusMapping["Odd"], "a value that stands for nothing is left as it is")
	assert.Equal(t, "in_progress", tc.LabelStatusMapping["Doing"])
}

func TestMappingIssuesChangeNothing(t *testing.T) {
	tc := TrackerConfig{LabelStatusMapping: map[string]string{"Ready": "todo"}}
	assert.Len(t, tc.MappingIssues(BoardConfig{}), 1)
	assert.Equal(t, "todo", tc.LabelStatusMapping["Ready"])
	assert.Empty(t, TrackerConfig{}.MappingIssues(BoardConfig{}))
}

func TestValidateMappingsSaysWhatToWrite(t *testing.T) {
	i18ntest.Set(t, "en")
	tc := TrackerConfig{
		LabelStatusMapping: map[string]string{"Ready": "todo", "Odd": "wip"},
		StatusMapping:      map[string]string{"Open": "planned"},
	}
	err := tc.ValidateMappings(BoardConfig{})
	var me *MappingError
	require.ErrorAs(t, err, &me)
	assert.Len(t, me.Issues, 2)
	msg := err.Error()
	assert.Contains(t, msg, `[tracker.label_status_mapping] "Ready" = 'todo'`)
	assert.Contains(t, msg, "Write 'planned'")
	assert.Contains(t, msg, `"Odd" = 'wip'`)
	assert.Contains(t, msg, "Valid statuses: planned, in_progress, review, validation, blocked, done.")
	assert.NotContains(t, msg, "Open", "a valid value is not said")

	require.NoError(t, TrackerConfig{LabelStatusMapping: map[string]string{"Ready": "planned"}}.ValidateMappings(BoardConfig{}))
}

// The real shape (anonymised): four labels map to 'todo' on a team whose board
// is empty. Reading migrates them to 'planned', and says nothing is refused.
func TestLoadConfigMigratesTodoOfAnAnonymisedRealConfig(t *testing.T) {
	i18ntest.Set(t, "en")
	repo, _ := setupGitTestRepo(t)
	writeFixtureConfig(t, repo)

	cfg, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.False(t, cfg.Board.HasCustomColumns())
	for _, label := range []string{"Todo", "Design to do", "Ready to dev", "US to do"} {
		assert.Equal(t, ClaimStatusPlanned, cfg.Tracker.LabelStatusMapping[label], label)
	}
	assert.Equal(t, ClaimStatusInProgress, cfg.Tracker.LabelStatusMapping["Dev doing"], "valid values are untouched")
	assert.Equal(t, []string{"Design to do", "Ready to dev", "Todo", "US to do"}, cfg.Tracker.UnassignedLabels)
	assert.Empty(t, cfg.Tracker.MappingIssues(cfg.Board), "nothing left to say once read")

	// The file keeps what it says until something writes it.
	raw, err := os.ReadFile(filepath.Join(repo.path, "config.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(raw), "'todo'")

	issues, err := repo.TrackerMappingIssues(t.Context())
	require.NoError(t, err)
	assert.Len(t, issues, 4, "the audit reads the file as it is written")
	for _, i := range issues {
		assert.True(t, i.Migrated(), i.Key)
		assert.Equal(t, "todo", i.Value)
	}
}

func TestSaveConfigRefusesAMappingThatIsNoStatus(t *testing.T) {
	i18ntest.Set(t, "en")
	repo, _ := setupGitTestRepo(t)

	err := repo.SaveConfig(t.Context(), &TeamConfig{Tracker: TrackerConfig{LabelStatusMapping: map[string]string{"Ready": "todo"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Write 'planned'")
	_, statErr := os.Stat(filepath.Join(repo.path, "config.toml"))
	assert.True(t, os.IsNotExist(statErr), "nothing written")

	err = repo.SaveConfig(t.Context(), &TeamConfig{Tracker: TrackerConfig{StatusMapping: map[string]string{"Open": "wip"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status_mapping")

	// A board that declares todo with the initial role accepts it.
	cfg := &TeamConfig{
		Board:   DefaultBoardConfig(),
		Tracker: TrackerConfig{LabelStatusMapping: map[string]string{"Ready": "todo"}},
	}
	require.NoError(t, repo.SaveConfig(t.Context(), cfg))
}

func TestSaveConfigWritesWhatTheMigrationRead(t *testing.T) {
	i18ntest.Set(t, "en")
	repo, _ := setupGitTestRepo(t)
	writeFixtureConfig(t, repo)

	cfg, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	require.NoError(t, repo.SaveConfig(t.Context(), cfg), "a config that was read can be written back")
	raw, err := os.ReadFile(filepath.Join(repo.path, "config.toml"))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "'todo'")
	assert.Contains(t, string(raw), "planned")
	issues, err := repo.TrackerMappingIssues(t.Context())
	require.NoError(t, err)
	assert.Empty(t, issues)
}

func TestRepairTrackerMappings(t *testing.T) {
	i18ntest.Set(t, "en")
	repo, _ := setupGitTestRepo(t)
	content := "[tracker]\n[tracker.label_status_mapping]\nReady = 'todo'\nOdd = 'wip'\nDoing = 'in_progress'\n"
	require.NoError(t, os.WriteFile(filepath.Join(repo.path, "config.toml"), []byte(content), 0o644))

	migrated, unknown, err := repo.RepairTrackerMappings(t.Context())
	require.NoError(t, err)
	require.Len(t, migrated, 1)
	assert.Equal(t, "Ready", migrated[0].Key)
	require.Len(t, unknown, 1)
	assert.Equal(t, "Odd", unknown[0].Key, "a value that stands for nothing is left to the team")

	raw, err := os.ReadFile(filepath.Join(repo.path, "config.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(raw), "Ready = 'planned'")
	assert.Contains(t, string(raw), "Odd = 'wip'")

	migrated, unknown, err = repo.RepairTrackerMappings(t.Context())
	require.NoError(t, err)
	assert.Empty(t, migrated, "nothing to do the second time")
	assert.Len(t, unknown, 1)
}

func writeFixtureConfig(t *testing.T, repo *Repo) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "gitlab_team_todo.toml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(repo.path, "config.toml"), data, 0o644))
}
