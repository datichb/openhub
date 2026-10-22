package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/storage/keychain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// stepOut is a file standing for stdout (the steps write to an *os.File).
func stepOut(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Create(filepath.Join(t.TempDir(), "out"))
	require.NoError(t, err)
	t.Cleanup(func() { f.Close() })
	return f
}

// The purge cleans what oh left in a project (files of a former deployment,
// the Dolt database and runtime files of Beads) and nothing else: the code of
// the project and the Beads files tracked by git stay.
func TestCleanPurgeProjectsKeepsTheUsersFiles(t *testing.T) {
	i18ntest.Set(t, "en")
	proj := t.TempDir()
	writeTree(t, proj, map[string]string{
		"src/main.go":                "package main",
		".legacy-deploy/agents/a.md": "agent",
		"legacy.json":                "{}",
		".beads/dolt/db/data":        "dolt",
		".beads/export-state/s":      "state",
		".beads/dolt-server.pid":     "123",
		".beads/dolt-server.log":     "log",
		".beads/issues.jsonl":        "{}",
		".beads/config.yaml":         "prefix: bd",
		"docs/legacy.json.notes":     "kept: not the file",
	})
	empty := t.TempDir()
	writeTree(t, empty, map[string]string{"keep.txt": "x"})

	out := stepOut(t)
	cleanPurgeProjects(out, []purgeProject{
		{name: "web", path: proj, toolFiles: []string{".legacy-deploy", "legacy.json"}, hasBeadsDolt: true},
		{name: "no-path", path: "", toolFiles: []string{".legacy-deploy"}, hasBeadsDolt: true},
		{name: "nothing", path: empty},
	})

	for _, gone := range []string{".legacy-deploy", "legacy.json", ".beads/dolt", ".beads/export-state", ".beads/dolt-server.pid", ".beads/dolt-server.log"} {
		assert.False(t, exists(filepath.Join(proj, gone)), "%s is removed", gone)
	}
	for _, kept := range []string{"src/main.go", ".beads/issues.jsonl", ".beads/config.yaml", "docs/legacy.json.notes"} {
		assert.True(t, exists(filepath.Join(proj, kept)), "%s stays", kept)
	}
	assert.True(t, exists(filepath.Join(empty, "keep.txt")))

	data, err := os.ReadFile(out.Name())
	require.NoError(t, err)
	assert.Contains(t, string(data), "2 project(s) cleaned", "a project without a path is not counted")
}

// The hub folder goes, session bundles included (they are read-only), and the
// open database is released first.
func TestRemoveHubDirRemovesReadOnlyBundles(t *testing.T) {
	i18ntest.Set(t, "en")
	home := configtest.Home(t)
	hub := filepath.Join(home, ".oh")
	writeTree(t, hub, map[string]string{
		"hub.toml":                     "[general]",
		"oh.db":                        "db",
		"bundles/abc123/agents/a.md":   "agent",
		"sessions/ses_a/results/x.txt": "x",
	})
	bundle := filepath.Join(hub, "bundles", "abc123")
	require.NoError(t, os.Chmod(filepath.Join(bundle, "agents", "a.md"), 0o444))
	require.NoError(t, os.Chmod(filepath.Join(bundle, "agents"), 0o555))
	require.NoError(t, os.Chmod(bundle, 0o555))
	t.Cleanup(func() { // if the test fails half way, let TempDir clean up
		_ = os.Chmod(bundle, 0o755)
		_ = os.Chmod(filepath.Join(bundle, "agents"), 0o755)
	})

	removeHubDir(stepOut(t), hub)
	assert.False(t, exists(hub), "the whole hub folder is removed")
	assert.True(t, exists(home), "only the hub: the home stays")
}

func TestRemoveHubDirOfAMissingHubIsHarmless(t *testing.T) {
	removeHubDir(stepOut(t), filepath.Join(t.TempDir(), "nope"))
}

func TestCleanToolDataRemovesTheListedFoldersOnly(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"data/tool/db": "x", "config/tool/c": "y", "config/other/keep": "z"})
	cleanToolData(stepOut(t), []string{filepath.Join(root, "data", "tool"), filepath.Join(root, "config", "tool"), filepath.Join(root, "missing")})
	assert.False(t, exists(filepath.Join(root, "data", "tool")))
	assert.False(t, exists(filepath.Join(root, "config", "tool")))
	assert.True(t, exists(filepath.Join(root, "config", "other", "keep")))
}

// The step counter shown as [n/total] follows the steps that will run.
func TestCountPurgeSteps(t *testing.T) {
	for _, tc := range []struct {
		name string
		inv  purgeInventory
		want int
	}{
		{"nothing found, binary kept", purgeInventory{keepBinary: true}, 2},
		{"nothing found, binary removed", purgeInventory{}, 3},
		{"secrets", purgeInventory{keepBinary: true, secrets: make([]keychain.SecretEntry, 1)}, 3},
		{"projects", purgeInventory{keepBinary: true, projects: []purgeProject{{name: "p"}}}, 3},
		{"tool data", purgeInventory{keepBinary: true, includeToolData: true}, 3},
		{"projects, tool data and binary", purgeInventory{includeToolData: true, projects: []purgeProject{{name: "p"}}}, 5},
	} {
		assert.Equal(t, tc.want, countPurgeSteps(tc.inv), tc.name)
	}
}

func TestFormatBytes(t *testing.T) {
	for in, want := range map[int64]string{
		0: "0 B", 1023: "1023 B", 1024: "1.0 KB", 1536: "1.5 KB", 5 << 20: "5.0 MB", 3 << 30: "3.0 GB",
	} {
		assert.Equal(t, want, formatBytes(in), "%d", in)
	}
}

// purgeFixture is a home with a hub holding a database with one project, and
// the leftovers a purge looks for: the files of a former deployment and the
// Dolt database of Beads in the project, and the folders of the tool (named
// by its adapter) in the home of the user.
type purgeFixture struct {
	home, hub, project string
	leftovers          []string // everything a purge would delete
}

func newPurgeFixture(t *testing.T) purgeFixture {
	t.Helper()
	loc := toolDataLocator()
	require.NotNil(t, loc, "the composition root registers a tool adapter")
	f := purgeFixture{home: configtest.Home(t)}
	f.hub = filepath.Join(f.home, ".oh")
	f.project = filepath.Join(t.TempDir(), "web")

	files := map[string]string{
		filepath.Join(f.hub, "hub", ".version"):          "dev",
		filepath.Join(f.project, "src", "main.go"):       "package main",
		filepath.Join(f.project, ".beads", "dolt", "db"): "dolt",
	}
	for _, rel := range loc.ProjectFiles() {
		files[filepath.Join(f.project, rel, "marker")] = "x"
	}
	for _, d := range loc.UserDataDirs() {
		require.True(t, strings.HasPrefix(d, f.home), "%s is under the home of the test", d)
		files[filepath.Join(d, "marker")] = "x"
	}
	for p, c := range files {
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(c), 0o644))
		f.leftovers = append(f.leftovers, p)
	}

	s, err := sqlite.Open(filepath.Join(f.hub, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	require.NoError(t, sqlite.NewProjectStore(s).Create(context.Background(),
		&domain.Project{ID: "web", Name: "web", Path: f.project, Status: domain.ProjectStatusActive}))
	prev := store
	store = s
	t.Cleanup(func() { store = prev })
	f.leftovers = append(f.leftovers, filepath.Join(f.hub, "oh.db"))
	return f
}

func (f purgeFixture) assertUntouched(t *testing.T) {
	t.Helper()
	for _, p := range f.leftovers {
		assert.FileExists(t, p, "the purge must not have deleted it")
	}
}

// purgeCmdFor is a purge command with its options, parsed with args.
func purgeCmdFor(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	c := &cobra.Command{Use: "purge"}
	c.Flags().Bool("dry-run", false, "")
	c.Flags().Bool("force", false, "")
	c.Flags().Bool("keep-binary", false, "")
	c.Flags().Bool("include-tool-data", false, "")
	addYesFlag(c)
	require.NoError(t, c.ParseFlags(args))
	c.SetContext(context.Background())
	return c
}

// runPurgeCaptured runs the purge and returns what it printed on stdout.
func runPurgeCaptured(t *testing.T, c *cobra.Command) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	prev := os.Stdout
	os.Stdout = w
	runErr := runPurge(c, nil)
	os.Stdout = prev
	require.NoError(t, w.Close())
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(data), runErr
}

// --dry-run lists what a purge would delete: in the project, in the hub and,
// with --include-tool-data, in the folders of the tool; it deletes nothing.
func TestPurgeDryRunListsAndDeletesNothing(t *testing.T) {
	i18ntest.Set(t, "en")
	f := newPurgeFixture(t)
	loc := toolDataLocator()

	out, err := runPurgeCaptured(t, purgeCmdFor(t, "--dry-run", "--keep-binary", "--include-tool-data"))
	require.NoError(t, err)

	assert.Contains(t, out, f.hub)
	assert.Contains(t, out, filepath.Join(f.project, ".beads", "dolt"))
	for _, rel := range loc.ProjectFiles() {
		assert.Contains(t, out, filepath.Join(f.project, rel))
	}
	for _, d := range loc.UserDataDirs() {
		assert.Contains(t, out, d)
	}
	assert.NotContains(t, out, filepath.Join(f.project, "src"), "the code of the project is not listed")
	f.assertUntouched(t)
}

// Without --include-tool-data the tool folders are not listed.
func TestPurgeDryRunLeavesToolDataOutByDefault(t *testing.T) {
	i18ntest.Set(t, "en")
	f := newPurgeFixture(t)
	out, err := runPurgeCaptured(t, purgeCmdFor(t, "--dry-run", "--keep-binary"))
	require.NoError(t, err)
	assert.Contains(t, out, f.hub)
	for _, d := range toolDataLocator().UserDataDirs() {
		assert.NotContains(t, out, d)
	}
	f.assertUntouched(t)
}

// Without --yes and without a terminal (a script, an agent) the purge shows
// its inventory, is refused with the usage code, and deletes nothing.
func TestPurgeWithoutTerminalIsRefusedAndDeletesNothing(t *testing.T) {
	i18ntest.Set(t, "en")
	terminalIs(t, false)
	f := newPurgeFixture(t)
	out, err := runPurgeCaptured(t, purgeCmdFor(t, "--keep-binary"))
	require.Error(t, err)
	assert.Equal(t, ExitUsage, ExitCode(err))
	assert.Contains(t, err.Error(), "--yes")
	assert.Contains(t, out, f.hub, "the inventory is shown before the question")
	f.assertUntouched(t)
}
