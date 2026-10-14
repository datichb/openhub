package bundle

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/workflow"
)

// Composed prompt snapshots (modernisation P0.3): the system prompt of every
// agent of the main workflows, built without project, is pinned in
// testdata/golden/prompts/<workflow>/<agent>.md, so that a content change is
// reviewed as a diff of what the model reads
// (go test ./internal/bundle -run ShippedWorkflowPromptSnapshots -update).

var snapshotWorkflows = []string{"audit", "implement-spec", "quick", "review", "spec", "ticket"}

func TestShippedWorkflowPromptSnapshots(t *testing.T) {
	hub, cat, env := shippedWorkflows(t)
	for _, id := range snapshotWorkflows {
		t.Run(id, func(t *testing.T) {
			r, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: id}, nil, env)
			require.False(t, diags.HasErrors(), "%v", diags)
			b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec})
			require.NoError(t, err)
			require.NotEmpty(t, b.Spec.Agents)

			dir := filepath.Join("testdata", "golden", "prompts", id)
			want := map[string]bool{}
			for _, a := range b.Spec.Agents {
				name := a.ID + ".md"
				want[name] = true
				assert.NotContains(t, a.Body, b.Dir, "%s/%s: the snapshot depends on the bundle directory", id, a.ID)
				assertGolden(t, filepath.Join("prompts", id, name), a.Body)
			}
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			var stale []string
			for _, e := range entries {
				if !want[e.Name()] {
					stale = append(stale, e.Name())
				}
			}
			sort.Strings(stale)
			if *updateGolden {
				for _, name := range stale {
					require.NoError(t, os.Remove(filepath.Join(dir, name)))
				}
				return
			}
			assert.Empty(t, stale, "%s: snapshots of agents no longer in the workflow (-update removes them): %s", id, strings.Join(stale, ", "))
		})
	}
}
