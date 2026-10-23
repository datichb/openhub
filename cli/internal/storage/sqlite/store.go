// Package sqlite provides SQLite-backed implementations of domain store interfaces.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/config"
	_ "modernc.org/sqlite"
)

// Store wraps the SQLite connection shared across all store implementations.
type Store struct {
	db    *sql.DB
	path  string
	newer *SchemaNewerError // schema written by a more recent oh (AllowNewer)
}

// DBPath returns the default path to the SQLite database.
func DBPath() string {
	return filepath.Join(config.HubDir(), "oh.db")
}

// OpenOptions tunes Open.
type OpenOptions struct {
	// AllowNewer opens a database whose schema was written by a more recent
	// oh (no migration is run; SchemaNewer reports it). Without it, such a
	// database is refused with ErrSchemaNewer unless OH_ALLOW_NEWER_SCHEMA=1.
	AllowNewer bool
}

// Open opens (or creates) the SQLite database and runs migrations.
func Open(path string) (*Store, error) {
	return OpenWith(path, OpenOptions{})
}

// OpenWith opens (or creates) the SQLite database and runs the pending
// migrations (see Migrate).
func OpenWith(path string, opts OpenOptions) (*Store, error) {
	s, err := open(path, true)
	if err != nil {
		return nil, err
	}
	if err := s.migrateWith(opts.AllowNewer || os.Getenv("OH_ALLOW_NEWER_SCHEMA") == "1"); err != nil {
		s.db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	return s, nil
}

// OpenRaw opens an existing database without running any migration (repair,
// doctor): the file must exist.
func OpenRaw(path string) (*Store, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	return open(path, false)
}

func open(path string, create bool) (*Store, error) {
	if create {
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("creating db directory: %w", err)
		}
		// The database holds server passwords and proxy tokens: owner-only
		// access. SQLite creates the -wal/-shm files with the mode of the main
		// file, so it is created (or fixed) before the first connection.
		_ = os.Chmod(filepath.Dir(path), 0o700)
		if f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600); err == nil {
			f.Close()
		}
	}
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		_ = os.Chmod(p, 0o600)
	}

	// Pragmas in the DSN apply to every pooled connection (the oh daemon and
	// the CLI/TUI processes share this database: wait for locks instead of
	// failing immediately).
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	// Switching a new database to WAL takes an exclusive lock that the busy
	// timeout does not always wait for (several processes opening it at the
	// first start): retried for a while.
	err = db.Ping()
	for wait := 20 * time.Millisecond; err != nil && isBusy(err) && wait < 5*time.Second; wait *= 2 {
		time.Sleep(wait)
		err = db.Ping()
	}
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("opening database: %w", err)
	}
	return &Store{db: db, path: path}, nil
}

func isBusy(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "SQLITE_BUSY") || strings.Contains(msg, "database is locked")
}

// OpenDefault opens the database at the default path.
func OpenDefault() (*Store, error) {
	return Open(DBPath())
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB (for sub-stores or transactions).
func (s *Store) DB() *sql.DB {
	return s.db
}

// SchemaVersion returns the current applied schema version.
func (s *Store) SchemaVersion() (int, error) {
	var version int
	row := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`)
	if err := row.Scan(&version); err != nil {
		return 0, fmt.Errorf("reading schema version: %w", err)
	}
	return version, nil
}

// IntegrityCheck runs SQLite's PRAGMA integrity_check and returns nil if the database is healthy.
func (s *Store) IntegrityCheck() error {
	rows, err := s.db.Query("PRAGMA integrity_check")
	if err != nil {
		return fmt.Errorf("integrity_check query failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return fmt.Errorf("scanning integrity_check row: %w", err)
		}
		if result != "ok" {
			return fmt.Errorf("database integrity failure: %s", result)
		}
	}
	return rows.Err()
}

// QuickCheck runs PRAGMA quick_check (lighter than IntegrityCheck: it skips the
// consistency of the indexes with the tables) and returns nil if the
// database is healthy, an error carrying what SQLite reports otherwise.
func (s *Store) QuickCheck() error {
	var res string
	if err := s.db.QueryRow(`PRAGMA quick_check`).Scan(&res); err != nil {
		return err
	}
	if res != "ok" {
		return errors.New(res)
	}
	return nil
}

// CountProjects counts the rows of the projects table.
func (s *Store) CountProjects(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects`).Scan(&n)
	return n, err
}

// sequentialMigrations is the last migration of the strictly ordered era;
// later versions are reserved by parallel branches (v5 phase 1+).
const sequentialMigrations = 31

// migration represents a single schema change with an optional rollback.
type migration struct {
	version      int
	up           string
	down         string // rollback; empty only with irreversible
	irreversible bool   // the data cannot be rebuilt: MigrateDown stops here
}

// appliedVersions returns the recorded migration versions.
func (s *Store) appliedVersions() (map[int]bool, error) {
	rows, err := s.db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("reading applied migrations: %w", err)
	}
	defer rows.Close()
	out := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

// MigrateDown rolls back migrations from the current version down to targetVersion (exclusive).
// Only reversible migrations can be rolled back; irreversible ones return an error.
func (s *Store) MigrateDown(targetVersion int) error {
	currentVersion, err := s.SchemaVersion()
	if err != nil {
		return err
	}
	if targetVersion >= currentVersion {
		return nil // nothing to do
	}
	applied, err := s.appliedVersions()
	if err != nil {
		return err
	}

	// Apply down-migrations in reverse order
	for i := len(schemaMigrations) - 1; i >= 0; i-- {
		m := schemaMigrations[i]
		if m.version <= targetVersion || m.version > currentVersion || (!applied[m.version] && m.version > sequentialMigrations) {
			continue
		}
		if m.irreversible {
			return fmt.Errorf("migration v%d is irreversible — cannot downgrade past this version", m.version)
		}
		if m.down == "" {
			return fmt.Errorf("migration v%d has no rollback defined", m.version)
		}
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("beginning transaction for rollback v%d: %w", m.version, err)
		}
		if _, err := tx.Exec(m.down); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("rollback v%d: %w", m.version, err)
		}
		if _, err := tx.Exec(`DELETE FROM schema_migrations WHERE version = ?`, m.version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("removing migration record v%d: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing rollback v%d: %w", m.version, err)
		}
	}
	return nil
}

// schemaMigrations is the ordered list of all schema migrations.
//
// Every migration has a down (the exact inverse of its up, in reverse order;
// an index on a column is dropped before the column) or is marked
// irreversible: TestEveryMigrationIsReversibleOrMarked and
// TestMigrationsRoundTrip enforce it. A down drops what its migration added
// (the data of those columns is lost) and does not undo data changes (v36,
// v42). The downs are for tests and development: oh never rolls back a
// user's database, it restores a backup (oh repair --restore).
var schemaMigrations = []migration{
	{
		version: 1,
		up: `CREATE TABLE IF NOT EXISTS projects (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL,
			path       TEXT NOT NULL UNIQUE,
			language   TEXT NOT NULL DEFAULT '',
			tracker    TEXT NOT NULL DEFAULT '',
			labels     TEXT NOT NULL DEFAULT '',
			agents     TEXT NOT NULL DEFAULT '',
			mcp        TEXT NOT NULL DEFAULT '',
			status     TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		down: `DROP TABLE IF EXISTS projects`,
	},
	{
		version: 2,
		up:      `CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status)`,
		down:    `DROP INDEX IF EXISTS idx_projects_status`,
	},
	{
		version: 3,
		up:      `CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name)`,
		down:    `DROP INDEX IF EXISTS idx_projects_name`,
	},
	{
		version: 4,
		up: `CREATE TABLE IF NOT EXISTS sessions (
			id         TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			ended_at   DATETIME,
			status     TEXT NOT NULL DEFAULT 'running',
			provider   TEXT NOT NULL DEFAULT '',
			model      TEXT NOT NULL DEFAULT '',
			tokens_in  INTEGER NOT NULL DEFAULT 0,
			tokens_out INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
		)`,
		down: `DROP TABLE IF EXISTS sessions`,
	},
	{
		version: 5,
		up:      `CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project_id)`,
		down:    `DROP INDEX IF EXISTS idx_sessions_project`,
	},
	{
		version: 6,
		up:      `CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status)`,
		down:    `DROP INDEX IF EXISTS idx_sessions_status`,
	},
	{
		version: 7,
		up:      `ALTER TABLE projects ADD COLUMN provider TEXT NOT NULL DEFAULT ''`,
		down:    `ALTER TABLE projects DROP COLUMN provider`,
	},
	{
		version: 8,
		up:      `ALTER TABLE projects ADD COLUMN model TEXT NOT NULL DEFAULT ''`,
		down:    `ALTER TABLE projects DROP COLUMN model`,
	},
	{
		version: 9,
		up:      `ALTER TABLE projects ADD COLUMN model_overrides TEXT NOT NULL DEFAULT ''`,
		down:    `ALTER TABLE projects DROP COLUMN model_overrides`,
	},
	{
		version: 10,
		up:      `ALTER TABLE projects ADD COLUMN mcp_config TEXT NOT NULL DEFAULT ''`,
		down:    `ALTER TABLE projects DROP COLUMN mcp_config`,
	},
	{
		version: 11,
		up:      `ALTER TABLE projects ADD COLUMN provider_config TEXT NOT NULL DEFAULT ''`,
		down:    `ALTER TABLE projects DROP COLUMN provider_config`,
	},
	{
		version: 12,
		up:      `DROP INDEX IF EXISTS idx_projects_name; CREATE UNIQUE INDEX idx_projects_name_unique ON projects(name)`,
		down:    `DROP INDEX IF EXISTS idx_projects_name_unique; CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name)`,
	},
	{
		version: 13,
		up: `CREATE TABLE IF NOT EXISTS agent_events (
			id            TEXT PRIMARY KEY,
			session_id    TEXT NOT NULL,
			project_id    TEXT NOT NULL,
			agent_name    TEXT NOT NULL,
			skills_loaded TEXT NOT NULL DEFAULT '[]',
			started_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			completed_at  DATETIME,
			status        TEXT NOT NULL DEFAULT 'success',
			tokens_in     INTEGER NOT NULL DEFAULT 0,
			tokens_out    INTEGER NOT NULL DEFAULT 0,
			cost_usd      REAL NOT NULL DEFAULT 0,
			error_message TEXT NOT NULL DEFAULT ''
		)`,
		down: `DROP TABLE IF EXISTS agent_events`,
	},
	{
		version: 14,
		up:      `CREATE INDEX IF NOT EXISTS idx_agent_events_session ON agent_events(session_id)`,
		down:    `DROP INDEX IF EXISTS idx_agent_events_session`,
	},
	{
		version: 15,
		up:      `CREATE INDEX IF NOT EXISTS idx_agent_events_agent ON agent_events(agent_name)`,
		down:    `DROP INDEX IF EXISTS idx_agent_events_agent`,
	},
	{
		version: 16,
		up:      `CREATE INDEX IF NOT EXISTS idx_agent_events_project ON agent_events(project_id)`,
		down:    `DROP INDEX IF EXISTS idx_agent_events_project`,
	},
	{
		version: 17,
		up:      `ALTER TABLE projects ADD COLUMN team_config TEXT NOT NULL DEFAULT ''`,
		down:    `ALTER TABLE projects DROP COLUMN team_config`,
	},
	{
		version: 18,
		up:      `ALTER TABLE projects ADD COLUMN tracker_config TEXT NOT NULL DEFAULT ''`,
		down:    `ALTER TABLE projects DROP COLUMN tracker_config`,
	},
	{
		version: 19,
		up:      `ALTER TABLE sessions ADD COLUMN launch_path TEXT NOT NULL DEFAULT ''`,
		down:    `ALTER TABLE sessions DROP COLUMN launch_path`,
	},
	{
		version: 20,
		up:      `ALTER TABLE projects ADD COLUMN team_id TEXT DEFAULT NULL`,
		down:    `ALTER TABLE projects DROP COLUMN team_id`,
	},
	{
		version: 21,
		up:      `ALTER TABLE projects ADD COLUMN workflow_config TEXT NOT NULL DEFAULT ''`,
		down:    `ALTER TABLE projects DROP COLUMN workflow_config`,
	},
	{
		version: 22,
		up:      `ALTER TABLE sessions ADD COLUMN member_id TEXT DEFAULT NULL`,
		down:    `ALTER TABLE sessions DROP COLUMN member_id`,
	},
	{
		version: 23,
		up:      `ALTER TABLE agent_events ADD COLUMN member_id TEXT DEFAULT NULL`,
		down:    `ALTER TABLE agent_events DROP COLUMN member_id`,
	},
	{
		version: 24,
		up: `ALTER TABLE sessions ADD COLUMN cost REAL NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN tokens_reasoning INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN tokens_cache_read INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN platform TEXT NOT NULL DEFAULT 'unknown';
ALTER TABLE sessions ADD COLUMN external_session_id TEXT DEFAULT NULL;
ALTER TABLE sessions ADD COLUMN slug TEXT DEFAULT NULL`,
		down: `ALTER TABLE sessions DROP COLUMN slug;
ALTER TABLE sessions DROP COLUMN external_session_id;
ALTER TABLE sessions DROP COLUMN platform;
ALTER TABLE sessions DROP COLUMN tokens_cache_read;
ALTER TABLE sessions DROP COLUMN tokens_reasoning;
ALTER TABLE sessions DROP COLUMN cost`,
	},
	{
		version: 25,
		up:      `ALTER TABLE sessions ADD COLUMN pid INTEGER NOT NULL DEFAULT 0`,
		down:    `ALTER TABLE sessions DROP COLUMN pid`,
	},
	{
		version: 26,
		up:      `ALTER TABLE sessions ADD COLUMN title TEXT DEFAULT NULL`,
		down:    `ALTER TABLE sessions DROP COLUMN title`,
	},
	{
		version: 27,
		up: `ALTER TABLE sessions ADD COLUMN type TEXT NOT NULL DEFAULT 'interactive';
ALTER TABLE sessions ADD COLUMN label TEXT DEFAULT NULL;
ALTER TABLE sessions ADD COLUMN correlation_id TEXT DEFAULT NULL`,
		down: `ALTER TABLE sessions DROP COLUMN correlation_id;
ALTER TABLE sessions DROP COLUMN label;
ALTER TABLE sessions DROP COLUMN type`,
	},
	{
		version: 28,
		up: `CREATE TABLE IF NOT EXISTS servers (
			group_key        TEXT PRIMARY KEY,
			adapter          TEXT NOT NULL,
			adapter_version  TEXT NOT NULL DEFAULT '',
			runtime          TEXT NOT NULL DEFAULT 'local',
			project_id       TEXT NOT NULL DEFAULT '',
			bundle_hash      TEXT NOT NULL DEFAULT '',
			pid              INTEGER NOT NULL DEFAULT 0,
			url              TEXT NOT NULL DEFAULT '',
			port             INTEGER NOT NULL DEFAULT 0,
			password         TEXT NOT NULL DEFAULT '',
			data_dir         TEXT NOT NULL DEFAULT '',
			work_dir         TEXT NOT NULL DEFAULT '',
			proxy_token      TEXT NOT NULL DEFAULT '',
			status           TEXT NOT NULL DEFAULT 'starting',
			created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_activity_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		down: `DROP TABLE IF EXISTS servers`,
	},
	{
		version: 29,
		up: `ALTER TABLE sessions ADD COLUMN workflow_id TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN entry_agent TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN bundle_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN group_key TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN runtime TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN mode TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN state TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_sessions_group ON sessions(group_key)`,
		down: `DROP INDEX IF EXISTS idx_sessions_group;
ALTER TABLE sessions DROP COLUMN state;
ALTER TABLE sessions DROP COLUMN mode;
ALTER TABLE sessions DROP COLUMN runtime;
ALTER TABLE sessions DROP COLUMN group_key;
ALTER TABLE sessions DROP COLUMN bundle_hash;
ALTER TABLE sessions DROP COLUMN entry_agent;
ALTER TABLE sessions DROP COLUMN workflow_id`,
	},
	{
		version: 30,
		up: `CREATE TABLE IF NOT EXISTS proxy_grants (
			token          TEXT PRIMARY KEY,
			owner          TEXT NOT NULL,
			provider       TEXT NOT NULL,
			region         TEXT NOT NULL DEFAULT '',
			source         TEXT NOT NULL DEFAULT '{}',
			allowed_models TEXT NOT NULL DEFAULT '[]',
			max_tokens     INTEGER NOT NULL DEFAULT 0,
			created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			revoked_at     DATETIME
		);
CREATE INDEX IF NOT EXISTS idx_proxy_grants_owner ON proxy_grants(owner)`,
		down: `DROP TABLE IF EXISTS proxy_grants`,
	},
	{
		version: 31,
		up:      `ALTER TABLE sessions ADD COLUMN state_changed_at DATETIME DEFAULT NULL`,
		down:    `ALTER TABLE sessions DROP COLUMN state_changed_at`,
	},
	{
		// v5 phase 1 (P1-T21): pinned workflows and UI settings. scope =
		// "global" | "project:<id>" | "team:<id>"; value = JSON.
		version: 32,
		up: `CREATE TABLE IF NOT EXISTS preferences (
			scope      TEXT NOT NULL,
			key        TEXT NOT NULL,
			value      TEXT NOT NULL DEFAULT 'null',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (scope, key)
		)`,
		down: `DROP TABLE IF EXISTS preferences`,
	},
	{
		version: 33,
		up: `CREATE TABLE IF NOT EXISTS pending_decisions (
			id          TEXT PRIMARY KEY,
			session_id  TEXT NOT NULL,
			group_key   TEXT NOT NULL DEFAULT '',
			kind        TEXT NOT NULL,
			tool_ref    TEXT NOT NULL DEFAULT '',
			payload     TEXT NOT NULL DEFAULT '{}',
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			resolved_at DATETIME,
			resolved_by TEXT NOT NULL DEFAULT '',
			resolution  TEXT NOT NULL DEFAULT ''
		);
CREATE INDEX IF NOT EXISTS idx_pending_decisions_open ON pending_decisions(resolved_at, session_id);
CREATE INDEX IF NOT EXISTS idx_pending_decisions_session ON pending_decisions(session_id)`,
		down: `DROP TABLE IF EXISTS pending_decisions`,
	},
	{
		// v5 phase 1 (P1-T16/T27): workflow launches. workflow_risk decides
		// automatic worktrees (O10); location = base | worktree; outputs =
		// typed session outputs (JSON object); parent_session_id = "chain
		// with…" (O7).
		version: 34,
		up: `ALTER TABLE sessions ADD COLUMN workflow_layer TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN workflow_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN workflow_risk TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN location TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN outputs TEXT NOT NULL DEFAULT '{}';
ALTER TABLE sessions ADD COLUMN parent_session_id TEXT NOT NULL DEFAULT ''`,
		down: `ALTER TABLE sessions DROP COLUMN parent_session_id;
ALTER TABLE sessions DROP COLUMN outputs;
ALTER TABLE sessions DROP COLUMN location;
ALTER TABLE sessions DROP COLUMN workflow_risk;
ALTER TABLE sessions DROP COLUMN workflow_version;
ALTER TABLE sessions DROP COLUMN workflow_layer`,
	},
	{
		// v5 phase 3 (P3-T03): workflow state of a session (checkpoints
		// passed, agents run, circuit breaker), JSON kept by the
		// CheckpointService.
		version: 35,
		up:      `ALTER TABLE sessions ADD COLUMN checkpoint_state TEXT NOT NULL DEFAULT ''`,
		down:    `ALTER TABLE sessions DROP COLUMN checkpoint_state`,
	},
	{
		// v5 phase 3 (P3-T25): projects.agents (agent selection of the
		// former per-project deployment) is neutralized: sessions ship the
		// agents of their workflow bundle. The selection is not kept.
		version: 36,
		up:      `UPDATE projects SET agents = ''`,
		down:    `SELECT 1`,
	},
	{
		// v5 phase 3 (P3-T25): projects.agents is dropped.
		version: 37,
		up:      `ALTER TABLE projects DROP COLUMN agents`,
		down:    `ALTER TABLE projects ADD COLUMN agents TEXT NOT NULL DEFAULT ''`,
	},
	{
		// v5 phase 2 (P2-T11): projects.workflow_config is neutralized. Its
		// content moves to workflow_config_legacy, read only by the migration
		// to the team-state (cmd/workflow_migrate.go), then cleared there.
		version: 38,
		up: `ALTER TABLE projects ADD COLUMN workflow_config_legacy TEXT NOT NULL DEFAULT '';
UPDATE projects SET workflow_config_legacy = workflow_config WHERE workflow_config != '';
UPDATE projects SET workflow_config = ''`,
		down: `UPDATE projects SET workflow_config = workflow_config_legacy WHERE workflow_config_legacy != '';
ALTER TABLE projects DROP COLUMN workflow_config_legacy`,
	},
	{
		// v5 phase 5 (P5-T08): reference of a remote session (GitLab target,
		// pipeline, packages, status), JSON kept by the remote service.
		version: 39,
		up:      `ALTER TABLE sessions ADD COLUMN remote_ref TEXT NOT NULL DEFAULT ''`,
		down:    `ALTER TABLE sessions DROP COLUMN remote_ref`,
	},
	{
		// v5 hardening (P3-T14, I6): persistent usage ledger — cost and
		// tokens per tool session and day (reported by the tool; root_id is
		// the oh session a subagent session works for), proxy traffic per
		// server group and day, budget raises.
		version: 40,
		up: `CREATE TABLE IF NOT EXISTS usage_sessions (
			day        TEXT NOT NULL,
			session_id TEXT NOT NULL,
			root_id    TEXT NOT NULL DEFAULT '',
			project_id TEXT NOT NULL DEFAULT '',
			group_key  TEXT NOT NULL DEFAULT '',
			cost_usd   REAL NOT NULL DEFAULT 0,
			tokens_in  INTEGER NOT NULL DEFAULT 0,
			tokens_out INTEGER NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (day, session_id)
		);
CREATE INDEX IF NOT EXISTS idx_usage_sessions_session ON usage_sessions(session_id);
CREATE INDEX IF NOT EXISTS idx_usage_sessions_root ON usage_sessions(root_id);
CREATE TABLE IF NOT EXISTS usage_proxy (
			day        TEXT NOT NULL,
			group_key  TEXT NOT NULL,
			requests   INTEGER NOT NULL DEFAULT 0,
			tokens_in  INTEGER NOT NULL DEFAULT 0,
			tokens_out INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (day, group_key)
		);
CREATE TABLE IF NOT EXISTS budget_extra (
			scope     TEXT NOT NULL,
			day       TEXT NOT NULL DEFAULT '',
			extra_usd REAL NOT NULL DEFAULT 0,
			PRIMARY KEY (scope, day)
		)`,
		down: `DROP TABLE IF EXISTS budget_extra;
DROP TABLE IF EXISTS usage_proxy;
DROP INDEX IF EXISTS idx_usage_sessions_root;
DROP INDEX IF EXISTS idx_usage_sessions_session;
DROP TABLE IF EXISTS usage_sessions`,
	},
	{
		// v5 phase 4 (P4-T09): execution settings of a project (dev
		// Dockerfile, build args, cache volumes, default workflow and
		// runtime), JSON.
		version: 41,
		up:      `ALTER TABLE projects ADD COLUMN exec_config TEXT NOT NULL DEFAULT ''`,
		down:    `ALTER TABLE projects DROP COLUMN exec_config`,
	},
	{
		// v5 corrections (A23, A31): git starting point of a session (its
		// results cover the whole session) and the tickets it works on
		// (team claims released when it ends), JSON. A21: the cost of a
		// session is its total with its subagents (usage ledger), also for
		// the sessions recorded before.
		version: 42,
		up: `ALTER TABLE sessions ADD COLUMN start_ref TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN tickets TEXT NOT NULL DEFAULT '';
UPDATE sessions SET
	cost = (SELECT SUM(u.cost_usd) FROM usage_sessions u WHERE u.root_id = sessions.id),
	tokens_in = MAX(tokens_in, (SELECT SUM(u.tokens_in) FROM usage_sessions u WHERE u.root_id = sessions.id)),
	tokens_out = MAX(tokens_out, (SELECT SUM(u.tokens_out) FROM usage_sessions u WHERE u.root_id = sessions.id))
WHERE (SELECT SUM(u.cost_usd) FROM usage_sessions u WHERE u.root_id = sessions.id) > cost`,
		down: `ALTER TABLE sessions DROP COLUMN tickets;
ALTER TABLE sessions DROP COLUMN start_ref`,
	},
	{
		// v5 critical (N16, oh gc): the worktree a session got from oh at
		// launch (automatic: removed by oh gc once clean, pushed and unused;
		// the sessions recorded before stay "unknown" and are never cleaned)
		// and the indexes of the clean-up queries.
		version: 43,
		up: `ALTER TABLE sessions ADD COLUMN worktree_auto INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN worktree_branch TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_sessions_state_ended ON sessions(state, ended_at);
CREATE INDEX IF NOT EXISTS idx_agent_events_started ON agent_events(started_at);
CREATE INDEX IF NOT EXISTS idx_pending_decisions_resolved ON pending_decisions(resolved_at);
CREATE INDEX IF NOT EXISTS idx_usage_sessions_day ON usage_sessions(day)`,
		down: `DROP INDEX IF EXISTS idx_usage_sessions_day;
DROP INDEX IF EXISTS idx_pending_decisions_resolved;
DROP INDEX IF EXISTS idx_agent_events_started;
DROP INDEX IF EXISTS idx_sessions_state_ended;
ALTER TABLE sessions DROP COLUMN worktree_branch;
ALTER TABLE sessions DROP COLUMN worktree_auto`,
	},
	{
		// Modernisation L4 (workflow_report): the reports filed by the agents
		// of a session (rp-<seq>); the bodies are files of the session folder.
		version: 44,
		up: `CREATE TABLE IF NOT EXISTS session_reports (
			session_id   TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			seq          INTEGER NOT NULL,
			agent        TEXT NOT NULL DEFAULT '',
			tool_session TEXT NOT NULL DEFAULT '',
			title        TEXT NOT NULL DEFAULT '',
			bytes        INTEGER NOT NULL DEFAULT 0,
			sha256       TEXT NOT NULL DEFAULT '',
			sections     TEXT NOT NULL DEFAULT '',
			created_at   DATETIME NOT NULL,
			PRIMARY KEY (session_id, seq)
		)`,
		down: `DROP TABLE IF EXISTS session_reports`,
	},
	{
		// Modernisation LD-J (wiki MCP of the project): the proposals for
		// docs/wiki/ (wp-<seq> per project), kept after their session (the
		// queue of the workflows without a commit).
		version: 45,
		up: `CREATE TABLE IF NOT EXISTS wiki_proposals (
			project_id   TEXT NOT NULL,
			seq          INTEGER NOT NULL,
			session_id   TEXT NOT NULL DEFAULT '',
			tool_session TEXT NOT NULL DEFAULT '',
			agent        TEXT NOT NULL DEFAULT '',
			page         TEXT NOT NULL,
			section      TEXT NOT NULL DEFAULT '',
			content      TEXT NOT NULL,
			tag          TEXT NOT NULL,
			source       TEXT NOT NULL DEFAULT '',
			status       TEXT NOT NULL DEFAULT 'pending',
			original     TEXT NOT NULL DEFAULT '',
			decided_by   TEXT NOT NULL DEFAULT '',
			decided_at   DATETIME,
			applied_by   TEXT NOT NULL DEFAULT '',
			applied_at   DATETIME,
			error        TEXT NOT NULL DEFAULT '',
			created_at   DATETIME NOT NULL,
			PRIMARY KEY (project_id, seq)
		);
CREATE INDEX IF NOT EXISTS idx_wiki_proposals_session ON wiki_proposals(session_id);
CREATE INDEX IF NOT EXISTS idx_wiki_proposals_status ON wiki_proposals(project_id, status)`,
		down: `DROP INDEX IF EXISTS idx_wiki_proposals_status;
DROP INDEX IF EXISTS idx_wiki_proposals_session;
DROP TABLE IF EXISTS wiki_proposals`,
	},
	{
		// Modernisation R6 (fan-out, experimental): the fan-out record of a
		// session (parent, child or merger; children, integration branch,
		// budget and events), JSON, written by oh only.
		version: 46,
		up:      `ALTER TABLE sessions ADD COLUMN fanout TEXT NOT NULL DEFAULT ''`,
		down:    `ALTER TABLE sessions DROP COLUMN fanout`,
	},
	{
		// Modernisation p5-fixes (wiki_propose replaces a line): the line of
		// the page a proposal replaces ('' = an addition). v47 is taken by
		// another lot.
		version: 48,
		up:      `ALTER TABLE wiki_proposals ADD COLUMN replaces TEXT NOT NULL DEFAULT ''`,
		down:    `ALTER TABLE wiki_proposals DROP COLUMN replaces`,
	},
	{
		// Modernisation p9-wiki-refusal: the reason the user gave when
		// refusing a wiki proposal ('' = none, or not refused). v50 is taken
		// by another lot.
		version: 49,
		up:      `ALTER TABLE wiki_proposals ADD COLUMN reason TEXT NOT NULL DEFAULT ''`,
		down:    `ALTER TABLE wiki_proposals DROP COLUMN reason`,
	},
}
