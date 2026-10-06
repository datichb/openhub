package sqlite

import (
	"context"
	"fmt"
)

// Legacy workflow configurations (v38): the former projects.workflow_config
// values, kept until they are migrated to the team-state.

// LegacyWorkflowConfigs returns the projects that still have a legacy
// workflow configuration (project id → JSON).
func (s *ProjectStore) LegacyWorkflowConfigs(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, workflow_config_legacy FROM projects WHERE workflow_config_legacy != ''`)
	if err != nil {
		return nil, fmt.Errorf("reading legacy workflow configs: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, cfg string
		if err := rows.Scan(&id, &cfg); err != nil {
			return nil, err
		}
		out[id] = cfg
	}
	return out, rows.Err()
}

// ClearLegacyWorkflowConfig forgets the legacy configuration of a project
// once it has been migrated.
func (s *ProjectStore) ClearLegacyWorkflowConfig(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE projects SET workflow_config_legacy = '' WHERE id = ?`, id)
	return err
}
