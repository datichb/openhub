package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/filelock"
)

// ErrSchemaNewer is returned when the database holds migrations unknown to
// this binary: a more recent oh wrote it (see SchemaNewerError).
var ErrSchemaNewer = errors.New("database schema is newer than this oh")

// SchemaNewerError lists the applied migrations this binary does not know.
type SchemaNewerError struct {
	Unknown []int // applied versions unknown to this binary
	Known   int   // MaxKnownVersion
}

func (e *SchemaNewerError) Error() string {
	return fmt.Sprintf("%v: applied migrations %v, this oh knows up to v%d", ErrSchemaNewer, e.Unknown, e.Known)
}

func (e *SchemaNewerError) Unwrap() error { return ErrSchemaNewer }

// MigrationError is a migration that failed; Backup is the copy of the
// database taken before the first migration of the run ("" if none).
type MigrationError struct {
	Version int
	Backup  string
	Err     error
}

func (e *MigrationError) Error() string {
	msg := fmt.Sprintf("migration v%d: %v", e.Version, e.Err)
	if e.Backup != "" {
		msg += fmt.Sprintf(" (backup: %s; restore with `oh repair --restore %s`)", e.Backup, e.Backup)
	}
	return msg
}

func (e *MigrationError) Unwrap() error { return e.Err }

// Lock and backup settings.
const (
	migrateLockWait = 30 * time.Second
	backupsKept     = 3
)

// MaxKnownVersion is the highest migration this binary knows.
func MaxKnownVersion() int {
	n := 0
	for _, m := range schemaMigrations {
		n = max(n, m.version)
	}
	return n
}

// SchemaNewer reports a database written by a more recent oh, opened with
// OpenOptions.AllowNewer (nil otherwise).
func (s *Store) SchemaNewer() *SchemaNewerError { return s.newer }

// Path is the database file.
func (s *Store) Path() string { return s.path }

// schemaState reads the applied migrations. A database without the
// schema_migrations table has none.
type schemaState struct {
	applied    map[int]bool
	maxApplied int
	pending    []migration
	unknown    []int
}

type querier interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

func readSchemaState(ctx context.Context, q querier) (schemaState, error) {
	st := schemaState{applied: map[int]bool{}}
	var n int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'`).Scan(&n); err != nil {
		return st, fmt.Errorf("reading applied migrations: %w", err)
	}
	if n > 0 {
		rows, err := q.QueryContext(ctx, `SELECT version FROM schema_migrations`)
		if err != nil {
			return st, fmt.Errorf("reading applied migrations: %w", err)
		}
		for rows.Next() {
			var v int
			if err := rows.Scan(&v); err != nil {
				rows.Close()
				return st, fmt.Errorf("reading applied migrations: %w", err)
			}
			st.applied[v] = true
			st.maxApplied = max(st.maxApplied, v)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return st, fmt.Errorf("reading applied migrations: %w", err)
		}
	}
	known := map[int]bool{}
	for _, m := range schemaMigrations {
		known[m.version] = true
		// Up to sequentialMigrations, a version below the highest applied one
		// counts as applied (historical behaviour). Above it, every missing
		// migration is applied: parallel branches reserve version numbers and
		// may merge out of order (a v32 merged after a v33 must run).
		if st.applied[m.version] || (m.version <= sequentialMigrations && m.version <= st.maxApplied) {
			continue
		}
		st.pending = append(st.pending, m)
	}
	for v := range st.applied {
		if v > sequentialMigrations && !known[v] {
			st.unknown = append(st.unknown, v)
		}
	}
	sort.Ints(st.unknown)
	return st, nil
}

// PendingMigrations lists the versions Migrate would apply.
func (s *Store) PendingMigrations() ([]int, error) {
	st, err := readSchemaState(context.Background(), s.db)
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, len(st.pending))
	for _, m := range st.pending {
		out = append(out, m.version)
	}
	return out, nil
}

// UnknownMigrations lists the applied versions this binary does not know.
func (s *Store) UnknownMigrations() ([]int, error) {
	st, err := readSchemaState(context.Background(), s.db)
	if err != nil {
		return nil, err
	}
	return st.unknown, nil
}

// Migrate applies the pending migrations (a database written by a more recent
// oh is refused with ErrSchemaNewer).
func (s *Store) Migrate() error { return s.migrateWith(false) }

func (s *Store) migrate() error { return s.migrateWith(false) }

// migrateWith applies the pending migrations.
//
// Several processes open the database at once after an upgrade (TUI, daemon,
// MCP servers): the fast path reads the state without any lock; when
// something is pending, the run holds an inter-process file lock, then each
// version runs in its own BEGIN IMMEDIATE transaction that reads
// schema_migrations again. Before the first migration of an existing
// database, a copy is written under <dir>/backups (VACUUM INTO).
func (s *Store) migrateWith(allowNewer bool) error {
	ctx := context.Background()
	st, err := readSchemaState(ctx, s.db)
	if err != nil {
		return err
	}
	if len(st.unknown) > 0 {
		e := &SchemaNewerError{Unknown: st.unknown, Known: MaxKnownVersion()}
		if !allowNewer {
			return e
		}
		// Never touch the schema of a more recent oh.
		s.newer = e
		return nil
	}
	if len(st.pending) == 0 {
		return nil
	}

	lctx, cancel := context.WithTimeout(ctx, migrateLockWait)
	defer cancel()
	unlock, err := filelock.LockContext(lctx, s.path+".migrate.lock")
	if err != nil {
		return fmt.Errorf("waiting for another oh to finish migrating the database: %w", err)
	}
	defer unlock()

	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}
	st, err = readSchemaState(ctx, conn)
	if err != nil {
		return err
	}
	if len(st.unknown) > 0 && !allowNewer {
		return &SchemaNewerError{Unknown: st.unknown, Known: MaxKnownVersion()}
	}
	backup := ""
	if len(st.pending) > 0 && st.maxApplied > 0 && os.Getenv("OH_SKIP_DB_BACKUP") != "1" {
		backup, err = s.backupBeforeMigration(ctx, conn, st.maxApplied)
		if err != nil {
			return fmt.Errorf("backing up the database before migrating (set OH_SKIP_DB_BACKUP=1 to skip): %w", err)
		}
	}
	for _, m := range st.pending {
		if err := runMigration(ctx, conn, m); err != nil {
			return &MigrationError{Version: m.version, Backup: backup, Err: err}
		}
	}
	return nil
}

// runMigration applies one migration in a BEGIN IMMEDIATE transaction (the
// schema change and its version record are committed together) unless
// another process recorded it meanwhile.
func runMigration(ctx context.Context, conn *sql.Conn, m migration) (err error) {
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_, _ = conn.ExecContext(ctx, "ROLLBACK")
		}
	}()
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, m.version).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		_, err = conn.ExecContext(ctx, "COMMIT")
		return err
	}
	for _, stmt := range splitStatements(m.up) {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			if ok, cerr := alreadyApplied(ctx, conn, stmt, err); cerr != nil || !ok {
				return err
			}
		}
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, m.version); err != nil {
		return fmt.Errorf("recording: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("committing: %w", err)
	}
	return nil
}

var (
	reAddColumn  = regexp.MustCompile(`(?is)^ALTER\s+TABLE\s+(\w+)\s+ADD\s+COLUMN\s+(\w+)`)
	reDropColumn = regexp.MustCompile(`(?is)^ALTER\s+TABLE\s+(\w+)\s+DROP\s+COLUMN\s+(\w+)`)
	reCreate     = regexp.MustCompile(`(?is)^CREATE\s+(?:UNIQUE\s+)?(TABLE|INDEX)\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)
)

// alreadyApplied tolerates a statement whose effect is already there (a
// database half-migrated by an older oh that did not lock): a column added
// twice, a column already dropped, a table or index that exists. The
// database state is checked, the error message alone is not trusted.
func alreadyApplied(ctx context.Context, conn *sql.Conn, stmt string, err error) (bool, error) {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "duplicate column"):
		if m := reAddColumn.FindStringSubmatch(stmt); m != nil {
			return tableHasColumn(ctx, conn, m[1], m[2])
		}
	case strings.Contains(msg, "no such column"):
		if m := reDropColumn.FindStringSubmatch(stmt); m != nil {
			has, err := tableHasColumn(ctx, conn, m[1], m[2])
			return !has && err == nil, err
		}
	case strings.Contains(msg, "already exists"):
		if m := reCreate.FindStringSubmatch(stmt); m != nil {
			var n int
			err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?`, strings.ToLower(m[1]), m[2]).Scan(&n)
			return n > 0, err
		}
	}
	return false, nil
}

func tableHasColumn(ctx context.Context, conn *sql.Conn, table, column string) (bool, error) {
	var n int
	err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&n)
	return n > 0, err
}

// splitStatements splits a migration on the semicolons outside quotes.
func splitStatements(sqlText string) []string {
	var out []string
	var b strings.Builder
	quote := rune(0)
	flush := func() {
		if s := strings.TrimSpace(b.String()); s != "" {
			out = append(out, s)
		}
		b.Reset()
	}
	for _, r := range sqlText {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ';':
			flush()
			continue
		}
		b.WriteRune(r)
	}
	flush()
	return out
}

// BackupDir is where the copies taken before a migration are kept.
func (s *Store) BackupDir() string { return filepath.Join(filepath.Dir(s.path), "backups") }

// Backup writes a consistent copy of the database (WAL included) to dest,
// owner-only. dest must not exist.
func (s *Store) Backup(dest string) error {
	conn, err := s.db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()
	return vacuumInto(context.Background(), conn, dest)
}

func vacuumInto(ctx context.Context, conn *sql.Conn, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}
	// VACUUM INTO accepts an empty file: created first with the right mode.
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	f.Close()
	if _, err := conn.ExecContext(ctx, `VACUUM INTO ?`, dest); err != nil {
		_ = os.Remove(dest)
		return err
	}
	return os.Chmod(dest, 0o600)
}

// backupBeforeMigration copies the database to
// <dir>/backups/<name>.v<from>-<time>.bak and keeps the last backupsKept.
func (s *Store) backupBeforeMigration(ctx context.Context, conn *sql.Conn, from int) (string, error) {
	dir := s.BackupDir()
	base := filepath.Base(s.path)
	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	dest := filepath.Join(dir, fmt.Sprintf("%s.v%d-%s.bak", base, from, stamp))
	if err := vacuumInto(ctx, conn, dest); err != nil {
		return "", err
	}
	pruneBackups(dir, base)
	return dest, nil
}

// Backups lists the copies taken before a migration, most recent first.
func (s *Store) Backups() []string { return listBackups(s.BackupDir(), filepath.Base(s.path)) }

func listBackups(dir, base string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, base+".v*-*.bak"))
	type entry struct {
		path string
		mod  time.Time
	}
	es := make([]entry, 0, len(matches))
	for _, m := range matches {
		if fi, err := os.Stat(m); err == nil {
			es = append(es, entry{m, fi.ModTime()})
		}
	}
	sort.Slice(es, func(i, j int) bool {
		if !es[i].mod.Equal(es[j].mod) {
			return es[i].mod.After(es[j].mod)
		}
		return es[i].path > es[j].path
	})
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.path
	}
	return out
}

func pruneBackups(dir, base string) {
	for i, p := range listBackups(dir, base) {
		if i >= backupsKept {
			_ = os.Remove(p)
		}
	}
}

// Restore replaces the database at path with a backup (the -wal and -shm
// files of the replaced database are removed). No process may use the
// database meanwhile; the replaced database is kept as <path>.replaced-<time>.
func Restore(backup, path string) (string, error) {
	src, err := OpenRaw(backup)
	if err != nil {
		return "", err
	}
	ierr := src.IntegrityCheck()
	src.Close()
	if ierr != nil {
		return "", ierr
	}
	kept := ""
	if _, err := os.Stat(path); err == nil {
		kept = fmt.Sprintf("%s.replaced-%s", path, time.Now().UTC().Format("20060102T150405Z"))
		if cur, err := OpenRaw(path); err == nil {
			err = cur.Backup(kept)
			cur.Close()
			if err != nil {
				kept = ""
			}
		}
		if kept == "" {
			// Unreadable database: keep the raw files.
			kept = path + ".replaced-" + time.Now().UTC().Format("20060102T150405Z")
			if err := os.Rename(path, kept); err != nil {
				return "", err
			}
		}
	}
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	data, err := os.ReadFile(backup)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return kept, nil
}
