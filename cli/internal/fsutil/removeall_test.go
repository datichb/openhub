package fsutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveAllWritableRemovesReadOnlyFilesAndFolders(t *testing.T) {
	root := filepath.Join(t.TempDir(), "clone")
	objects := filepath.Join(root, ".git", "objects", "ab")
	require.NoError(t, os.MkdirAll(objects, 0o755))
	obj := filepath.Join(objects, "cdef")
	require.NoError(t, os.WriteFile(obj, []byte("x"), 0o444))
	require.NoError(t, os.Chmod(objects, 0o555))

	require.NoError(t, RemoveAllWritable(root))
	_, err := os.Stat(root)
	assert.True(t, os.IsNotExist(err))

	assert.NoError(t, RemoveAllWritable(root), "a folder that is gone is not an error")
}
