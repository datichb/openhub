package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/datichb/openhub/cli/internal/daemon/api"
	"github.com/datichb/openhub/cli/internal/services/wiki"
)

// WikiIndex returns the index of the wiki of the session of a tool session.
func (c *Client) WikiIndex(ctx context.Context, toolSession string) (api.WikiIndex, error) {
	var out api.WikiIndex
	err := c.do(ctx, http.MethodGet, api.RouteWikiIndex+"?tool_session="+url.QueryEscape(toolSession), nil, &out)
	return out, err
}

// WikiPage reads a page of the wiki (or one of its sections).
func (c *Client) WikiPage(ctx context.Context, toolSession, page, section string) (api.WikiPage, error) {
	q := url.Values{"tool_session": {toolSession}, "page": {page}}
	if section != "" {
		q.Set("section", section)
	}
	var out api.WikiPage
	err := c.do(ctx, http.MethodGet, api.RouteWikiPage+"?"+q.Encode(), nil, &out)
	return out, err
}

// WikiPropose queues a proposal.
func (c *Client) WikiPropose(ctx context.Context, req api.WikiProposeRequest) (api.WikiProposal, error) {
	var out api.WikiProposal
	err := c.do(ctx, http.MethodPost, api.RouteWikiPropose, req, &out)
	return out, err
}

// WikiPending lists the open proposals of the project (or of the session).
func (c *Client) WikiPending(ctx context.Context, toolSession string, sessionOnly bool) ([]api.WikiProposal, error) {
	q := url.Values{"tool_session": {toolSession}}
	if sessionOnly {
		q.Set("scope", "session")
	}
	var out []api.WikiProposal
	err := c.do(ctx, http.MethodGet, api.RouteWikiPending+"?"+q.Encode(), nil, &out)
	return out, err
}

// WikiRefused lists the recent refusals of the project (or of the session),
// with the reason the user gave. A daemon without the route answers 404.
func (c *Client) WikiRefused(ctx context.Context, toolSession string, sessionOnly bool) ([]api.WikiProposal, error) {
	q := url.Values{"tool_session": {toolSession}}
	if sessionOnly {
		q.Set("scope", "session")
	}
	var out []api.WikiProposal
	err := c.do(ctx, http.MethodGet, api.RouteWikiRefused+"?"+q.Encode(), nil, &out)
	return out, err
}

// WikiApply applies proposals (documentarian only).
func (c *Client) WikiApply(ctx context.Context, toolSession string, ids []string) ([]wiki.Result, error) {
	var out []wiki.Result
	err := c.do(ctx, http.MethodPost, api.RouteWikiApply, api.WikiApplyRequest{ToolSession: toolSession, IDs: ids}, &out)
	return out, err
}

// WikiCheck checks the wiki of the session, restricted to a change when
// opts restricts it (LD-H, LD-D).
func (c *Client) WikiCheck(ctx context.Context, toolSession string, opts wiki.CheckOptions) (wiki.CheckReport, error) {
	q := url.Values{"tool_session": {toolSession}}
	for _, p := range opts.Paths {
		q.Add("path", p)
	}
	if opts.Base != "" {
		q.Set("base", opts.Base)
	}
	if opts.Head != "" {
		q.Set("head", opts.Head)
	}
	if opts.SinceSync {
		q.Set("since_sync", "1")
	}
	var out wiki.CheckReport
	err := c.do(ctx, http.MethodGet, api.RouteWikiCheck+"?"+q.Encode(), nil, &out)
	return out, err
}
