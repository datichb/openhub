package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// regionFormShell records the inline forms shown.
type regionFormShell struct {
	mockShell
	forms []InlineFormConfig
}

func (s *regionFormShell) ShowInlineForm(cfg InlineFormConfig) { s.forms = append(s.forms, cfg) }

// The Bedrock region of the Provider view is checked as it is typed (same
// rule as the credential proxy): invalid name refused, unknown region
// warned; a valid value is applied.
func TestProviderViewRegionCheckedAsTyped(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("en")
	t.Cleanup(func() { i18n.SetLocale(prev) })
	cfg := &config.Config{}
	v := NewProviderView(nil, ProviderViewConfig{GetConfig: func() *config.Config { return cfg }})
	sh := &regionFormShell{}
	v.SetShell(sh)
	v.list = widgets.NewSectionedList()
	v.buildLines()
	v.renderList()
	found := false
	for i, it := range v.list.GetItems() {
		if ref, ok := it.Reference.(int); ok && v.lines[ref].key == "aws_region" {
			v.list.SelectIndex(i)
			found = true
		}
	}
	require.True(t, found, "aws_region line shown for bedrock")
	v.editCurrent()
	require.Len(t, sh.forms, 1)
	f := sh.forms[0].Fields[0]
	require.NotNil(t, f.Validate)
	msg, invalid := f.Validate("x.evil.com#")
	assert.True(t, invalid)
	assert.Contains(t, msg, "Invalid AWS region")
	msg, invalid = f.Validate("xx-new-9")
	assert.False(t, invalid)
	assert.Contains(t, msg, "not known")
	_, invalid = f.Validate("eu-west-3")
	assert.False(t, invalid)
	sh.forms[0].OnSubmit(map[string]string{"value": "eu-west-3"}, nil)
	assert.Equal(t, "eu-west-3", cfg.Provider.Bedrock.AWSRegion)
}

func TestValidationText(t *testing.T) {
	assert.Empty(t, ValidationText("", true))
	assert.Contains(t, ValidationText("bad [x]", true), "✗ bad")
	assert.Contains(t, ValidationText("hmm", false), "⚠ hmm")
}
