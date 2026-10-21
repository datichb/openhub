package bundle

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// instructionsFixture writes a small project with instruction files and a
// wiki index.
func instructionsFixture(t *testing.T, wiki bool) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, writeInstructionsFixture(dir, wiki))
	return dir
}

// sharedInstructionsFixture is instructionsFixture built once for the tests
// that only read the project (see sharedProjectDir).
func sharedInstructionsFixture(t *testing.T, wiki bool) string {
	t.Helper()
	return sharedProjectDir(t, fmt.Sprintf("instructions-wiki-%t", wiki), func(dir string) error {
		return writeInstructionsFixture(dir, wiki)
	})
}

func writeInstructionsFixture(dir string, wiki bool) error {
	files := map[string]string{
		"CONVENTIONS.md":    "# Conventions Atlas\n\nRègles de code.\n\n## Nommage\n\n- snake_case.\n\n```md\n## Pas une section\n```\n\n## Tests\n\n- Table-driven.\n",
		"ONBOARDING.md":     "---\nupdated: 2026-10-01\n---\n\n# Onboarding Atlas\n\n## Démarrer\n\nmake dev.\n",
		".claude/CLAUDE.md": "Sans titre : réponds en français.\n",
	}
	if wiki {
		files["docs/wiki/index.md"] = "---\nupdated: 2026-10-01\n---\n\n# Atlas — Index Wiki\n\n## Stack critique\nGo.\n\n" +
			"## God nodes — concepts les plus connectés\n\n| Concept | Pages liées | Criticité |\n|---|---|---|\n" +
			"| Commande | [business/orders.md](business/orders.md), [business/billing.md](business/billing.md) | Critique |\n\n*(Vide si aucun concept)*\n\n## Zones d'ombre\n\n- Rétention.\n"
		// The pages oh computes the god nodes from (LD-J).
		files["docs/wiki/business/orders.md"] = "---\nupdated: 2026-10-01\n---\n\n# Commandes\n\n- Une commande.\n"
		files["docs/wiki/business/billing.md"] = "---\nupdated: 2026-10-01\n---\n\n# Facturation\n\n- Facture par commande.\n"
	}
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// instructionsOf returns the project instructions of a composed prompt: the
// session contract line (level none) and the instructions block.
func instructionsOf(body string) string {
	var out string
	for line := range strings.SplitSeq(body, "\n") {
		if strings.HasPrefix(line, "- **Instructions du projet**") {
			out += line + "\n"
		}
	}
	if i := strings.Index(body, instructionsHeading); i >= 0 {
		out += body[i:]
	}
	return out
}

// L7: the project instructions of every agent of the libre workflow, at
// its level, are pinned in testdata/golden/instructions/libre/<agent>.md
// (go test ./internal/bundle -run ProjectInstructions -update).
func TestProjectInstructionsGolden(t *testing.T) {
	t.Parallel()
	hub, _, _ := shippedWorkflows(t)
	_, b := shippedBuild(t, sharedInstructionsFixture(t, true), workflow.Ref{Layer: workflow.LayerHub, ID: "libre"})
	files, err := bricks.FindAgentFiles(hub)
	require.NoError(t, err)
	levels := map[string]bool{}
	for _, a := range b.Spec.Agents {
		fm, err := bricks.ParseAgentFrontmatter(files[a.ID])
		require.NoError(t, err)
		levels[fm.InstructionsLevel()] = true
		got := instructionsOf(a.Body)
		switch fm.InstructionsLevel() {
		case bricks.InstructionsFull:
			assert.Contains(t, got, "## CONVENTIONS.md\n\n# Conventions Atlas", a.ID)
			assert.NotContains(t, got, "God nodes", a.ID)
		case bricks.InstructionsIndex:
			assert.NotContains(t, got, "Règles de code.", a.ID)
			assert.Contains(t, got, "| Commande |", a.ID)
		case bricks.InstructionsNone:
			assert.NotContains(t, got, instructionsHeading, a.ID)
			assert.Contains(t, got, "`project_instructions: none`", a.ID)
		}
		assertGolden(t, filepath.Join("instructions", "libre", a.ID+".md"), got)
	}
	assert.Len(t, levels, 3, "libre covers the three levels")
}

// The levels of the shipped agents (modernisation plan, L7).
func TestShippedInstructionLevels(t *testing.T) {
	files, err := bricks.FindAgentFiles(repoHub(t))
	require.NoError(t, err)
	want := map[string]string{
		"orchestrator": "none", "conductor": "none",
		"orchestrator-dev": "index", "auditor": "index", "pathfinder": "index", "planner": "index",
		"designer": "index", "documentarian": "index", "onboarder": "index", "release-manager": "index",
	}
	for id, path := range files {
		fm, err := bricks.ParseAgentFrontmatter(path)
		require.NoError(t, err)
		level := want[id]
		if level == "" {
			level = bricks.InstructionsFull
		}
		assert.Equal(t, level, fm.InstructionsLevel(), id)
	}
}

func TestInstructionsRender(t *testing.T) {
	p, err := readInstructions(instructionsFixture(t, false), nil)
	require.NoError(t, err)
	require.Len(t, p.Files, 3)
	assert.Nil(t, p.Wiki, "no docs/wiki/index.md")
	conv := p.Files[1]
	assert.Equal(t, "CONVENTIONS.md", conv.Path)
	assert.Equal(t, "Conventions Atlas", conv.Title)
	assert.Equal(t, []string{"Nommage", "Tests"}, conv.Sections, "code fences aside")
	assert.Equal(t, "Onboarding Atlas", p.Files[0].Title, "frontmatter aside")
	assert.Empty(t, p.Files[2].Title)

	allow := []sessionspec.PermissionRule{{Action: "*", Resource: "*", Effect: sessionspec.EffectAllow}}
	full := p.render(bricks.InstructionsFull, allow)
	assert.True(t, strings.HasPrefix(full, instructionsHeading))
	assert.Contains(t, full, "\n## CONVENTIONS.md\n\n"+conv.Body+"\n")
	assert.Empty(t, p.render(bricks.InstructionsNone, allow))

	index := p.render(bricks.InstructionsIndex, allow)
	assert.Contains(t, index, "- `CONVENTIONS.md` : Conventions Atlas (≈ ")
	assert.Contains(t, index, "; sections : Nommage · Tests\n")
	assert.NotContains(t, index, "Wiki du projet")
	assert.NotContains(t, index, "lecture")

	rules := append(allow, sessionspec.PermissionRule{Action: sessionspec.ActionRead, Resource: ".claude/*", Effect: sessionspec.EffectDeny})
	assert.Contains(t, p.render(bricks.InstructionsIndex, rules), "- `.claude/CLAUDE.md` (≈ 9 tokens) — lecture non autorisée pour toi")
	ask := []sessionspec.PermissionRule{{Action: "*", Resource: "*", Effect: sessionspec.EffectAsk}}
	assert.Contains(t, p.render(bricks.InstructionsIndex, ask), "— lecture soumise à confirmation")

	assert.Empty(t, projectInstructions{}.render(bricks.InstructionsIndex, allow), "no instructions, no wiki")
	assert.Empty(t, instructionsContract(bricks.InstructionsNone, projectInstructions{}))
	assert.Empty(t, instructionsContract(bricks.InstructionsIndex, p))
	assert.NotEmpty(t, instructionsContract(bricks.InstructionsNone, p))
}

// LD-J: the wiki index of the prompts comes from the WikiService, god nodes
// computed by oh from the pages; the same data renders as the table of the
// index.
func TestWikiIndexOf(t *testing.T) {
	t.Parallel()
	assert.Nil(t, wikiIndexOf(""), "no project")
	assert.Nil(t, wikiIndexOf(instructionsFixture(t, false)), "no wiki")
	w := wikiIndexOf(instructionsFixture(t, true))
	require.NotNil(t, w)
	assert.Equal(t, "Atlas — Index Wiki", w.Title)
	assert.Equal(t, []string{"Stack critique", "God nodes — concepts les plus connectés", "Zones d'ombre"}, w.Sections)
	assert.Equal(t, []string{"| Concept | Pages liées | Criticité |", "|---|---|---|",
		"| Commande | [business/orders.md](business/orders.md), [business/billing.md](business/billing.md) | Critique |"}, w.GodNodes, "declared row kept")

	// A declared god node cited by one page only is not one; a header
	// without a row is no table.
	dir := t.TempDir()
	write := func(rel, text string) {
		p := filepath.Join(dir, "docs", "wiki", filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(text), 0o644))
	}
	write("index.md", "# W\n\n## God nodes\n\n| C | P | X |\n|---|---|---|\n| Seul | [a.md](a.md) | Normale |\n\n## Autre\n")
	write("a.md", "# A\n\nSeul.\n")
	w = wikiIndexOf(dir)
	require.NotNil(t, w)
	assert.Nil(t, w.GodNodes)

	// At most indexMaxGodNodes rows.
	var rows strings.Builder
	for i := range indexMaxGodNodes + 3 {
		fmt.Fprintf(&rows, "| `Concept%dX` | [a.md](a.md), [b.md](b.md) | Normale |\n", i)
	}
	write("index.md", "## God nodes — concepts\n\n| C | P | X |\n|---|---|---|\n"+rows.String()+"\n## Carte\n\n| pas | un | god node |\n")
	var both strings.Builder
	for i := range indexMaxGodNodes + 3 {
		fmt.Fprintf(&both, "`Concept%dX` ", i)
	}
	write("a.md", "# A\n\n"+both.String()+"\n")
	write("b.md", "# B\n\n"+both.String()+"\n")
	w = wikiIndexOf(dir)
	require.NotNil(t, w)
	assert.Len(t, w.GodNodes, 2+indexMaxGodNodes)

	// Wiki without instruction files: the index carries the wiki alone.
	p := projectInstructions{Wiki: w}
	allow := []sessionspec.PermissionRule{{Action: "*", Resource: "*", Effect: sessionspec.EffectAllow}}
	assert.Contains(t, p.render(bricks.InstructionsIndex, allow), "## Wiki du projet")
	assert.Empty(t, p.render(bricks.InstructionsFull, allow), "full: the instruction files only")
}

func TestFrontmatterProjectInstructions(t *testing.T) {
	fm, err := bricks.ParseAgentFrontmatterFromBytes([]byte("---\nid: a\n---\n"))
	require.NoError(t, err)
	assert.Equal(t, bricks.InstructionsFull, fm.InstructionsLevel(), "default")
	fm, err = bricks.ParseAgentFrontmatterFromBytes([]byte("---\nid: a\nproject_instructions: index\n---\n"))
	require.NoError(t, err)
	assert.Equal(t, bricks.InstructionsIndex, fm.InstructionsLevel())
	_, err = bricks.ParseAgentFrontmatterFromBytes([]byte("---\nid: a\nproject_instructions: some\n---\n"))
	assert.ErrorContains(t, err, "project_instructions")
}

// L7: an agent at the level index may read each file of its index, on its
// exact path only; an explicit refusal of the agent and the guards win.
func TestIndexReadRules(t *testing.T) {
	t.Parallel()
	hub, cat, env := shippedWorkflows(t)
	r, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "libre"}, nil, env)
	require.False(t, diags.HasErrors(), "%v", diags)
	project := instructionsFixture(t, true)
	for _, f := range []string{"docs/team.md", ".oh/sessions/notes.md"} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(project, f)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(project, f), []byte("# Équipe\n"), 0o644))
	}
	extra := []string{"docs/team.md", ".oh/sessions/notes.md"}
	b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), ProjectPath: project, Spec: r.Spec, ExtraInstructionFiles: extra})
	require.NoError(t, err)
	dev := findAgent(b.Spec.Agents, "orchestrator-dev")
	require.NotNil(t, dev)
	rules := append(slices.Clip(dev.Permissions), b.Spec.AgentGuard...)
	read := func(f string) sessionspec.Effect { return ToolEffect(rules, sessionspec.ActionRead, f) }
	for _, f := range []string{".claude/CLAUDE.md", "docs/team.md", "ONBOARDING.md", "CONVENTIONS.md", wikiIndexFile} {
		assert.Equal(t, sessionspec.EffectAllow, read(f), f)
	}
	for _, f := range []string{"src/main.go", "docs/other.md", ".claude/settings.json", "README.md"} {
		assert.Equal(t, sessionspec.EffectDeny, read(f), "%s: no wider read", f)
	}
	assert.Equal(t, sessionspec.EffectDeny, ToolEffect(rules, sessionspec.ActionEdit, ".claude/CLAUDE.md"), "read only")
	assert.Equal(t, sessionspec.EffectDeny, read(".oh/sessions/notes.md"), "the session data guard wins")
	idx := instructionsOf(dev.Body)
	assert.Contains(t, idx, "- `.oh/sessions/notes.md` : Équipe (≈ 2 tokens) — lecture non autorisée pour toi")
	assert.Contains(t, idx, "- `.claude/CLAUDE.md` (≈ 9 tokens)\n")
	assert.Contains(t, idx, "- `docs/team.md` : Équipe (≈ 2 tokens)\n")

	// The generated reads sit after the agent's own rules, before every guard.
	files, err := bricks.FindAgentFiles(hub)
	require.NoError(t, err)
	fm, err := bricks.ParseAgentFrontmatter(files["orchestrator-dev"])
	require.NoError(t, err)
	req := Request{HubDir: hub, Spec: r.Spec}
	p, err := readInstructions(project, extra)
	require.NoError(t, err)
	without := req.agentPermissions(fm, projectInstructions{})
	with := req.agentPermissions(fm, p)
	gen := instructionReadRules(bricks.InstructionsIndex, ConvertPermissions(fm.Permission), p)
	require.Len(t, with, len(without)+len(gen))
	k := -1
	for i := 0; i+len(gen) <= len(with); i++ {
		if slices.Equal(with[i:i+len(gen)], gen) {
			k = i
		}
	}
	require.GreaterOrEqual(t, k, 0)
	assert.Equal(t, sessionspec.EffectDeny, ToolEffect(without[:k], sessionspec.ActionRead, "src/main.go"), "after the own rules")
	assert.Equal(t, gen, with[k:k+len(gen)])
	assert.Equal(t, without, slices.Concat(with[:k], with[k+len(gen):]))
	assert.Equal(t, without[k:], with[k+len(gen):], "the guards follow")

	// Other levels: no generated read.
	assert.Empty(t, instructionReadRules(bricks.InstructionsFull, nil, p))
	assert.Empty(t, instructionReadRules(bricks.InstructionsNone, nil, p))
}

func TestInstructionReadRulesKeepExplicit(t *testing.T) {
	p := projectInstructions{Files: []instructionFile{{Path: "CONVENTIONS.md"}, {Path: ".claude/CLAUDE.md"}, {Path: "../outside.md"}, {Path: "docs/*.md"}}}
	paths := func(rules []sessionspec.PermissionRule) []string {
		var out []string
		for _, r := range rules {
			assert.Equal(t, sessionspec.ActionRead, r.Action)
			assert.Equal(t, sessionspec.EffectAllow, r.Effect)
			out = append(out, r.Resource)
		}
		return out
	}
	closed := []sessionspec.PermissionRule{{Action: sessionspec.ActionRead, Resource: "*", Effect: sessionspec.EffectDeny}}
	assert.Equal(t, []string{"CONVENTIONS.md", ".claude/CLAUDE.md"}, paths(instructionReadRules(bricks.InstructionsIndex, closed, p)),
		"a closed default is opened on the exact paths; never outside the project or a pattern")

	deny := append(slices.Clip(closed), sessionspec.PermissionRule{Action: sessionspec.ActionRead, Resource: ".claude/*", Effect: sessionspec.EffectDeny})
	gen := instructionReadRules(bricks.InstructionsIndex, deny, p)
	assert.Equal(t, []string{"CONVENTIONS.md"}, paths(gen), "an explicit deny stays")
	idx := p.render(bricks.InstructionsIndex, append(deny, gen...))
	assert.Contains(t, idx, "- `.claude/CLAUDE.md` (≈ 0 tokens) — lecture non autorisée pour toi")
	assert.NotContains(t, idx, "- `CONVENTIONS.md` (≈ 0 tokens) —")

	ask := []sessionspec.PermissionRule{{Action: "*", Resource: ".claude/CLAUDE.md", Effect: sessionspec.EffectAsk}}
	assert.Equal(t, []string{"CONVENTIONS.md"}, paths(instructionReadRules(bricks.InstructionsIndex, ask, p)), "an explicit ask stays")
}
