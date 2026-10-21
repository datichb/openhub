package bundle

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// N6 (DC6): one checkpoint protocol. Every v5 session runs a workflow; its
// checkpoints only pass through workflow_checkpoint. The content shipped to
// the agents of a workflow with checkpoints never asks a checkpoint with the
// `question` tool (oh closes such a question: the header or the text names
// a checkpoint id or label) and never relays a checkpoint by ending the
// session.

var (
	questionFieldRe = regexp.MustCompile(`^\s*(?:header|question)\s*:\s*"(.*)"`)
	anyCheckpointRe = regexp.MustCompile(`(?i)\bcp(?:[- ]?[0-9]+|-[a-z][a-z0-9]*)\b`)
	endSessionRe    = regexp.MustCompile(`(?i)terminer la session|end the session`)
	questionToolRe  = regexp.MustCompile("(?i)(?:outil|tool) `question`|via `question`|`question\\(`")
	negationRe      = regexp.MustCompile(`(?i)jamais|never|n'utilise|ne pas|ne l'appelle|pas par|not through|ni par|sans nommer|without naming`)
)

// checkpointNamer matches the ids (cp-2, CP 2, cp2) and labels (≥ 8
// characters, as the imitation check of the daemon) of a workflow.
type checkpointNamer struct {
	ids    []*regexp.Regexp
	labels []string
}

func newCheckpointNamer(s *workflow.Spec) checkpointNamer {
	var n checkpointNamer
	for _, id := range s.Checkpoints.Keys() {
		cp, _ := s.Checkpoints.Get(id)
		rest := strings.TrimPrefix(strings.ToLower(id), "cp-")
		n.ids = append(n.ids, regexp.MustCompile(`(?i)\bcp[-_ ]?`+regexp.QuoteMeta(rest)+`\b`))
		for _, l := range []string{cp.Label.Text("fr"), cp.Label.Text("en")} {
			if l = strings.ToLower(strings.TrimSpace(l)); len(l) >= 8 {
				n.labels = append(n.labels, l)
			}
		}
	}
	return n
}

func (n checkpointNamer) names(text string) bool {
	if anyCheckpointRe.MatchString(text) {
		return true
	}
	for _, re := range n.ids {
		if re.MatchString(text) {
			return true
		}
	}
	lower := strings.ToLower(text)
	for _, l := range n.labels {
		if strings.Contains(lower, l) {
			return true
		}
	}
	return false
}

// checkpointProtocolViolations lists the lines of text that ask or relay a
// checkpoint outside workflow_checkpoint.
func checkpointProtocolViolations(n checkpointNamer, text string) []string {
	var out []string
	lines := strings.Split(text, "\n")
	lastCP := -100
	for i, line := range lines {
		if m := questionFieldRe.FindStringSubmatch(line); m != nil && n.names(m[1]) {
			out = append(out, "question naming a checkpoint: "+strings.TrimSpace(line))
		}
		if questionToolRe.MatchString(line) && n.names(line) && !negationRe.MatchString(line) {
			out = append(out, "checkpoint through `question`: "+strings.TrimSpace(line))
		}
		if anyCheckpointRe.MatchString(line) {
			lastCP = i
		}
		if endSessionRe.MatchString(line) && i-lastCP <= 12 {
			out = append(out, "session ended at a checkpoint: "+strings.TrimSpace(line))
		}
	}
	return out
}

func TestShippedCheckpointProtocol(t *testing.T) {
	_, cat, _ := shippedWorkflows(t)
	for _, ref := range cat.Refs() {
		t.Run(ref.ID, func(t *testing.T) {
			r, b := shippedBuild(t, "", ref)
			if r.Spec.Checkpoints.Len() == 0 {
				return
			}
			n := newCheckpointNamer(r.Spec)
			var bad []string
			require.NoError(t, filepath.WalkDir(b.Dir, func(p string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() || filepath.Ext(p) != ".md" {
					return err
				}
				data, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				rel, _ := filepath.Rel(b.Dir, p)
				for _, v := range checkpointProtocolViolations(n, string(data)) {
					bad = append(bad, rel+": "+v)
				}
				return nil
			}))
			sort.Strings(bad)
			assert.Empty(t, bad, "content shipped with workflow %s", ref.ID)
		})
	}
}

func TestCheckpointProtocolLint(t *testing.T) {
	n := newCheckpointNamer(parseSpec(t, "apiVersion: oh/v1\nkind: Workflow\nid: x\nrisk: write\ncheckpoints:\n  cp-2: { label: Commit ou correction }\n  cp-feature: {}\n"))
	for _, bad := range []string{
		`    header: "CP-2 — Ticket #<ID>",`,
		`    question: "[OrchestratorDev — CP 2 | Ticket]\nCommit ?",`,
		`    question: "Commit ou correction du ticket ?",`,
		"Pose CP-2 via l'outil `question`.",
		"## CP-1\n\n→ **TERMINER LA SESSION**",
	} {
		assert.NotEmpty(t, checkpointProtocolViolations(n, bad), bad)
	}
	for _, ok := range []string{
		`    header: "Commit ?",`,
		"Le checkpoint passe par `workflow_checkpoint`, jamais par l'outil `question`.",
		"Pour toute confirmation, utilise l'outil `question`, sans nommer de checkpoint.",
		"→ **TERMINER LA SESSION**",
	} {
		assert.Empty(t, checkpointProtocolViolations(n, ok), ok)
	}
}

// The skills generated from the workflow replace the static files of the
// same reference: those are stubs, never edited by hand (N6).
func TestStaticGeneratedSkillsAreStubs(t *testing.T) {
	hub := repoHub(t)
	for ref := range generatedTemplates {
		p := filepath.Join(hub, "skills", ref+".md")
		data, err := os.ReadFile(p)
		if os.IsNotExist(err) {
			continue
		}
		require.NoError(t, err)
		body := string(data)
		assert.Contains(t, body, "GÉNÉRÉ", ref)
		assert.LessOrEqual(t, strings.Count(body, "\n"), 15, "%s: a stub, the content comes from bundle/templates", ref)
	}
}

// N6: every skill of the hub is reachable — listed by an agent (skills,
// native_skills), required by a reachable skill, generated, or cited by
// the content ([SKILL:…], `category/name`, « skill `name` »). The legacy
// skills of the deployment era (orchestrator-modes, orchestrator-ticket-
// routing) were orphans.
func TestNoOrphanSkills(t *testing.T) {
	hub := repoHub(t)
	index := hubSkillIndex(hub, nil)
	reached := map[string]bool{}
	var roots []string
	files, err := bricks.FindAgentFiles(hub)
	require.NoError(t, err)
	for _, f := range files {
		fm, err := bricks.ParseAgentFrontmatter(f)
		require.NoError(t, err)
		roots = append(roots, fm.Skills...)
		roots = append(roots, fm.NativeSkills...)
	}
	for _, dir := range []string{"agents", "skills", "workflows/prompts"} {
		require.NoError(t, filepath.WalkDir(filepath.Join(hub, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || (filepath.Ext(p) != ".md" && filepath.Ext(p) != ".tmpl") {
				return err
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			text := string(data)
			for _, m := range skillTagRe.FindAllStringSubmatch(text, -1) {
				roots = append(roots, m[1])
			}
			for _, m := range codeSpanRe.FindAllStringSubmatch(text, -1) {
				roots = append(roots, m[1]) // `category/name` or a bare skill name (mention index)
			}
			for _, m := range skillPhraseRe.FindAllStringSubmatch(text, -1) {
				for _, c := range codeSpanRe.FindAllStringSubmatch(m[1], -1) {
					roots = append(roots, c[1])
				}
			}
			return nil
		}))
	}
	var known []string
	for _, r := range roots {
		if ref, ok := index[r]; ok {
			known = append(known, ref)
		}
	}
	closure, _ := NewSkillCatalog(hub).Closure(known)
	for _, r := range closure {
		reached[r] = true
	}
	for ref := range generatedTemplates {
		reached[ref] = true
	}
	var orphans []string
	for key, ref := range index {
		// Stack skills are selected from the project.
		if key == ref && !reached[ref] && !strings.HasPrefix(ref, "developer/stacks/") {
			orphans = append(orphans, ref)
		}
	}
	sort.Strings(orphans)
	assert.Empty(t, orphans, "skills that no agent, skill or prompt reaches")
}
