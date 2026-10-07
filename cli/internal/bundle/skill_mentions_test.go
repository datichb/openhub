package bundle

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/workflow"
)

var inlinedHeadingRe = regexp.MustCompile("(?m)^> Skill `([a-z0-9_-]+)` : incluse ci-dessous")

// Every skill named in an agent body or a prompt template of a shipped
// workflow is loadable (shipped on demand) or inlined in the agent under a
// heading that names it: otherwise the model calls the skill tool and gets
// « Unable to load skill » (v5 finalisation, Q3-1: cp-1 never declared in
// ticket).
func TestShippedWorkflowSkillMentionsResolve(t *testing.T) {
	hub, cat, env := shippedWorkflows(t)
	for _, ref := range cat.Refs() {
		t.Run(ref.ID, func(t *testing.T) {
			r, diags := workflow.Check(cat, ref, nil, env)
			require.False(t, diags.HasErrors(), "%v", diags)
			b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec})
			require.NoError(t, err)
			generated, err := specSkills(hub, r.Spec)
			require.NoError(t, err)
			index := hubSkillIndex(hub, generated)

			loadable := map[string]bool{}
			for _, sk := range b.Spec.Skills {
				loadable[sk.ID] = true
			}
			headed := map[string]map[string]bool{}
			for _, a := range b.Spec.Agents {
				headed[a.ID] = map[string]bool{}
				for _, m := range inlinedHeadingRe.FindAllStringSubmatch(a.Body, -1) {
					headed[a.ID][m[1]] = true
				}
			}
			check := func(where, agent, text string) {
				for _, ref := range mentionedSkills(text, index) {
					id := skillID(ref)
					if !loadable[id] && !headed[agent][id] {
						t.Errorf("%s names skill %q, neither inlined in %s nor shipped on demand", where, id, agent)
					}
				}
			}
			for _, a := range b.Spec.Agents {
				check("agent "+a.ID, a.ID, a.Body)
			}
			if r.Spec.Prompt != nil && r.Spec.Prompt.Template != "" {
				src, err := os.ReadFile(filepath.Join(hub, "workflows", r.Spec.Prompt.Template))
				require.NoError(t, err)
				check("prompt "+r.Spec.Prompt.Template, b.Spec.EntryAgent, string(src))
			}

			// Each inlined skill carries its heading.
			files, err := bricks.FindAgentFiles(hub)
			require.NoError(t, err)
			loader := newSkillLoader(hub, generated)
			var deny []string
			if r.Spec.Skills != nil {
				deny = r.Spec.Skills.Deny
			}
			for _, a := range b.Spec.Agents {
				fm, err := bricks.ParseAgentFrontmatter(files[a.ID])
				require.NoError(t, err)
				docs, err := loader.closure(fm.Skills, denyList(deny))
				require.NoError(t, err)
				for _, d := range docs {
					assert.True(t, headed[a.ID][d.ID], "agent %s: inlined skill %s has no heading", a.ID, d.ID)
				}
			}
		})
	}
}

func TestMentionedSkills(t *testing.T) {
	index := map[string]string{
		"shared/hub-workflow-reference": "shared/hub-workflow-reference",
		"hub-workflow-reference":        "shared/hub-workflow-reference",
		"posture/expert-posture":        "posture/expert-posture",
		"expert-posture":                "posture/expert-posture",
	}
	got := mentionedSkills("voir skill `shared/hub-workflow-reference`, `expert-posture`, `bd show`, `hub-workflow-reference` encore", index)
	assert.Equal(t, []string{"shared/hub-workflow-reference", "posture/expert-posture"}, got)
	assert.True(t, strings.HasPrefix(inlinedSkillHeading("x"), "> Skill `x` : incluse ci-dessous"))
}

func TestHubSkillIndexAmbiguousName(t *testing.T) {
	hub := t.TempDir()
	for _, p := range []string{"skills/a/dup.md", "skills/b/dup.md", "skills/a/one.md", "skills/templates/annex.md"} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(hub, p)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(hub, p), []byte("x"), 0o644))
	}
	ix := hubSkillIndex(hub, map[string]string{"workflow/workflow-map": "x"})
	assert.Equal(t, "a/one", ix["one"])
	assert.Equal(t, "a/dup", ix["a/dup"])
	assert.NotContains(t, ix, "dup")
	assert.NotContains(t, ix, "annex")
	assert.Equal(t, "workflow/workflow-map", ix["workflow-map"])
}
