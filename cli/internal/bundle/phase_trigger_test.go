package bundle

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bricks"
)

// Modernisation L6: a phase skill an agent loads on demand (ID with
// -phase-<n>) is named in its composed prompt (inviting text: body and
// inlined core) on a line that gives the moment to load it, a phase number
// (« | Phase 3 | … | `auditor-phase-3` | », « charge `x-phase-2-3` avant la
// phase 2 »). Without it the agent never knows when to load the phase and
// skips it (S6).

// phaseSkillRe matches the ID of a phase skill.
var phaseSkillRe = regexp.MustCompile(`-phase-\d`)

// phaseMomentRe matches a phase number on a line.
var phaseMomentRe = regexp.MustCompile(`(?i)\bphases?\s+\d`)

func TestPhaseSkillsHaveTrigger(t *testing.T) {
	hub, cat, _ := shippedWorkflows(t)
	files, err := bricks.FindAgentFiles(hub)
	require.NoError(t, err)
	agentIDs := map[string]bool{}
	for id := range files {
		agentIDs[id] = true
	}
	for _, ref := range cat.Refs() {
		b := shippedBundle(t, ref.ID)
		for _, a := range b.Spec.Agents {
			text := invitingText(a.Body, a.ID, agentIDs)
			for _, id := range a.Skills {
				if !phaseSkillRe.MatchString(id) {
					continue
				}
				assert.True(t, hasPhaseTrigger(text, id),
					"%s/%s: phase skill %s is not named in the prompt with the phase to load it at (« | Phase N | … | `%s` | »)", ref.ID, a.ID, id, id)
			}
		}
	}
}

// hasPhaseTrigger reports whether a line of text names the skill id in a
// code span (« `id` » or « `cat/id` ») and, besides the name, a phase number.
func hasPhaseTrigger(text, id string) bool {
	span := regexp.MustCompile("`(?:[a-z0-9_-]+/)?" + regexp.QuoteMeta(id) + "`")
	for _, ln := range strings.Split(text, "\n") {
		if span.MatchString(ln) && phaseMomentRe.MatchString(span.ReplaceAllString(ln, "")) {
			return true
		}
	}
	return false
}

func TestHasPhaseTrigger(t *testing.T) {
	assert.True(t, hasPhaseTrigger("| Phase 3 | Délégation | `auditor-phase-3` |", "auditor-phase-3"))
	assert.True(t, hasPhaseTrigger("charge `quality/debugger-phase-2-3` avant la phase 2", "debugger-phase-2-3"))
	assert.False(t, hasPhaseTrigger("voir `auditor-phase-3`", "auditor-phase-3"), "no moment")
	assert.False(t, hasPhaseTrigger("Phase 3 : auditor-phase-3", "auditor-phase-3"), "not named in a code span")
	assert.False(t, hasPhaseTrigger("Phase 3\n`auditor-phase-3`", "auditor-phase-3"), "moment on another line")
}
