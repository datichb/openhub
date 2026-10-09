package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// The region is checked as it is typed with the rule of the credential
// proxy: invalid name refused, unknown Bedrock region only warned.
func TestRegionProblem(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("en")
	t.Cleanup(func() { i18n.SetLocale(prev) })

	msg, invalid := RegionProblem("eu-west-1")
	assert.Empty(t, msg)
	assert.False(t, invalid)
	msg, invalid = RegionProblem("")
	assert.Empty(t, msg, "default chain")
	assert.False(t, invalid)
	msg, invalid = RegionProblem("x.evil.com#")
	assert.True(t, invalid)
	assert.Contains(t, msg, "Invalid AWS region")
	msg, invalid = RegionProblem("xx-new-9")
	assert.False(t, invalid)
	assert.Contains(t, msg, "not known")
}
