package cmd

import (
	"context"
	"sync"
	"testing"

	"github.com/datichb/openhub/cli/internal/adapters"
)

// withoutV2 makes the tests run as if no supported tool were installed
// (detection error err; nil = tool missing).
func withoutV2(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		err = adapters.ErrToolNotInstalled
	}
	prev := detectV2
	detectV2 = func(context.Context) (adapters.ToolAdapter, adapters.ToolInfo, error) {
		return nil, adapters.ToolInfo{DisplayName: "faketool"}, err
	}
	v5Once, v5Adapter, v5Tool, v5Err = sync.Once{}, nil, adapters.ToolInfo{}, nil
	t.Cleanup(func() {
		detectV2 = prev
		v5Once, v5Adapter, v5Tool, v5Err = sync.Once{}, nil, adapters.ToolInfo{}, nil
	})
}
