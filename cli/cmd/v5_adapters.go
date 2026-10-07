package cmd

import (
	"context"
	"sync"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	"github.com/datichb/openhub/cli/internal/runsvc"
)

// The composition root of the tool adapters (D19): the only file of oh,
// outside internal/adapters/<tool>, that names a tool. Everything else asks
// toolRegistry for a neutral adapters.ToolAdapter.

var toolRegistry = newToolRegistry()

func newToolRegistry() *adapters.Registry {
	r := &adapters.Registry{}
	r.Register(opencodev2.Name, func(string) adapters.ToolAdapter { return opencodev2.New("", ohCacheDir()) })
	return r
}

// detectTool returns the first installed and supported tool (D3: oh v5 runs
// sessions only on a supported release).
func detectTool(ctx context.Context) (adapters.ToolAdapter, adapters.ToolInfo, error) {
	return toolRegistry.Detect(ctx)
}

// detectedAdapters returns the adapter recorded for a server group, detected
// once (nil when unknown or not installed).
func detectedAdapters(ctx context.Context) func(name string) adapters.ToolAdapter {
	var (
		mu   sync.Mutex
		done = map[string]adapters.ToolAdapter{}
	)
	return func(name string) adapters.ToolAdapter {
		mu.Lock()
		defer mu.Unlock()
		if a, ok := done[name]; ok {
			return a
		}
		a, ok := toolRegistry.Get(name)
		if ok {
			if _, err := a.Detect(ctx); err != nil {
				a = nil
			}
		} else {
			a = nil
		}
		done[name] = a
		return a
	}
}

// preferredAdapter is the adapter of the installed tool, else the first
// registered one (where the tool is not installed yet: job images).
func preferredAdapter() adapters.ToolAdapter {
	if v5Adapter != nil {
		return v5Adapter
	}
	if names := toolRegistry.Names(); len(names) > 0 {
		a, _ := toolRegistry.Get(names[0])
		return a
	}
	return nil
}

// toolProviderID is the provider id of the tool for a hub provider.
func toolProviderID(hubProvider string) string {
	ad := preferredAdapter()
	if ad == nil {
		return hubProvider
	}
	return runsvc.ToolProviderID(ad, hubProvider)
}
