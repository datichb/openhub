package sqlite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

// v38 moves projects.workflow_config to workflow_config_legacy (fixture: a
// database as left by v37 with a project override).
func TestMigrationV38NeutralizesWorkflowConfig(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ctx := context.Background()
	require.NoError(t, ps.Create(ctx, &domain.Project{ID: "web", Name: "web", Path: "/tmp/web", Status: domain.ProjectStatusActive}))
	require.NoError(t, ps.Create(ctx, &domain.Project{ID: "api", Name: "api", Path: "/tmp/api", Status: domain.ProjectStatusActive}))

	const legacy = `{"overrides":{"checkpoint_overrides":[{"id":"cp-1","action":"modify","label":"Go"}]}}`
	_, err := s.DB().Exec(`DELETE FROM schema_migrations WHERE version = 38;
		ALTER TABLE projects DROP COLUMN workflow_config_legacy;
		UPDATE projects SET workflow_config = ? WHERE id = 'web'`, legacy)
	require.NoError(t, err)

	require.NoError(t, s.migrate())
	var neutral string
	require.NoError(t, s.DB().QueryRow(`SELECT workflow_config FROM projects WHERE id = 'web'`).Scan(&neutral))
	assert.Empty(t, neutral, "the column is neutralized")
	got, err := ps.LegacyWorkflowConfigs(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"web": legacy}, got, "kept for the team-state migration")

	require.NoError(t, ps.ClearLegacyWorkflowConfig(ctx, "web"))
	got, err = ps.LegacyWorkflowConfigs(ctx)
	require.NoError(t, err)
	assert.Empty(t, got)
	require.NoError(t, s.migrate(), "idempotent")

	// Rollback restores the former column.
	_, err = s.DB().Exec(`UPDATE projects SET workflow_config_legacy = ? WHERE id = 'api'`, legacy)
	require.NoError(t, err)
	require.NoError(t, s.MigrateDown(37))
	var cfg string
	require.NoError(t, s.DB().QueryRow(`SELECT workflow_config FROM projects WHERE id = 'api'`).Scan(&cfg))
	assert.Equal(t, legacy, cfg)
}

// v36 neutralizes then v37 drops projects.agents (fixture: a database as
// left by v35 with an agent selection); the projects stay readable.
func TestMigrationV36V37DropProjectAgents(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ctx := context.Background()
	require.NoError(t, ps.Create(ctx, &domain.Project{ID: "web", Name: "web", Path: "/tmp/web", Status: domain.ProjectStatusActive, Provider: "bedrock"}))
	_, err := s.DB().Exec(`DELETE FROM schema_migrations WHERE version IN (36, 37);
		ALTER TABLE projects ADD COLUMN agents TEXT NOT NULL DEFAULT '';
		UPDATE projects SET agents = 'coder,reviewer' WHERE id = 'web'`)
	require.NoError(t, err)

	require.NoError(t, s.migrate())
	assert.False(t, hasColumn(t, s, "projects", "agents"), "v37 drops the column")
	p, err := ps.Get(ctx, "web")
	require.NoError(t, err)
	assert.Equal(t, "bedrock", p.Provider)
	require.NoError(t, s.migrate(), "idempotent")

	require.NoError(t, s.MigrateDown(35))
	assert.True(t, hasColumn(t, s, "projects", "agents"), "rollback re-adds an empty column")
	require.NoError(t, s.migrate())
	assert.False(t, hasColumn(t, s, "projects", "agents"))
}

func hasColumn(t *testing.T, s *Store, table, column string) bool {
	t.Helper()
	rows, err := s.DB().Query(`SELECT name FROM pragma_table_info(?)`, table)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		if name == column {
			return true
		}
	}
	return false
}
