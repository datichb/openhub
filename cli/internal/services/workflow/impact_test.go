package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wf "github.com/datichb/openhub/cli/internal/workflow"
)

type fakeAgents map[string]wf.AgentInfo

func (f fakeAgents) Agent(id string) (wf.AgentInfo, bool) { a, ok := f[id]; return a, ok }

func specOf(t *testing.T, yaml string) *wf.Spec {
	t.Helper()
	doc, diags := wf.Parse([]byte(yaml), wf.Source{Layer: wf.LayerTeam})
	require.False(t, diags.HasErrors(), "%v", diags)
	return doc.Spec
}

const impactBase = `apiVersion: oh/v1
kind: Workflow
id: x
risk: read
isolation: strict
entry: { agent: lead }
agents:
  lead: { role: workflow }
  reader: { role: workflow }
checkpoints:
  cp-1: { mandatory: true, remote: forbid, mode: { manuel: pause, semi-auto: pause } }
  cp-2: { mode: { manuel: pause } }
modes: { allowed: [manuel, semi-auto] }
beads: { allow: [show] }
runtime: { allowed: [local] }
limits: { budget_usd: 2 }
`

func TestImpact(t *testing.T) {
	agents := fakeAgents{"lead": {ID: "lead"}, "reader": {ID: "reader"}, "dev": {ID: "dev", Edits: true}, "doc": {ID: "doc"}}
	from := specOf(t, impactBase)
	to := specOf(t, `apiVersion: oh/v1
kind: Workflow
id: x
risk: write
isolation: standard
entry: { agent: lead }
agents:
  lead: { role: workflow }
  dev: { role: workflow }
  doc: { role: independent }
checkpoints:
  cp-1: { remote: defer, mode: { manuel: pause, semi-auto: auto } }
  cp-3: { mode: { manuel: pause } }
modes: { allowed: [manuel, semi-auto, auto] }
beads: { allow: [show, close] }
runtime: { allowed: [local, remote] }
limits: { budget_usd: 5 }
mcp: [gitlab]
code_mode: true
inputs:
  goal: { type: string }
`)
	r := impactOf(from, to, agents)
	byCode := map[string]ImpactLevel{}
	for _, it := range r.Items {
		byCode[it.Code] = it.Level
		assert.NotEmpty(t, it.Message, it.Code)
	}
	for code, level := range map[string]ImpactLevel{
		"risk_raised": ImpactWiden, "isolation_loosened": ImpactWiden, "agent_added_writes": ImpactWiden,
		"agent_added": ImpactInfo, "agent_removed": ImpactInfo, "remote_allowed": ImpactWiden,
		"mode_added": ImpactWiden, "checkpoint_not_mandatory": ImpactWiden, "checkpoint_remote_loosened": ImpactWiden,
		"checkpoint_loosened": ImpactWiden, "checkpoint_removed": ImpactWiden, "checkpoint_added": ImpactInfo,
		"beads_widened": ImpactWiden, "budget_raised": ImpactWiden, "mcp_added": ImpactWiden,
		"code_mode_enabled": ImpactWiden, "inputs_added": ImpactInfo,
	} {
		assert.Equal(t, level, byCode[code], code)
	}
	assert.False(t, r.New)

	// The reverse change only hardens, except that it drops cp-3.
	back := impactOf(to, from, agents)
	for _, it := range back.Widenings() {
		assert.Equal(t, "checkpoints.cp-3", it.Path, it.Code)
	}
	assert.Empty(t, impactOf(from, from, agents).Items)

	n := impactOf(nil, to, agents)
	assert.True(t, n.New)
	assert.Empty(t, n.Widenings())
}
