package filelock

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLockSerializes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "lock")
	unlock, err := Lock(path)
	require.NoError(t, err)
	got := make(chan struct{})
	go func() {
		u, err := Lock(path)
		if err == nil {
			u()
		}
		close(got)
	}()
	select {
	case <-got:
		t.Fatal("second lock acquired while held")
	case <-time.After(200 * time.Millisecond):
	}
	unlock()
	select {
	case <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("second lock never acquired")
	}
	assert.True(t, ProcessAlive(os.Getpid()))
	assert.False(t, ProcessAlive(0))
}

func TestTryLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	unlock, err := TryLock(path)
	require.NoError(t, err)
	_, err = TryLock(path)
	assert.ErrorIs(t, err, ErrLocked)
	unlock()
	again, err := TryLock(path)
	require.NoError(t, err)
	again()
}

func TestLockContextExpires(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	unlock, err := Lock(path)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = LockContext(ctx, path)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	unlock()
	u, err := LockContext(context.Background(), path)
	require.NoError(t, err)
	u()
}
