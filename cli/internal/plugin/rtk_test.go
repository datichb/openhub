package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRTKPluginSourceEmbedded(t *testing.T) {
	// Verify the embedded source is non-empty and looks like TypeScript
	assert.Greater(t, len(rtkPluginSource), 100)
	assert.Contains(t, string(rtkPluginSource), "opencode")
}

func TestRTKInstallAndRemove(t *testing.T) {
	// Skip if rtk binary not available (CI might not have it)
	_, err := CheckRTKBinary()
	if err != nil {
		t.Skip("rtk binary not available, skipping install test")
	}

	// Override plugins dir via HOME
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create the expected directory structure
	dir := filepath.Join(tmpHome, ".config", "opencode", "plugins")

	// Install
	err = RTKInstall()
	require.NoError(t, err)

	// Verify file exists
	pluginPath := filepath.Join(dir, "rtk.ts")
	_, err = os.Stat(pluginPath)
	require.NoError(t, err)

	// Verify content matches embedded
	content, err := os.ReadFile(pluginPath)
	require.NoError(t, err)
	assert.Equal(t, rtkPluginSource, content)

	// Install again (should backup and succeed)
	err = RTKInstall()
	require.NoError(t, err)

	// Verify backup was created
	backupDir := filepath.Join(dir, ".backup")
	entries, err := os.ReadDir(backupDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)

	// Remove
	err = RTKRemove()
	require.NoError(t, err)

	// Verify file is gone
	_, err = os.Stat(pluginPath)
	assert.True(t, os.IsNotExist(err))
}

func TestRTKRemoveNotInstalled(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	err := RTKRemove()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "n'est pas installé")
}

func TestRTKStatus(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Not installed
	status := RTKStatus()
	assert.False(t, status.Installed)

	// Install
	dir := filepath.Join(tmpHome, ".config", "opencode", "plugins")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "rtk.ts"), []byte("fake"), 0o644)

	status = RTKStatus()
	assert.True(t, status.Installed)
	assert.Contains(t, status.Path, "rtk.ts")
}

func TestIsVersionAtLeast(t *testing.T) {
	tests := []struct {
		version  string
		minimum  string
		expected bool
	}{
		{"0.45.0", "0.42.0", true},
		{"0.42.0", "0.42.0", true},
		{"0.41.9", "0.42.0", false},
		{"1.0.0", "0.42.0", true},
		{"0.42.1", "0.42.0", true},
	}
	for _, tt := range tests {
		result := IsVersionAtLeast(tt.version, tt.minimum)
		assert.Equal(t, tt.expected, result, "version=%s minimum=%s", tt.version, tt.minimum)
	}
}
