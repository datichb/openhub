package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
)

// strictIsolation hands execution.strict_isolation to the tool adapter, which
// refuses an unpinned release of the tool under it (S-38).
func TestStrictIsolationReadsTheConfiguration(t *testing.T) {
	t.Setenv("OH_HOME", t.TempDir())
	config.Reset()
	t.Cleanup(config.Reset)
	assert.False(t, strictIsolation(), "off by default")

	require.NoError(t, config.Update(func(c *config.Config) error {
		c.Execution.StrictIsolation = true
		return nil
	}))
	assert.True(t, strictIsolation())
}

func TestToolRegistryAdapterIsBuilt(t *testing.T) {
	names := toolRegistry.Names()
	require.NotEmpty(t, names)
	ad, ok := toolRegistry.Get(names[0])
	require.True(t, ok)
	assert.NotNil(t, ad)
}
