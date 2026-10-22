package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnpinnedToolWarning(t *testing.T) {
	pinned, recommended := toolPins()
	require.NotEmpty(t, pinned)
	require.NotEmpty(t, recommended)
	assert.Empty(t, unpinnedToolWarning(recommended), "the recommended version is pinned")
	assert.Empty(t, unpinnedToolWarning("v"+recommended), "a leading v is ignored")

	w := unpinnedToolWarning("2.0.1")
	assert.Contains(t, w, "2.0.1")
	for _, v := range pinned {
		assert.Contains(t, w, v, "the pinned versions are listed")
	}
	assert.Contains(t, w, recommended)
}
