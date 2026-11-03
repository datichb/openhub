package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// A pre-release or a dev build newer than the latest published release
// is never told to "update" to it; a newer release is a warning only.
func TestOhUpdateResultNeverOffersOlder(t *testing.T) {
	i18ntest.Set(t, "en")
	for _, current := range []string{"5.0.0-test", "5.0.0", "v4.2.0-195-gfe9ed932", "v4.2.0-195-gfe9ed932-dirty"} {
		r := ohUpdateResult(current, "4.2.0")
		assert.True(t, r.OK, current)
		assert.False(t, r.Warn, current)
		assert.NotContains(t, r.Detail, "available", current)
	}
	r := ohUpdateResult("4.2.0", "4.2.0")
	assert.True(t, r.OK)
	assert.False(t, r.Warn)

	r = ohUpdateResult("5.0.0-test", "5.0.0")
	assert.True(t, r.OK, "an update is not a failure")
	assert.True(t, r.Warn)
	assert.Contains(t, r.Detail, "5.0.0 available")

	r = ohUpdateResult("dev", "4.2.0")
	assert.True(t, r.OK)
	assert.False(t, r.Warn)
}
