package cmd

import (
	"github.com/datichb/openhub/cli/internal/app"
	remotesvc "github.com/datichb/openhub/cli/internal/services/remote"
)

// Remote launches (`oh run … --runtime remote`): the preparation and the
// sending are services/launch; this is what wires the remote service to the
// app.

// newRemoteSendService wires the remote service for sending.
func newRemoteSendService(a *app.App) *remotesvc.Service {
	svc := newRemoteService(a)
	svc.Sessions = a.Sessions
	svc.Remote = a.Remotes
	if toolAdapter != nil {
		svc.ToolName, svc.ToolVersion = toolAdapter.Name(), toolInfo.Version
	}
	return svc
}
