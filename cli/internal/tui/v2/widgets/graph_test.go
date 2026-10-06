package widgets

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/workflow"
)

var updateGolden = flag.Bool("update", false, "update golden files")

const graphSpec = `apiVersion: oh/v1
kind: Workflow
id: ticket-hotfix
entry: { agent: orchestrator-dev }
agents:
  orchestrator-dev: { role: workflow }
  developer: { role: workflow, after: cp-1 }
  reviewer: { role: workflow, after: developer }
  documentarian: { role: independent }
  ghost: { role: disabled }
checkpoints:
  cp-1: { label: Démarrer, mode: { manuel: pause, semi-auto: auto } }
  cp-2: { label: Commit ou correction, mandatory: true, remote: defer }
  cp-off: { disabled: true }
modes: { default: semi-auto, allowed: [manuel, semi-auto] }
`

func graphSpecOf(t *testing.T) *workflow.Spec {
	t.Helper()
	doc, diags := workflow.Parse([]byte(graphSpec), workflow.Source{Layer: workflow.LayerTeam})
	require.False(t, diags.HasErrors(), "%v", diags)
	return doc.Spec
}

func TestSpecGraphModel(t *testing.T) {
	m := SpecGraphModel(graphSpecOf(t), "semi-auto", "fr", "▶ Start", func(p string) string {
		if p == "agents.documentarian" {
			return "↳ hub:ticket"
		}
		return ""
	})
	require.Len(t, m.Checkpoints, 2, "disabled checkpoint hidden")
	assert.Equal(t, "auto", m.Checkpoints[0].Behavior)
	assert.Equal(t, "pause", m.Checkpoints[1].Behavior, "absent behavior = pause")
	assert.Equal(t, "remote: defer", m.Checkpoints[1].Extra)
	assert.True(t, m.Checkpoints[1].Locked)
	place := map[string]string{}
	for _, a := range m.Agents {
		place[a.ID] = a.After
	}
	assert.Equal(t, map[string]string{"orchestrator-dev": "", "developer": "cp-1", "reviewer": "cp-1", "documentarian": ""}, place,
		"reviewer follows developer: placed under cp-1; disabled agent hidden")

	l := ComputeModelLayout(m)
	assert.Equal(t, ElementStart, l.Elements[0].Type)
	for _, e := range l.Elements {
		assert.NotEqual(t, "▶→cp-1", e.ID, "no insertion before the start column")
	}
}

// Golden rendering of the editor graph (P2-T14).
func TestSpecGraphGolden(t *testing.T) {
	g := NewModelGraph(SpecGraphModel(graphSpecOf(t), "semi-auto", "fr", "▶ Start", nil), false)
	sc := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, sc.Init())
	sc.SetSize(100, 24)
	g.SetRect(0, 0, 100, 24)
	g.Draw(sc)
	sc.Show()
	cells, w, h := sc.GetContents()
	var b strings.Builder
	for y := range h {
		var line strings.Builder
		for x := range w {
			c := cells[y*w+x]
			if len(c.Runes) == 0 {
				line.WriteRune(' ')
			} else {
				line.WriteRune(c.Runes[0])
			}
		}
		b.WriteString(strings.TrimRight(line.String(), " ") + "\n")
	}
	got := b.String()
	path := filepath.Join("testdata", "editor_graph.golden")
	if *updateGolden {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden file (go test ./internal/tui/v2/widgets -run Golden -update)")
	assert.Equal(t, string(want), got)
}
