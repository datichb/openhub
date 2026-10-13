package bundle

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

func TestInvitingText(t *testing.T) {
	agents := map[string]bool{"orchestrator": true, "developer": true}
	body := strings.Join([]string{
		"# Format", "voir `a`",
		"## Règles pour le consommateur (orchestrator)", "voir `b`",
		"### Détail", "voir `c`",
		"## Suite", "voir `d`",
		"## Relation avec `e`", "voir `f`",
		"```", "## Injection dans un bloc de code", "```", "voir `g`",
		"## Injection des skills", "voir `h`",
		inlinedSkillHeading("next"), "voir `i`",
		"## Règles pour le consommateur (agent appelant)", "voir `j`",
		"## Règles pour le producteur (orchestrator)", "voir `k`",
		"## Pour l'orchestrator", "voir `l`",
		"## Pour les developer", "voir `m`",
	}, "\n")
	got := invitingText(body, "developer", agents)
	for _, kept := range []string{"`a`", "`d`", "`i`", "`j`", "`m`"} {
		assert.Contains(t, got, kept)
	}
	for _, dropped := range []string{"`b`", "`c`", "`e`", "`f`", "`g`", "`h`", "`k`", "`l`"} {
		assert.NotContains(t, got, dropped)
	}
	// The consumer named reads its own rules.
	assert.Contains(t, invitingText(body, "orchestrator", agents), "`b`")
}

func delegatingAgentMD(id, mode, extra, native, callee, body string) string {
	perm := ""
	if callee != "" {
		perm = "permission:\n  task:\n    \"*\": deny\n    " + callee + ": allow\n"
	}
	return "---\nid: " + id + "\ndescription: test agent\nmode: " + mode + "\n" + perm + extra +
		"skills: []\nnative_skills: [" + native + "]\n---\n\n# " + id + "\n\n" + body + "\n"
}

// L2: each agent of a bundle sees only its own skills; the bundle ships
// their union.
func TestBuildAgentCatalogues(t *testing.T) {
	hub := fakeHub(t, map[string]string{
		"agents/test/solo.md": delegatingAgentMD("solo", "primary", "", "a/lead-native", "helper",
			"Délègue à `helper` : charge la skill `a/domain`, prompt [SKILL:a/tagged].\n\n## Règles pour le consommateur (helper)\n\nVoir `a/consumer`."),
		"agents/test/helper.md":                           delegatingAgentMD("helper", "subagent", "skills_from_callers: true\nstack_skills: true\n", "a/helper-native", "", "Aide."),
		"skills/a/lead-native.md":                         skillMD("lead-native", "requires: [a/dep]\n", "Voir aussi `a/further` et `dev-standards-golang`.\n\n## Relation avec `a/related`\n\nx"),
		"skills/a/further.md":                             skillMD("further", "", "Puis `a/furthest`."),
		"skills/a/furthest.md":                            skillMD("furthest", "", "x"),
		"skills/a/related.md":                             skillMD("related", "", "x"),
		"skills/a/tagged.md":                              skillMD("tagged", "", "x"),
		"skills/a/dep.md":                                 skillMD("dep", "", "x"),
		"skills/a/domain.md":                              skillMD("domain", "", "x"),
		"skills/a/consumer.md":                            skillMD("consumer", "", "x"),
		"skills/a/helper-native.md":                       skillMD("helper-native", "", "x"),
		"skills/a/extra.md":                               skillMD("extra", "", "x"),
		"skills/a/prompted.md":                            skillMD("prompted", "", "x"),
		"skills/developer/stacks/dev-standards-golang.md": skillMD("dev-standards-golang", "", "x"),
	})
	project := t.TempDir()
	writeProjectFile(t, project, "go.mod", "module example.com/x\n")
	b, err := buildSolo(t, hub, func(r *Request) {
		r.ExtraSkills = []string{"a/extra"}
		r.PromptTemplate = "Commence par `a/prompted`."
		r.ProjectPath = project
	})
	require.NoError(t, err)
	solo, helper := findAgent(b.Spec.Agents, "solo"), findAgent(b.Spec.Agents, "helper")
	require.NotNil(t, solo)
	require.NotNil(t, helper)
	assert.Equal(t, []string{"dep", "domain", "extra", "further", "furthest", "lead-native", "prompted"}, solo.Skills,
		"natives and their requires, mentions, extra and prompt skills, skills named by its on-demand skills (transitively); "+
			"consumer rules naming another agent, Relation sections, delegation tags and stack skills left out")
	assert.Equal(t, []string{"dev-standards-golang", "domain", "further", "furthest", "helper-native", "tagged"}, helper.Skills,
		"its natives, the stack skills and the skills its caller names in its prompt or its on-demand skills (not the caller's natives)")
	assert.Equal(t, []string{"dep", "dev-standards-golang", "domain", "extra", "further", "furthest", "helper-native", "lead-native", "prompted", "tagged"}, b.Spec.SkillIDs(),
		"the bundle ships the union of the catalogues")
	assert.Equal(t, []string{"dep", "domain", "extra", "further", "furthest", "lead-native", "prompted"}, idsOf(VisibleSkills(*solo, b.Spec.Skills)))
	assert.Equal(t, "", absentNote(solo.Body))

	// Without stack_skills, skills_from_callers nor PromptTemplate.
	hub2 := fakeHub(t, map[string]string{
		"agents/test/solo.md":                             delegatingAgentMD("solo", "primary", "", "", "helper", "Charge `a/domain`."),
		"agents/test/helper.md":                           delegatingAgentMD("helper", "subagent", "", "", "", "Aide."),
		"skills/a/domain.md":                              skillMD("domain", "", "x"),
		"skills/developer/stacks/dev-standards-golang.md": skillMD("dev-standards-golang", "", "x"),
	})
	b, err = buildSolo(t, hub2, func(r *Request) { r.ProjectPath = project })
	require.NoError(t, err)
	assert.Equal(t, []string{"domain"}, findAgent(b.Spec.Agents, "solo").Skills)
	helper = findAgent(b.Spec.Agents, "helper")
	assert.NotNil(t, helper.Skills, "an empty catalogue is not nil (nil = every skill)")
	assert.Empty(t, helper.Skills)
	assert.Equal(t, []string{"domain"}, b.Spec.SkillIDs(), "no agent takes the stack skills: not shipped")
}

func absentNote(body string) string {
	if i := strings.Index(body, "> Skills absentes"); i >= 0 {
		return body[i:]
	}
	return ""
}

func idsOf(skills []sessionspec.SkillDef) []string {
	out := []string{}
	for _, s := range skills {
		out = append(out, s.ID)
	}
	return out
}

// Compatibility: an agent without catalogue (bundle built before L2) sees
// every bundle skill.
func TestVisibleSkillsWithoutCatalogue(t *testing.T) {
	skills := []sessionspec.SkillDef{{ID: "a"}, {ID: "b"}}
	assert.Equal(t, []string{"a", "b"}, idsOf(VisibleSkills(sessionspec.AgentDef{ID: "x"}, skills)))
	assert.Equal(t, []string{"b"}, idsOf(VisibleSkills(sessionspec.AgentDef{ID: "x", Skills: []string{"b"}}, skills)))
	assert.Empty(t, idsOf(VisibleSkills(sessionspec.AgentDef{ID: "x", Skills: []string{}}, skills)))
}

type catalogueCase struct {
	project string
	ref     workflow.Ref
	spec    *workflow.Spec
	b       *Bundle
}

// shippedCatalogues builds every shipped workflow, without project and with
// the common-stack project.
func shippedCatalogues(t *testing.T) (string, []catalogueCase) {
	t.Helper()
	hub, cat, env := shippedWorkflows(t)
	var out []catalogueCase
	for _, project := range []string{"", capProject(t)} {
		for _, ref := range cat.Refs() {
			r, diags := workflow.Check(cat, ref, nil, env)
			require.False(t, diags.HasErrors(), "%v", diags)
			b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), ProjectPath: project, Spec: r.Spec})
			require.NoError(t, err)
			out = append(out, catalogueCase{project: project, ref: ref, spec: r.Spec, b: b})
		}
	}
	return hub, out
}

// skillTagLineRe finds the agent a line delegates to (task(subagent_type: "x" …).
var skillTagLineRe = regexp.MustCompile(`subagent_type:\s*"([a-z0-9-]+)"`)

// L2: every skill an agent is invited to load is in its catalogue (or
// inlined in its prompt): its native skills, the skills named in its
// prompt, the stack skills for the agents with stack_skills, the workflow
// extra and prompt skills for the entry agent, the [SKILL:…] tags its
// callers put in the delegation prompts written for it, and the skills
// named by the on-demand skills of its catalogue (transitive closure).
func TestAgentSkillReachability(t *testing.T) {
	hub, cases := shippedCatalogues(t)
	files, err := bricks.FindAgentFiles(hub)
	require.NoError(t, err)
	agentIDs := map[string]bool{}
	for id := range files {
		agentIDs[id] = true
	}
	for _, c := range cases {
		generated, err := specSkills(hub, c.spec)
		require.NoError(t, err)
		index := hubSkillIndex(hub, generated)
		loader := newSkillLoader(hub, generated)
		signals := bricks.Signals{}
		if c.project != "" {
			signals = bricks.DetectSignals(c.project)
		}
		keep := frontendFilter(rootFilter(loader, c.b.Spec.Plugins, c.b.Spec.MCP, ""), c.project, signals)
		var deny []string
		if c.spec.Skills != nil {
			deny = c.spec.Skills.Deny
		}
		denied := denyList(deny)
		var prompt string
		if c.spec.Prompt != nil && c.spec.Prompt.Template != "" {
			data, err := os.ReadFile(filepath.Join(hub, "workflows", c.spec.Prompt.Template))
			require.NoError(t, err)
			prompt = string(data)
		}
		callers := callersOf(c.b.Spec.SubagentGraph)
		bodies := map[string]string{}
		for _, a := range c.b.Spec.Agents {
			bodies[a.ID] = a.Body
		}
		envs := bundleWhenEnvs(c.spec, c.project, c.b.Spec)
		for _, a := range c.b.Spec.Agents {
			fm, err := bricks.ParseAgentFrontmatter(files[a.ID])
			require.NoError(t, err)
			env := envs[a.ID]
			inlined := map[string]bool{}
			for _, in := range InlinedSkills(a.Body) {
				inlined[in.ID] = true
			}
			where := c.ref.ID + " " + a.ID
			if c.project != "" {
				where += " (stacks)"
			}
			invited := map[string]string{}
			for _, ref := range fm.NativeSkills {
				invited[ref] = "native"
			}
			for _, ref := range selfMentionedSkills(invitingText(a.Body, a.ID, agentIDs), index) {
				if !bricks.IsStackSkill(ref) {
					invited[ref] = "named in its prompt"
				}
			}
			if fm.StackSkills && c.project != "" {
				for _, ref := range bricks.ResolveStackSkills(c.project) {
					invited[ref] = "stack"
				}
			}
			if a.ID == c.b.Spec.EntryAgent {
				if c.spec.Skills != nil {
					for _, ref := range c.spec.Skills.Extra {
						invited[ref] = "workflow extra"
					}
				}
				for _, ref := range mentionedSkills(prompt, index) {
					invited[ref] = "named in the prompt template"
				}
			}
			for _, caller := range callers[a.ID] {
				for _, ln := range strings.Split(bodies[caller], "\n") {
					if m := skillTagLineRe.FindStringSubmatch(ln); m != nil && m[1] == a.ID {
						for _, tag := range skillTagRe.FindAllStringSubmatch(ln, -1) {
							invited[tag[1]] = "tag of a delegation prompt of " + caller
						}
					}
				}
			}
			// The skills named by an on-demand skill of its catalogue (L2,
			// reviewer → dev-standards-backend → dev-standards-migration).
			for _, id := range a.Skills {
				d, err := loader.load(index[id])
				if err != nil {
					continue
				}
				body := filterFor(d.Ref, string(d.body()), env)
				for _, ref := range selfMentionedSkills(invitingText(body, a.ID, agentIDs), index) {
					if !bricks.IsStackSkill(ref) {
						if _, ok := invited[ref]; !ok {
							invited[ref] = "named by the on-demand skill " + id
						}
					}
				}
			}
			for ref, why := range invited {
				id := skillID(ref)
				if inlined[id] || (why != "workflow extra" && (!keep(ref) || !deliverable(loader, ref, denied))) {
					continue // inlined, or left out of the session (plugin, MCP, frontend, denied)
				}
				if d, err := loader.load(ref); err == nil && !skillWhen(d, env) {
					continue // its when: fails for this agent (L3)
				}
				assert.Contains(t, a.Skills, id, "%s: %s (%s) is not in its catalogue", where, id, why)
			}
		}
	}
}

// L2: no agent sees a skill it cannot reach (catalogue = reachableSkills,
// computed apart from the build), nor a skill inlined in its prompt, nor a
// stack skill without stack_skills; every shipped skill is in a catalogue.
func TestNoSpuriousCatalogue(t *testing.T) {
	hub, cases := shippedCatalogues(t)
	files, err := bricks.FindAgentFiles(hub)
	require.NoError(t, err)
	stacks, err := filepath.Glob(filepath.Join(hub, "skills", "developer", "stacks", "*.md"))
	require.NoError(t, err)
	stackIDs := map[string]bool{}
	for _, f := range stacks {
		stackIDs[strings.TrimSuffix(filepath.Base(f), ".md")] = true
	}
	for _, c := range cases {
		reach := reachableSkills(t, hub, c.spec, c.b, c.project)
		inSome := map[string]bool{}
		for _, a := range c.b.Spec.Agents {
			where := c.ref.ID + " " + a.ID
			require.NotNil(t, a.Skills, "%s: no catalogue", where)
			assert.True(t, slices.IsSorted(a.Skills), "%s: catalogue not sorted", where)
			assert.Equal(t, idsOf(reach[a.ID]), idsOf(VisibleSkills(a, c.b.Spec.Skills)), "%s: catalogue ≠ reachable skills", where)
			fm, err := bricks.ParseAgentFrontmatter(files[a.ID])
			require.NoError(t, err)
			inlined := map[string]bool{}
			for _, in := range InlinedSkills(a.Body) {
				inlined[in.ID] = true
			}
			for _, id := range a.Skills {
				inSome[id] = true
				assert.False(t, inlined[id], "%s: %s is inlined and in the catalogue", where, id)
				if stackIDs[id] {
					assert.True(t, fm.StackSkills, "%s: stack skill %s without stack_skills", where, id)
				}
			}
		}
		for _, s := range c.b.Spec.Skills {
			assert.True(t, inSome[s.ID], "%s: %s is shipped but in no catalogue", c.ref.ID, s.ID)
		}
	}
}
