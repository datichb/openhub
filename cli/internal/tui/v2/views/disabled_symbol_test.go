package views

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// « Disabled » is « ○ » in grey, never a red cross: off is not an error (the
// table of meanings reserves ✗ for a failure).
func TestDisabledIsAGreyCircle(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	frLocale(t)

	text := disabledText()
	assert.Contains(t, text, theme.SymInactive+" désactivé")
	assert.Contains(t, text, theme.ColorTag(theme.TextMutedHex), "grey")
	assert.NotContains(t, text, theme.SymFailure)
	assert.NotContains(t, text, theme.ColorTag(theme.ErrorHex), "not red")

	for _, o := range triStateOptions() {
		if o.Value == "false" {
			assert.True(t, strings.HasPrefix(o.Label, theme.SymInactive+" "), "the option « disabled »: %q", o.Label)
		}
	}

	v := NewMCPView(nil, MCPViewConfig{})
	off := MCPService{Name: "jira", Enabled: false}
	assert.Contains(t, v.hubItemText(off), theme.SymInactive)
	assert.NotContains(t, v.hubItemText(off), theme.SymFailure)
	gitlab := MCPService{Name: "gitlab", Enabled: true}
	assert.Contains(t, v.hubItemSubtext(gitlab), theme.SymInactive, "write off")
	assert.NotContains(t, v.hubItemSubtext(gitlab), theme.SymFailure)
}
