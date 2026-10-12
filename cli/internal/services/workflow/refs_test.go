package workflow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wf "github.com/datichb/openhub/cli/internal/workflow"
)

func TestRefs(t *testing.T) {
	refs, err := testService(t).Refs(context.Background(), Context{})
	require.NoError(t, err)
	var names []string
	for _, r := range refs {
		names = append(names, r.String())
	}
	assert.Equal(t, []string{"hub:feature", "hub:quick", "hub:review", "hub:ticket"}, names)
}

func TestExtendsCandidates(t *testing.T) {
	refs := []wf.Ref{{Layer: wf.LayerHub, ID: "ticket"}, {Layer: wf.LayerTeam, ID: "ticket"}, {Layer: wf.LayerTeam, ID: "hotfix"},
		{Layer: wf.LayerProject, ID: "ticket"}}
	str := func(l []wf.Ref) []string {
		var out []string
		for _, r := range l {
			out = append(out, r.String())
		}
		return out
	}
	assert.Equal(t, []string{"hub:ticket", "team:ticket"}, str(ExtendsCandidates(refs, wf.LayerTeam, "team:hotfix")),
		"same or less specific layer, self excluded")
	assert.Equal(t, []string{"hub:ticket", "team:ticket", "team:hotfix"}, str(ExtendsCandidates(refs, wf.LayerProject, "project:ticket")))
	assert.Equal(t, []string{"hub:ticket"}, str(ExtendsCandidates(refs, wf.LayerHub, "")))
}

func TestResolutionDelegation(t *testing.T) {
	res, err := testService(t).Resolve(context.Background(), Context{}, "ticket", ResolveOpts{})
	require.NoError(t, err)
	g := res.Delegation()
	assert.Equal(t, []string{"developer", "reviewer"}, g["orchestrator-dev"], "explicit calls")
}
