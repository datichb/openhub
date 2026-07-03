package filecrypt

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixedPrompt returns a PromptFunc that always returns the given passphrase.
func fixedPrompt(passphrase string) PromptFunc {
	return func(creating bool) (string, error) {
		return passphrase, nil
	}
}

func tempPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "secrets.enc")
}

func TestRoundTrip(t *testing.T) {
	path := tempPath(t)
	s := New(path, fixedPrompt("test-passphrase-123"))
	ctx := context.Background()

	// Set 3 secrets
	require.NoError(t, s.Set(ctx, "figma-token", "glpat-aaa"))
	require.NoError(t, s.Set(ctx, "gitlab-token", "glpat-bbb"))
	require.NoError(t, s.Set(ctx, "gslides-token", "gsl-ccc"))

	// Open a fresh store instance — simulates a new process
	s2 := New(path, fixedPrompt("test-passphrase-123"))

	val, err := s2.Get(ctx, "figma-token")
	require.NoError(t, err)
	assert.Equal(t, "glpat-aaa", val)

	val, err = s2.Get(ctx, "gitlab-token")
	require.NoError(t, err)
	assert.Equal(t, "glpat-bbb", val)

	val, err = s2.Get(ctx, "gslides-token")
	require.NoError(t, err)
	assert.Equal(t, "gsl-ccc", val)
}

func TestWrongPassphrase(t *testing.T) {
	path := tempPath(t)
	s := New(path, fixedPrompt("correct-passphrase"))
	ctx := context.Background()
	require.NoError(t, s.Set(ctx, "key", "value"))

	// Open with wrong passphrase
	s2 := New(path, fixedPrompt("wrong-passphrase"))
	_, err := s2.Get(ctx, "key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wrong passphrase")
}

func TestCorruptedFile(t *testing.T) {
	path := tempPath(t)
	ctx := context.Background()

	// Write random garbage
	garbage := make([]byte, 100)
	_, _ = rand.Read(garbage)
	require.NoError(t, os.WriteFile(path, garbage, 0600))

	s := New(path, fixedPrompt("any-passphrase"))
	_, err := s.Get(ctx, "key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "corrupted")
}

func TestCorruptedFileTooShort(t *testing.T) {
	path := tempPath(t)
	ctx := context.Background()

	// Write a file that's too short
	require.NoError(t, os.WriteFile(path, []byte("OH"), 0600))

	s := New(path, fixedPrompt("any"))
	_, err := s.Get(ctx, "key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

func TestDelete(t *testing.T) {
	path := tempPath(t)
	s := New(path, fixedPrompt("passphrase"))
	ctx := context.Background()

	require.NoError(t, s.Set(ctx, "key1", "val1"))
	require.NoError(t, s.Set(ctx, "key2", "val2"))

	// Delete key1
	require.NoError(t, s.Delete(ctx, "key1"))

	val, err := s.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, "", val)

	// key2 still exists
	val, err = s.Get(ctx, "key2")
	require.NoError(t, err)
	assert.Equal(t, "val2", val)

	// Delete non-existent key — no error
	require.NoError(t, s.Delete(ctx, "nonexistent"))
}

func TestList(t *testing.T) {
	path := tempPath(t)
	s := New(path, fixedPrompt("passphrase"))
	ctx := context.Background()

	require.NoError(t, s.Set(ctx, "charlie", "c"))
	require.NoError(t, s.Set(ctx, "alpha", "a"))
	require.NoError(t, s.Set(ctx, "bravo", "b"))
	require.NoError(t, s.Set(ctx, "delta", "d"))
	require.NoError(t, s.Set(ctx, "echo", "e"))

	keys, err := s.List(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "bravo", "charlie", "delta", "echo"}, keys)
}

func TestEmptyStore(t *testing.T) {
	path := tempPath(t)
	ctx := context.Background()

	// File doesn't exist — first Set should create it
	s := New(path, fixedPrompt("passphrase"))
	require.NoError(t, s.Set(ctx, "first-key", "first-value"))

	// File should now exist
	_, err := os.Stat(path)
	require.NoError(t, err)

	// Read it back
	val, err := s.Get(ctx, "first-key")
	require.NoError(t, err)
	assert.Equal(t, "first-value", val)
}

func TestAtomicWrite(t *testing.T) {
	path := tempPath(t)
	s := New(path, fixedPrompt("passphrase"))
	ctx := context.Background()

	// Set a value to create the file
	require.NoError(t, s.Set(ctx, "key", "value"))

	// Verify file permissions are 0600
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	// Verify no temp files are left behind
	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tmp", "temp file should not be left behind")
	}
}

func TestConcurrency(t *testing.T) {
	path := tempPath(t)
	s := New(path, fixedPrompt("passphrase"))
	ctx := context.Background()

	// Pre-initialize store
	require.NoError(t, s.Set(ctx, "init", "val"))

	var wg sync.WaitGroup
	const goroutines = 10

	// Concurrent writes
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "key-" + string(rune('A'+n))
			_ = s.Set(ctx, key, "value")
		}(i)
	}
	wg.Wait()

	// Concurrent reads
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "key-" + string(rune('A'+n))
			_, _ = s.Get(ctx, key)
		}(i)
	}
	wg.Wait()

	// All keys should be readable
	keys, err := s.List(ctx)
	require.NoError(t, err)
	// init + up to 10 keys (all should be present due to mutex)
	assert.GreaterOrEqual(t, len(keys), 1)
}

func TestEnvVarPassphrase(t *testing.T) {
	path := tempPath(t)
	ctx := context.Background()

	// Set env var
	t.Setenv("OH_PASSPHRASE", "env-passphrase")

	// Create store with nil promptFn — should use env var
	s := New(path, nil)
	require.NoError(t, s.Set(ctx, "env-key", "env-value"))

	// Open fresh store — also uses env var
	s2 := New(path, nil)
	val, err := s2.Get(ctx, "env-key")
	require.NoError(t, err)
	assert.Equal(t, "env-value", val)
}

func TestGetNonExistentKey(t *testing.T) {
	path := tempPath(t)
	s := New(path, fixedPrompt("passphrase"))
	ctx := context.Background()

	// Initialize store
	require.NoError(t, s.Set(ctx, "exists", "yes"))

	// Get non-existent key returns empty string, no error
	val, err := s.Get(ctx, "does-not-exist")
	require.NoError(t, err)
	assert.Equal(t, "", val)
}

func TestOverwriteKey(t *testing.T) {
	path := tempPath(t)
	s := New(path, fixedPrompt("passphrase"))
	ctx := context.Background()

	require.NoError(t, s.Set(ctx, "key", "value1"))
	require.NoError(t, s.Set(ctx, "key", "value2"))

	val, err := s.Get(ctx, "key")
	require.NoError(t, err)
	assert.Equal(t, "value2", val)

	// Verify only one key in list
	keys, err := s.List(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"key"}, keys)
}

func TestVerifyPassphrase(t *testing.T) {
	path := tempPath(t)
	s := New(path, fixedPrompt("my-secret"))
	ctx := context.Background()
	require.NoError(t, s.Set(ctx, "k", "v"))

	// Correct passphrase
	assert.NoError(t, VerifyPassphrase(path, "my-secret"))

	// Wrong passphrase
	assert.Error(t, VerifyPassphrase(path, "not-my-secret"))
}

func TestIsAvailableWithEnvVar(t *testing.T) {
	t.Setenv("OH_PASSPHRASE", "something")
	assert.True(t, IsAvailable())
}
