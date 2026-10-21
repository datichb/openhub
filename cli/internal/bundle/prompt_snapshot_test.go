package bundle

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/testutil/golden"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Composed prompt snapshots (modernisation P0.3): the system prompt of every
// agent of the main workflows, built without project, is pinned in
// testdata/golden/prompts/<workflow>/<agent>.md.golden, so that a content change is
// reviewed as a diff of what the model reads
// (go test ./internal/bundle -run ShippedWorkflowPromptSnapshots -update).

var snapshotWorkflows = []string{"audit", "implement-spec", "quick", "review", "spec", "ticket"}

func TestShippedWorkflowPromptSnapshots(t *testing.T) {
	for _, id := range snapshotWorkflows {
		t.Run(id, func(t *testing.T) {
			_, b := shippedBuild(t, "", workflow.Ref{Layer: workflow.LayerHub, ID: id})
			require.NotEmpty(t, b.Spec.Agents)

			dir := filepath.Join("testdata", "golden", "prompts", id)
			want := map[string]bool{}
			for _, a := range b.Spec.Agents {
				name := a.ID + ".md.golden"
				want[name] = true
				assert.NotContains(t, a.Body, b.Dir, "%s/%s: the snapshot depends on the bundle directory", id, a.ID)
				golden.Assert(t, path.Join("golden", "prompts", id, a.ID+".md"), a.Body)
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
			if golden.Updating() {
				for _, name := range stale {
					require.NoError(t, os.Remove(filepath.Join(dir, name)))
				}
				return
			}
			assert.Empty(t, stale, "%s: snapshots of agents no longer in the workflow (-update removes them): %s", id, strings.Join(stale, ", "))
		})
	}
}
