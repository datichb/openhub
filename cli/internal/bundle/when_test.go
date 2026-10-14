package bundle

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/workflow"
)

func roleEnv(agent string, roles ...string) whenEnv {
	env := whenEnv{agent: agent, roles: map[string]bool{}, calls: map[string]bool{}, mcp: map[string]bool{}, plugin: map[string]bool{}, flags: map[string]bool{}}
	for _, r := range roles {
		env.roles[r] = true
	}
	return env
}

func TestParseWhen(t *testing.T) {
	for _, ok := range []string{"", "role=subagent", "role=entry,subagent", "agent!=developer", "calls=developer mcp=gitlab", "websearch", "!headless shell", "plugin=context-mode", "role=subagent | calls=developer", "method=tdd", "method!=standard", "!writes", "writes role=subagent"} {
		_, err := parseWhen(ok)
		assert.NoError(t, err, ok)
	}
	for _, bad := range []string{"role=boss", "role", "colour=red", "websearch=true", "agent=", "agent=Dev", "role=entry |", "| role=entry", "method=waterfall", "method"} {
		_, err := parseWhen(bad)
		assert.Error(t, err, bad)
	}
}

func TestWhenHolds(t *testing.T) {
	sub := roleEnv("developer", roleSubagent)
	sub.calls["documentarian"] = true
	sub.flags["shell"] = true
	entry := roleEnv("orchestrator", roleEntry)
	both := roleEnv("reviewer", roleEntry, roleSubagent)
	tdd := roleEnv("developer", roleSubagent)
	tdd.method = "tdd"
	cases := []struct {
		expr string
		env  whenEnv
		want bool
	}{
		{"role=subagent", sub, true},
		{"role=subagent", entry, false},
		{"role=entry", both, true},
		{"role=subagent", both, true},
		{"role!=subagent", both, false},
		{"agent=developer", sub, true},
		{"agent!=developer", sub, false},
		{"calls=documentarian", sub, true},
		{"calls=documentarian", entry, false},
		{"shell", sub, true},
		{"!shell", sub, false},
		{"role=subagent shell", sub, true},
		{"role=subagent websearch", sub, false},
		{"mcp=gitlab", sub, false},
		{"role=entry | calls=documentarian", sub, true},
		{"role=entry | agent=reviewer", sub, false},
		{"method=tdd", tdd, true},
		{"method=tdd", sub, false},
		{"method!=tdd", sub, true},
	}
	for _, c := range cases {
		e, err := parseWhen(c.expr)
		require.NoError(t, err)
		assert.Equal(t, c.want, e.holds(c.env), "%s for %s", c.expr, c.env.agent)
	}
	e, _ := parseWhen("role=entry")
	assert.True(t, e.holdsAny([]whenEnv{sub, entry}), "union")
	assert.False(t, e.holdsAny([]whenEnv{sub}))
	assert.True(t, e.holdsAny(nil), "no reader: kept")
}

func TestFilterWhen(t *testing.T) {
	text := strings.Join([]string{
		"# Titre",
		"",
		"Commun.",
		"",
		"<!-- oh:when role=entry -->",
		"## Agent d'entrée",
		"",
		"Utilise `question`.",
		"<!-- /oh:when -->",
		"",
		"<!-- oh:when role=subagent -->",
		"## Sous-agent",
		"",
		"Questions bloquantes.",
		"<!-- oh:when agent=developer -->",
		"Producteur.",
		"<!-- /oh:when -->",
		"<!-- /oh:when -->",
		"",
		"Fin.",
		"",
	}, "\n")
	got, err := filterWhen(text, []whenEnv{roleEnv("pathfinder", roleSubagent)})
	require.NoError(t, err)
	assert.Equal(t, "# Titre\n\nCommun.\n\n## Sous-agent\n\nQuestions bloquantes.\n\nFin.\n", got)

	got, err = filterWhen(text, []whenEnv{roleEnv("orchestrator", roleEntry)})
	require.NoError(t, err)
	assert.Equal(t, "# Titre\n\nCommun.\n\n## Agent d'entrée\n\nUtilise `question`.\n\nFin.\n", got)

	got, err = filterWhen(text, []whenEnv{roleEnv("orchestrator", roleEntry), roleEnv("developer", roleSubagent)})
	require.NoError(t, err)
	assert.Contains(t, got, "Utilise `question`.")
	assert.Contains(t, got, "Producteur.")
	assert.NotContains(t, got, "oh:when")

	plain := "pas de marqueur\n"
	got, err = filterWhen(plain, nil)
	require.NoError(t, err)
	assert.Equal(t, plain, got)
}

func TestScanWhenErrors(t *testing.T) {
	for name, text := range map[string]string{
		"open":      "<!-- oh:when role=entry -->\nx\n",
		"close":     "x\n<!-- /oh:when -->\n",
		"inline":    "texte <!-- oh:when role=entry --> suite\n",
		"no expr":   "<!-- oh:when -->\nx\n<!-- /oh:when -->\n",
		"bad key":   "<!-- oh:when colour=red -->\nx\n<!-- /oh:when -->\n",
		"close arg": "<!-- oh:when role=entry -->\nx\n<!-- /oh:when role=entry -->\n",
	} {
		_, err := scanWhen(text)
		assert.Error(t, err, name)
		out, ferr := filterWhen(text, nil)
		assert.Error(t, ferr, name)
		assert.Equal(t, text, out, "%s: a malformed text is kept whole", name)
	}
}

// hubTexts returns the agents, skills and prompt templates of the hub.
func hubTexts(t *testing.T) map[string]string {
	t.Helper()
	hub := repoHub(t)
	out := map[string]string{}
	for _, dir := range []string{"agents", "skills", "workflows"} {
		require.NoError(t, filepath.WalkDir(filepath.Join(hub, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || (!strings.HasSuffix(path, ".md") && !strings.HasSuffix(path, ".tmpl")) {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(hub, path)
			out[filepath.ToSlash(rel)] = string(data)
			return nil
		}))
	}
	return out
}

// L3: the markers of the hub content are well formed and balanced.
func TestHubWhenMarkersBalanced(t *testing.T) {
	n := 0
	for rel, text := range hubTexts(t) {
		blocks, err := scanWhen(text)
		assert.NoError(t, err, rel)
		n += len(blocks)
	}
	assert.Positive(t, n, "the hub uses oh:when markers")
}

// whenBlocks returns the text of the blocks of text whose condition keeps
// them for a subagent only (not for an entry agent).
func subagentOnlyBlocks(text string) []string {
	var out []string
	var stack []bool // subagent only
	var cur strings.Builder
	for _, ln := range strings.Split(text, "\n") {
		if m := whenMarkerRe.FindStringSubmatch(ln); m != nil {
			if m[1] == "/" {
				if stack[len(stack)-1] && !anyTrue(stack[:len(stack)-1]) {
					out = append(out, cur.String())
					cur.Reset()
				}
				stack = stack[:len(stack)-1]
				continue
			}
			e, _ := parseWhen(m[2])
			stack = append(stack, !e.holds(roleEnv("x", roleEntry)) && e.holds(withAllFlags(roleEnv("x", roleSubagent))))
			continue
		}
		if anyTrue(stack) {
			cur.WriteString(ln + "\n")
		}
	}
	return out
}

func anyTrue(bs []bool) bool {
	for _, b := range bs {
		if b {
			return true
		}
	}
	return false
}

// withAllFlags is env where every agent, call and flag condition holds.
func withAllFlags(env whenEnv) whenEnv {
	env.agent = "*"
	for _, k := range whenFlagKeys {
		env.flags[k] = true
	}
	return env
}

var questionCallRe = regexp.MustCompile("question\\(|outil `question` (?:pour|avec)|appelle(?:r)? (?:l'outil )?`question`")

// L3 (C-6): a subagent never asks the user: no block written for subagents
// calls the question tool.
func TestNoQuestionInSubagentBlocks(t *testing.T) {
	for rel, text := range hubTexts(t) {
		for _, b := range subagentOnlyBlocks(text) {
			for _, ln := range strings.Split(b, "\n") {
				if questionCallRe.MatchString(ln) && !strings.Contains(ln, "n'appelle pas") && !strings.Contains(ln, "pas l'outil `question`") && !strings.Contains(ln, "jamais") {
					t.Errorf("%s: a subagent block calls question: %s", rel, ln)
				}
			}
		}
	}
}

// L3: an agent's own text has no block for a role it never has in the
// shipped workflows (dead text).
func TestNoDeadRoleBlocksInAgents(t *testing.T) {
	hub, cat, env := shippedWorkflows(t)
	roles := map[string]map[string]bool{}
	for _, ref := range cat.Refs() {
		r, diags := workflow.Check(cat, ref, nil, env)
		require.False(t, diags.HasErrors(), "%v", diags)
		b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec})
		require.NoError(t, err)
		selectable := r.Spec.Entry != nil && r.Spec.Entry.Selectable
		for id, e := range bundleWhenEnvs(r.Spec, "", b.Spec) {
			if roles[id] == nil {
				roles[id] = map[string]bool{}
			}
			for k := range e.roles {
				roles[id][k] = true
			}
			if selectable { // any member may be the entry agent (oh run libre --agent <id>)
				roles[id][roleEntry] = true
			}
		}
	}
	texts := hubTexts(t)
	for rel, text := range texts {
		if !strings.HasPrefix(rel, "agents/") {
			continue
		}
		id := strings.TrimSuffix(filepath.Base(rel), ".md")
		has := roles[id]
		if has == nil {
			continue
		}
		for _, ln := range strings.Split(text, "\n") {
			m := whenMarkerRe.FindStringSubmatch(ln)
			if m == nil || m[1] == "/" {
				continue
			}
			e, _ := parseWhen(m[2])
			alive := false
			for r := range has {
				for _, m := range append([]workflow.Method{workflow.MethodStandard}, workflow.Methods...) {
					env := withAllFlags(roleEnv(id, r))
					env.method = string(m)
					alive = alive || e.holds(env)
					// R4: a negated flag (`!writes`) holds without the flag.
					bare := roleEnv(id, r)
					bare.agent, bare.method = "*", string(m)
					alive = alive || e.holds(bare)
				}
			}
			assert.True(t, alive, "%s: block %q never kept (roles of %s: %v)", rel, strings.TrimSpace(m[2]), id, has)
		}
	}
	for _, id := range []string{"orchestrator", "conductor"} {
		assert.False(t, roles[id][roleSubagent], "%s is always the entry agent", id)
		b := agentBody(t, shippedBundle(t, map[string]string{"orchestrator": "feature", "conductor": "sweep"}[id]), id)
		assert.NotContains(t, b, "### Questions bloquantes` de ton bloc de retour", "%s: subagent text in an entry agent", id)
	}
}

// L3: the composed prompt of an agent that is only a subagent does not
// invite it to call question.
func TestSubagentPromptsDoNotCallQuestion(t *testing.T) {
	for _, wf := range []string{"feature", "ticket", "sweep", "cadrage"} {
		b := shippedBundle(t, wf)
		envs := bundleWhenEnvs(nil, "", b.Spec)
		for _, a := range b.Spec.Agents {
			if envs[a.ID].roles[roleEntry] {
				continue
			}
			assert.NotContains(t, a.Body, "question({", "%s/%s", wf, a.ID)
		}
	}
}
