package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway"
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/runtime/container"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// gatewayGranter is the part of the daemon client issuing gateway tokens.
type gatewayGranter interface {
	IssueGatewayGrant(ctx context.Context, req daemon.GatewayGrantRequest) (daemon.GatewayGrantResponse, error)
}

// gatewaySessionEnv gives the sessions that run outside the machine
// (container) their Beads gateway token (P4-T07). Locally, bd is run
// directly (D9). The token is minted at each start and resume, never
// written by oh (the daemon keeps only its hash).
func gatewaySessionEnv(dc func(ctx context.Context) (gatewayGranter, error)) runsvc.SessionEnvFunc {
	return func(ctx context.Context, r runsvc.SessionEnvRequest) (map[string]string, error) {
		if r.Runtime != sessionspec.RuntimeContainer {
			return nil, nil
		}
		if r.GatewayURL == "" {
			return nil, errors.New("beads gateway: the server group has no gateway address")
		}
		c, err := dc(ctx)
		if err != nil {
			return nil, err
		}
		g, err := c.IssueGatewayGrant(ctx, daemon.GatewayGrantRequest{
			SessionID: r.SessionID, GroupKey: r.GroupKey, ProjectID: r.ProjectID, WorkflowID: r.WorkflowID,
			Location: r.Location, BeadsAllow: r.BeadsAllow, GatewayURL: r.GatewayURL,
		})
		var apiErr *daemon.APIError
		if errors.As(err, &apiErr) && (apiErr.Status == http.StatusNotFound || apiErr.Status == http.StatusMethodNotAllowed) {
			return nil, errors.New(i18n.T("cmd.gateway.beads.daemon_outdated"))
		}
		if err != nil {
			return nil, fmt.Errorf("beads gateway: %w", err)
		}
		return map[string]string{beadswire.EnvURL: r.GatewayURL, beadswire.EnvToken: g.Token}, nil
	}
}

// gatewayView tells the daemon how the runtime of a server group sees the
// machine (container mounts saved at preparation).
func gatewayView(servers domain.ServerStore) func(ctx context.Context, group string) (gateway.View, error) {
	return func(ctx context.Context, group string) (gateway.View, error) {
		srv, err := servers.Get(ctx, group)
		if err != nil {
			return gateway.View{}, err
		}
		if sessionspec.RuntimeKind(srv.Runtime) != sessionspec.RuntimeContainer {
			return gateway.View{}, nil
		}
		spec, err := container.LoadSpec(srv.DataDir)
		if err != nil {
			return gateway.View{}, err
		}
		return gateway.View{Paths: spec.Paths, Locations: spec.Locations}, nil
	}
}
