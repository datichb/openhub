package cmd

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/daemon"
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

	env, err := hook(ctx, runsvc.SessionEnvRequest{SessionID: "ses_a", Runtime: sessionspec.RuntimeLocal})
	require.NoError(t, err)
	assert.Nil(t, env, "local sessions run bd directly")
	assert.Empty(t, fg.got)

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
