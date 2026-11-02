package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The project wizards leave the model empty when the field is left empty: the
// project then inherits the model of the hub (cascade) instead of pinning a
// fallback model on every project created.
func TestWizardProjectModelStaysEmptyWhenLeftEmpty(t *testing.T) {
	assert.Empty(t, wizardProjectModel(""))
	assert.Empty(t, wizardProjectModel("   "), "blanks are an empty field")
	assert.Equal(t, "claude-opus-4-8", wizardProjectModel("  claude-opus-4-8 "), "a model typed is kept, trimmed")
}
