package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/datichb/openhub/cli/internal/domain"
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

// MigrateTeamConfigToTeamID migrates projects from the legacy ProjectTeamConfig
// (team_config JSON column) to the new TeamID column. For each project that has
// a team_config but no team_id, it parses the JSON and sets team_id based on the mode:
//   - "" or "inherit" → team_id = activeTeamID
//   - "disabled"      → team_id stays NULL
//   - "custom"        → team_id = activeTeamID (simplified — single-team setup)
//
// Returns the number of projects migrated.
func (s *ProjectStore) MigrateTeamConfigToTeamID(ctx context.Context, activeTeamID string) (int, error) {
	if activeTeamID == "" {
		return 0, nil // no team configured at hub level — nothing to migrate
	}

	rows, err := s.db.QueryContext(ctx, `SELECT id, team_config FROM projects WHERE team_id IS NULL AND team_config != ''`)
	if err != nil {
		return 0, fmt.Errorf("querying projects for team migration: %w", err)
	}
	defer rows.Close()

	type candidate struct {
		id         string
		teamConfig string
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.teamConfig); err != nil {
			return 0, fmt.Errorf("scanning project for team migration: %w", err)
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	count := 0
	for _, c := range candidates {
		var tc domain.ProjectTeamConfig
		if err := json.Unmarshal([]byte(c.teamConfig), &tc); err != nil {
			slog.Warn("team migration: invalid team_config JSON, skipping", "project", c.id, "error", err)
			continue
		}

		switch tc.Mode {
		case domain.ProjectTeamModeDisabled:
			// disabled → team_id stays NULL, nothing to do
			continue
		case "", domain.ProjectTeamModeInherit, domain.ProjectTeamModeCustom:
			// inherit or custom → attach to the active team
			_, err := s.db.ExecContext(ctx, `UPDATE projects SET team_id = ? WHERE id = ?`, activeTeamID, c.id)
			if err != nil {
				return count, fmt.Errorf("migrating project %s team_id: %w", c.id, err)
			}
			count++
			slog.Info("team migration: set team_id", "project", c.id, "team_id", activeTeamID, "old_mode", tc.Mode)
		default:
			slog.Warn("team migration: unknown mode, skipping", "project", c.id, "mode", tc.Mode)
		}
	}

	return count, nil
}
