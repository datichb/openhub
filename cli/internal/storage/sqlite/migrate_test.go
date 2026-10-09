package sqlite

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func countMigrations(t *testing.T, s *Store) (rows, distinct int) {
	t.Helper()
	require.NoError(t, s.DB().QueryRow(`SELECT COUNT(*), COUNT(DISTINCT version) FROM schema_migrations`).Scan(&rows, &distinct))
	return rows, distinct
}

// N14: several connections (TUI, daemon, MCP servers after an upgrade) open
// a new database at once: every one succeeds, each version recorded once.
func TestConcurrentOpenMigratesOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oh.db")
	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	stores := make([]*Store, n)
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			stores[i], errs[i] = Open(path)
		}(i)
	}
	wg.Wait()
	for i := range n {
		require.NoError(t, errs[i])
		defer stores[i].Close()
	}
	rows, distinct := countMigrations(t, stores[0])
	assert.Equal(t, len(schemaMigrations), rows)
	assert.Equal(t, rows, distinct)
}

// The child process of TestMultiProcessOpen.
func TestHelperOpenProcess(t *testing.T) {
	path := os.Getenv("OH_TEST_MIGRATE_DB")
	if path == "" {
		t.Skip("helper process")
	}
	s, err := Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	s.Close()
}

// N14: the same race across processes, on a new database and on a database
// left at an older version.
func TestMultiProcessOpen(t *testing.T) {
	for _, tc := range []struct {
		name string
		prep func(t *testing.T, path string)
	}{
		{"new", func(*testing.T, string) {}},
		{"older", func(t *testing.T, path string) {
			s, err := Open(path)
			require.NoError(t, err)
			_, err = s.DB().Exec(`ALTER TABLE sessions DROP COLUMN tickets;
ALTER TABLE sessions DROP COLUMN start_ref;
DELETE FROM schema_migrations WHERE version IN (41, 42);
ALTER TABLE projects DROP COLUMN exec_config`)
			require.NoError(t, err)
			s.Close()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "oh.db")
			tc.prep(t, path)
			const n = 6
			cmds := make([]*exec.Cmd, n)
			outs := make([]*lockedBuf, n)
			for i := range n {
				c := exec.Command(os.Args[0], "-test.run=^TestHelperOpenProcess$", "-test.count=1")
				c.Env = append(os.Environ(), "OH_TEST_MIGRATE_DB="+path)
				outs[i] = &lockedBuf{}
				c.Stdout, c.Stderr = outs[i], outs[i]
				require.NoError(t, c.Start())
				cmds[i] = c
			}
			for i, c := range cmds {
				assert.NoError(t, c.Wait(), "process %d: %s", i, outs[i].String())
			}
			s, err := Open(path)
			require.NoError(t, err)
			defer s.Close()
			rows, distinct := countMigrations(t, s)
			assert.Equal(t, rows, distinct)
			pending, err := s.PendingMigrations()
			require.NoError(t, err)
			assert.Empty(t, pending)
			has, err := hasColumnDB(s, "projects", "exec_config")
			require.NoError(t, err)
			assert.True(t, has)
		})
	}
}

type lockedBuf struct {
	mu sync.Mutex
	b  []byte
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.b = append(l.b, p...)
	return len(p), nil
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return string(l.b)
}

func hasColumnDB(s *Store, table, col string) (bool, error) {
	var n int
	err := s.DB().QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, col).Scan(&n)
	return n > 0, err
}

// N14: a database half-migrated by an older oh (schema change done, version
// not recorded) opens: the existing column, dropped column and index are
// recognised from the schema itself.
func TestHalfMigratedDatabaseOpens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oh.db")
	s, err := Open(path)
	require.NoError(t, err)
	_, err = s.DB().Exec(`DELETE FROM schema_migrations WHERE version IN (12, 37, 41, 42)`)
	require.NoError(t, err)
	s.Close()

	s, err = Open(path)
	require.NoError(t, err, "duplicate column / no such column / index exists tolerated")
	defer s.Close()
	pending, err := s.PendingMigrations()
	require.NoError(t, err)
	assert.Empty(t, pending)
	rows, distinct := countMigrations(t, s)
	assert.Equal(t, rows, distinct)
}

// A real failure is not swallowed: a statement that fails for another reason.
func TestAlreadyAppliedChecksTheSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oh.db")
	s, err := Open(path)
	require.NoError(t, err)
	defer s.Close()
	withMigration(t, migration{version: 9001, up: `ALTER TABLE nosuch ADD COLUMN x TEXT`})
	err = s.migrate()
	var me *MigrationError
	require.ErrorAs(t, err, &me)
	assert.Equal(t, 9001, me.Version)
}

func withMigration(t *testing.T, m migration) {
	t.Helper()
	saved := schemaMigrations
	schemaMigrations = append(append([]migration{}, saved...), m)
	t.Cleanup(func() { schemaMigrations = saved })
}

// N14: a copy is taken before migrating an existing database; it opens and
// holds the rows still in the WAL; three are kept.
func TestBackupBeforeMigration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "oh.db")
	s, err := Open(path)
	require.NoError(t, err)
	_, err = s.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1', 'one', '/tmp/one')`)
	require.NoError(t, err)
	assert.Empty(t, s.Backups(), "no backup for a new database")

	for i := range 5 {
		_, err = s.DB().Exec(`DELETE FROM schema_migrations WHERE version = 42`)
		require.NoError(t, err)
		require.NoError(t, s.migrate(), "run %d", i)
	}
	backups := s.Backups()
	require.Len(t, backups, backupsKept)
	fi, err := os.Stat(backups[0])
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
	assert.Contains(t, filepath.Base(backups[0]), fmt.Sprintf("oh.db.v%d-", MaxKnownVersion()))
	s.Close()

	b, err := OpenRaw(backups[0])
	require.NoError(t, err)
	defer b.Close()
	var name string
	require.NoError(t, b.DB().QueryRow(`SELECT name FROM projects WHERE id = 'p1'`).Scan(&name))
	assert.Equal(t, "one", name)
	require.NoError(t, b.IntegrityCheck())
}

// N14: no backup, no migration (unless OH_SKIP_DB_BACKUP=1).
func TestBackupFailureRefusesToMigrate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "oh.db")
	s, err := Open(path)
	require.NoError(t, err)
	_, err = s.DB().Exec(`DELETE FROM schema_migrations WHERE version = 42`)
	require.NoError(t, err)
	s.Close()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "backups"), []byte("x"), 0o600))

	_, err = Open(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OH_SKIP_DB_BACKUP")

	t.Setenv("OH_SKIP_DB_BACKUP", "1")
	s, err = Open(path)
	require.NoError(t, err)
	s.Close()
}

// N14: a failed migration names the backup; Restore puts it back.
func TestMigrationErrorNamesTheBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oh.db")
	s, err := Open(path)
	require.NoError(t, err)
	_, err = s.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1', 'one', '/tmp/one')`)
	require.NoError(t, err)
	s.Close()

	withMigration(t, migration{version: 9002, up: `CREATE TABLE t9002 (x TEXT); INSERT INTO nosuch VALUES (1)`})
	_, err = Open(path)
	var me *MigrationError
	require.ErrorAs(t, err, &me)
	require.NotEmpty(t, me.Backup)
	assert.Contains(t, err.Error(), "oh repair --restore")

	raw, err := OpenRaw(path)
	require.NoError(t, err)
	var n int
	require.NoError(t, raw.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 't9002'`).Scan(&n))
	assert.Zero(t, n, "the failed migration is rolled back")
	_, err = raw.DB().Exec(`DELETE FROM projects`)
	require.NoError(t, err)
	raw.Close()

	kept, err := Restore(me.Backup, path)
	require.NoError(t, err)
	assert.FileExists(t, kept)
	raw, err = OpenRaw(path)
	require.NoError(t, err)
	defer raw.Close()
	require.NoError(t, raw.DB().QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&n))
	assert.Equal(t, 1, n)
}

// N14: a schema written by a more recent oh is refused, unless allowed.
func TestSchemaNewer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oh.db")
	s, err := Open(path)
	require.NoError(t, err)
	_, err = s.DB().Exec(`INSERT INTO schema_migrations (version) VALUES (999)`)
	require.NoError(t, err)
	s.Close()

	_, err = Open(path)
	require.ErrorIs(t, err, ErrSchemaNewer)
	var ne *SchemaNewerError
	require.ErrorAs(t, err, &ne)
	assert.Equal(t, []int{999}, ne.Unknown)
	assert.Equal(t, MaxKnownVersion(), ne.Known)

	s, err = OpenWith(path, OpenOptions{AllowNewer: true})
	require.NoError(t, err)
	require.NotNil(t, s.SchemaNewer())
	s.Close()

	t.Setenv("OH_ALLOW_NEWER_SCHEMA", "1")
	s, err = Open(path)
	require.NoError(t, err)
	s.Close()
}

// N14: nothing pending → no lock file, no backup.
func TestFastPathTakesNoLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oh.db")
	s, err := Open(path)
	require.NoError(t, err)
	s.Close()
	require.NoError(t, os.Remove(path+".migrate.lock"))
	s, err = Open(path)
	require.NoError(t, err)
	defer s.Close()
	_, err = os.Stat(path + ".migrate.lock")
	assert.True(t, errors.Is(err, os.ErrNotExist))
	assert.Empty(t, s.Backups())
}

func TestOpenRawNeedsAnExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oh.db")
	_, err := OpenRaw(path)
	require.Error(t, err)
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err), "OpenRaw never creates the database")
}

func TestSplitStatements(t *testing.T) {
	assert.Equal(t, []string{"A", "B 'x;y'", "C"}, splitStatements("A;\n B 'x;y' ;\nC;"))
}

// N13: the last failed migration is kept for oh doctor, cleared on success.
func TestLastMigrationError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oh.db")
	s, err := Open(path)
	require.NoError(t, err)
	s.Close()
	assert.Empty(t, LastMigrationError(path))
	withMigration(t, migration{version: 9003, up: `INSERT INTO nosuch VALUES (1)`})
	_, err = Open(path)
	require.Error(t, err)
	assert.Contains(t, LastMigrationError(path), "migration v9003")
	schemaMigrations = schemaMigrations[:len(schemaMigrations)-1]
	withMigration(t, migration{version: 9003, up: `CREATE TABLE t9003 (x TEXT)`})
	s, err = Open(path)
	require.NoError(t, err)
	s.Close()
	assert.Empty(t, LastMigrationError(path))
}
