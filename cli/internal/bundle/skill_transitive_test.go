package bundle

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// crossReferences are skills named by another skill for context, not loaded
// by the agents of the bundle (C0.3 review): the reason is kept with each.
var crossReferences = map[string]string{
	"living-docs-enrichment → onboarder-workflow":     "where the onboarder writes the wiki, for the reader",
	"onboarder-phase-3-4 → dev-standards-testing":     "agent recommendation written in the report",
	"dev-standards-backend → dev-standards-migration": "destructive migration protocol, the developer's: the reviewer only reads the backend standards",
	"planner-design-templates → prototype-protocol":   "the designer's protocol, named for a design delegation: triage has no designer (r9b, its planner launches no agent)",
}

// C0.3: a skill named inside a shipped skill is loadable in that bundle
// (shipped on demand, or inlined in an agent). The mention scan of the build
// only reads agent bodies: a skill named only by an on-demand skill must be
// pulled in by `requires:`, otherwise the agent following that skill gets
// « Unable to load skill ». Skills gated by a plugin the workflow does not
// load, by an MCP server the session does not have or by a when: condition
// (L3), and stack skills
// (shipped when the project uses the stack), are absent on purpose.
func TestShippedSkillMentionsResolveTransitively(t *testing.T) {
	t.Parallel()
	hub, cat, _ := shippedWorkflows(t)
	index := hubSkillIndex(hub, nil)
	loader := newSkillLoader(hub, nil)
	used := map[string]bool{}
	for _, ref := range cat.Refs() {
		t.Run(ref.ID, func(t *testing.T) {
			b := shippedBundle(t, ref.ID)
			loadable := map[string]bool{}
			for _, s := range b.Spec.Skills {
				loadable[s.ID] = true
			}
			for _, a := range b.Spec.Agents {
				for _, line := range strings.Split(a.Body, "\n") {
					if rest, ok := strings.CutPrefix(line, "> Skill `"); ok {
						if id, _, ok := strings.Cut(rest, "`"); ok {
							loadable[id] = true
						}
					}
				}
			}
			var missing []string
			for _, s := range b.Spec.Skills {
				body, err := os.ReadFile(filepath.Join(b.Dir, skillsDir, s.ID, "SKILL.md"))
				require.NoError(t, err)
				for _, m := range mentionedSkills(string(body), index) {
					d, err := loader.load(m)
					if err != nil || d.Front.Plugin != "" || d.Front.MCP != "" || d.Front.When != "" || d.ID == s.ID || strings.HasPrefix(d.Ref, "developer/stacks/") {
						continue
					}
					pair := s.ID + " → " + d.ID
					if _, ok := crossReferences[pair]; ok {
						used[pair] = true
						continue
					}
					if !loadable[d.ID] {
						missing = append(missing, pair)
					}
				}
			}
			sort.Strings(missing)
			assert.Empty(t, missing, "skills named by a shipped skill but not loadable in the %s bundle", ref.ID)
		})
	}
	for pair := range crossReferences {
		assert.True(t, used[pair], "crossReferences: %s is no longer named by a shipped skill, remove it", pair)
	}
}
