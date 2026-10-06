package cmd

import (
	"context"
	"sync"
	"testing"

	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
)

// withoutV2 makes the tests run as if no supported opencode were installed
// (detection error err; nil = opencode missing).
func withoutV2(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		err = opencodev2.ErrNotInstalled
	}
	prev := detectV2
	detectV2 = func(context.Context) (*opencodev2.Adapter, error) { return nil, err }
	v5Once, v5Adapter, v5Err = sync.Once{}, nil, nil
	t.Cleanup(func() {
		detectV2 = prev
		v5Once, v5Adapter, v5Err = sync.Once{}, nil, nil
	})
}
