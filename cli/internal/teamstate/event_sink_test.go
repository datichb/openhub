package teamstate

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collector is a sink that keeps what it receives.
type collector struct {
	mu  sync.Mutex
	got []Event
}

func (c *collector) sink(_ context.Context, _ EventSource, e Event) {
	c.mu.Lock()
	c.got = append(c.got, e)
	c.mu.Unlock()
}

func (c *collector) types() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, e := range c.got {
		out = append(out, e.Type+"/"+e.Actor)
	}
	return out
}

// installSink sets the sink of the program for one test. The tests that use it
// do not run in parallel: the sink is global.
func installSink(t *testing.T) *collector {
	t.Helper()
	c := &collector{}
	SetEventSink(c.sink)
	t.Cleanup(func() { SetEventSink(nil) })
	return c
}

// An event reaches the sink once its transaction is committed, whoever wrote
// it: AppendEvent, or the transaction of a publication.
func TestEventsReachTheSinkAfterTheCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("runs git")
	}
	c := installSink(t)
	a, _ := twoClones(t)
	ctx := context.Background()

	require.NoError(t, a.AppendEvent(ctx, Event{Actor: "alice", Type: EventClaimTaken, Project: "api", Ticket: "bd-7"}))
	require.True(t, WaitEventSinks(5*time.Second))
	assert.Equal(t, []string{"claim.taken/alice"}, c.types())

	require.NoError(t, a.UpdateConfig(WithActor(ctx, "alice"), func(cfg *TeamConfig) error {
		cfg.Tracker.Type = "jira"
		return nil
	}))
	require.True(t, WaitEventSinks(5*time.Second))
	assert.Equal(t, []string{"claim.taken/alice", "config_changed/alice"}, c.types(), "the config_changed of a publication too")
}

// A build that fails, or a transaction whose push is refused and rebuilt, announces nothing twice.
func TestEventsOfARebuiltOrFailedTransactionAreNotRepeated(t *testing.T) {
	if testing.Short() {
		t.Skip("runs git")
	}
	c := installSink(t)
	a, b := twoClones(t)
	ctx := context.Background()

	err := a.Transact(ctx, func(ctx context.Context, tx *Tx) (TxResult, error) {
		if err := tx.AppendEvent(Event{Actor: "alice", Type: EventClaimReleased, Project: "api"}); err != nil {
			return TxResult{}, err
		}
		return TxResult{}, errors.New("boom")
	})
	require.Error(t, err)
	require.True(t, WaitEventSinks(5*time.Second))
	assert.Empty(t, c.types(), "a failed build announces nothing")

	a.beforePush = func() {
		a.beforePush = nil
		require.NoError(t, b.AppendEvent(ctx, Event{Actor: "bob", Type: EventClaimTaken, Project: "api", Ticket: "bd-1"}))
	}
	require.NoError(t, a.Transact(ctx, func(ctx context.Context, tx *Tx) (TxResult, error) {
		return TxResult{Message: "x"}, tx.AppendEvent(Event{Actor: "alice", Type: EventClaimTransferred, Project: "api", Ticket: "bd-2", Data: map[string]any{"to": "bob"}})
	}))
	require.True(t, WaitEventSinks(5*time.Second))
	assert.ElementsMatch(t, []string{"claim.taken/bob", "claim.transferred/alice"}, c.types(), "each event once, although alice's push was refused and rebuilt")
}
