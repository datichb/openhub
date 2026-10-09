package sqlite

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

// N26 (v5 critical): the password of a tool server never stays in oh.db.
func TestServerPasswordsOutOfTheDatabase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "oh.db")
	st, err := Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	ss := NewServerStore(st)
	const pw = "s3cr3t-password-0123456789abcdef"

	require.NoError(t, ss.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "x", Password: pw, Status: domain.ServerReady}))
	got, err := ss.Get(ctx, "g1")
	require.NoError(t, err)
	assert.Equal(t, pw, got.Password, "read back for attach and resume")
	var col string
	require.NoError(t, st.DB().QueryRow(`SELECT password FROM servers WHERE group_key='g1'`).Scan(&col))
	assert.NotContains(t, col, pw)
	assert.Contains(t, col, passwordHashPrefix)
	_, _ = st.DB().Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	for _, f := range []string{path, path + "-wal"} {
		data, _ := os.ReadFile(f)
		assert.False(t, bytes.Contains(data, []byte(pw)), "%s holds no password", f)
	}
	info, err := os.Stat(ss.passwordFile("g1"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	dinfo, _ := os.Stat(filepath.Join(dir, "run", "server-passwords"))
	assert.Equal(t, os.FileMode(0o700), dinfo.Mode().Perm())

	// A password file changed behind oh's back is not used.
	require.NoError(t, os.WriteFile(ss.passwordFile("g1"), []byte("other"), 0o600))
	got, _ = ss.Get(ctx, "g1")
	assert.Empty(t, got.Password)

	// Database of an older oh: moved at start-up.
	_, err = st.DB().Exec(`UPDATE servers SET password = ? WHERE group_key = 'g1'`, pw)
	require.NoError(t, err)
	n, _ := ss.CountClearPasswords(ctx)
	assert.Equal(t, 1, n)
	got, _ = ss.Get(ctx, "g1")
	assert.Equal(t, pw, got.Password, "legacy value still usable before the move")
	moved, err := ss.MovePasswords(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, moved)
	n, _ = ss.CountClearPasswords(ctx)
	assert.Zero(t, n)
	got, _ = ss.Get(ctx, "g1")
	assert.Equal(t, pw, got.Password)
	list, _ := ss.List(ctx)
	require.Len(t, list, 1)
	assert.Equal(t, pw, list[0].Password)
	data, _ := os.ReadFile(path)
	assert.False(t, bytes.Contains(data, []byte(pw)), "free pages vacuumed")

	require.NoError(t, ss.Delete(ctx, "g1"))
	_, err = os.Stat(ss.passwordFile("g1"))
	assert.True(t, os.IsNotExist(err), "password file removed with the server")
}
