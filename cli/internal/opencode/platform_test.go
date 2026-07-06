package opencode

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssetNameForDarwinArm64(t *testing.T) {
	name, format, err := AssetNameFor("darwin", "arm64")
	require.NoError(t, err)
	assert.Equal(t, "opencode-darwin-arm64.zip", name)
	assert.Equal(t, "zip", format)
}

func TestAssetNameForDarwinAmd64(t *testing.T) {
	name, format, err := AssetNameFor("darwin", "amd64")
	require.NoError(t, err)
	assert.Equal(t, "opencode-darwin-x64.zip", name)
	assert.Equal(t, "zip", format)
}

func TestAssetNameForLinuxArm64(t *testing.T) {
	name, format, err := AssetNameFor("linux", "arm64")
	require.NoError(t, err)
	assert.Equal(t, "opencode-linux-arm64.tar.gz", name)
	assert.Equal(t, "tar.gz", format)
}

func TestAssetNameForLinuxAmd64(t *testing.T) {
	name, format, err := AssetNameFor("linux", "amd64")
	require.NoError(t, err)
	assert.Equal(t, "opencode-linux-x64.tar.gz", name)
	assert.Equal(t, "tar.gz", format)
}

func TestAssetNameForUnsupported(t *testing.T) {
	_, _, err := AssetNameFor("windows", "amd64")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported platform")
}

func TestAssetNameForFreebsd(t *testing.T) {
	_, _, err := AssetNameFor("freebsd", "arm64")
	assert.Error(t, err)
}

func TestAssetNameCurrentPlatform(t *testing.T) {
	// Should succeed on darwin or linux CI
	name, format, err := AssetName()
	require.NoError(t, err)
	assert.NotEmpty(t, name)
	assert.NotEmpty(t, format)
	assert.Contains(t, name, "opencode-")
}
