package fanout

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

// p6-debts: a driver started again says by whom in the events: the daemon
// (what it always wrote), or the user (from the TUI).
func TestDriverRelaunched(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ by, want string }{
		{RelaunchByDaemon, "started again by the daemon (waiting)"},
		{RelaunchByUser, "started again by the user (waiting)"},
		{"", "started again by the daemon (waiting)"},
	} {
		t.Run(tc.by, func(t *testing.T) {
			f := newFixture(t, "ev-1")
			require.NoError(t, f.driver.Relaunched(context.Background(), "ses_parent", tc.by))
			p, _ := f.store.Get(context.Background(), "ses_parent")
			require.NotEmpty(t, p.Fanout.Events)
			last := p.Fanout.Events[len(p.Fanout.Events)-1]
			assert.Equal(t, domain.FanoutEventDriver, last.Kind)
			assert.Equal(t, tc.want, last.Message)
		})
	}
}

// p6-debts: a merger that gives up (idle, the merge not concluded) is
// stopped after its own guard, not the 10 minutes of a child.
func TestMergerGuard(t *testing.T) {
	t.Parallel()
	d := &Driver{}
	assert.Equal(t, 2*time.Minute, d.mergerGuard())
	d.IdleTimeout = 30 * time.Second
	assert.Equal(t, 30*time.Second, d.mergerGuard())
	d.MergerIdleTimeout = 5 * time.Minute
	assert.Equal(t, 5*time.Minute, d.mergerGuard())
}
