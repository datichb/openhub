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

var (
	skillTagRe    = regexp.MustCompile(`\[SKILL:([a-z0-9][a-z0-9/_-]*)\]`)
	skillPhraseRe = regexp.MustCompile("\\b[Ss]kills?\\s+((?:\\*\\*)?`[a-z0-9][a-z0-9/_-]*`(?:\\*\\*)?(?:\\s*(?:,|et|and|ou|or|\\+)\\s*(?:\\*\\*)?`[a-z0-9][a-z0-9/_-]*`(?:\\*\\*)?)*)")
	codeSpanRe    = regexp.MustCompile("`([a-z0-9][a-z0-9/_-]*)`")
)

// QB3: every skill reference of the hub content (agents/, skills/,
// workflows/prompts/) names an existing skill: a [SKILL:…] tag, a
// `category/name` code span, or « skill `name` » (orchestrator-dev cited
// dev-standards-security-hardening, the orchestrator injected
// [SKILL:planning/planner-subagent]… for skills merged long ago).
func TestHubSkillReferencesExist(t *testing.T) {
	hub := repoHub(t)
	generated := map[string]string{"workflow/workflow-map": "", "shared/hub-workflow-reference": ""}
	index := hubSkillIndex(hub, generated)
	cats := map[string]bool{}
	entries, err := os.ReadDir(filepath.Join(hub, "skills"))
	require.NoError(t, err)
	for _, e := range entries {
		if e.IsDir() && e.Name() != "templates" {
			cats[e.Name()] = true
		}
	}
	var files []string
	for _, dir := range []string{"agents", "skills", "workflows/prompts"} {
		_ = filepath.WalkDir(filepath.Join(hub, dir), func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && (strings.HasSuffix(p, ".md") || strings.HasSuffix(p, ".tmpl")) {
				files = append(files, p)
			}
			return nil
		})
	}
	require.NotEmpty(t, files)
	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)
		text := string(data)
		var refs []string
		for _, m := range skillTagRe.FindAllStringSubmatch(text, -1) {
			refs = append(refs, m[1])
		}
		for _, m := range codeSpanRe.FindAllStringSubmatch(text, -1) {
			if cat, name, ok := strings.Cut(m[1], "/"); ok && cats[cat] && strings.Trim(name, "/") != "" {
				refs = append(refs, m[1])
			}
		}
		for _, m := range skillPhraseRe.FindAllStringSubmatch(text, -1) {
			for _, c := range codeSpanRe.FindAllStringSubmatch(m[1], -1) {
				refs = append(refs, c[1])
			}
		}
		rel, _ := filepath.Rel(hub, f)
		for _, ref := range refs {
			if _, ok := index[ref]; !ok {
				t.Errorf("%s: skill %q does not exist", rel, ref)
			}
		}
	}
}

// QB3: the audit bundle ships every domain checklist of the hub
// (auditor/audit-<domain>, loaded by auditor-subagent).
func TestAuditBundleShipsDomainChecklists(t *testing.T) {
	hub, cat, env := shippedWorkflows(t)
	r, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "audit"}, nil, env)
	require.False(t, diags.HasErrors(), "%v", diags)
	b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec})
	require.NoError(t, err)
	shipped := map[string]bool{}
	for _, s := range b.Spec.Skills {
		shipped[s.ID] = true
	}
	files, err := filepath.Glob(filepath.Join(hub, "skills", "auditor", "audit-*.md"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	for _, f := range files {
		id := strings.TrimSuffix(filepath.Base(f), ".md")
		if id == "audit-handoff-format" || id == "audit-protocol-light" {
			continue // formats and light protocol, not domain checklists
		}
		assert.True(t, shipped[id], "audit bundle does not ship %s", id)
	}
}
