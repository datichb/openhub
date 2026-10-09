//go:build windows

package opencodev2

import (
	"context"

	"github.com/datichb/openhub/cli/internal/adapters"
)

// Windows: the servers are not looked for (local runtime only, the daemon
// lives in the oh process and stops them with it).
func listServerProcesses(context.Context) ([]adapters.ServerProcess, error) { return nil, nil }
