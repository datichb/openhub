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

func TestSubscribePollsWithoutDaemon(t *testing.T) {
	svc, ctx := newTestService(t)
	svc.PollEvery = 30 * time.Millisecond
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunActive}))

	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	changes := svc.Subscribe(cctx)
	time.Sleep(60 * time.Millisecond) // first poll = baseline
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
