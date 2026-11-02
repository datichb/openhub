package modelcheck

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bricks"
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
	// The rule of the bundle and of the frontmatter: a short name, or
	// provider/model, with an optional #variant.
	for _, ok := range []string{"anthropic/claude-sonnet-4-5", "amazon-bedrock/eu.anthropic.claude-sonnet-4-5", "openrouter/anthropic/claude-opus-4#high",
		"claude-sonnet-5-5", "claude-sonnet-5-5#high", "claude-opus-4-6"} {
		assert.NoError(t, CheckModel(ok), ok)
	}
	for _, bad := range []string{"", "anthropic/", "/claude", "a b/c", "claude sonnet", "anthropic/claude#", "claude#", "#high"} {
		assert.Error(t, CheckModel(bad), bad)
	}
}

// The error gives an example taken from the default model of oh (not an old
// literal) and says that anthropic/… is converted for the provider of the
// session.
// A short name carries a version number: "sonnet" or "claude-sonnet" would
// pin a model nobody can name, and each provider would convert it differently.
// A full provider/model reference is taken as written.
func TestCheckModelShortNameNeedsAVersion(t *testing.T) {
	for _, bad := range []string{"sonnet", "claude-sonnet", "claude-sonnet#high", "opus", "claude-opus-latest"} {
		err := CheckModel(bad)
		require.Error(t, err, bad)
		msg := err.Error()
		assert.Contains(t, msg, `"`+bad+`"`, bad)
		assert.Contains(t, msg, bricks.FallbackModel, "the message gives the expected form: "+bad)
		assert.Contains(t, msg, "version", bad)
	}
	for _, ok := range []string{"claude-sonnet-5-5", "claude-opus-4", "claude-haiku-4-5#low", "gpt-5",
		"anthropic/claude-sonnet", "amazon-bedrock/eu.anthropic.claude-sonnet", "anthropic/sonnet#high"} {
		assert.NoError(t, CheckModel(ok), ok)
	}
	// The digit of the variant is not a version.
	assert.Error(t, CheckModel("claude-sonnet#v2"))
}

func TestCheckModelErrorExplainsTheForms(t *testing.T) {
	err := CheckModel("claude sonnet")
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, `"claude sonnet"`)
	assert.Contains(t, msg, "anthropic/"+bricks.FallbackModel, "provider/model example")
	assert.Contains(t, msg, bricks.NormalizeModelForProvider(bricks.FallbackModel, "bedrock"), "what Bedrock receives")
	assert.NotContains(t, msg, "claude-sonnet-4-5", "no old literal example")
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
