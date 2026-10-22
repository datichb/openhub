package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var writers = map[string]func(string, []byte, os.FileMode) error{
	"sync":   WriteFileAtomic,
	"nosync": WriteFileAtomicNoSync,
}

func entries(t *testing.T, dir string) []string {
	t.Helper()
	list, err := os.ReadDir(dir)
	require.NoError(t, err)
	var names []string
	for _, e := range list {
		names = append(names, e.Name())
	}
	return names
}

func TestWriteFileAtomic_CreatesAndReplaces(t *testing.T) {
	for name, write := range writers {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "f.txt")

			require.NoError(t, write(path, []byte("one"), 0o600))
			got, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, "one", string(got))

			require.NoError(t, write(path, []byte("two"), 0o600))
			got, err = os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, "two", string(got))

			assert.Equal(t, []string{"f.txt"}, entries(t, dir), "no temporary file left behind")
		})
	}
}

func TestWriteFileAtomic_ExactPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	old := setUmask(0o077)
	defer setUmask(old)
	for name, write := range writers {
		for _, perm := range []os.FileMode{0o600, 0o644, 0o755} {
			t.Run(name+"/"+perm.String(), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "f")
				require.NoError(t, write(path, []byte("x"), perm))
				info, err := os.Stat(path)
				require.NoError(t, err)
				assert.Equal(t, perm, info.Mode().Perm())
			})
		}
	}
}

func TestWriteFileAtomic_ReplacementTakesNewPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	path := filepath.Join(t.TempDir(), "f")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o666))
	require.NoError(t, WriteFileAtomic(path, []byte("new"), 0o600))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestWriteFileAtomic_MissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent", "f")
	err := WriteFileAtomic(path, []byte("x"), 0o600)
	require.Error(t, err)
}

func TestWriteFileAtomic_RenameFailureCleansUp(t *testing.T) {
	for name, write := range writers {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			// A non-empty directory cannot be replaced by a file: rename fails.
			path := filepath.Join(dir, "target")
			require.NoError(t, os.MkdirAll(filepath.Join(path, "child"), 0o755))

			err := write(path, []byte("x"), 0o600)
			require.Error(t, err)
			assert.Equal(t, []string{"target"}, entries(t, dir), "temporary file removed")
			assert.DirExists(t, filepath.Join(path, "child"), "the target is untouched")
		})
	}
}

func TestWriteFileAtomic_RelativePath(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, WriteFileAtomic("rel.txt", []byte("x"), 0o644))
	got, err := os.ReadFile("rel.txt")
	require.NoError(t, err)
	assert.Equal(t, "x", string(got))
	assert.Equal(t, []string{"rel.txt"}, entries(t, "."))
}

func TestWriteFileAtomic_ConcurrentWritersUseDistinctTemporaries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- WriteFileAtomic(path, []byte(strings.Repeat("x", i+1)), 0o600)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	assert.Equal(t, []string{"f"}, entries(t, dir))
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("x", len(got)), string(got), "the file is one complete write")
}
