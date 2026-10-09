package daemon

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// N16: the logs the live processes keep open are copied then truncated.
func TestRotateLogs(t *testing.T) {
	p := shortPaths(t)
	servers := t.TempDir()
	srvLog := filepath.Join(servers, "g1", "data", "oh-server.log")
	require.NoError(t, os.MkdirAll(filepath.Dir(srvLog), 0o700))
	require.NoError(t, os.WriteFile(srvLog, bytes.Repeat([]byte("s"), 100), 0o600))
	require.NoError(t, os.WriteFile(p.Log(), bytes.Repeat([]byte("d"), 100), 0o600))
	small := filepath.Join(servers, "g2", "data", "oh-server.log")
	require.NoError(t, os.MkdirAll(filepath.Dir(small), 0o700))
	require.NoError(t, os.WriteFile(small, []byte("s"), 0o600))

	d := &Daemon{opts: Options{Paths: p, ServersDir: servers, LogMaxBytes: 50, LogKeep: 2}}
	d.rotateLogs()
	for _, f := range []string{p.Log(), srvLog} {
		fi, err := os.Stat(f)
		require.NoError(t, err)
		assert.Zero(t, fi.Size(), f)
		assert.FileExists(t, f+".1")
	}
	assert.NoFileExists(t, small+".1", "below the limit")
}

// N16: ohd.log is rotated before a daemon is spawned (nobody writes it).
func TestEnsureRotatesTheLog(t *testing.T) {
	p := shortPaths(t)
	require.NoError(t, os.MkdirAll(p.Dir, 0o700))
	require.NoError(t, os.WriteFile(p.Log(), bytes.Repeat([]byte("x"), 200), 0o600))
	ctx := context.Background()
	c, _, err := Ensure(ctx, p, EnsureOptions{
		Executable:  os.Args[0],
		Args:        []string{"-test.run=^TestHelperDaemon$"},
		Env:         append(os.Environ(), "OHD_TEST_HELPER_DIR="+p.Dir),
		Version:     "helper",
		LogMaxBytes: 100,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Shutdown(ctx, false) })
	fi, err := os.Stat(p.Log() + ".1")
	require.NoError(t, err)
	assert.EqualValues(t, 200, fi.Size())
}
