package runsvc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// S-28: [execution] env_allow reaches the server group the tool starts.
func TestStartSessionPassesEnvAllowToTheServer(t *testing.T) {
	f := newRTFixture(t)
	req := f.request(f.project)
	req.EnvAllow = []string{"SSH_AUTH_SOCK", "MYCO_*"}
	_, err := f.svc.StartSession(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, f.ad.started, 1)
	assert.Equal(t, []string{"SSH_AUTH_SOCK", "MYCO_*"}, f.ad.started[0].EnvAllow)
}
