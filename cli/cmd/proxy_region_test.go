package cmd

import (
	"bytes"
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
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

// oh init / oh provider setup: the custom region is checked as it is typed
// (message under the field) and the step refuses an invalid one.
func TestBedrockRegionCheckedAsTyped(t *testing.T) {
	useLocale(t, "en")
	assert.Equal(t, i18n.T("cmd.init.wizard_region_required"), bedrockRegionStepProblem(""))
	assert.Contains(t, bedrockRegionStepProblem("x.evil.com#"), "Invalid AWS region")
	assert.Empty(t, bedrockRegionStepProblem("xx-new-9"), "unknown region: warning only")
	assert.Empty(t, bedrockRegionStepProblem("eu-west-1"))

	form := tview.NewForm()
	region, idx, custom := "", 0, true
	addBedrockRegionDropDown(form, &region, &idx, &custom, func() {})
	var input *tview.InputField
	var check *tview.TextView
	for i := 0; i < form.GetFormItemCount(); i++ {
		switch it := form.GetFormItem(i).(type) {
		case *tview.InputField:
			input = it
		case *tview.TextView:
			check = it
		}
	}
	require.NotNil(t, input)
	require.NotNil(t, check, "message line under the custom region")
	assert.Empty(t, check.GetText(true))
	input.SetText("x.evil.com#")
	assert.Equal(t, "x.evil.com#", region)
	assert.Contains(t, check.GetText(true), "Invalid AWS region")
	input.SetText("xx-new-9")
	assert.Contains(t, check.GetText(true), "not known")
	input.SetText("eu-west-1")
	assert.Empty(t, check.GetText(true))
}
