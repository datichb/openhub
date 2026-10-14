package bundle

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/workflow/hubcat"
)

// B1: spec runs without a request (oh run feature, oh start): the conductor
// asks for it first with question; with a request or a source page (spike,
// incident next:), it does not ask.
func TestShippedSpecAsksForAMissingRequest(t *testing.T) {
	hub, cat, _ := shippedWorkflows(t)
	hc, err := hubcat.New(hub)
	require.NoError(t, err)
	ask := "Aucune demande donnée au lancement"
	for _, c := range []struct {
		name   string
		inputs map[string]any
		asks   bool
	}{
		{"none", map[string]any{}, true},
		{"request", map[string]any{"request": "Exporter les factures en CSV"}, false},
		{"source", map[string]any{"source": "docs/wiki/decisions/cache.md"}, false},
	} {
		r, diags := workflow.ResolveSpec(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "spec"}, &workflow.SessionOptions{Inputs: c.inputs})
		require.False(t, diags.HasErrors(), "%s: %v", c.name, diags)
		out, err := workflow.RenderPrompt(r, hc, workflow.PromptContext{Project: "demo", Location: "/src/demo", Mode: "semi-auto", Lang: "fr", Workflow: "spec"})
		require.NoError(t, err)
		assert.Equal(t, c.asks, strings.Contains(out, ask), c.name)
		if c.asks {
			assert.Contains(t, out, "l'outil `question`", c.name)
		}
		if req, ok := c.inputs["request"].(string); ok {
			assert.Contains(t, out, req)
		}
		if src, ok := c.inputs["source"].(string); ok {
			assert.Contains(t, out, src)
		}
	}
}
