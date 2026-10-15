package fanout

import (
	"context"
	"testing"

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
