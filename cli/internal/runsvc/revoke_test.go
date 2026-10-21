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
func TestRevokeOwner(t *testing.T) {
	for _, tc := range []struct {
		name        string
		failRevokes int
		calls       int
		revoked     []string
		logged      []string // empty: nothing is logged
	}{
		{"refused once, then revoked", 1, 2, []string{"g1"}, []string{"proxy grants not revoked, trying again", "daemon unreachable"}},
		{"refused twice: one retry, no more", 5, 2, nil, []string{"still not revoked"}},
		{"revoked at once: silent", 0, 1, []string{"g1"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureLogs(t)
			dc := &fakeDaemon{failRevokes: tc.failRevokes}
			revokeOwner(context.Background(), dc, "g1")
			assert.Equal(t, tc.calls, dc.revokeCalls, "revocation attempts")
			assert.Equal(t, tc.revoked, dc.revoked, "revoked owners")
			if len(tc.logged) == 0 {
				assert.Empty(t, logs.String(), "nothing to log")
			}
			for _, msg := range tc.logged {
				assert.Contains(t, logs.String(), msg)
			}
		})
	}
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
