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
	p, err := ps.Get(ctx, "web")
	require.NoError(t, err)
	assert.Nil(t, p.WorkflowConfig, "the column is neutralized")
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
