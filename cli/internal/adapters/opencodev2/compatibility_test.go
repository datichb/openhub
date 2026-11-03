package opencodev2

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckVersion(t *testing.T) {
	assert.NoError(t, CheckVersion("5.0.1", "2.0.20"))
	assert.NoError(t, CheckVersion("dev", "2.0.0"))

	var ue *UnsupportedError
	err := CheckVersion("5.0.1", "1.18.29")
	require.True(t, errors.As(err, &ue), "opencode V1 refused")
	assert.Equal(t, "1.18.29", ue.Found)
	assert.Equal(t, "2.0.0", ue.Min)
	assert.True(t, errors.As(CheckVersion("dev", "3.0.0"), &ue), "beyond the supported major")
	assert.True(t, errors.As(CheckVersion("dev", ""), &ue), "unknown version")
}

// Each minor version of oh has its own entry: without it the version falls
// back on "default", which says nothing about it (release checklist).
func TestOhMinorVersionsHaveAnEntry(t *testing.T) {
	var m compatMatrix
	require.NoError(t, json.Unmarshal(compatibilityJSON, &m))
	for _, minor := range []string{"5.0", "5.1", "5.2"} {
		r, ok := m.OhVersions[minor]
		require.True(t, ok, "oh %s has no entry in compatibility.json", minor)
		assert.Equal(t, r, SupportedRange(minor+".0"))
		assert.Equal(t, m.Default.OpencodeRecommended, r.OpencodeRecommended)
	}
}
