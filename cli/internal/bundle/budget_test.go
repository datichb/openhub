package bundle

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/workflow"
)

// Budget baseline (C4): the estimated context cost of every agent of every
// shipped workflow, built without project and with the common-stack project
// of the capability lint (stack skills count). Pinned in
// testdata/budget.golden (go test ./internal/bundle -run TestBudgetBaseline
// -update); an agent prompt growing by more than budgetGrowth over the
// golden is reported on its own.

const budgetGrowth = 0.10

type budgetLine struct {
	project, workflow, agent                              string
	prompt, inlined, inlinedCount, catalog, catalogTokens int
}

func (l budgetLine) key() string { return l.project + " " + l.workflow + " " + l.agent }

const budgetRow = "%-8s %-22s %-28s %8d %8d %4d %4d %6d\n"

func TestBudgetBaseline(t *testing.T) {
	hub, cat, env := shippedWorkflows(t)
	projects := []struct{ name, path string }{{"none", ""}, {"stacks", capProject(t)}}

	var lines []budgetLine
	totals := map[string]budgetLine{}
	type inlinedUse struct {
		tokens int
		agents map[string]bool
	}
	inlined := map[string]*inlinedUse{}
	for _, p := range projects {
		for _, ref := range cat.Refs() {
			r, diags := workflow.Check(cat, ref, nil, env)
			require.False(t, diags.HasErrors(), "%v", diags)
			b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), ProjectPath: p.path, Spec: r.Spec})
			require.NoError(t, err)
			tot := budgetLine{project: p.name, workflow: ref.ID, agent: "TOTAL", catalog: len(b.Spec.Skills)}
			for _, sk := range b.Spec.Skills {
				tot.catalogTokens += SkillCatalogTokens(sk)
			}
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
	out.WriteString("# TOTAL: sums of the agents; catalog = every skill of the bundle.\n")
	out.WriteString("# Regenerate: go test ./internal/bundle -run TestBudgetBaseline -update\n\n")
	fmt.Fprintf(&out, "%-8s %-22s %-28s %8s %8s %4s %4s %6s\n", "project", "workflow", "agent",
		"prompt", "inlined", "n", "cat", "cat_tk")
	for i, l := range lines {
		fmt.Fprintf(&out, budgetRow, l.project, l.workflow, l.agent, l.prompt, l.inlined, l.inlinedCount, l.catalog, l.catalogTokens)
		if i+1 == len(lines) || lines[i+1].project != l.project || lines[i+1].workflow != l.workflow {
			tot := totals[l.project+" "+l.workflow+" TOTAL"]
			fmt.Fprintf(&out, budgetRow, tot.project, tot.workflow, tot.agent, tot.prompt, tot.inlined, tot.inlinedCount, tot.catalog, tot.catalogTokens)
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
	if !*updateGolden {
		pinned := readBudgetGolden(t, name)
		for _, l := range lines {
			if was, ok := pinned[l.key()]; ok && float64(l.prompt) > float64(was)*(1+budgetGrowth) {
				t.Errorf("%s: prompt %d tokens, +%.0f %% over the baseline %d (review the growth, then -update)",
					l.key(), l.prompt, 100*float64(l.prompt-was)/float64(was), was)
			}
		}
	}
	assertGoldenFile(t, name, out.String())
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
		if len(fields) != 8 || fields[2] == "TOTAL" {
			continue
		}
		if n, err := strconv.Atoi(fields[3]); err == nil {
			out[fields[0]+" "+fields[1]+" "+fields[2]] = n
		}
	}
	require.NoError(t, sc.Err())
	return out
}

func TestInlinedSkills(t *testing.T) {
	body := "agent\n\n---\n\n" + inlinedSkillHeading("a") + "aaaa\n\n---\n\n" + inlinedSkillHeading("b") + "bb" +
		absentSkillsNote([]string{"c"}) + sessionRoleNote("x", "x", nil) + "\n\n---\n\n# Project instructions\n"
	got := InlinedSkills(body)
	require.Len(t, got, 2)
	assert.Equal(t, "a", got[0].ID)
	assert.Equal(t, EstimateTokens(inlinedSkillHeading("a")+"aaaa"), got[0].Tokens)
	assert.Equal(t, "b", got[1].ID)
	assert.Equal(t, EstimateTokens(inlinedSkillHeading("b")+"bb"), got[1].Tokens)
	assert.Empty(t, InlinedSkills("no skill"))
}
