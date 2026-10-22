package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
)

func TestUnpinnedToolWarning(t *testing.T) {
	assert.Empty(t, unpinnedToolWarning(opencodev2.RecommendedVersion()), "the recommended version is pinned")
	assert.Empty(t, unpinnedToolWarning("v"+opencodev2.RecommendedVersion()), "a leading v is ignored")

	w := unpinnedToolWarning("2.0.1")
	assert.Contains(t, w, "2.0.1")
	pins, err := opencodev2.PinnedIntegrity()
	require.NoError(t, err)
	for v := range pins {
		assert.Contains(t, w, v, "the pinned versions are listed")
	}
	assert.Contains(t, w, opencodev2.RecommendedVersion())
}
