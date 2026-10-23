package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/datichb/openhub/cli/internal/daemon/api"
)

// AttachedClients lists the interactive clients attached to a session.
func (c *Client) AttachedClients(ctx context.Context, sessionID string) ([]api.AttachedClient, error) {
	var out []api.AttachedClient
	err := c.do(ctx, http.MethodGet, api.RouteClients+"?session="+url.QueryEscape(sessionID), nil, &out)
	return out, err
}
