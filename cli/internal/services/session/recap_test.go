package session

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestRecap(t *testing.T) {
	svc, ctx := newTestService(t)
	since := time.Now().Add(-time.Minute)
	before, after := since.Add(-time.Hour), time.Now()
	for _, s := range []domain.Session{
		{ID: "ses_w", State: domain.RunWaiting, StateChangedAt: &after, Cost: 0.5},
		{ID: "ses_s", State: domain.RunSleeping, StateChangedAt: &after, Cost: 0.25},
		{ID: "ses_d", State: domain.RunStopped, StateChangedAt: &after},
		{ID: "ses_old", State: domain.RunIdle, StateChangedAt: &before, Cost: 9},
	} {
		s.ProjectID, s.Status, s.GroupKey = "p1", domain.SessionStatusRunning, "g1"
		require.NoError(t, svc.Sessions.Create(ctx, &s))
	}
	require.NoError(t, svc.Raise(ctx, &domain.Decision{SessionID: "ses_w", Kind: domain.DecisionQuestion, ToolRef: "frm_1"}))

	r, err := svc.Recap(ctx, since)
	require.NoError(t, err)
	assert.Equal(t, AbsenceRecap{Changed: 3, Waiting: 1, Sleeping: 1, Finished: 1, Cost: 0.75, NewDecisions: 1, OpenDecisions: 1}, r)
	assert.False(t, r.Empty())

	r, err = svc.Recap(ctx, time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.True(t, r.Empty())
}
