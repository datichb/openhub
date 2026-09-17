package deploy

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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

// ValidateDeployedSkillRefs scans deployed skill .md files for references to
// agent names and checkpoint IDs, and returns warnings for any that reference
// agents disabled in the resolved workflow.
//
// This is a best-effort heuristic — it scans for known agent IDs as whole-word
// matches in the Markdown content. It does NOT block the deploy; warnings are
// informational to help the user understand potential inconsistencies.
func ValidateDeployedSkillRefs(skillsDir string, wf *workflow.WorkflowDefinition) []string {
	if wf == nil {
		return nil
	}

	// Build set of disabled agent IDs.
	disabledSet := make(map[string]bool)
	for _, a := range wf.Agents {
		if a.Role == workflow.RoleDisabled {
			disabledSet[a.AgentID] = true
		}
	}
	if len(disabledSet) == 0 {
		return nil // nothing to check
	}

	// Build regex matching any disabled agent ID as a whole word.
	var patterns []string
	for id := range disabledSet {
		patterns = append(patterns, regexp.QuoteMeta(id))
	}
	re := regexp.MustCompile(`\b(` + strings.Join(patterns, "|") + `)\b`)

	var warnings []string

	_ = filepath.WalkDir(skillsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil //nolint:nilerr // unreadable markdown files are silently skipped during workflow discovery
		}

		matches := re.FindAllString(string(data), -1)
		if len(matches) == 0 {
			return nil
		}

		// Deduplicate matches per file.
		seen := make(map[string]bool)
		for _, m := range matches {
			seen[m] = true
		}

		rel, _ := filepath.Rel(skillsDir, path)
		for agent := range seen {
			warnings = append(warnings, fmt.Sprintf(
				"skill %q references disabled agent %q", rel, agent,
			))
		}
		return nil
	})

	return warnings
}
