package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/daemon/api"
	"github.com/datichb/openhub/cli/internal/domain"
)

func TestFollowFiltersTheSession(t *testing.T) {
	svc, ctx := newTestService(t)
	_, err := svc.Follow(ctx, "ses_a")
	assert.ErrorIs(t, err, ErrNoLive)

	svc.Live = func(context.Context, string) (<-chan api.StreamEvent, error) { return nil, api.ErrNotRunning }
	_, err = svc.Follow(ctx, "ses_a")
	assert.ErrorIs(t, err, ErrNoLive)

	src := make(chan api.StreamEvent, 5)
	src <- api.StreamEvent{Feed: &domain.FeedItem{SessionID: "ses_a", Kind: domain.FeedText, Text: "hi"}}
	src <- api.StreamEvent{Change: &domain.SessionChange{SessionID: "ses_a"}}
	src <- api.StreamEvent{Feed: &domain.FeedItem{SessionID: "ses_b", Kind: domain.FeedText}}
	src <- api.StreamEvent{Resync: true, Seq: 4} // nothing to show for it
	close(src)
	svc.Live = func(_ context.Context, id string) (<-chan api.StreamEvent, error) {
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

// The live feed shows the oh session total (root + subagents), not the
// root tool session alone.
func TestFollowShowsTheSessionTotal(t *testing.T) {
	svc, ctx := newTestService(t)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", Cost: 3.41}))
	src := make(chan api.StreamEvent, 2)
	src <- api.StreamEvent{Feed: &domain.FeedItem{SessionID: "ses_a", Kind: domain.FeedUsage, Cost: 0.88}}
	close(src)
	svc.Live = func(context.Context, string) (<-chan api.StreamEvent, error) { return src, nil }
	feed, err := svc.Follow(ctx, "ses_a")
	require.NoError(t, err)
	it := <-feed
	assert.InDelta(t, 3.41, it.Cost, 1e-9)
}

// Without a live stream there is nothing to follow, and nothing is read from
// the stores either: the channel stays quiet and closes with the context.
func TestSubscribeWithoutStreamDoesNotPoll(t *testing.T) {
	svc, ctx := newTestService(t)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunActive}))
	svc.ReconnectEvery = time.Millisecond
	cctx, cancel := context.WithCancel(ctx)
	changes := svc.Subscribe(cctx)
	s, _ := svc.Sessions.Get(ctx, "ses_a")
	s.State = domain.RunWaiting
	require.NoError(t, svc.Sessions.Update(ctx, s))
	require.NoError(t, svc.Raise(ctx, &domain.Decision{SessionID: "ses_a", Kind: domain.DecisionError}))
	cancel()
	select {
	case c, ok := <-changes:
		assert.False(t, ok, "no change was read from the stores: %+v", c)
	case <-time.After(5 * time.Second):
		t.Fatal("the channel did not close")
	}
}

// A resync from the daemon (entries may be missing) becomes a "reload
// everything" change, between the changes that came before and after it.
func TestSubscribeReloadsOnResync(t *testing.T) {
	svc, ctx := newTestService(t)
	src := make(chan api.StreamEvent, 3)
	src <- api.StreamEvent{Change: &domain.SessionChange{SessionID: "ses_1"}}
	src <- api.StreamEvent{Resync: true, Seq: 9}
	src <- api.StreamEvent{Change: &domain.SessionChange{SessionID: "ses_2"}}
	svc.Live = func(context.Context, string) (<-chan api.StreamEvent, error) { return src, nil }
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	changes := svc.Subscribe(cctx)
	got := nextIDs(t, changes, 3)
	assert.Equal(t, []string{"ses_1", "", "ses_2"}, got)
}

// A stream that ended or could not be opened is tried again; once it is open
// after a gap, the changes of the gap are unknown: a reload comes first.
func TestSubscribeReconnectsAndReloads(t *testing.T) {
	svc, ctx := newTestService(t)
	svc.ReconnectEvery = time.Millisecond
	first := make(chan api.StreamEvent, 1)
	first <- api.StreamEvent{Change: &domain.SessionChange{SessionID: "ses_1"}}
	close(first) // the daemon dropped the stream
	last := make(chan api.StreamEvent, 1)
	last <- api.StreamEvent{Change: &domain.SessionChange{SessionID: "ses_2"}}
	calls := 0
	svc.Live = func(context.Context, string) (<-chan api.StreamEvent, error) {
		switch calls++; calls {
		case 1:
			return first, nil
		case 2:
			return nil, api.ErrNotRunning
		default:
			return last, nil
		}
	}
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	got := nextIDs(t, svc.Subscribe(cctx), 3)
	assert.Equal(t, []string{"ses_1", "", "ses_2"}, got)
}

func nextIDs(t *testing.T, ch <-chan domain.SessionChange, n int) []string {
	t.Helper()
	var ids []string
	for len(ids) < n {
		select {
		case c, ok := <-ch:
			require.True(t, ok, "the channel closed")
			ids = append(ids, c.SessionID)
		case <-time.After(10 * time.Second):
			t.Fatalf("only %v", ids)
		}
	}
	return ids
}

func TestSubscribeUsesTheStream(t *testing.T) {
	svc, ctx := newTestService(t)
	src := make(chan api.StreamEvent, 1)
	src <- api.StreamEvent{Change: &domain.SessionChange{SessionID: "ses_x", State: domain.RunIdle}}
	svc.Live = func(context.Context, string) (<-chan api.StreamEvent, error) { return src, nil }
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	select {
	case c := <-svc.Subscribe(cctx):
		assert.Equal(t, "ses_x", c.SessionID)
	case <-time.After(2 * time.Second):
		t.Fatal("no change")
	}
}

// A panic of the live stream is logged and the subscription starts again:
// the views that read it do not wait on a channel that closed without a word.
func TestSubscribeSurvivesAPanic(t *testing.T) {
	svc, ctx := newTestService(t)
	src := make(chan api.StreamEvent, 1)
	src <- api.StreamEvent{Change: &domain.SessionChange{SessionID: "ses_after", State: domain.RunIdle}}
	svc.ReconnectEvery = time.Millisecond
	calls := 0
	svc.Live = func(context.Context, string) (<-chan api.StreamEvent, error) {
		if calls++; calls == 1 {
			panic("the stream exploded")
		}
		return src, nil
	}
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// The panic is a gap: the reload comes first, then the stream.
	assert.Equal(t, []string{"", "ses_after"}, nextIDs(t, svc.Subscribe(cctx), 2))
}

func TestFollowOtherErrorAndCancel(t *testing.T) {
	svc, ctx := newTestService(t)
	boom := errors.New("boom")
	svc.Live = func(context.Context, string) (<-chan api.StreamEvent, error) { return nil, boom }
	_, err := svc.Follow(ctx, "ses_a")
	require.ErrorIs(t, err, boom)

	// A reader that leaves while the feed is full stops the follow.
	src := make(chan api.StreamEvent, 200)
	for range 200 {
		src <- api.StreamEvent{Feed: &domain.FeedItem{SessionID: "ses_a", Kind: domain.FeedText}}
	}
	svc.Live = func(context.Context, string) (<-chan api.StreamEvent, error) { return src, nil }
	cctx, cancel := context.WithCancel(ctx)
	feed, err := svc.Follow(cctx, "ses_a")
	require.NoError(t, err)
	cancel()
	require.Eventually(t, func() bool {
		for {
			select {
			case _, ok := <-feed:
				if !ok {
					return true
				}
			default:
				return false
			}
		}
	}, 5*time.Second, 5*time.Millisecond)
}
