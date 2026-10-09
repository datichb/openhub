package daemon

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

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

// N16: a task runs once the grants are restored, then every Every; a panic
// does not stop it; it never marks the daemon busy.
func TestRunTask(t *testing.T) {
	d := &Daemon{restored: make(chan struct{}), stop: make(chan struct{})}
	runs := make(chan struct{}, 10)
	n := 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go d.runTask(ctx, Task{Name: "t", Every: 10 * time.Millisecond, Run: func(context.Context) {
		n++
		runs <- struct{}{}
		if n == 1 {
			panic("boom")
		}
	}})
	select {
	case <-runs:
		t.Fatal("ran before the grants were restored")
	case <-time.After(50 * time.Millisecond):
	}
	close(d.restored)
	for range 3 {
		select {
		case <-runs:
		case <-time.After(2 * time.Second):
			t.Fatal("task not run again")
		}
	}
	assert.False(t, d.periodicBusy)
}
