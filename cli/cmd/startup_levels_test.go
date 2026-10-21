package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// runOhHome runs oh with OH_HOME=home (stdin empty).
func runOhHome(t *testing.T, home string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(testBinary, args...)
	cmd.Env = append(os.Environ(), "OH_HOME="+home, "OH_LANG=", "NO_COLOR=1")
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		require.True(t, ok, "running oh: %v", err)
		code = ee.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}

func newOhHome(t *testing.T) string {
	t.Helper()
	home := filepath.Join(t.TempDir(), "oh")
	require.NoError(t, os.MkdirAll(home, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, "hub.toml"), []byte("[cli]\nlanguage = \"en\"\nsetup_done = true\n"), 0o600))
	return home
}

// N13: with a hub.toml that does not load, oh doctor still runs every
// check, reports the TOML error with its position and exits with 1; a full
// command points to oh doctor.
func TestDoctorWithBrokenConfig(t *testing.T) {
	t.Parallel()
	home := newOhHome(t)
	require.NoError(t, os.WriteFile(filepath.Join(home, "hub.toml"), []byte("[cli]\nlanguage = \"en\nx"), 0o600))

	out, _, code := runOhHome(t, home, "doctor")
	assert.Equal(t, 1, code, out)
	assert.Contains(t, out, "does not load")
	assert.Contains(t, out, "line 2")
	assert.Contains(t, out, "git", "the other checks run")
	assert.Contains(t, out, "Database")

	out, _, code = runOhHome(t, home, "repair", "--check-only")
	assert.Equal(t, 0, code, out)
	assert.Contains(t, out, "Database missing")

	_, errOut, code := runOhHome(t, home, "project", "list")
	assert.NotEqual(t, 0, code)
	assert.Contains(t, errOut, "oh doctor")
}

// N13: oh doctor --fix restores the latest valid backup of hub.toml.
func TestDoctorFixRestoresConfig(t *testing.T) {
	t.Parallel()
	home := newOhHome(t)
	good, err := os.ReadFile(filepath.Join(home, "hub.toml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(home, "hub.toml.bak-20261001T000000.000000000Z"), good, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, "hub.toml"), []byte("[cli\n"), 0o600))

	_, errOut, code := runOhHome(t, home, "doctor", "--fix")
	assert.NotEqual(t, 0, code)
	assert.Contains(t, errOut, "--yes", "no terminal: --yes required")
	out, _, _ := runOhHome(t, home, "doctor", "--fix", "--yes")
	assert.Contains(t, out, "Configuration restored")
	got, err := os.ReadFile(filepath.Join(home, "hub.toml"))
	require.NoError(t, err)
	assert.Equal(t, string(good), string(got))
	broken, _ := filepath.Glob(filepath.Join(home, "hub.toml.broken-*"))
	assert.Len(t, broken, 1, "the broken file is kept")
}

// N13 + N14: a migration that fails is shown by oh doctor, oh repair says
// what is pending and restores the backup taken before the migration.
func TestFailedMigrationDoctorAndRepair(t *testing.T) {
	t.Parallel()
	home := newOhHome(t)
	_, errOut, code := runOhHome(t, home, "project", "list")
	require.Equal(t, 0, code, errOut)
	db := filepath.Join(home, "oh.db")
	raw, err := sqlite.OpenRaw(db)
	require.NoError(t, err)
	// v42 reads usage_sessions: without it, it fails (not "already applied").
	_, err = raw.DB().Exec(`DELETE FROM schema_migrations WHERE version = 42; DROP TABLE usage_sessions`)
	require.NoError(t, err)
	raw.Close()

	_, errOut, code = runOhHome(t, home, "project", "list")
	assert.NotEqual(t, 0, code)
	assert.Contains(t, errOut, "migration v42 failed")
	assert.Contains(t, errOut, "oh repair --restore")

	out, _, code := runOhHome(t, home, "doctor")
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "migration failed")

	out, _, code = runOhHome(t, home, "repair")
	assert.Equal(t, 0, code, out)
	assert.Contains(t, out, "Pending migrations: v42")
	assert.Contains(t, out, "Last failed migration")
	out, _, code = runOhHome(t, home, "repair", "--migrate")
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "oh repair --restore")

	backups, _ := filepath.Glob(filepath.Join(home, "backups", "oh.db.v*-*.bak"))
	require.NotEmpty(t, backups)
	_, errOut, code = runOhHome(t, home, "repair", "--restore", backups[0])
	assert.NotEqual(t, 0, code, "no terminal: --auto required")
	assert.Contains(t, errOut, "--auto")
	out, _, code = runOhHome(t, home, "repair", "--restore", backups[0], "--auto")
	assert.Equal(t, 0, code, out)
	assert.Contains(t, out, "Database restored")
}

// N13: a corrupt database does not stop oh repair (no MustApp) nor doctor.
func TestRepairCorruptDatabase(t *testing.T) {
	t.Parallel()
	home := newOhHome(t)
	require.NoError(t, os.WriteFile(filepath.Join(home, "oh.db"), []byte(strings.Repeat("garbage!", 1024)), 0o600))
	out, _, code := runOhHome(t, home, "repair")
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "Recovery options")
	assert.Contains(t, out, "last resort")
	saved, _ := filepath.Glob(filepath.Join(home, "oh.db.corrupt-*"))
	assert.NotEmpty(t, saved)

	out, _, code = runOhHome(t, home, "doctor")
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "oh repair")
}

// N13: the MCP server workflow talks to the daemon only: it starts with a
// database that cannot open, and never creates one.
func TestMCPServeWorkflowNeedsNoDatabase(t *testing.T) {
	t.Parallel()
	home := newOhHome(t)
	_, errOut, code := runOhHome(t, home, "mcp", "serve", "workflow")
	assert.Equal(t, 0, code, errOut)
	assert.NoFileExists(t, filepath.Join(home, "oh.db"))

	require.NoError(t, os.Mkdir(filepath.Join(home, "oh.db"), 0o700))
	_, errOut, code = runOhHome(t, home, "mcp", "serve", "workflow")
	assert.Equal(t, 0, code, errOut)
	assert.NotContains(t, errOut, "database")
}
