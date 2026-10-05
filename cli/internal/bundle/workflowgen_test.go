package bundle

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/workflow"
)

var updateGolden = flag.Bool("update", false, "update golden files")

const genSpec = `apiVersion: oh/v1
kind: Workflow
id: ticket
description: { fr: Implémenter un ticket Beads, en: Implement a Beads ticket }
risk: write
entry: { agent: orchestrator-dev }
agents:
  orchestrator-dev: { role: workflow, calls: [developer, reviewer] }
  developer: { role: workflow, after: cp-1, calls: [] }
  reviewer: { role: workflow, after: developer, mode: subagent }
  documentarian: { role: independent }
checkpoints:
  cp-1:
    label: { fr: Démarrer, en: Start }
    mode: { manuel: pause, semi-auto: auto, auto: auto }
    remote: auto
  cp-2:
    label: Commit ou correction
    description: Montrer le diff avant de committer.
    mandatory: true
    mode: { manuel: pause, semi-auto: pause, auto: skip }
  cp-3:
    label: Revue
    condition: le diff dépasse 300 lignes
    mode: { manuel: pause, semi-auto: conditional, auto: auto }
modes: { default: semi-auto, allowed: [manuel, semi-auto, auto] }
circuit_breaker: { max_consecutive_subagents: 12 }
beads: { allow: [show, update, close] }
runtime: { default: local, allowed: [local, remote] }
outputs:
  - { id: branch, type: branch, label: Branche de travail }
  - { id: mr, type: merge_request }
`

func generateFor(t *testing.T, src string) map[string]string {
	t.Helper()
	spec := parseSpec(t, src)
	_, graph := workflow.SubagentGraph(spec, nil)
	modes := map[string]workflow.AgentMode{"developer": workflow.ModeSubagent}
	gen, err := GenerateWorkflowSkills(spec, graph, func(id string) workflow.AgentMode { return modes[id] })
	require.NoError(t, err)
	return gen
}

func TestGenerateWorkflowSkills_Golden(t *testing.T) {
	gen := generateFor(t, genSpec)
	require.Len(t, gen, len(generatedTemplates))
	for ref, content := range gen {
		golden := filepath.Join("testdata", "golden", strings.ReplaceAll(ref, "/", "__")+".md")
		if *updateGolden {
			require.NoError(t, os.MkdirAll(filepath.Dir(golden), 0o755))
			require.NoError(t, os.WriteFile(golden, []byte(content), 0o644))
			continue
		}
		want, err := os.ReadFile(golden)
		require.NoError(t, err, "missing golden file (go test ./internal/bundle -run Golden -update)")
		assert.Equal(t, string(want), content, ref)
	}
}

func TestGenerateWorkflowSkills_Content(t *testing.T) {
	m := generateFor(t, genSpec)[WorkflowMapSkill]
	assert.True(t, strings.HasPrefix(m, "---\nname: workflow-map\n"), "frontmatter name = skill id")
	assert.Contains(t, m, "Implémenter un ticket Beads")
	assert.Contains(t, m, "| `orchestrator-dev` | workflow (entrée) | primary | — | `developer`, `reviewer` |")
	assert.Contains(t, m, "| `developer` | workflow | subagent | `cp-1` | — |", "explicit empty calls: no delegation")
	assert.Contains(t, m, "| `reviewer` | workflow | subagent | `developer` | — |")
	assert.Contains(t, m, "| `documentarian` | independent | primary | — | — |")
	assert.Contains(t, m, "| **cp-1** — Démarrer | pause | auto | auto | auto |")
	assert.Contains(t, m, "| **cp-2** — Commit ou correction (obligatoire) | pause | pause | pause | defer |", "mandatory: never skipped")
	assert.Contains(t, m, "(conditional) : le diff dépasse 300 lignes")
	assert.Contains(t, m, "après 12 lancements `task` consécutifs")
	assert.Contains(t, m, "`branch` (branch) : Branche de travail")
	assert.Contains(t, m, "Commandes Beads autorisées : `show`, `update`, `close`")
	assert.Less(t, strings.Index(m, "cp-1"), strings.Index(m, "cp-2"), "declaration order kept")

	routing := generateFor(t, genSpec)[ticketRoutingSkill]
	assert.NotContains(t, routing, "pathfinder", "agents absent from the workflow are not mentioned")

	// Minimal workflow: implicit conductor, no checkpoint, no remote column.
	quick := generateFor(t, "apiVersion: oh/v1\nkind: Workflow\nid: quick\nrisk: read\n")[WorkflowMapSkill]
	assert.Contains(t, quick, "Agent d'entrée : `conductor`")
	assert.Contains(t, quick, "Aucun checkpoint")
	assert.Contains(t, quick, "aucune modification de fichier")
	assert.NotContains(t, quick, "distant")
	assert.NotContains(t, quick, "Sorties à déclarer")
}

func TestBuildFromWorkflowSpec_InlinesGeneratedSkills(t *testing.T) {
	b, err := Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), Spec: parseSpec(t, genSpec)})
	require.NoError(t, err)
	od := findAgent(b.Spec.Agents, "orchestrator-dev")
	require.NotNil(t, od)
	assert.Contains(t, od.Body, "| **cp-2** — Commit ou correction (obligatoire)", "workflow modes generated from the YAML are inlined")
	assert.Contains(t, od.Body, "- Modes autorisés : `manuel`, `semi-auto`, `auto`")
	assert.NotContains(t, od.Body, "Protocole de sélection du mode (CP-0)", "not the skill generated from the legacy workflow")
}
