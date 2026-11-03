package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// 10 §1: the header of the hub and of the project tells the tool, its version
// and its adapter.
func TestTUIToolLine(t *testing.T) {
	i18ntest.Set(t, "fr")
	prevTool, prevVer := toolInfo, detectedVersion.Load()
	t.Cleanup(func() {
		toolInfo = prevTool
		if prevVer != nil {
			detectedVersion.Store(prevVer)
		} else {
			detectedVersion.Store("")
		}
	})

	toolInfo = adapters.ToolInfo{Name: "acme-v1", DisplayName: "acme"}
	detectedVersion.Store("1.2.3")
	assert.Equal(t, "acme v1.2.3 · adaptateur acme-v1 ✔", tuiToolLine())

	i18ntest.Set(t, "en")
	assert.Equal(t, "acme v1.2.3 · adapter acme-v1 ✔", tuiToolLine())
}
