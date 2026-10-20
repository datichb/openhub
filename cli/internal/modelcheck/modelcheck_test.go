package modelcheck

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func catalog() Catalog {
	return NewCatalog(map[string]string{"reviewer": "quality", "debugger": "quality", "planner": "planning", "developer": "developer"})
}

func TestCheckAgent(t *testing.T) {
	require.NoError(t, catalog().CheckAgent("reviewer"))
	err := catalog().CheckAgent("reviwer")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"reviwer"`)
	assert.Contains(t, err.Error(), "reviewer")
	err = catalog().CheckAgent("zzz")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "Did you mean", "nothing close: no suggestion")
}

func TestCheckFamily(t *testing.T) {
	require.NoError(t, catalog().CheckFamily("quality"))
	err := catalog().CheckFamily("qualty")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quality")
	assert.Equal(t, []string{"developer", "planning", "quality"}, catalog().Families())
	assert.Error(t, catalog().CheckFamily("reviewer"), "an agent is not a family")
}

func TestCheckModel(t *testing.T) {
	for _, ok := range []string{"anthropic/claude-sonnet-4-5", "amazon-bedrock/eu.anthropic.claude-sonnet-4-5", "openrouter/anthropic/claude-opus-4#high"} {
		assert.NoError(t, CheckModel(ok), ok)
	}
	for _, bad := range []string{"", "claude-sonnet-4-5", "anthropic/", "/claude", "a b/c", "anthropic/claude#"} {
		assert.Error(t, CheckModel(bad), bad)
	}
}

func TestNearOrdersByDistance(t *testing.T) {
	got := Near("dev", []string{"developer", "debugger", "reviewer", "planner"})
	assert.Equal(t, "developer", got[0])
	assert.LessOrEqual(t, len(got), maxSuggestions)
	assert.Empty(t, Near("", nil))
}

func TestEmptyCatalog(t *testing.T) {
	assert.True(t, Catalog{}.Empty())
	assert.False(t, catalog().Empty())
}
