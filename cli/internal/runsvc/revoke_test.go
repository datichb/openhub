package runsvc

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// captureLogs sends the default logger to a buffer for the test.
func captureLogs(t *testing.T) *lockedBuffer {
	t.Helper()
	logs := &lockedBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return logs
}

// S-29: a refused revocation is logged and tried once more, not dropped.
func TestRevokeOwnerLogsAndRetries(t *testing.T) {
	logs := captureLogs(t)
	dc := &fakeDaemon{failRevokes: 1}
	revokeOwner(context.Background(), dc, "g1")
	assert.Equal(t, 2, dc.revokeCalls)
	assert.Equal(t, []string{"g1"}, dc.revoked)
	assert.Contains(t, logs.String(), "proxy grants not revoked, trying again")
	assert.Contains(t, logs.String(), "daemon unreachable")
}

func TestRevokeOwnerGivesUpAfterOneRetry(t *testing.T) {
	logs := captureLogs(t)
	dc := &fakeDaemon{failRevokes: 5}
	revokeOwner(context.Background(), dc, "g1")
	assert.Equal(t, 2, dc.revokeCalls, "one retry, no more")
	assert.Empty(t, dc.revoked)
	assert.Contains(t, logs.String(), "still not revoked")
}

func TestRevokeOwnerNoLogOnSuccess(t *testing.T) {
	logs := captureLogs(t)
	dc := &fakeDaemon{}
	revokeOwner(context.Background(), dc, "g1")
	assert.Equal(t, 1, dc.revokeCalls)
	assert.Empty(t, logs.String())
}

// The retry does not depend on the caller's context: Ctrl+C cancels it
// while the cleanup revokes.
func TestRevokeOwnerRetriesAfterCancelledContext(t *testing.T) {
	captureLogs(t)
	ctx, cancel := context.WithCancel(context.Background())
	dc := &ctxDaemon{fakeDaemon: &fakeDaemon{}}
	cancel()
	revokeOwner(ctx, dc, "g1")
	require.Equal(t, 2, dc.revokeCalls)
	assert.Equal(t, []string{"g1"}, dc.revoked, "the second try ran on a live context")
}

type ctxDaemon struct{ *fakeDaemon }

func (d *ctxDaemon) RevokeOwner(ctx context.Context, owner string) error {
	if err := ctx.Err(); err != nil {
		d.mu.Lock()
		d.revokeCalls++
		d.mu.Unlock()
		return errors.New("cancelled: " + err.Error())
	}
	return d.fakeDaemon.RevokeOwner(ctx, owner)
}

// The cleanup of a server that failed its isolation check revokes its
// grants even when the daemon refuses the first time (S-29).
func TestCleanupRevocationRetried(t *testing.T) {
	logs := captureLogs(t)
	f := newRTFixture(t)
	f.ad.attestErr = errors.New("tool API down")
	f.dc.failRevokes = 1
	_, err := f.svc.StartSession(context.Background(), f.request(f.project))
	require.Error(t, err)
	assert.Equal(t, 2, f.dc.revokeCalls)
	assert.NotEmpty(t, f.dc.revoked)
	assert.Contains(t, logs.String(), "proxy grants not revoked, trying again")
}
