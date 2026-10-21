package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/gc"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

func TestDescribeOrphans(t *testing.T) {
	i18ntest.Set(t, "en")
	o := []orphanServer{{ServerProcess: adapters.ServerProcess{PID: 1, LogPath: "/a"}, Reason: gc.OrphanTemp}, {ServerProcess: adapters.ServerProcess{PID: 2, WorkDir: "/b"}, Reason: gc.OrphanUnowned}}
	assert.Equal(t, "pid 1 (interrupted test or discovery, /a); and 1 more", describeOrphans(o, 1))
	assert.Equal(t, "pid 1 (interrupted test or discovery, /a); pid 2 (oh no longer runs it, /b)", describeOrphans(o, 3))
}
