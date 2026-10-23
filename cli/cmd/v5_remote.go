package cmd

import (
	"github.com/datichb/openhub/cli/internal/app"
	remotesvc "github.com/datichb/openhub/cli/internal/services/remote"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// Remote launches (`oh run … --runtime remote`): the preparation and the
// sending are services/launch; this is what wires the remote service to the
// app.

// newRemoteSendService wires the remote service for sending.
func newRemoteSendService(a *app.App) *remotesvc.Service {
	svc := newRemoteService(a)
	svc.Sessions = a.Sessions
	svc.Remote = sqlite.NewRemoteStore(store)
	if v5Adapter != nil {
		svc.ToolName, svc.ToolVersion = v5Adapter.Name(), v5Tool.Version
	}
	return svc
}
