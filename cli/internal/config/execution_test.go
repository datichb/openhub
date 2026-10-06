package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutionConfigRoundtrip(t *testing.T) {
	Reset()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".oh"), 0o755))

	c := &Config{Name: "hub", Execution: ExecutionConfig{Runtime: "container", Engine: "podman", KeepImages: 3,
		OpencodeVersion: "2.0.20", StrictIsolation: true}}
	require.NoError(t, Save(c))
	data, err := os.ReadFile(filepath.Join(tmpDir, ".oh", "hub.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "[execution]")

	Reset()
	got, err := Load()
	require.NoError(t, err)
	assert.Equal(t, c.Execution, got.Execution)
}

func TestExecutionImagesDefault(t *testing.T) {
	assert.Equal(t, 2, ExecutionConfig{}.Images())
	assert.Equal(t, 2, ExecutionConfig{KeepImages: -1}.Images())
	assert.Equal(t, 5, ExecutionConfig{KeepImages: 5}.Images())
}
