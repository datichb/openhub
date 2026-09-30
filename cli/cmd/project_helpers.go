package cmd

import (
	"context"
	"fmt"

	"github.com/datichb/openhub/cli/internal/domain"
)

// upsertProject creates a new project or updates an existing one matched by
// path. The match is by filesystem path (the natural business key for a
// project).
//
// Behavior:
//   - If an active project already exists at p.Path, its mutable fields are
//     merged from p (non-zero values only) and the project is updated.
//   - If an archived project exists at p.Path, ErrAlreadyExists is returned
//     — we do not silently resurrect archived projects.
//   - Otherwise, a new project is created.
//
// Returns the final project (created or updated) and whether it was an update.
func upsertProject(ctx context.Context, store domain.ProjectStore, p *domain.Project) (*domain.Project, bool, error) {
	existing, err := store.GetByPath(ctx, p.Path)
	if err == nil && existing != nil {
		// Archived project: refuse to resurrect.
		if existing.Status == domain.ProjectStatusArchived {
			return nil, false, fmt.Errorf("project at path %q is archived: %w", p.Path, domain.ErrAlreadyExists)
		}

		// Active project at same path: merge non-zero fields and update.
		mergeProjectFields(existing, p)
		if err := store.Update(ctx, existing); err != nil {
			return nil, false, fmt.Errorf("update project: %w", err)
		}
		return existing, true, nil
	}

	// No existing project — create.
	if err := store.Create(ctx, p); err != nil {
		return nil, false, fmt.Errorf("create project: %w", err)
	}
	return p, false, nil
}

// mergeProjectFields copies non-zero fields from src into dst.
// Fields that are zero-valued in src are left untouched in dst.
func mergeProjectFields(dst, src *domain.Project) {
	if src.Name != "" {
		dst.Name = src.Name
	}
	if src.Language != "" {
		dst.Language = src.Language
	}
	if src.Provider != "" {
		dst.Provider = src.Provider
	}
	if src.Model != "" {
		dst.Model = src.Model
	}
	if len(src.Labels) > 0 {
		dst.Labels = src.Labels
	}
	if len(src.Agents) > 0 {
		dst.Agents = src.Agents
	}
	if len(src.MCP) > 0 {
		dst.MCP = src.MCP
	}
	if src.MCPConfig != nil {
		dst.MCPConfig = src.MCPConfig
	}
	if src.ProviderConfig != nil {
		dst.ProviderConfig = src.ProviderConfig
	}
	if src.ModelOverrides != nil {
		dst.ModelOverrides = src.ModelOverrides
	}
	if src.TrackerConfig != nil {
		dst.TrackerConfig = src.TrackerConfig
	}
	if src.WorkflowConfig != nil {
		dst.WorkflowConfig = src.WorkflowConfig
	}
	if src.TeamID != nil {
		dst.TeamID = src.TeamID
	}
	if src.Status != "" {
		dst.Status = src.Status
	}
}
