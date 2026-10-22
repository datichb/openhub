package sqlite

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

// A migration either undoes itself or says it cannot: an empty down without
// irreversible would fail at the rollback, in the middle of a downgrade.
func TestEveryMigrationIsReversibleOrMarked(t *testing.T) {
	last := 0
	for _, m := range schemaMigrations {
		assert.Greater(t, m.version, last, "versions are strictly increasing")
		last = m.version
		assert.NotEmpty(t, strings.TrimSpace(m.up), "v%d has no up", m.version)
		switch {
		case m.irreversible:
			assert.Empty(t, m.down, "v%d is irreversible: it must not carry a down", m.version)
		default:
			assert.NotEmpty(t, strings.TrimSpace(m.down),
				"v%d has no down and is not marked irreversible: write its inverse (DROP COLUMN, DROP INDEX before the column it covers) or set irreversible", m.version)
		}
	}
}

// schemaOf describes the schema of the store: columns of every table (in
// order, with type, nullability, default, key) and the indexes.
func schemaOf(t *testing.T, s *Store) []string {
	t.Helper()
	ctx := context.Background()
	rows, err := s.DB().QueryContext(ctx, `SELECT type, name, tbl_name FROM sqlite_master
		WHERE name NOT LIKE 'sqlite_%' AND name != 'schema_migrations' AND sql IS NOT NULL`)
	require.NoError(t, err)
	type obj struct{ kind, name, table string }
	var objs []obj
	for rows.Next() {
		var o obj
		require.NoError(t, rows.Scan(&o.kind, &o.name, &o.table))
		objs = append(objs, o)
	}
	require.NoError(t, rows.Err())
	rows.Close()

	var out []string
	for _, o := range objs {
		switch o.kind {
		case "table":
			cols, err := s.DB().QueryContext(ctx, `SELECT cid, name, type, "notnull", COALESCE(dflt_value, 'NULL'), pk FROM pragma_table_info(?)`, o.name)
			require.NoError(t, err)
			for cols.Next() {
				var cid, notnull, pk int
				var name, typ, dflt string
				require.NoError(t, cols.Scan(&cid, &name, &typ, &notnull, &dflt, &pk))
				out = append(out, fmt.Sprintf("table %s col %d %s %s notnull=%d default=%s pk=%d", o.name, cid, name, typ, notnull, dflt, pk))
			}
			require.NoError(t, cols.Err())
			cols.Close()
		case "index":
			out = append(out, fmt.Sprintf("index %s on %s", o.name, o.table))
		}
	}
	sort.Strings(out)
	return out
}

// Rolling back to just before a migration and applying it again gives the
// schema it had: for every reversible migration, so a down that does not
// match its up (wrong column, an index still covering a dropped column) is
// caught here and not in a downgrade.
func TestMigrationsRoundTrip(t *testing.T) {
	t.Setenv("OH_SKIP_DB_BACKUP", "1")
	s := openTestStore(t)
	want := schemaOf(t, s)
	top, err := s.SchemaVersion()
	require.NoError(t, err)

	for i := len(schemaMigrations) - 1; i >= 0; i-- {
		m := schemaMigrations[i]
		if m.irreversible {
			break // nothing above the first irreversible migration can be rolled back past it
		}
		t.Run(fmt.Sprintf("v%d", m.version), func(t *testing.T) {
			require.NoError(t, s.MigrateDown(m.version-1), "rolling back v%d", m.version)
			v, err := s.SchemaVersion()
			require.NoError(t, err)
			assert.Less(t, v, m.version)
			assert.NotEqual(t, want, schemaOf(t, s), "the rollback changed the schema")
			require.NoError(t, s.Migrate())
			v, err = s.SchemaVersion()
			require.NoError(t, err)
			assert.Equal(t, top, v)
			assert.Equal(t, want, schemaOf(t, s), "rollback then migrate does not give back the schema")
		})
	}
}

// Rolling back all the way to an empty database works and applying everything
// again gives the same schema (the downs are in the right order).
func TestMigrationsRollBackToEmpty(t *testing.T) {
	t.Setenv("OH_SKIP_DB_BACKUP", "1")
	for _, m := range schemaMigrations {
		if m.irreversible {
			t.Skipf("v%d is irreversible", m.version)
		}
	}
	s := openTestStore(t)
	want := schemaOf(t, s)
	require.NoError(t, s.MigrateDown(0))
	assert.Empty(t, schemaOf(t, s), "no table or index is left")
	require.NoError(t, s.Migrate())
	assert.Equal(t, want, schemaOf(t, s))
}

// An irreversible migration stops the rollback with its version, before
// anything above it is undone half way (checked on a copy of the list).
func TestMigrateDownStopsAtAnIrreversibleMigration(t *testing.T) {
	t.Setenv("OH_SKIP_DB_BACKUP", "1")
	s := openTestStore(t)
	saved := schemaMigrations
	t.Cleanup(func() { schemaMigrations = saved })
	patched := append([]migration{}, saved...)
	for i := range patched {
		if patched[i].version == 40 {
			patched[i].down, patched[i].irreversible = "", true
		}
	}
	schemaMigrations = patched

	err := s.MigrateDown(30)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "v40 is irreversible")
	v, verr := s.SchemaVersion()
	require.NoError(t, verr)
	assert.Equal(t, 40, v, "the migrations above v40 were rolled back, v40 stays")
}

// The session and project rows survive a rollback to v23 and the migrations
// that follow; the columns added since come back with their defaults.
func TestMigrationsRoundTripKeepsTheRows(t *testing.T) {
	t.Setenv("OH_SKIP_DB_BACKUP", "1")
	s := openTestStore(t)
	ctx := context.Background()
	seedProject(t, s, "web")
	require.NoError(t, NewSessionStore(s).Create(ctx, &domain.Session{
		ID: "ses_a", ProjectID: "web", Status: domain.SessionStatusRunning, Provider: "bedrock", Model: "m", TokensIn: 12, TokensOut: 34,
		WorkflowID: "feature", Mode: "manuel", GroupKey: "g1",
	}))

	require.NoError(t, s.MigrateDown(23))
	assert.False(t, hasColumn(t, s, "sessions", "workflow_id"))
	assert.False(t, hasColumn(t, s, "sessions", "cost"))
	var provider string
	var in, out int64
	require.NoError(t, s.DB().QueryRow(`SELECT provider, tokens_in, tokens_out FROM sessions WHERE id = 'ses_a'`).Scan(&provider, &in, &out))
	assert.Equal(t, "bedrock", provider)
	assert.EqualValues(t, 12, in)
	assert.EqualValues(t, 34, out)

	require.NoError(t, s.Migrate())
	got, err := NewSessionStore(s).Get(ctx, "ses_a")
	require.NoError(t, err)
	assert.Equal(t, "bedrock", got.Provider)
	assert.EqualValues(t, 12, got.TokensIn)
	assert.Empty(t, got.WorkflowID, "the column dropped by the rollback comes back at its default")
	p, err := NewProjectStore(s).Get(ctx, "web")
	require.NoError(t, err)
	assert.Equal(t, "web", p.Name)
}
