package session

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
)

func TestFollowFiltersTheSession(t *testing.T) {
	svc, ctx := newTestService(t)
	_, err := svc.Follow(ctx, "ses_a")
	assert.ErrorIs(t, err, ErrNoLive)

	svc.Live = func(context.Context, string) (<-chan daemon.StreamEvent, error) { return nil, daemon.ErrNotRunning }
	_, err = svc.Follow(ctx, "ses_a")
	assert.ErrorIs(t, err, ErrNoLive)

	src := make(chan daemon.StreamEvent, 4)
	src <- daemon.StreamEvent{Feed: &domain.FeedItem{SessionID: "ses_a", Kind: domain.FeedText, Text: "hi"}}
	src <- daemon.StreamEvent{Change: &domain.SessionChange{SessionID: "ses_a"}}
	src <- daemon.StreamEvent{Feed: &domain.FeedItem{SessionID: "ses_b", Kind: domain.FeedText}}
	close(src)
	svc.Live = func(_ context.Context, id string) (<-chan daemon.StreamEvent, error) {
		assert.Equal(t, "ses_a", id)
		return src, nil
	}
	feed, err := svc.Follow(ctx, "ses_a")
	require.NoError(t, err)
	var got []domain.FeedItem
	for it := range feed {
		got = append(got, it)
	}
	require.Len(t, got, 1)
	assert.Equal(t, "hi", got[0].Text)
}

// A21: the live feed shows the oh session total (root + subagents), not the
// root tool session alone.
func TestFollowShowsTheSessionTotal(t *testing.T) {
	svc, ctx := newTestService(t)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", Cost: 3.41}))
	src := make(chan daemon.StreamEvent, 2)
	src <- daemon.StreamEvent{Feed: &domain.FeedItem{SessionID: "ses_a", Kind: domain.FeedUsage, Cost: 0.88}}
	close(src)
	svc.Live = func(context.Context, string) (<-chan daemon.StreamEvent, error) { return src, nil }
	feed, err := svc.Follow(ctx, "ses_a")
	require.NoError(t, err)
	it := <-feed
	assert.InDelta(t, 3.41, it.Cost, 1e-9)
}

// listSignal reports the end of each poll (ListOpen is read last).
type listSignal struct {
	domain.DecisionStore
	polled chan struct{}
}

func (l listSignal) ListOpen(ctx context.Context, f domain.DecisionFilter) ([]domain.Decision, error) {
	out, err := l.DecisionStore.ListOpen(ctx, f)
	select {
	case l.polled <- struct{}{}:
	default:
	}
	return out, err
}

func TestSubscribePollsWithoutDaemon(t *testing.T) {
	svc, ctx := newTestService(t)
	svc.PollEvery = 30 * time.Millisecond
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunActive}))

	// The changes come after the first poll (the baseline), however late it
	// runs under load.
	polled := make(chan struct{}, 1)
	svc.Decisions = listSignal{DecisionStore: svc.Decisions, polled: polled}
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	changes := svc.Subscribe(cctx)
	select {
	case <-polled:
	case <-time.After(5 * time.Second):
		t.Fatal("no first poll")
	}
	s, _ := svc.Sessions.Get(ctx, "ses_a")
	s.State = domain.RunWaiting
	require.NoError(t, svc.Sessions.Update(ctx, s))
	require.NoError(t, svc.Raise(ctx, &domain.Decision{SessionID: "ses_a", Kind: domain.DecisionError}))
	select {
	case c := <-changes:
		assert.Equal(t, "ses_a", c.SessionID)
		assert.Equal(t, domain.RunWaiting, c.State)
		assert.True(t, c.Decisions)
	case <-time.After(2 * time.Second):
		t.Fatal("no change")
	}
}

func TestSubscribeUsesTheStream(t *testing.T) {
	svc, ctx := newTestService(t)
	src := make(chan daemon.StreamEvent, 1)
	src <- daemon.StreamEvent{Change: &domain.SessionChange{SessionID: "ses_x", State: domain.RunIdle}}
	svc.Live = func(context.Context, string) (<-chan daemon.StreamEvent, error) { return src, nil }
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	select {
	case c := <-svc.Subscribe(cctx):
		assert.Equal(t, "ses_x", c.SessionID)
	case <-time.After(2 * time.Second):
		t.Fatal("no change")
	}
}
