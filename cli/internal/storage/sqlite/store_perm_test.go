//go:build !windows

package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// E14-M4: pragmas apply to every pooled connection; files are owner-only.
func TestOpenPragmasAndPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "hub")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, "oh.db")
	st, err := Open(path)
	require.NoError(t, err)
	defer st.Close()

	ctx := context.Background()
	conns := make([]interface{ Close() error }, 0, 3)
	for i := 0; i < 3; i++ {
		c, err := st.DB().Conn(ctx)
		require.NoError(t, err)
		conns = append(conns, c)
		var timeout, fk int
		require.NoError(t, c.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&timeout))
		require.NoError(t, c.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk))
		assert.Equal(t, 5000, timeout)
		assert.Equal(t, 1, fk)
	}
	for _, c := range conns {
		c.Close()
	}
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		fi, err := os.Stat(p)
		if os.IsNotExist(err) {
			continue
		}
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm(), p)
	}
	fi, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), fi.Mode().Perm())
}
