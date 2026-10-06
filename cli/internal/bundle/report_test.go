package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShow(t *testing.T) {
	b, err := Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), Spec: parseSpec(t, ticketSpec), Provider: "bedrock"})
	require.NoError(t, err)
	r := Show(b)

	assert.Equal(t, b.Spec.Hash, r.Hash)
	require.NotEmpty(t, r.Agents)
	assert.Equal(t, "orchestrator-dev", r.Agents[0].ID, "entry agent first")
	assert.True(t, r.Agents[0].Entry)
	assert.Equal(t, []string{"developer", "reviewer"}, r.Agents[0].Calls)
	assert.Positive(t, r.Budget.EntryAgent)
	assert.GreaterOrEqual(t, r.Budget.Agents, r.Budget.EntryAgent)
	assert.Equal(t, r.Budget.EntryAgent+r.Budget.SkillCatalog, r.Budget.Initial)
	if len(r.Skills) > 0 {
		assert.Positive(t, r.Budget.Skills)
		assert.Positive(t, r.Budget.SkillCatalog)
	}
	assert.NotNil(t, r.MCP)
	assert.NotEmpty(t, r.DefaultModel)
}

func TestEstimateTokens(t *testing.T) {
	assert.Equal(t, 0, EstimateTokens(""))
	assert.Equal(t, 1, EstimateTokens("abcd"))
	assert.Equal(t, 2, EstimateTokens("ééééé"))
}
