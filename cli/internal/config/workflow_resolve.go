package config

import (
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// WorkflowSource describes which configuration level defined a workflow override.
type WorkflowSource string

const (
	WorkflowSourceBase    WorkflowSource = "base"
	WorkflowSourceHub     WorkflowSource = "hub"
	WorkflowSourceTeam    WorkflowSource = "team"
	WorkflowSourceProject WorkflowSource = "project"
)

// ResolvedWorkflow is the fully resolved workflow definition along with
// metadata about which levels contributed overrides and whether the result
// is locked (enforced by a higher level).
type ResolvedWorkflow struct {
	// Definition is the fully resolved workflow.
	Definition workflow.WorkflowDefinition

	// Sources lists which levels contributed overrides (in application order).
	Sources []WorkflowSource

	// Locked is true when a higher level (team) has enforced the workflow,
	// meaning the project level cannot modify it.
	Locked bool

	// LockedBy indicates which level enforced the lock.
	LockedBy WorkflowSource
}

// ResolveWorkflow applies the 3-level override cascade (hub → team → project)
// on top of the base workflow and returns the fully resolved result.
//
// The resolution order is:
//  1. Start with the base workflow (hardcoded)
//  2. Apply hub overrides from hub.toml [workflow.overrides]
//  3. Apply team overrides from team-state config.toml [workflow.overrides]
//     — if the team override is enforced, project overrides are skipped
//  4. Apply project overrides from SQLite workflow_config
//
// If teamCfg is nil, the team level is skipped.
// If project is nil or has no WorkflowConfig, the project level is skipped.
func ResolveWorkflow(
	hubCfg *Config,
	teamCfg *teamstate.TeamConfig,
	project *domain.Project,
) (*ResolvedWorkflow, error) {
	result := &ResolvedWorkflow{
		Sources: []WorkflowSource{WorkflowSourceBase},
	}

	// Collect overrides in order.
	var overrides []workflow.WorkflowOverride

	// 1. Hub overrides
	if hubCfg != nil && hubCfg.Workflow != nil && hubCfg.Workflow.Overrides != nil {
		overrides = append(overrides, *hubCfg.Workflow.Overrides)
		result.Sources = append(result.Sources, WorkflowSourceHub)
	}

	// 2. Team overrides
	teamEnforced := false
	if teamCfg != nil && teamCfg.Workflow != nil && teamCfg.Workflow.Overrides != nil {
		ov := *teamCfg.Workflow.Overrides
		if teamCfg.Workflow.IsEnforced() {
			ov.Enforced = true
			teamEnforced = true
		}
		overrides = append(overrides, ov)
		result.Sources = append(result.Sources, WorkflowSourceTeam)
	}

	// 3. Project overrides (skipped if team-enforced)
	if !teamEnforced && project != nil && project.WorkflowConfig != nil && project.WorkflowConfig.Overrides != nil {
		overrides = append(overrides, *project.WorkflowConfig.Overrides)
		result.Sources = append(result.Sources, WorkflowSourceProject)
	}

	if teamEnforced {
		result.Locked = true
		result.LockedBy = WorkflowSourceTeam
	}

	// Resolve.
	base := workflow.BaseWorkflow()
	resolved, err := workflow.Resolve(base, overrides...)
	if err != nil {
		return nil, err
	}

	result.Definition = resolved
	return result, nil
}
