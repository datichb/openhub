package cmd

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/gateway/bdargs"
	"github.com/datichb/openhub/cli/internal/runtime/container"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

func TestEngineDetail(t *testing.T) {
	e := container.Engine{Details: []string{"mount=virtiofs", "arch=aarch64"}}
	assert.Equal(t, "virtiofs", engineDetail(e, "mount"))
	assert.Equal(t, "", engineDetail(e, "rootless"))
}

func TestGatewayReachCheck(t *testing.T) {
	check := func(key string, ok bool, detail string) views.DoctorCheck {
		return views.DoctorCheck{Name: key, OK: ok, Detail: detail}
	}
	assert.True(t, gatewayReachCheck(check, "beads", "u", http.StatusMethodNotAllowed, "busybox", false).OK)
	assert.True(t, gatewayReachCheck(check, "mcp", "u", http.StatusUnauthorized, "busybox", false).OK)
	assert.False(t, gatewayReachCheck(check, "beads", "u", http.StatusNotFound, "busybox", false).OK)
	mac := gatewayReachCheck(check, "beads", "u", 0, "busybox", false)
	linux := gatewayReachCheck(check, "beads", "u", 0, "busybox", true)
	assert.False(t, mac.OK)
	assert.False(t, linux.OK)
	assert.NotEqual(t, mac.Detail, linux.Detail, "Linux: hint about the second listener")
}

func TestDoctorChecksRegistered(t *testing.T) {
	assert.GreaterOrEqual(t, len(doctorChecks), 2, "container and gateway checks, one file each")
}

func TestBdVersionCheck(t *testing.T) {
	same := bdVersionResult(bdargs.GeneratedFor, nil)
	assert.True(t, same.OK)
	assert.False(t, same.Warn)

	other := bdVersionResult("bd version 99.1.0 (Homebrew)\n", nil)
	assert.True(t, other.OK, "a warning, not a failure")
	assert.True(t, other.Warn)
	assert.Contains(t, other.Detail, "99.1.0")
	assert.Contains(t, other.Detail, bdargs.VersionNumber(bdargs.GeneratedFor))
	assert.Contains(t, other.Detail, "go run ./internal/gateway/bdargs/gen/main.go")

	unreadable := bdVersionResult("", errors.New("exit status 1"))
	assert.True(t, unreadable.Warn)
	assert.Contains(t, unreadable.Detail, "exit status 1")
}
