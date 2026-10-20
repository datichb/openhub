package gateway

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func captureLogs(t *testing.T) *logBuffer {
	t.Helper()
	logs := &logBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return logs
}

func issued(t *testing.T) (*Store, string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gateway.json")
	st, err := OpenStore(path)
	require.NoError(t, err)
	tok, err := st.Issue(Grant{SessionID: "ses_a", GroupKey: "g1", Location: "/p"})
	require.NoError(t, err)
	return st, path, tok
}

// S-29: a revocation that cannot be written is logged and written again, so
// that the grant does not come back with the next daemon.
func TestRevokeSavesAgainAfterAnError(t *testing.T) {
	logs := captureLogs(t)
	st, path, tok := issued(t)
	writes := 0
	st.writeFile = func(name string, data []byte, perm os.FileMode) error {
		writes++
		if writes == 1 {
			return errors.New("no space left on device")
		}
		return os.WriteFile(name, data, perm)
	}

	st.RevokeOwner("g1")

	assert.Equal(t, 2, writes)
	_, ok := st.Lookup(tok)
	assert.False(t, ok)
	reopened, err := OpenStore(path)
	require.NoError(t, err)
	assert.Zero(t, reopened.Len(), "the revocation reached the file")
	assert.Contains(t, logs.String(), "saving the revoked grants failed, trying again")
	assert.Contains(t, logs.String(), "no space left on device")
}

func TestRevokeLogsWhenTheSecondSaveFails(t *testing.T) {
	logs := captureLogs(t)
	st, _, tok := issued(t)
	writes := 0
	st.writeFile = func(string, []byte, os.FileMode) error {
		writes++
		return errors.New("read-only file system")
	}

	st.RevokeSession("ses_a")

	assert.Equal(t, 2, writes, "one retry, no more")
	_, ok := st.Lookup(tok)
	assert.False(t, ok, "revoked in memory anyway")
	assert.Contains(t, logs.String(), "revoked grants not saved")
}
