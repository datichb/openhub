package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/buildinfo"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// v5 runtime wiring: tool adapter, oh daemon, RunService.

func ohRunDir() string     { return filepath.Join(config.HubDir(), "run") }
func ohServersDir() string { return filepath.Join(config.HubDir(), "servers") }
func ohCacheDir() string   { return filepath.Join(config.HubDir(), "cache") }

// ensureDaemon returns a client to ohd, starting it when needed.
func ensureDaemon(ctx context.Context) (*daemon.Client, daemon.Health, error) {
	return daemon.Ensure(ctx, daemon.Paths{Dir: ohRunDir()}, daemon.EnsureOptions{Version: buildinfo.Version})
}

// detectV2Adapter returns the opencode V2 adapter, or an error when the
// installed opencode is not a V2 release.
func detectV2Adapter(ctx context.Context) (*opencodev2.Adapter, error) {
	a := opencodev2.New("", ohCacheDir())
	if _, err := a.Detect(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

// newRunService wires the RunService for the current app.
func newRunService(ctx context.Context, a *app.App) (*runsvc.Service, error) {
	if store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if !v5Available(ctx) {
		return nil, fmt.Errorf("opencode V2 is required for v5 sessions: %v", v5Err)
	}
	ad := v5Adapter
	return &runsvc.Service{
		Adapter:    ad,
		AdapterVer: ad.Ver,
		Servers:    sqlite.NewServerStore(store),
		Sessions:   a.Sessions,
		Secrets:    a.Secrets,
		ServersDir: ohServersDir(),
		Daemon: func(ctx context.Context) (runsvc.DaemonClient, error) {
			c, _, err := ensureDaemon(ctx)
			if err != nil {
				return nil, err
			}
			return c, nil
		},
	}, nil
}
