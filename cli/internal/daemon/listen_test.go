package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDaemonProxyListenerPersisted(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	c, _ := e.start(t, nil, time.Hour)

	for _, bad := range []string{"0.0.0.0", "8.8.8.8", "not-an-ip"} {
		_, err := c.ProxyListen(ctx, bad)
		assert.Error(t, err, bad)
	}
	u, err := c.ProxyListen(ctx, "127.0.0.1")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(u, "http://127.0.0.1:"))
	again, err := c.ProxyListen(ctx, "127.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, u, again, "idempotent")

	// The additional listener serves the proxy (unknown token → 401).
	resp, err := http.Get(u + "/amazon-bedrock/model/x/converse")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	data, err := os.ReadFile(e.paths.State())
	require.NoError(t, err)
	var st stateFile
	require.NoError(t, json.Unmarshal(data, &st))
	assert.Equal(t, []string{"127.0.0.1"}, st.ListenHosts)
	assert.NotZero(t, st.ProxyPort)
}
