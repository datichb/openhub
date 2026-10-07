package cmd

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

type fakeGranter struct {
	got []daemon.GatewayGrantRequest
	err error
}

func (f *fakeGranter) IssueGatewayGrant(_ context.Context, r daemon.GatewayGrantRequest) (daemon.GatewayGrantResponse, error) {
	f.got = append(f.got, r)
	return daemon.GatewayGrantResponse{Token: "ohg_t"}, f.err
}

func TestGatewaySessionEnv(t *testing.T) {
	ctx := context.Background()
	fg := &fakeGranter{}
	hook := gatewaySessionEnv(func(context.Context) (gatewayGranter, error) { return fg, nil })

	// QB1: local sessions go through the gateway too (fake bd first on their
	// PATH); a group started by an older oh has no gateway address.
	env, err := hook(ctx, runsvc.SessionEnvRequest{SessionID: "ses_a", Runtime: sessionspec.RuntimeLocal})
	require.NoError(t, err)
	assert.Nil(t, env, "group without gateway address: real bd until the server restarts")
	assert.Empty(t, fg.got)
	env, err = hook(ctx, runsvc.SessionEnvRequest{SessionID: "ses_l", Runtime: sessionspec.RuntimeLocal, BeadsAllow: []string{"show"},
		GatewayURL: "http://127.0.0.1:1/oh-gateway"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"OH_GATEWAY_URL": "http://127.0.0.1:1/oh-gateway", "OH_GATEWAY_TOKEN": "ohg_t"}, env)
	assert.Equal(t, []string{"show"}, fg.got[0].BeadsAllow)
	fg.got = nil

	env, err = hook(ctx, runsvc.SessionEnvRequest{SessionID: "ses_a", GroupKey: "g", ProjectID: "p", Location: "/p",
		WorkflowID: "ticket", Runtime: sessionspec.RuntimeContainer, BeadsAllow: []string{"show"}, GatewayURL: "http://h:1/oh-gateway"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"OH_GATEWAY_URL": "http://h:1/oh-gateway", "OH_GATEWAY_TOKEN": "ohg_t"}, env)
	assert.Equal(t, daemon.GatewayGrantRequest{SessionID: "ses_a", GroupKey: "g", ProjectID: "p", WorkflowID: "ticket", Location: "/p",
		BeadsAllow: []string{"show"}, GatewayURL: "http://h:1/oh-gateway"}, fg.got[0])

	fg.err = &daemon.APIError{Status: http.StatusNotFound}
	_, err = hook(ctx, runsvc.SessionEnvRequest{Runtime: sessionspec.RuntimeContainer})
	assert.ErrorContains(t, err, "gateway address")
	_, err = hook(ctx, runsvc.SessionEnvRequest{Runtime: sessionspec.RuntimeContainer, GatewayURL: "http://h:1/oh-gateway"})
	assert.ErrorContains(t, err, "oh daemon stop")
}

func TestOhMCPCommand(t *testing.T) {
	spec := sessionspec.BundleSpec{MCP: []sessionspec.MCPServerDef{
		sessionspec.WorkflowMCPDef(),
		{Name: "gitlab", Type: "local", Command: []string{"/old/path/oh", "mcp", "serve", "gitlab", "--token-key", "k"}, Environment: map[string]string{"GITLAB_URL": "https://gl"}},
		{Name: "fs", Type: "local", Command: []string{"npx", "fs"}},
	}}
	srv := domain.Server{GroupKey: "g", WorkDir: "/p"}
	c, err := ohMCPCommand(spec, srv, "gitlab")
	require.NoError(t, err)
	assert.Equal(t, []string{sessionspec.OhExecutable(), "mcp", "serve", "gitlab", "--token-key", "k"}, c.Argv)
	assert.Equal(t, "https://gl", c.Env["GITLAB_URL"])
	assert.Equal(t, "/p", c.Dir)
	c, err = ohMCPCommand(spec, srv, "workflow")
	require.NoError(t, err)
	assert.Equal(t, []string{sessionspec.OhExecutable(), "mcp", "serve", "workflow"}, c.Argv)
	_, err = ohMCPCommand(spec, srv, "fs")
	assert.Error(t, err, "only oh servers are run on the machine")
}
