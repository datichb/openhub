package opencodev2

import (
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
