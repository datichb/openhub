package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
)

// N18: which tool servers belong to nobody.
func TestClassifyOrphans(t *testing.T) {
	tmp := t.TempDir()
	hub := filepath.Join(t.TempDir(), "oh")
	other := filepath.Join(t.TempDir(), "other-oh")
	gone := filepath.Join(t.TempDir(), "gone-oh")
	for _, d := range []string{filepath.Join(hub, "servers", "g1", "data"), filepath.Join(other, "servers", "g9", "data")} {
		require.NoError(t, os.MkdirAll(d, 0o700))
	}
	require.NoError(t, os.WriteFile(filepath.Join(other, "oh.db"), nil, 0o600))
	srvLog := func(home, g string) string { return filepath.Join(home, "servers", g, "data", "oh-server.log") }
	procs := []adapters.ServerProcess{
		{PID: 1, PPID: 1, WorkDir: filepath.Join(tmp, "TestContractX", "001", "proj"), LogPath: filepath.Join(tmp, "TestContractX", "001", "data", "oh-server.log")},
		{PID: 2, PPID: 1, WorkDir: filepath.Join(tmp, "oh-natives-123", "work")},
		{PID: 3, PPID: 4242, WorkDir: filepath.Join(tmp, "TestRunning", "proj")},  // its test still runs
		{PID: 4, PPID: 1, WorkDir: "/work/project", LogPath: srvLog(hub, "g1")},   // live server of this hub
		{PID: 5, PPID: 1, WorkDir: "/work/project", LogPath: srvLog(hub, "g2")},   // oh no longer runs it
		{PID: 6, PPID: 1, WorkDir: "/work/project", LogPath: srvLog(hub, "g3")},   // being started (recent log)
		{PID: 7, PPID: 1, WorkDir: "/work/project", LogPath: srvLog(other, "g9")}, // another hub
		{PID: 8, PPID: 1, WorkDir: "/work/project", LogPath: srvLog(gone, "g7")},  // hub deleted
		{PID: 9, PPID: 1, WorkDir: "/work/project", LogPath: "/var/log/mine.log"}, // not started by oh
		{PID: 10, PPID: 1, WorkDir: filepath.Join(tmp, "mine")},                   // a temp dir of the user
	}
	live := map[int]bool{4: true}
	recent := func(p string) bool { return filepath.Base(filepath.Dir(filepath.Dir(p))) == "g3" }
	got := map[int]string{}
	for _, o := range classifyOrphans(procs, hub, tmp, live, fileExists, recent) {
		got[o.PID] = o.Reason
	}
	assert.Equal(t, map[int]string{1: orphanTemp, 2: orphanTemp, 5: orphanUnowned, 8: orphanHubGone}, got)

	// Without the database, the servers of this hub are left alone.
	got = map[int]string{}
	for _, o := range classifyOrphans(procs, hub, tmp, nil, fileExists, recent) {
		got[o.PID] = o.Reason
	}
	assert.NotContains(t, got, 5)
}

func TestDescribeOrphans(t *testing.T) {
	useLocale(t, "en")
	o := []orphanServer{{adapters.ServerProcess{PID: 1, LogPath: "/a"}, orphanTemp}, {adapters.ServerProcess{PID: 2, WorkDir: "/b"}, orphanUnowned}}
	assert.Equal(t, "pid 1 (interrupted test or discovery, /a); and 1 more", describeOrphans(o, 1))
	assert.Equal(t, "pid 1 (interrupted test or discovery, /a); pid 2 (oh no longer runs it, /b)", describeOrphans(o, 3))
}
