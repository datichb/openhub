package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutionConfigRoundtrip(t *testing.T) {
	tmpDir := configtest.Home(t)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".oh"), 0o755))

	c := &config.Config{Name: "hub", Execution: config.ExecutionConfig{Runtime: "container", Engine: "podman", KeepImages: 3,
		ToolVersion: "2.0.20", StrictIsolation: true, ProxyFirstByteTimeoutMinutes: 9,
		EnvAllow: []string{"SSH_AUTH_SOCK", "MYCO_*"}}}
	require.NoError(t, config.Save(c))
	data, err := os.ReadFile(filepath.Join(tmpDir, ".oh", "hub.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "[execution]")

	config.Reset()
	got, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, c.Execution, got.Execution)
}

func TestExecutionImagesDefault(t *testing.T) {
	assert.Equal(t, 2, config.ExecutionConfig{}.Images())
	assert.Equal(t, 2, config.ExecutionConfig{KeepImages: -1}.Images())
	assert.Equal(t, 5, config.ExecutionConfig{KeepImages: 5}.Images())
}

func TestProxyFirstByteTimeout(t *testing.T) {
	var e config.ExecutionConfig
	assert.Equal(t, 5*time.Minute, e.ProxyFirstByteTimeout(), "default")
	e.ProxyFirstByteTimeoutMinutes = 12
	assert.Equal(t, 12*time.Minute, e.ProxyFirstByteTimeout())
	e.ProxyFirstByteTimeoutMinutes = -3
	assert.Equal(t, 5*time.Minute, e.ProxyFirstByteTimeout(), "negative = default")
}
