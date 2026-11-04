package teamstate

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collectMappingNotices replaces the handler and the memory of the notices
// for the test, and captures the log at every level.
func collectMappingNotices(t *testing.T) (notices *[]MappingNotice, logs *bytes.Buffer) {
	t.Helper()
	mappingNotices.Lock()
	prevSeen, prevHandler := mappingNotices.seen, mappingNotices.handler
	mappingNotices.seen = nil
	mappingNotices.Unlock()
	got := &[]MappingNotice{}
	SetMappingNoticeHandler(func(n MappingNotice) { *got = append(*got, n) })

	buf := &bytes.Buffer{}
	prevLog := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(prevLog)
		mappingNotices.Lock()
		mappingNotices.seen, mappingNotices.handler = prevSeen, prevHandler
		mappingNotices.Unlock()
	})
	return got, buf
}

func TestMappingNoticeIsOneLinePerCloneWithTheDetailInTheLog(t *testing.T) {
	notices, logs := collectMappingNotices(t)
	repo, _ := setupGitTestRepo(t)
	writeFixtureConfig(t, repo)

	cfg, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "planned", cfg.Tracker.LabelStatusMapping["Todo"], "the read is unchanged: todo is read as planned")

	require.Len(t, *notices, 1, "one notice for the four values")
	n := (*notices)[0]
	assert.Equal(t, filepath.Base(repo.path), n.Team)
	assert.Equal(t, repo.path, n.Path)
	assert.Equal(t, 4, n.Migrated)
	assert.Zero(t, n.Invalid)
	assert.Equal(t, "todo", n.Example.Value)
	assert.Equal(t, "planned", n.Example.Status)

	out := logs.String()
	assert.Equal(t, 4, strings.Count(out, "teamstate.config.mapping_migrated"), "the detail of each value is in the log")
	for _, key := range []string{"key=Todo", "key=\"Design to do\"", "key=\"Ready to dev\"", "key=\"US to do\""} {
		assert.Contains(t, out, key)
	}
	assert.Contains(t, out, "table=label_status_mapping")
	assert.Contains(t, out, "used=planned")
	assert.Contains(t, out, "config="+repo.path)
	assert.NotContains(t, out, "level=WARN", "no line of the log reaches the terminal at the default level")

	logs.Reset()
	_, err = repo.LoadConfig(t.Context())
	require.NoError(t, err)
	_, err = repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Len(t, *notices, 1, "a second read of the same clone says nothing more")
	assert.Empty(t, logs.String(), "nor does the log")
}

func TestMappingNoticeForValuesThatStandForNothing(t *testing.T) {
	notices, logs := collectMappingNotices(t)
	repo, _ := setupGitTestRepo(t)
	content := "[tracker]\n[tracker.label_status_mapping]\nOdd = 'wip'\nOther = 'later'\nDoing = 'in_progress'\n"
	require.NoError(t, os.WriteFile(filepath.Join(repo.path, "config.toml"), []byte(content), 0o644))

	cfg, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "wip", cfg.Tracker.LabelStatusMapping["Odd"], "a value that stands for nothing is left as it is")

	require.Len(t, *notices, 1)
	n := (*notices)[0]
	assert.Zero(t, n.Migrated)
	assert.Equal(t, 2, n.Invalid)
	assert.Equal(t, 2, strings.Count(logs.String(), "teamstate.config.mapping_invalid"))
	assert.NotContains(t, logs.String(), "level=WARN")
}

func TestMappingNoticeCountsBothKinds(t *testing.T) {
	notices, _ := collectMappingNotices(t)
	repo, _ := setupGitTestRepo(t)
	content := "[tracker]\n[tracker.label_status_mapping]\nReady = 'todo'\nOdd = 'wip'\n"
	require.NoError(t, os.WriteFile(filepath.Join(repo.path, "config.toml"), []byte(content), 0o644))

	_, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	require.Len(t, *notices, 1, "one notice for the clone, whatever the kinds")
	assert.Equal(t, 1, (*notices)[0].Migrated)
	assert.Equal(t, 1, (*notices)[0].Invalid)
}

func TestMappingNoticeIsPerClone(t *testing.T) {
	notices, _ := collectMappingNotices(t)
	first, _ := setupGitTestRepo(t)
	second, _ := setupGitTestRepo(t)
	writeFixtureConfig(t, first)
	writeFixtureConfig(t, second)

	for _, r := range []*Repo{first, second, first, second} {
		_, err := r.LoadConfig(t.Context())
		require.NoError(t, err)
	}
	require.Len(t, *notices, 2, "one per clone")
	assert.NotEqual(t, (*notices)[0].Path, (*notices)[1].Path)
}

func TestMappingNoticeNotRaisedForValidMappings(t *testing.T) {
	notices, logs := collectMappingNotices(t)
	repo, _ := setupGitTestRepo(t)
	content := "[tracker]\n[tracker.label_status_mapping]\nReady = 'planned'\nDoing = 'in_progress'\n"
	require.NoError(t, os.WriteFile(filepath.Join(repo.path, "config.toml"), []byte(content), 0o644))

	_, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Empty(t, *notices)
	assert.Empty(t, logs.String())
}
