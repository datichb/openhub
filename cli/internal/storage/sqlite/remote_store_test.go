package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestRemoteStore(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	seedProject(t, st, "p1")
	ss := NewSessionStore(st)
	require.NoError(t, ss.Create(ctx, &domain.Session{ID: "ses_r", ProjectID: "p1", Status: domain.SessionStatusRunning, Runtime: "remote"}))
	require.NoError(t, ss.Create(ctx, &domain.Session{ID: "ses_l", ProjectID: "p1", Status: domain.SessionStatusRunning}))
	rs := NewRemoteStore(st)

	got, err := rs.GetRemoteRef(ctx, "ses_l")
	require.NoError(t, err)
	assert.Nil(t, got, "local session")
	_, err = rs.GetRemoteRef(ctx, "ses_x")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.ErrorIs(t, rs.SetRemoteRef(ctx, "ses_x", domain.RemoteRef{}), domain.ErrNotFound)

	ref := domain.RemoteRef{Target: "acme", Pipeline: 812, Status: domain.RemoteSent, Tickets: []string{"bd-42"}, SentAt: time.Now().UTC().Truncate(time.Second)}
	require.NoError(t, rs.SetRemoteRef(ctx, "ses_r", ref))
	got, err = rs.GetRemoteRef(ctx, "ses_r")
	require.NoError(t, err)
	assert.Equal(t, ref.Pipeline, got.Pipeline)
	assert.Equal(t, ref.Tickets, got.Tickets)
	assert.True(t, ref.SentAt.Equal(got.SentAt))

	// A session update (other columns) keeps the reference.
	s, err := ss.Get(ctx, "ses_r")
	require.NoError(t, err)
	s.State = domain.RunActive
	require.NoError(t, ss.Update(ctx, s))
	all, err := rs.ListRemote(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 1)
	assert.Equal(t, int64(812), all["ses_r"].Pipeline)
}
