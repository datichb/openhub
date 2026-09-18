package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcquireInitLock_HappyPath(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, "test.lock")

	require.NoError(t, acquireInitLock(lock))
	defer releaseInitLock(lock)

	// File should exist and contain PID
	data, err := os.ReadFile(lock)
	require.NoError(t, err)
	assert.Contains(t, string(data), fmt.Sprintf("%d", os.Getpid()))
}

func TestAcquireInitLock_AlreadyHeld(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, "test.lock")

	require.NoError(t, acquireInitLock(lock))
	defer releaseInitLock(lock)

	err := acquireInitLock(lock)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "lockfile") || strings.Contains(err.Error(), "exists"),
		"error should mention lockfile or exists, got: %v", err)
}

func TestAcquireInitLock_StaleLockRemoved(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, "test.lock")

	// Create a stale lock (>1h old)
	require.NoError(t, os.WriteFile(lock, []byte("old-pid"), 0o600))
	staleTime := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(lock, staleTime, staleTime))

	// Should succeed (stale lock removed)
	require.NoError(t, acquireInitLock(lock))
	releaseInitLock(lock)
}

func TestAcquireInitLock_RecentLockBlocks(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, "test.lock")

	// Create a recent lock (<1h old)
	require.NoError(t, os.WriteFile(lock, []byte("recent-pid"), 0o600))
	recentTime := time.Now().Add(-30 * time.Minute)
	require.NoError(t, os.Chtimes(lock, recentTime, recentTime))

	err := acquireInitLock(lock)
	require.Error(t, err, "lock < 1h old should block")
}

func TestAcquireInitLock_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, "nested", "deep", "test.lock")

	require.NoError(t, acquireInitLock(lock))
	releaseInitLock(lock)
}

func TestReleaseInitLock_Idempotent(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, "nonexistent.lock")

	// Should not panic on non-existent file
	assert.NotPanics(t, func() {
		releaseInitLock(lock)
	})
}

func TestAcquireInitLock_ConcurrentRace(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, "race.lock")

	var wg sync.WaitGroup
	successes := make(chan bool, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := acquireInitLock(lock)
			successes <- (err == nil)
		}()
	}

	wg.Wait()
	close(successes)

	successCount := 0
	for s := range successes {
		if s {
			successCount++
		}
	}

	assert.Equal(t, 1, successCount, "exactly one goroutine should acquire the lock")
	releaseInitLock(lock)
}
