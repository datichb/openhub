package bundle

import (
	"errors"

	"github.com/datichb/openhub/cli/internal/workflow"
)

// SkillCatalog checks the skills of a workflow with the rules applied when
// the bundle is built: `requires:` closure and one skill per identifier. It
// implements workflow.SkillCatalog (workflow validation).
type SkillCatalog struct {
	loader *skillLoader
}

// NewSkillCatalog reads the skills of hubDir (then the community skills).
func NewSkillCatalog(hubDir string) *SkillCatalog {
	return &SkillCatalog{loader: newSkillLoader(hubDir, nil)}
}

// HasSkill implements workflow.SkillCatalog.
func (c *SkillCatalog) HasSkill(ref string) bool {
	_, err := c.loader.load(ref)
	return err == nil
}

// Closure implements workflow.SkillCatalog. Each root is resolved on its own
// so that a problem names the skill it comes from.
func (c *SkillCatalog) Closure(roots []string) ([]string, []workflow.SkillIssue) {
	var out []string
	var issues []workflow.SkillIssue
	seen := map[string]bool{}
	ids := skillIndex{}
	for _, root := range roots {
		if !c.HasSkill(root) {
			issues = append(issues, workflow.SkillIssue{Skill: root, Kind: "missing", Detail: root})
			continue
		}
		docs, err := c.loader.closure([]string{root}, denyList(nil))
		if err != nil {
			kind := "missing"
			var ce *cycleError
			if errors.As(err, &ce) {
				kind = "cycle"
			}
			issues = append(issues, workflow.SkillIssue{Skill: root, Kind: kind, Detail: err.Error()})
			continue
		}
		for _, d := range docs {
			if seen[d.Ref] {
				continue
			}
			seen[d.Ref] = true
			if err := ids.add(d); err != nil {
				issues = append(issues, workflow.SkillIssue{Skill: d.ID, Kind: "duplicate", Detail: err.Error()})
				continue
			}
			out = append(out, d.Ref)
		}
	}
	return out, issues
}
