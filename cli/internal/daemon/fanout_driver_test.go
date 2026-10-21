package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/filelock"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

// p6-debts: the driver of a fan-out that stopped before the merge
// checkpoint is started again by the daemon, one at a time.
func TestEnsureFanoutDriver(t *testing.T) {
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	sessions := sqlite.NewSessionStore(st)
	add := func(id string, state domain.RunState, f *domain.SessionFanout) {
		require.NoError(t, sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, State: state, Fanout: f}))
	}
	parent := func(s domain.FanoutStatus) *domain.SessionFanout {
		return &domain.SessionFanout{Role: domain.RoleParent, Experimental: true, Status: s}
	}
	add("ses_ready", domain.RunIdle, parent(domain.FanoutReady))
	add("ses_waiting", domain.RunCompleted, parent(domain.FanoutWaiting))
	add("ses_merged", domain.RunIdle, parent(domain.FanoutMerged))
	add("ses_stopped", domain.RunStopped, parent(domain.FanoutRunning))
	add("ses_child", domain.RunIdle, &domain.SessionFanout{Role: domain.RoleChild, Experimental: true, Status: domain.FanoutRunning})
	add("ses_plain", domain.RunIdle, nil)

	dir := t.TempDir()
	var started []string
	d := &Daemon{opts: Options{Sessions: sessions, SessionsDir: dir, FanoutDriver: func(_ context.Context, id string) error {
		started = append(started, id)
		return nil
	}}}
	for _, id := range []string{"ses_ready", "ses_waiting", "ses_merged", "ses_stopped", "ses_child", "ses_plain", "ses_none"} {
		d.ensureFanoutDriver(ctx, id)
	}
	assert.Equal(t, []string{"ses_ready", "ses_waiting"}, started, "only fan-outs not over")

	d.ensureFanoutDriver(ctx, "ses_ready")
	assert.Len(t, started, 2, "at most once per driverRetry")

	// A driver that runs holds its lock: not started twice.
	old := driverRetry
	driverRetry = 0
	t.Cleanup(func() { driverRetry = old })
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ses_ready"), 0o700))
	unlock, err := filelock.TryLock(filepath.Join(dir, "ses_ready", "fanout.lock"))
	require.NoError(t, err)
	d.ensureFanoutDriver(ctx, "ses_ready")
	assert.Len(t, started, 2, "a driver runs")
	unlock()
	time.Sleep(time.Millisecond)
	d.ensureFanoutDriver(ctx, "ses_ready")
	assert.Len(t, started, 3)
}
