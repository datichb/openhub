package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
)

// The tool registry hands out an adapter that reads execution.strict_isolation
// to decide on an unpinned opencode release (S-38).
func TestToolRegistryWiresStrictPins(t *testing.T) {
	ad, ok := toolRegistry.Get(opencodev2.Name)
	require.True(t, ok)
	oc, ok := ad.(*opencodev2.Adapter)
	require.True(t, ok)
	require.NotNil(t, oc.StrictPins)
	assert.False(t, oc.StrictPins(), "strict isolation is off by default")
}
