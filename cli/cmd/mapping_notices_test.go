package cmd

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// v5.3.1: a team clone whose tracker mappings hold values that are not
// statuses says it once, in one sentence that names the command to run; the
// detail of each value stays in the log.

func migratedNotice() teamstate.MappingNotice {
	return teamstate.MappingNotice{Team: "t-sru-team", Migrated: 4,
		Example: teamstate.MappingIssue{Value: "todo", Status: "planned"}}
}

func TestMappingNoticeTextNamesTheCloneTheCountAndTheCommand(t *testing.T) {
	i18ntest.Set(t, "fr")
	text := mappingNoticeText(migratedNotice())
	assert.Equal(t, "t-sru-team : 4 valeur(s) de config.toml lue(s) comme un statut du board (ex. 'todo' → 'planned') — lancez `oh doctor --fix`", text)

	i18ntest.Set(t, "en")
	text = mappingNoticeText(migratedNotice())
	assert.Equal(t, "t-sru-team: 4 value(s) in config.toml read as a board status (e.g. 'todo' → 'planned') — run `oh doctor --fix`", text)
}

func TestMappingNoticeTextForValuesThatStandForNothing(t *testing.T) {
	invalid := teamstate.MappingNotice{Team: "t-sru-team", Invalid: 3}

	i18ntest.Set(t, "fr")
	text := mappingNoticeText(invalid)
	assert.Equal(t, "t-sru-team : 3 valeur(s) de config.toml ne désigne(nt) aucune colonne du board — corrigez config.toml (`oh doctor`)", text)
	assert.NotContains(t, text, "--fix", "nothing doctor --fix could rewrite")
	assert.NotEqual(t, mappingNoticeText(migratedNotice()), text)

	i18ntest.Set(t, "en")
	assert.Equal(t, "t-sru-team: 3 value(s) in config.toml stand for no column of the board — fix config.toml (`oh doctor`)", mappingNoticeText(invalid))

	both := migratedNotice()
	both.Invalid = 2
	i18ntest.Set(t, "fr")
	text = mappingNoticeText(both)
	assert.Contains(t, text, "4 valeur(s)")
	assert.Contains(t, text, "`oh doctor --fix`")
	assert.Contains(t, text, "2 autre(s) ne désigne(nt) aucune colonne")
	assert.Equal(t, 1, strings.Count(text+"\n", "\n"), "one line")
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() {
		var b bytes.Buffer
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()
	func() {
		defer func() { os.Stderr = orig; _ = w.Close() }()
		fn()
	}()
	return <-done
}

func TestWarnMappingNoticeWritesOneLineOnStderr(t *testing.T) {
	i18ntest.Set(t, "fr")
	out := captureStderr(t, func() { warnMappingNotice(migratedNotice()) })
	assert.Equal(t, 1, strings.Count(out, "\n"))
	assert.Contains(t, out, "t-sru-team : 4 valeur(s)")
	assert.Contains(t, out, "oh doctor --fix")
}

func TestMappingNoticesAreHeldUntilTheTUIIsUp(t *testing.T) {
	i18ntest.Set(t, "en")
	holdNoticesForTUI()
	t.Cleanup(func() { releaseHeldNotices(func(string) {}) })

	out := captureStderr(t, func() { warnMappingNotice(migratedNotice()) })
	assert.Empty(t, out, "nothing above the TUI")

	var shown []string
	releaseHeldNotices(func(s string) { shown = append(shown, s) })
	require.Len(t, shown, 1)
	assert.Contains(t, shown[0], "oh doctor --fix")

	releaseHeldNotices(func(s string) { shown = append(shown, s) })
	assert.Len(t, shown, 1, "released once")
}

func TestMappingNoticeInJSONLogsGoesThroughSlog(t *testing.T) {
	i18ntest.Set(t, "en")
	logJSON = true
	t.Cleanup(func() { logJSON = false })
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	out := captureStderr(t, func() { warnMappingNotice(migratedNotice()) })
	assert.Empty(t, out)
	assert.Contains(t, logs.String(), "oh doctor --fix")
}

// The whole chain: reading the anonymised config of a GitLab team with four
// 'todo' values says one sentence on stderr, once.
func TestReadingTheTeamConfigWarnsOncePerProcess(t *testing.T) {
	i18ntest.Set(t, "fr")
	data, err := os.ReadFile(filepath.Join("..", "internal", "teamstate", "testdata", "gitlab_team_todo.toml"))
	require.NoError(t, err)
	dir := filepath.Join(t.TempDir(), "t-sru-team")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), data, 0o644))
	repo := teamstate.NewRepo("", dir)

	out := captureStderr(t, func() {
		for range 3 {
			_, err := repo.LoadConfig(t.Context())
			require.NoError(t, err)
		}
	})
	assert.Equal(t, 1, strings.Count(out, "\n"), out)
	assert.Contains(t, out, "t-sru-team : 4 valeur(s)")
	assert.Contains(t, out, "`oh doctor --fix`")
	assert.NotContains(t, out, "mapping_migrated")
	assert.NotContains(t, out, dir, "the name of the clone, not its path")
}
