package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func useHub(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("OH_HOME", dir)
	config.Reset()
	t.Cleanup(config.Reset)
}

func importInto(t *testing.T, hub, archive string) {
	t.Helper()
	useHub(t, hub)
	io, _, _ := testIO()
	application = &app.App{Config: &config.Config{}, IO: io}
	t.Cleanup(func() { application = nil })
	require.NoError(t, runImport(importCmd, []string{archive}))
}

func projectNames(t *testing.T, dbPath string) []string {
	t.Helper()
	s, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	defer s.Close()
	list, err := sqlite.NewProjectStore(s).List(t.Context(), "")
	require.NoError(t, err)
	var names []string
	for _, p := range list {
		names = append(names, p.Name)
	}
	return names
}

// A write still in oh.db-wal (database open, never checkpointed) is in the
// archive, and oh import restores it.
func TestExportKeepsTheWritesStillInTheWAL(t *testing.T) {
	src := t.TempDir()
	useHub(t, src)
	live, _ := newProjectStore(t, sqlite.DBPath(), proj("p1", "recent-write", "/work/recent"))

	// Precondition: the file alone does not hold the write.
	data, err := os.ReadFile(sqlite.DBPath())
	require.NoError(t, err)
	lone := filepath.Join(t.TempDir(), "oh.db")
	require.NoError(t, os.WriteFile(lone, data, 0o600))
	require.Empty(t, projectNames(t, lone), "the write is still in the WAL, not in oh.db")
	require.NotNil(t, live)

	archive := filepath.Join(t.TempDir(), "backup.tar.gz")
	io, out, _ := testIO()
	require.NoError(t, exportArchive(io.Out, archive))
	assert.Contains(t, out.String(), "oh.db")
	entries, err := os.ReadDir(src)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".export-", "the temporary copy is removed")
	}

	dst := t.TempDir()
	importInto(t, dst, archive)
	assert.Equal(t, []string{"recent-write"}, projectNames(t, filepath.Join(dst, "oh.db")))
}

// A journal left by the database an import replaces is not replayed on the
// restored one.
func TestImportRemovesTheStaleJournal(t *testing.T) {
	src := t.TempDir()
	useHub(t, src)
	newProjectStore(t, sqlite.DBPath(), proj("p1", "kept", "/work/kept"))
	archive := filepath.Join(t.TempDir(), "backup.tar.gz")
	io, _, _ := testIO()
	require.NoError(t, exportArchive(io.Out, archive))

	dst := t.TempDir()
	stale := filepath.Join(dst, "oh.db-wal")
	require.NoError(t, os.WriteFile(stale, []byte("stale journal"), 0o600))
	useHub(t, dst)
	io, _, _ = testIO()
	application = &app.App{Config: &config.Config{}, IO: io}
	t.Cleanup(func() { application = nil })
	require.NoError(t, importCmd.Flags().Set("overwrite", "true"))
	t.Cleanup(func() { _ = importCmd.Flags().Set("overwrite", "false") })
	require.NoError(t, os.WriteFile(filepath.Join(dst, "oh.db"), []byte("old"), 0o600))
	require.NoError(t, runImport(importCmd, []string{archive}))

	assert.NoFileExists(t, stale)
	assert.Equal(t, []string{"kept"}, projectNames(t, filepath.Join(dst, "oh.db")))
}

// A database that cannot be copied consistently is still archived, with a
// warning.
func TestExportFallsBackToTheRawFileWhenTheCopyFails(t *testing.T) {
	src := t.TempDir()
	useHub(t, src)
	require.NoError(t, os.WriteFile(sqlite.DBPath(), []byte("not a sqlite database, long enough to be read as a header......"), 0o600))

	archive := filepath.Join(t.TempDir(), "backup.tar.gz")
	io, out, _ := testIO()
	require.NoError(t, exportArchive(io.Out, archive))
	assert.Contains(t, out.String(), "Ajouté: oh.db")
	assert.Contains(t, out.String(), "WAL")
}

func TestExportWithoutDatabase(t *testing.T) {
	useHub(t, t.TempDir())
	archive := filepath.Join(t.TempDir(), "backup.tar.gz")
	io, out, _ := testIO()
	require.NoError(t, exportArchive(io.Out, archive))
	assert.Contains(t, out.String(), "Ignoré (absent): oh.db")
}
