package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// newProjectStore opens the database at path with projects; closeStore
// checkpoints it into the file (what an archive reads).
func newProjectStore(t *testing.T, path string, projects ...domain.Project) (ps *sqlite.ProjectStore, closeStore func()) {
	t.Helper()
	s, err := sqlite.Open(path)
	require.NoError(t, err)
	closeStore = func() { _ = s.Close() }
	t.Cleanup(closeStore)
	ps = sqlite.NewProjectStore(s)
	for i := range projects {
		require.NoError(t, ps.Create(t.Context(), &projects[i]))
	}
	return ps, closeStore
}

func proj(id, name, path string) domain.Project {
	return domain.Project{ID: id, Name: name, Path: path, Status: domain.ProjectStatusActive}
}

// backupArchive builds an `oh export` archive holding the database at dbPath
// and a hub.toml.
func backupArchive(t *testing.T, dbPath string) string {
	t.Helper()
	db, err := os.ReadFile(dbPath)
	require.NoError(t, err)
	hubToml := []byte("name = \"FromBackup\"\n")
	h := sha256.New()
	h.Write(db)
	h.Write(hubToml)
	manifest, err := json.Marshal(backupManifest{Version: "1", CreatedAt: time.Now(), Checksum: hex.EncodeToString(h.Sum(nil)), Files: []string{"oh.db", "hub.toml"}})
	require.NoError(t, err)

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	require.NoError(t, writeTarEntry(tw, "manifest.json", manifest))
	require.NoError(t, writeTarEntry(tw, "oh.db", db))
	require.NoError(t, writeTarEntry(tw, "hub.toml", hubToml))
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	out := filepath.Join(t.TempDir(), "backup.tar.gz")
	require.NoError(t, os.WriteFile(out, buf.Bytes(), 0o600))
	return out
}

func TestImportMergeAddsOnlyUnknownProjects(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	config.Reset()
	t.Cleanup(config.Reset)

	backupDB := filepath.Join(t.TempDir(), "backup.db")
	_, closeBackup := newProjectStore(t, backupDB,
		proj("a", "shared", "/work/shared"),
		proj("b", "only-in-backup", "/work/backup"),
		proj("c", "renamed", "/work/mine"), // same path as a live project
	)
	closeBackup()
	live, _ := newProjectStore(t, sqlite.DBPath(), proj("a", "shared", "/work/shared"), proj("z", "local", "/work/mine"))
	require.NoError(t, os.WriteFile(filepath.Join(home, "hub.toml"), []byte("name = \"Live\"\n"), 0o600))

	io, out, _ := testIO()
	application = &app.App{Config: &config.Config{}, Projects: live, IO: io}
	t.Cleanup(func() { application = nil })

	cmd := importCmd
	require.NoError(t, cmd.Flags().Set("merge", "true"))
	t.Cleanup(func() { _ = cmd.Flags().Set("merge", "false") })
	require.NoError(t, runImport(cmd, []string{backupArchive(t, backupDB)}))

	projects, err := live.List(t.Context(), "")
	require.NoError(t, err)
	names := map[string]bool{}
	for _, p := range projects {
		names[p.Name] = true
	}
	assert.Equal(t, map[string]bool{"shared": true, "local": true, "only-in-backup": true}, names)
	assert.Contains(t, out.String(), "only-in-backup")

	hub, err := os.ReadFile(filepath.Join(home, "hub.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(hub), "Live", "hub.toml is not replaced by a merge")
}

func TestImportMergeExcludesOverwrite(t *testing.T) {
	io, _, _ := testIO()
	application = &app.App{Config: &config.Config{}, IO: io}
	t.Cleanup(func() { application = nil })
	require.NoError(t, importCmd.Flags().Set("merge", "true"))
	require.NoError(t, importCmd.Flags().Set("overwrite", "true"))
	t.Cleanup(func() {
		_ = importCmd.Flags().Set("merge", "false")
		_ = importCmd.Flags().Set("overwrite", "false")
	})
	assert.Error(t, runImport(importCmd, []string{"unused.tar.gz"}))
}
