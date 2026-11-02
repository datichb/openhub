package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
)

func TestParseContainerMemory(t *testing.T) {
	for in, want := range map[string]string{
		"":       "",
		"  ":     "",
		"4g":     "4g",
		"4G":     "4g",
		" 512m ": "512m",
		"512M":   "512m",
		"1024g":  "1024g",
		"2048m":  "2048m",
	} {
		got, err := config.ParseContainerMemory(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, bad := range []string{"4", "g", "0g", "-4g", "4 g", "4gb", "1.5g", "511m", "1025g", "4g,rw", "--privileged", "4t", "99999999999999999999g", "0x10m"} {
		_, err := config.ParseContainerMemory(bad)
		assert.Error(t, err, bad)
	}
}

func TestContainerMemoryDefaultsToNoLimit(t *testing.T) {
	assert.Equal(t, "", config.ExecutionConfig{}.ContainerMemory)
}
