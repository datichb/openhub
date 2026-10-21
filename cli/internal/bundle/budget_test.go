package bundle

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/testutil/golden"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Budget baseline (C4): the estimated context cost of every agent of every
// shipped workflow, built without project, with the common-stack project
// of the capability lint (stack skills count) and with a project of about
// 4 000 tokens of instructions and a wiki index (L7: instruction levels). Pinned in
// testdata/budget.golden (go test ./internal/bundle -run TestBudgetBaseline
// -update); an agent prompt growing by more than budgetGrowth over the
// golden is reported on its own.

// budgetGrowth is the growth of an agent prompt tolerated over the golden:
// 2 % since the end of P1 (modernisation plan P0.2; 10 % during the content
// trimming).
const budgetGrowth = 0.02

type budgetLine struct {
	project, workflow, agent                              string
	prompt, inlined, inlinedCount, catalog, catalogTokens int
	reach, reachTokens                                    int
}

func (l budgetLine) key() string { return l.project + " " + l.workflow + " " + l.agent }

const budgetRow = "%-8s %-22s %-28s %8d %8d %4d %4d %6d %4d %6d\n"

// budgetColumns is the field count of a budget row.
const budgetColumns = 10

func TestBudgetBaseline(t *testing.T) {
	t.Parallel()
	_, cat, _ := shippedWorkflows(t)
	projects := []struct{ name, path string }{{"none", ""}, {"stacks", capProject(t)}, {"instruct", instructProject(t)}}

	warmRefs(t, "", projects[1].path, projects[2].path)

	var lines []budgetLine
	totals := map[string]budgetLine{}
	type inlinedUse struct {
		tokens int
		agents map[string]bool
	}
	inlined := map[string]*inlinedUse{}
	for _, p := range projects {
		for _, ref := range cat.Refs() {
			_, b := shippedBuild(t, p.path, ref)
			tot := budgetLine{project: p.name, workflow: ref.ID, agent: "TOTAL", catalog: len(b.Spec.Skills)}
			for _, sk := range b.Spec.Skills {
				tot.catalogTokens += SkillCatalogTokens(sk)
			}
			reach := shippedReach(t, p.path, ref)
			reachedByOne := map[string]bool{}
			for _, a := range b.Spec.Agents {
				l := budgetLine{project: p.name, workflow: ref.ID, agent: a.ID, prompt: EstimateTokens(a.Body)}
				for _, in := range InlinedSkills(a.Body) {
					l.inlined += in.Tokens
					l.inlinedCount++
					u := inlined[in.ID]
					if u == nil {
						u = &inlinedUse{agents: map[string]bool{}}
						inlined[in.ID] = u
					}
					u.tokens = max(u.tokens, in.Tokens)
					u.agents[a.ID] = true
				}
				for _, sk := range VisibleSkills(a, b.Spec.Skills) {
					l.catalog++
					l.catalogTokens += SkillCatalogTokens(sk)
				}
				for _, sk := range reach[a.ID] {
					l.reach++
					l.reachTokens += SkillCatalogTokens(sk)
					if !reachedByOne[sk.ID] {
						reachedByOne[sk.ID] = true
						tot.reach++
						tot.reachTokens += SkillCatalogTokens(sk)
					}
				}
				lines = append(lines, l)
				tot.prompt += l.prompt
				tot.inlined += l.inlined
				tot.inlinedCount += l.inlinedCount
			}
			totals[tot.key()] = tot
		}
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].key() < lines[j].key() })

	var out strings.Builder
	out.WriteString("# Budget baseline: estimated tokens (runes/4, as oh bundle show --budget).\n")
	out.WriteString("# prompt = composed system prompt; inlined = part taken by the inlined skills (count);\n")
	out.WriteString("# catalog = on-demand skills visible to the agent and their catalogue entries (id: description).\n")
	out.WriteString("# reach = on-demand skills the agent can reach (native skills, skills named in its prompt, stack\n")
	out.WriteString("# skills for the agents with stack_skills, extra and prompt skills for the entry agent, skills\n")
	out.WriteString("# named by its callers for the agents with skills_from_callers, their requires closure, then the\n")
	out.WriteString("# skills named by the on-demand skills reached, transitively) and their catalogue entries.\n")
	out.WriteString("# Per-agent catalogue (L2): catalog = reach.\n")
	out.WriteString("# TOTAL: sums of the agents; catalog = every skill of the bundle; reach = skills reached by an agent.\n")
	out.WriteString("# Regenerate: go test ./internal/bundle -run TestBudgetBaseline -update\n\n")
	fmt.Fprintf(&out, "%-8s %-22s %-28s %8s %8s %4s %4s %6s %4s %6s\n", "project", "workflow", "agent",
		"prompt", "inlined", "n", "cat", "cat_tk", "rch", "rch_tk")
	for i, l := range lines {
		fmt.Fprintf(&out, budgetRow, l.project, l.workflow, l.agent, l.prompt, l.inlined, l.inlinedCount, l.catalog, l.catalogTokens, l.reach, l.reachTokens)
		if i+1 == len(lines) || lines[i+1].project != l.project || lines[i+1].workflow != l.workflow {
			tot := totals[l.project+" "+l.workflow+" TOTAL"]
			fmt.Fprintf(&out, budgetRow, tot.project, tot.workflow, tot.agent, tot.prompt, tot.inlined, tot.inlinedCount, tot.catalog, tot.catalogTokens, tot.reach, tot.reachTokens)
		}
	}
	out.WriteString("\n# Inlined skills: largest section (tokens) and the agents inlining it.\n")
	ids := make([]string, 0, len(inlined))
	for id := range inlined {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if inlined[ids[i]].tokens != inlined[ids[j]].tokens {
			return inlined[ids[i]].tokens > inlined[ids[j]].tokens
		}
		return ids[i] < ids[j]
	})
	for _, id := range ids {
		agents := make([]string, 0, len(inlined[id].agents))
		for a := range inlined[id].agents {
			agents = append(agents, a)
		}
		sort.Strings(agents)
		fmt.Fprintf(&out, "%-40s %6d %s\n", id, inlined[id].tokens, strings.Join(agents, ","))
	}

	name := filepath.Join("testdata", "budget.golden")
	if !golden.Updating() {
		pinned := readBudgetGolden(t, name)
		for _, l := range lines {
			if was, ok := pinned[l.key()]; ok && float64(l.prompt) > float64(was)*(1+budgetGrowth) {
				t.Errorf("%s: prompt %d tokens, +%.0f %% over the baseline %d (review the growth, then -update)",
					l.key(), l.prompt, 100*float64(l.prompt-was)/float64(was), was)
			}
		}
	}
	golden.Assert(t, "budget", out.String())
}

// instructProject copies testdata/instructions-project: about 4 000 tokens
// of instruction files (ONBOARDING.md, CONVENTIONS.md, .claude/CLAUDE.md)
// and docs/wiki/index.md.
func instructProject(t *testing.T) string {
	t.Helper()
	return sharedProjectDir(t, "instruct", func(dir string) error {
		return os.CopyFS(dir, os.DirFS(filepath.Join("testdata", "instructions-project")))
	})
}

// readBudgetGolden returns the pinned prompt tokens by "project workflow agent".
func readBudgetGolden(t *testing.T, name string) map[string]int {
	t.Helper()
	out := map[string]int{}
	f, err := os.Open(name)
	if err != nil {
		return out
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != budgetColumns || fields[2] == "TOTAL" {
			continue
		}
		if n, err := strconv.Atoi(fields[3]); err == nil {
			out[fields[0]+" "+fields[1]+" "+fields[2]] = n
		}
	}
	require.NoError(t, sc.Err())
	return out
}

// reachableSkills returns, by agent, the bundle skills the agent can reach
// on demand (L2): its native skills, the skills named in its composed prompt
// (inlined ones and sections that do not invite it aside), the stack skills
// of the project when it sets stack_skills, for the entry agent the workflow
// `skills.extra` and the skills named in the prompt template, for an agent
// that sets skills_from_callers the skills its callers name in their prompt
// or in the on-demand skills they reach (their natives and inlined skills aside), their requires closure and, transitively, the skills named by the
// on-demand skills reached, among the skills it may load ([SKILL:…] tags
// invite the callee, stack skills come from the stack only). It is
// written apart from the build: the catalogue of each agent must match it.
func reachableSkills(t *testing.T, hub string, spec *workflow.Spec, b *Bundle, projectPath string) map[string][]sessionspec.SkillDef {
	t.Helper()
	generated, err := specSkills(hub, spec)
	require.NoError(t, err)
	loader := newSkillLoader(hub, generated)
	index := hubSkillIndex(hub, generated)
	files, err := bricks.FindAgentFiles(hub)
	require.NoError(t, err)
	agentIDs := map[string]bool{}
	for id := range files {
		agentIDs[id] = true
	}
	var extra []string
	if spec.Skills != nil {
		extra = spec.Skills.Extra
	}
	prompt := ""
	if spec.Prompt != nil && spec.Prompt.Template != "" {
		data, err := os.ReadFile(filepath.Join(hub, "workflows", spec.Prompt.Template))
		require.NoError(t, err)
		prompt = string(data)
	}
	type agentReach struct {
		fm      *bricks.AgentFrontmatter
		inlined map[string]bool
		named   []string // skills named in the prompt of the agent
		self    []string // skills the prompt invites the agent to load
	}
	byID := map[string]*agentReach{}
	for _, a := range b.Spec.Agents {
		fm, err := bricks.ParseAgentFrontmatter(files[a.ID])
		require.NoError(t, err)
		ar := &agentReach{fm: fm, inlined: map[string]bool{}}
		for _, in := range InlinedSkills(a.Body) {
			ar.inlined[in.ID] = true
		}
		text := invitingText(a.Body, a.ID, agentIDs)
		for _, ref := range mentionedSkills(text, index) {
			if !ar.inlined[skillID(ref)] {
				ar.named = append(ar.named, ref)
			}
		}
		for _, ref := range selfMentionedSkills(text, index) {
			if !ar.inlined[skillID(ref)] && !bricks.IsStackSkill(ref) {
				ar.self = append(ar.self, ref)
			}
		}
		byID[a.ID] = ar
	}
	callers := callersOf(b.Spec.SubagentGraph)
	envs := bundleWhenEnvs(spec, projectPath, b.Spec)
	// reach returns the on-demand skills an agent reaches from roots.
	reach := func(id string, roots []string) map[string]*skillDoc {
		ar, env := byID[id], envs[id]
		reached := map[string]*skillDoc{}
		queue := roots
		for len(queue) > 0 {
			root := queue[0]
			queue = queue[1:]
			// An undeliverable mention is not shipped: its closure may fail.
			docs, err := loader.closureFor([]string{root}, denyList(nil), func(d *skillDoc) bool { return !skillWhen(d, env) })
			if err != nil {
				continue
			}
			for _, d := range docs {
				if ar.inlined[d.ID] || reached[d.ID] != nil {
					continue
				}
				reached[d.ID] = d
				// Skills named by an on-demand skill it loads reach it too.
				body := filterFor(d.Ref, string(d.body()), env)
				for _, ref := range selfMentionedSkills(invitingText(body, id, agentIDs), index) {
					if !bricks.IsStackSkill(ref) {
						queue = append(queue, ref)
					}
				}
			}
		}
		return reached
	}
	reached := map[string]map[string]*skillDoc{}
	// The skills_from_callers agents also reach the skills named by the
	// on-demand skills their callers reach: until nothing changes.
	for changed := true; changed; {
		changed = false
		for _, a := range b.Spec.Agents {
			ar := byID[a.ID]
			roots := append(append([]string(nil), ar.fm.NativeSkills...), ar.self...)
			if ar.fm.StackSkills && projectPath != "" {
				roots = append(roots, bricks.ResolveStackSkills(projectPath)...)
			}
			if a.ID == b.Spec.EntryAgent {
				roots = append(roots, extra...)
				roots = append(roots, mentionedSkills(prompt, index)...)
			}
			if ar.fm.SkillsFromCallers {
				for _, c := range callers[a.ID] {
					natives := map[string]bool{}
					for _, n := range byID[c].fm.NativeSkills {
						natives[n] = true
					}
					named := slices.Clone(byID[c].named)
					for _, d := range reached[c] {
						body := filterFor(d.Ref, string(d.body()), envs[c])
						for _, ref := range mentionedSkills(invitingText(body, a.ID, agentIDs), index) {
							if !bricks.IsStackSkill(ref) {
								named = append(named, ref)
							}
						}
					}
					for _, ref := range named {
						if !natives[ref] && !byID[c].inlined[skillID(ref)] {
							roots = append(roots, ref)
						}
					}
				}
			}
			r := reach(a.ID, roots)
			if len(r) != len(reached[a.ID]) {
				changed = true
			}
			reached[a.ID] = r
		}
	}
	out := map[string][]sessionspec.SkillDef{}
	for _, a := range b.Spec.Agents {
		for _, sk := range b.Spec.Skills {
			if reached[a.ID][sk.ID] != nil && ToolEffect(a.Permissions, sessionspec.ActionSkill, sk.ID) != sessionspec.EffectDeny {
				out[a.ID] = append(out[a.ID], sk)
			}
		}
	}
	return out
}

func TestInlinedSkills(t *testing.T) {
	body := "agent\n\n---\n\n" + inlinedSkillHeading("a") + "aaaa\n\n---\n\n" + inlinedSkillHeading("b") + "bb" +
		absentSkillsNote([]string{"c"}) + contractHeading + " à partir du workflow `w`." + "\n\n---\n\n# Project instructions\n"
	got := InlinedSkills(body)
	require.Len(t, got, 2)
	assert.Equal(t, "a", got[0].ID)
	assert.Equal(t, EstimateTokens(inlinedSkillHeading("a")+"aaaa"), got[0].Tokens)
	assert.Equal(t, "b", got[1].ID)
	assert.Equal(t, EstimateTokens(inlinedSkillHeading("b")+"bb"), got[1].Tokens)
	assert.Empty(t, InlinedSkills("no skill"))
}
