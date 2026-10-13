package opencodev2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoredKey(t *testing.T) {
	export := []byte(`[{"integrationID":"anthropic","value":{"key":"a"}},{"integrationID":"amazon-bedrock","value":{"key":"b"}}]`)
	k, err := storedKey(export, "amazon-bedrock")
	require.NoError(t, err)
	assert.Equal(t, "b", k)
	k, err = storedKey(export, "openrouter")
	require.NoError(t, err)
	assert.Empty(t, k)
	_, err = storedKey([]byte("nope"), "amazon-bedrock")
	assert.Error(t, err)
}
