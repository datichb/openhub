package bundle

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bricks"
)

// Modernisation L5: a subagent returns a short envelope (shared/handoff-envelope)
// and files its whole report with workflow_report; the statuses and the report
// sections of each agent are in its payload skill <agent>-payload; the title
// of the return comes from the session contract (the real caller), never from
// an agent name written in a skill.

// envelopeTitle opens the envelope skill in a composed prompt (French).
const envelopeTitle = "# Enveloppe de retour" //nolint:misspell // French title

const (
	envelopeMaxLines = 15
	payloadMinTokens = 200
	payloadMaxTokens = 400
)

// returnHeadingRe matches a return heading naming an agent: `## Retour vers x`.
var returnHeadingRe = regexp.MustCompile("Retour vers ([a-z][a-z0-9-]*)")

// contractCallerRe reads the callers from the « Retour » line of a contract.
var contractCallerRe = regexp.MustCompile("`## Retour vers ([a-z][a-z0-9-]*)`")

func TestHandoffEnvelopeFormat(t *testing.T) {
	d, err := newSkillLoader(repoHub(t), nil).load("shared/handoff-envelope")
	require.NoError(t, err)
	assert.Equal(t, "role=subagent", d.Front.When, "the envelope is for subagents only")
	body := string(d.body())
	start := strings.Index(body, "```\n")
	require.GreaterOrEqual(t, start, 0, "the envelope template is a code block")
	block := body[start+4:]
	end := strings.Index(block, "```")
	require.GreaterOrEqual(t, end, 0)
	lines := strings.Split(strings.TrimRight(block[:end], "\n"), "\n")
	assert.LessOrEqual(t, len(lines), envelopeMaxLines, "envelope of %d lines", len(lines))
	assert.Equal(t, "## Retour vers <appelant>", lines[0], "the title names the caller given by the contract")
	tmpl := strings.Join(lines, "\n")
	for _, field := range []string{"**Agent**", "**Statut**", "**Rapport** : rp-", "### Questions bloquantes", "### Suite proposée", "### Résumé", "task_id"} {
		assert.Contains(t, tmpl, field, "envelope field %s", field)
	}
	for _, rule := range []string{"`workflow_report`", "**avant** l'enveloppe", "non enregistré", "### Rapport complet", "`en-attente`"} {
		assert.Contains(t, body, rule, "envelope rule %s", rule)
	}
}

// payloadAgents lists the agents of the hub that have a payload skill.
func payloadAgents(t *testing.T, hub string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for id, ref := range hubSkillIndex(hub, nil) {
		if agent, ok := strings.CutSuffix(id, payloadSuffix); ok && !strings.Contains(id, "/") {
			out[agent] = ref
		}
	}
	return out
}

func TestPayloadSkills(t *testing.T) {
	hub := repoHub(t)
	files, err := bricks.FindAgentFiles(hub)
	require.NoError(t, err)
	loader := newSkillLoader(hub, nil)
	payloads := payloadAgents(t, hub)
	require.NotEmpty(t, payloads)
	for agent, ref := range payloads {
		t.Run(agent, func(t *testing.T) {
			require.Contains(t, files, agent, "%s: no agent %s", ref, agent)
			d, err := loader.load(ref)
			require.NoError(t, err)
			body := string(d.body())
			tokens := utf8.RuneCountInString(body) / 4
			assert.GreaterOrEqual(t, tokens, payloadMinTokens, "%s: %d tokens", ref, tokens)
			assert.LessOrEqual(t, tokens, payloadMaxTokens, "%s: %d tokens", ref, tokens)
			assert.Equal(t, "role=subagent", d.Front.When, "%s: the payload goes with the envelope", ref)
			require.NotEmpty(t, d.Front.Statuses, "%s: statuses: missing", ref)
			for _, s := range d.Front.Statuses {
				assert.Contains(t, body, "| `"+s+"` |", "%s: status %s has no row", ref, s)
				assert.NotEqual(t, "en-attente", s, "%s: en-attente is the envelope's own status", ref)
			}
			assert.Contains(t, body, "## Rapport", "%s: report sections", ref)
			assert.NotRegexp(t, returnHeadingRe, body, "%s: the return title comes from the contract", ref)
			fm, err := bricks.ParseAgentFrontmatter(files[agent])
			require.NoError(t, err)
			assert.Contains(t, fm.Skills, ref, "%s does not inline %s", agent, ref)
			assert.Contains(t, fm.Skills, "shared/handoff-envelope", "%s does not inline the envelope", agent)
		})
	}
}

// Every agent a shipped workflow launches has a payload, and its prompt
// names as return title only its callers; a caller reads the statuses of
// the agents it launches in its contract.
func TestReturnTitlesFromContract(t *testing.T) {
	hub, cat, _ := shippedWorkflows(t)
	payloads := payloadAgents(t, hub)
	for _, ref := range cat.Refs() {
		t.Run(ref.ID, func(t *testing.T) {
			b := shippedBundle(t, ref.ID)
			ids := map[string]bool{}
			for _, a := range b.Spec.Agents {
				ids[a.ID] = true
			}
			for _, a := range b.Spec.Agents {
				callers := map[string]bool{}
				for _, m := range contractCallerRe.FindAllStringSubmatch(contractOf(a.Body), -1) {
					callers[m[1]] = true
				}
				if len(callers) > 0 {
					assert.Contains(t, payloads, a.ID, "%s/%s is launched by an agent but has no payload", ref.ID, a.ID)
					assert.Contains(t, a.Body, envelopeTitle, "%s/%s: no envelope", ref.ID, a.ID)
				} else {
					assert.NotContains(t, a.Body, envelopeTitle, "%s/%s: envelope for an agent no one launches", ref.ID, a.ID)
				}
				for _, m := range returnHeadingRe.FindAllStringSubmatch(a.Body, -1) {
					if !ids[m[1]] && m[1] != "appelant" {
						continue // not an agent id (« Retour vers le … »)
					}
					assert.True(t, callers[m[1]] || m[1] == a.ID, "%s/%s names « Retour vers %s », not one of its callers", ref.ID, a.ID, m[1])
				}
			}
		})
	}
}

// Skills never name a caller in a return title: the contract does.
func TestSkillsNameNoReturnRecipient(t *testing.T) {
	hub := repoHub(t)
	files, err := bricks.FindAgentFiles(hub)
	require.NoError(t, err)
	root := filepath.Join(hub, "skills")
	require.NoError(t, filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(hub, p)
		for i, line := range strings.Split(string(data), "\n") {
			for _, m := range returnHeadingRe.FindAllStringSubmatch(line, -1) {
				if _, isAgent := files[m[1]]; isAgent {
					t.Errorf("%s:%d: « Retour vers %s » written in a skill: the title comes from the session contract", rel, i+1, m[1])
				}
			}
		}
		return nil
	}))
	removed := []string{"retranscription-coordinateur", "handoff-bloc-unique-rule", "phase-0-validation-loop", "standalone-execution-protocol", "subagent-execution-protocol"}
	index := hubSkillIndex(hub, nil)
	for id := range index {
		assert.False(t, strings.HasSuffix(id, "-handoff-format") || strings.HasSuffix(id, "-execution-modes") || slices.Contains(removed, id),
			"%s: replaced by the envelope, the payloads and execution-protocol (L5)", id)
	}
}
