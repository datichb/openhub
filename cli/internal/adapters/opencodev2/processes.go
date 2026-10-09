package opencodev2

import (
	"context"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
)

var _ adapters.ServerProcessLister = (*Adapter)(nil)

// ServerProcesses implements adapters.ServerProcessLister: the
// `opencode serve --hostname …` processes (the command line oh starts).
func (a *Adapter) ServerProcesses(ctx context.Context) ([]adapters.ServerProcess, error) {
	return listServerProcesses(ctx)
}

// StopServerProcess implements adapters.ServerProcessLister.
func (a *Adapter) StopServerProcess(ctx context.Context, pid int) error {
	return AttachServer("", "", pid).Stop(ctx, 3*time.Second)
}
