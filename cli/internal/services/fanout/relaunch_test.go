package fanout

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

// p6-debts: a driver started again by the daemon says so in the events.
func TestDriverRelaunched(t *testing.T) {
	f := newFixture(t, "ev-1")
	require.NoError(t, f.driver.Relaunched(context.Background(), "ses_parent"))
	p, _ := f.store.Get(context.Background(), "ses_parent")
	require.NotEmpty(t, p.Fanout.Events)
	assert.Equal(t, domain.FanoutEventDriver, p.Fanout.Events[len(p.Fanout.Events)-1].Kind)
}

// p6-debts: a merger that gives up (idle, the merge not concluded) is
// stopped after its own guard, not the 10 minutes of a child.
func TestMergerGuard(t *testing.T) {
	d := &Driver{}
	assert.Equal(t, 2*time.Minute, d.mergerGuard())
	d.IdleTimeout = 30 * time.Second
	assert.Equal(t, 30*time.Second, d.mergerGuard())
	d.MergerIdleTimeout = 5 * time.Minute
	assert.Equal(t, 5*time.Minute, d.mergerGuard())
}
