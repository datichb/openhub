package bundle

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// r5-fixes: in spec, the planner writes its spec only once cp-understanding
// is passed, and oh refuses cp-converge, cp-spec and cp-tickets before the
// checkpoint each one waits for.
func TestShippedSpecOrder(t *testing.T) {
	wf := shippedBundle(t, "spec").Spec.Workflow
	require.NotNil(t, wf)
	assert.Equal(t, []sessionspec.WritesGate{{Agent: "planner", Paths: []string{"docs/wiki/specs/**", "docs/wiki/index.md"}, After: "cp-understanding"}}, wf.WritesGates)
	after := map[string]string{}
	for _, c := range wf.Checkpoints {
		after[c.ID] = c.After
	}
	assert.Equal(t, map[string]string{"cp-understanding": "", "cp-converge": "cp-understanding", "cp-spec": "cp-converge", "cp-tickets": "cp-spec"}, after)

	for _, mode := range []string{"manuel", "semi-auto", "auto"} {
		rules := SessionRules(wf, mode, nil)
		assert.Equal(t, sessionspec.EffectAsk, ToolEffect(rules, sessionspec.ActionEdit, "docs/wiki/specs/x.md"), mode)
		passed := &domain.CheckpointState{Passed: map[string]time.Time{"cp-understanding": time.Now()}}
		for _, r := range SessionRules(wf, mode, passed) {
			assert.NotEqual(t, sessionspec.ActionEdit, r.Action, "%s: %v", mode, r)
		}
	}
	cp, _ := wf.Checkpoint("cp-converge")
	assert.Equal(t, "cp-understanding", CheckpointWaitsFor(wf, "auto", nil, cp), "auto: cp-understanding is still called")
}

// r5-fixes (S37): orchestrator-dev reads its tickets with `bd show`, even
// after `cd <project> &&` (cd is not checked as a command); a line that
// chains a command it may not run (echo, which, a pipe to head) is refused
// as a whole, which its protocol says.
func TestOrchestratorDevReadsTicketsAlone(t *testing.T) {
	for _, wf := range []string{"ticket", "implement-spec", "libre"} {
		b := shippedBundle(t, wf)
		rules := append(agentRules(b, "orchestrator-dev", sessionspec.ActionShell), b.Spec.AgentGuard...)
		require.NotEmpty(t, rules, wf)
		for _, cmd := range []string{"bd show ev-1", "bd show ev-1 ev-2", "cd /tmp/p/project && bd show ev-1", "bd show ev-1 2>&1"} {
			assert.Equal(t, sessionspec.EffectAllow, ShellEffect(rules, cmd), "%s: %s", wf, cmd)
		}
		for _, cmd := range []string{
			`cd /tmp/p/project && bd show ev-1 && echo "---" && bd show ev-2`,
			"cd /tmp/p/project && which bd && bd show ev-1 2>&1",
			"bd show ev-1 | head -30",
		} {
			assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rules, cmd), "%s: %s", wf, cmd)
		}
		for _, a := range b.Spec.Agents {
			if a.ID == "orchestrator-dev" {
				assert.True(t, strings.Contains(a.Body, "`bd show <ID> <ID>…` (tous les tickets en une commande, seule sur sa ligne"), wf)
			}
		}
	}
}
