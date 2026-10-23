package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
)

func TestRead_DoesNotReplaceTheSharedConfig(t *testing.T) {
	home := configtest.Home(t)
	seedHub(t, home, "name = \"a\"\n")

	shared, err := config.Load()
	require.NoError(t, err)
	fresh, err := config.Read() // leaves the cache alone
	require.NoError(t, err)
	assert.NotSame(t, shared, fresh)
	require.NoError(t, config.Update(func(c *config.Config) error { c.Name = "b"; return nil }))
	again, err := config.Load()
	require.NoError(t, err)
	assert.Same(t, shared, again, "the shared Config is not replaced")
	assert.Equal(t, "b", shared.Name)
	assert.Equal(t, "a", fresh.Name)
}
