package cmd

import (
	"context"
	"sync"
	"testing"

	"github.com/datichb/openhub/cli/internal/adapters"
)

// withoutTool makes the tests run as if no supported tool were installed
// (detection error err; nil = tool missing).
func withoutTool(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		err = adapters.ErrToolNotInstalled
	}
	prev := detectSupportedTool
	detectSupportedTool = func(context.Context) (adapters.ToolAdapter, adapters.ToolInfo, error) {
		return nil, adapters.ToolInfo{DisplayName: "faketool"}, err
	}
	toolOnce, toolAdapter, toolInfo, toolErr = sync.Once{}, nil, adapters.ToolInfo{}, nil
	t.Cleanup(func() {
		detectSupportedTool = prev
		toolOnce, toolAdapter, toolInfo, toolErr = sync.Once{}, nil, adapters.ToolInfo{}, nil
	})
}
