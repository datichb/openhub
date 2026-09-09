package deploy

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/workflow"
)

// WorkflowDeployResult holds the outputs of the workflow resolution phase
// that are consumed by subsequent deploy phases.
type WorkflowDeployResult struct {
	// Resolved is the fully resolved workflow definition.
	Resolved workflow.WorkflowDefinition

	// GeneratedSkills maps skill refs (e.g. "orchestrator/orchestrator-workflow-modes")
	// to their generated Markdown content.
	GeneratedSkills map[string]string

	// DisabledAgents lists agent IDs that should be excluded from deploy.
	DisabledAgents []string
}

// ResolveAndPrepareWorkflow performs the workflow resolution and skill generation.
// This is intended to be called early in the deploy pipeline (Phase 0) so that
// subsequent phases can use the resolved workflow.
//
// Parameters:
//   - base: the base workflow (typically workflow.BaseWorkflow())
//   - overrides: ordered list of overrides (hub, team, project)
//
// Returns a WorkflowDeployResult with generated skills and disabled agent list.
func ResolveAndPrepareWorkflow(base workflow.WorkflowDefinition, overrides ...workflow.WorkflowOverride) (*WorkflowDeployResult, error) {
	// 1. Resolve the workflow.
	resolved, err := workflow.Resolve(base, overrides...)
	if err != nil {
		return nil, fmt.Errorf("resolving workflow: %w", err)
	}

	// 2. Generate dynamic skills from the resolved workflow.
	skills, err := GenerateWorkflowSkills(&resolved)
	if err != nil {
		return nil, fmt.Errorf("generating workflow skills: %w", err)
	}

	// 3. Identify disabled agents.
	disabled := DisabledAgentIDs(&resolved)

	return &WorkflowDeployResult{
		Resolved:        resolved,
		GeneratedSkills: skills,
		DisabledAgents:  disabled,
	}, nil
}

// WriteGeneratedSkills writes the generated skill files to the deploy target
// directory.  Each generated skill replaces its static equivalent.
//
// Generated skills are written to <targetDir>/skills/generated/<name>/SKILL.md
// following the opencode native skill format.
func WriteGeneratedSkills(targetDir string, skills map[string]string) error {
	for ref, content := range skills {
		// Extract the skill name (last component of the ref).
		// e.g. "orchestrator/orchestrator-workflow-modes" → "orchestrator-workflow-modes"
		name := filepath.Base(ref)
		skillDir := filepath.Join(targetDir, "skills", name)

		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			return fmt.Errorf("creating skill dir %s: %w", skillDir, err)
		}

		skillPath := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(skillPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("writing skill %s: %w", skillPath, err)
		}
	}
	return nil
}

// IsSkillGenerated checks whether a skill ref has a generated replacement
// in the WorkflowDeployResult.
func (r *WorkflowDeployResult) IsSkillGenerated(skillRef string) bool {
	if r == nil {
		return false
	}
	_, ok := r.GeneratedSkills[skillRef]
	return ok
}

// IsAgentDisabled checks whether an agent ID is disabled by the workflow.
func (r *WorkflowDeployResult) IsAgentDisabled(agentID string) bool {
	if r == nil {
		return false
	}
	for _, id := range r.DisabledAgents {
		if id == agentID {
			return true
		}
	}
	return false
}
