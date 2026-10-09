package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// S-21 (audit v5): the Bedrock region is checked when it is configured.
func TestCheckBedrockRegion(t *testing.T) {
	var warn bytes.Buffer
	assert.NoError(t, checkBedrockRegion(&warn, "eu-west-1"))
	assert.Empty(t, warn.String())
	assert.NoError(t, checkBedrockRegion(&warn, "xx-new-9"))
	assert.NotEmpty(t, warn.String(), "well formed but unknown: warning")
	assert.Error(t, checkBedrockRegion(&warn, "x.evil.com#"))
	assert.NoError(t, checkBedrockRegion(&warn, ""))
	assert.Error(t, configFieldMap["provider.bedrock.aws_region"].Set(nil, "x.evil.com#"))
}
