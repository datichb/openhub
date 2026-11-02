package api

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoute(t *testing.T) {
	assert.Equal(t, "/health", Route(RouteHealth))
	assert.Equal(t, "/owners/a%2Fb%20c/grants", Route(RouteOwnerGrants, "owner", "a/b c"))
	assert.Equal(t, "/servers/g/touch", Route(RouteServerTouch, "group", "g"))
	assert.Equal(t, "/clients/{id}", Route(RouteClient), "a parameter not given stays a pattern")
}

func TestPaths(t *testing.T) {
	p := Paths{Dir: filepath.FromSlash("/run/oh")}
	assert.Equal(t, filepath.FromSlash("/run/oh/ohd.sock"), p.Socket())
	assert.Equal(t, filepath.FromSlash("/run/oh/spawn.lock"), p.SpawnLock())
	assert.Equal(t, filepath.FromSlash("/run/oh/capability"), p.CapabilityFile())
	assert.Equal(t, filepath.FromSlash("/s/g/lock"), GroupLockPath(filepath.FromSlash("/s"), "g"))
	assert.Equal(t, filepath.FromSlash("/s/g/proxy_url"), ProxyURLPath(filepath.FromSlash("/s"), "g"))
}

func TestErrors(t *testing.T) {
	err := error(&APIError{Method: "GET", Path: "/x", Status: 409, Message: "no"})
	assert.Equal(t, "ohd GET /x: 409 no", err.Error())
	var ae *APIError
	require.True(t, errors.As(err, &ae))

	ue := error(&UntrustedSocketError{Path: "/run/ohd.sock"})
	assert.ErrorIs(t, ue, ErrUntrustedSocket)
	assert.NotErrorIs(t, ue, ErrNotRunning)
}
