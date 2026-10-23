package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// opencode refuses to launch a `primary` agent through task ("Agent X cannot
// run as a subagent"): a primary agent that a workflow agent may call (itself
// included) is shipped with mode `all`; the other agents keep their mode.

func modesOf(t *testing.T, src string) map[string]string {
	t.Helper()
	b, err := Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), Spec: parseSpec(t, src), Provider: "bedrock"})
	require.NoError(t, err)
	out := map[string]string{}
	for _, a := range b.Spec.Agents {
		out[a.ID] = a.Mode
	}
	return out
}

// Bug 14: the reviewer launches reviewer sessions (standard+adversarial, all).
func TestPrimaryAgentCallingItselfIsAll(t *testing.T) {
	t.Parallel()
	modes := modesOf(t, `apiVersion: oh/v1
kind: Workflow
id: review
risk: read
entry: { agent: reviewer }
agents:
  reviewer: { role: workflow, calls: [reviewer] }
`)
	assert.Equal(t, map[string]string{"reviewer": "all"}, modes)
}

func TestPrimaryAgentCalledByAnotherIsAll(t *testing.T) {
	t.Parallel()
	modes := modesOf(t, `apiVersion: oh/v1
kind: Workflow
id: cross
risk: read
entry: { agent: orchestrator }
agents:
  orchestrator: { role: workflow, calls: [planner] }
  planner: { role: workflow }
`)
	assert.Equal(t, "primary", modes["orchestrator"], "the entry nobody calls stays primary")
	assert.Equal(t, "all", modes["planner"])
}

func TestSubagentCalledStaysSubagent(t *testing.T) {
	t.Parallel()
	modes := modesOf(t, `apiVersion: oh/v1
kind: Workflow
id: pure
risk: write
entry: { agent: orchestrator-dev }
agents:
  orchestrator-dev: { role: workflow, calls: [developer, reviewer] }
  developer: { role: workflow }
  reviewer: { role: workflow, mode: subagent }
`)
	assert.Equal(t, "primary", modes["orchestrator-dev"])
	assert.Equal(t, "subagent", modes["developer"], "frontmatter subagent")
	assert.Equal(t, "subagent", modes["reviewer"], "workflow subagent")
}

func TestPrimaryAgentNotCalledStaysPrimary(t *testing.T) {
	t.Parallel()
	modes := modesOf(t, `apiVersion: oh/v1
kind: Workflow
id: alone
risk: read
entry: { agent: reviewer }
agents:
  reviewer: { role: workflow, calls: [] }
  documentarian: { role: independent }
`)
	assert.Equal(t, "primary", modes["reviewer"])
	assert.Equal(t, "primary", modes["documentarian"])
}

func TestModeAllWrittenByHandIsKept(t *testing.T) {
	t.Parallel()
	modes := modesOf(t, `apiVersion: oh/v1
kind: Workflow
id: hand
risk: read
entry: { agent: reviewer }
agents:
  reviewer: { role: workflow, calls: [] }
  documentarian: { role: independent, mode: all }
`)
	assert.Equal(t, "primary", modes["reviewer"])
	assert.Equal(t, "all", modes["documentarian"])
}

// What the workflow writes wins: `mode: primary` on a called agent stays
// (oh workflow validate reports it).
func TestExplicitPrimaryOnCalledAgentIsKept(t *testing.T) {
	t.Parallel()
	modes := modesOf(t, `apiVersion: oh/v1
kind: Workflow
id: explicit
risk: read
entry: { agent: orchestrator }
agents:
  orchestrator: { role: workflow, calls: [planner] }
  planner: { role: workflow, mode: primary }
`)
	assert.Equal(t, "primary", modes["planner"])
}

// A delegation derived from the task permission of the agent counts as well:
// it is rendered in the bundle as an allowed subagent.
func TestPrimaryAgentDerivedFromTaskPermissionIsAll(t *testing.T) {
	t.Parallel()
	modes := modesOf(t, ticketSpec)
	assert.Equal(t, "all", modes["designer"], "pathfinder (independent) may launch the designer")
}

// The workflow map says the mode the bundle ships.
func TestWorkflowMapShowsAll(t *testing.T) {
	t.Parallel()
	spec := parseSpec(t, `apiVersion: oh/v1
kind: Workflow
id: review
risk: read
entry: { agent: reviewer }
agents:
  reviewer: { role: workflow, calls: [reviewer] }
`)
	gen, err := specSkills(repoHub(t), spec)
	require.NoError(t, err)
	assert.Contains(t, gen[WorkflowMapSkill], "| `reviewer` | workflow (entrée) | all | — | `reviewer` |")
}
