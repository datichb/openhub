package cmd

import (
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestBdSchemaCheck(t *testing.T) {
	ok := bdSchemaResult([]byte(`{"schema_version":1,"types":{"issue":{"properties":{}}}}`), nil)
	assert.True(t, ok.OK)
	assert.True(t, ok.Warn, "fields oh reads are missing")
	assert.Contains(t, ok.Detail, "issue.title")

	full, err := os.ReadFile("../internal/beads/bdschema/schema_bd.json")
	require.NoError(t, err)
	good := bdSchemaResult(full, nil)
	assert.False(t, good.Warn, good.Detail)

	bad := bdSchemaResult(nil, errors.New("unknown command"))
	assert.True(t, bad.Warn)
	assert.Contains(t, bad.Detail, "unknown command")
}
