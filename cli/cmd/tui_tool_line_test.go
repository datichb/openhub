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
	prevTool, prevVer := v5Tool, v5Ver.Load()
	t.Cleanup(func() {
		v5Tool = prevTool
		if prevVer != nil {
			v5Ver.Store(prevVer)
		} else {
			v5Ver.Store("")
		}
	})

	v5Tool = adapters.ToolInfo{Name: "acme-v1", DisplayName: "acme"}
	v5Ver.Store("1.2.3")
	assert.Equal(t, "acme v1.2.3 · adaptateur acme-v1 ✔", tuiToolLine())

	i18ntest.Set(t, "en")
	assert.Equal(t, "acme v1.2.3 · adapter acme-v1 ✔", tuiToolLine())
}
