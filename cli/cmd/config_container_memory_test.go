package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// execution.container_memory: validated like the TUI field, normalized,
// unset by `oh config unset`, listed by `oh config list --all`.
func TestConfigContainerMemory(t *testing.T) {
	i18ntest.Set(t, "en")
	c := &config.Config{}
	f, ok := lookupConfigField("execution.container_memory")
	require.True(t, ok, "settable key")

	require.NoError(t, f.Set(c, " 4G "))
	assert.Equal(t, "4g", c.Execution.ContainerMemory)
	require.NoError(t, f.Set(c, "512m"))
	assert.Equal(t, "512m", c.Execution.ContainerMemory)

	for _, bad := range []string{"", "4", "4gb", "256m", "2000g", "-1g", "4g --privileged"} {
		err := f.Set(c, bad)
		require.Error(t, err, bad)
		assert.Contains(t, err.Error(), "execution.container_memory", bad)
		assert.Equal(t, "512m", c.Execution.ContainerMemory, "a refused value changes nothing: %s", bad)
	}

	f.Unset(c)
	assert.Empty(t, c.Execution.ContainerMemory)

	rows := map[string]configListRow{}
	for _, r := range configListAll(c) {
		rows[r.Key] = r
	}
	assert.Equal(t, configListRow{Key: "execution.container_memory", Value: "", Origin: "default"}, rows["execution.container_memory"])
	require.NoError(t, f.Set(c, "8g"))
	assert.Equal(t, "8g", configListAllValue(t, c, "execution.container_memory"))
}

func configListAllValue(t *testing.T, c *config.Config, key string) string {
	t.Helper()
	for _, r := range configListAll(c) {
		if r.Key == key {
			return r.Value
		}
	}
	t.Fatalf("%s not listed", key)
	return ""
}
