package session

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestListCountResolvePick(t *testing.T) {
	svc, ctx := newTestService(t)
	now := time.Now()
	for i, s := range []domain.Session{
		{ID: "ses_AAA111", State: domain.RunWaiting},
		{ID: "ses_AAB222", State: domain.RunSleeping},
		{ID: "ses_CCC333", State: domain.RunStopped},
		{ID: "legacy", State: ""},
	} {
		s.ProjectID, s.Status, s.StartedAt = "p1", domain.SessionStatusRunning, now.Add(time.Duration(i)*time.Minute)
		if s.ID != "legacy" {
			s.GroupKey = "g1"
		}
		require.NoError(t, svc.Sessions.Create(ctx, &s))
	}
	require.NoError(t, svc.Raise(ctx, &domain.Decision{SessionID: "ses_AAA111", Kind: domain.DecisionPermission, ToolRef: "per_1", Payload: domain.DecisionPayload{Action: "shell", Resources: []string{"npm test"}}}))
	require.NoError(t, svc.Raise(ctx, &domain.Decision{SessionID: "ses_AAA111", Kind: domain.DecisionQuestion, ToolRef: "frm_1"}))
	require.NoError(t, svc.Raise(ctx, &domain.Decision{SessionID: "ses_AAA111", Kind: domain.DecisionQuestion, ToolRef: "frm_2"}))

	views, err := svc.List(ctx, ListFilter{})
	require.NoError(t, err)
	require.Len(t, views, 2, "open v5 sessions only")
	assert.Equal(t, "ses_AAB222", views[0].Session.ID, "most recent first")
	assert.Len(t, views[1].Decisions, 3)
	assert.Equal(t, "? 2  ! 1", DecisionBadges(views[1].Decisions))
	all, _ := svc.List(ctx, ListFilter{All: true})
	assert.Len(t, all, 3)

	c, err := svc.Count(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, Counts{Running: 1, Decisions: 3}, c)

	s, err := svc.Resolve(ctx, "AAA")
	require.NoError(t, err)
	assert.Equal(t, "ses_AAA111", s.ID)
	_, err = svc.Resolve(ctx, "ses_AA")
	assert.ErrorIs(t, err, ErrAmbiguous)
	_, err = svc.Resolve(ctx, "zzz")
	assert.ErrorIs(t, err, domain.ErrNotFound)

	d, err := svc.Pick(ctx, "AAA", domain.DecisionPermission)
	require.NoError(t, err)
	assert.Equal(t, "shell npm test", DecisionSummary(*d))
	_, err = svc.Pick(ctx, "AAA", domain.DecisionQuestion)
	var several *ErrSeveralDecisions
	require.ErrorAs(t, err, &several)
	assert.Len(t, several.Decisions, 2)
	d, err = svc.Pick(ctx, "question:ses_AAA111:frm_2")
	require.NoError(t, err)
	assert.Equal(t, "frm_2", d.ToolRef)
	_, err = svc.Pick(ctx, "AAB", domain.DecisionPermission)
	assert.ErrorIs(t, err, ErrNoDecision)
}

func TestFormat(t *testing.T) {
	q := domain.Decision{Kind: domain.DecisionQuestion, Payload: domain.DecisionPayload{Title: "Questions",
		Fields: []domain.DecisionField{{Key: "q0", Description: "Which colour?"}, {Key: "q1"}}}}
	assert.Equal(t, "Which colour? (+1)", DecisionSummary(q))
	assert.Equal(t, "boom", DecisionSummary(domain.Decision{Kind: domain.DecisionError, Payload: domain.DecisionPayload{Message: "boom\nstack"}})[:4])
	assert.Equal(t, "⏸", KindIcon(domain.DecisionCheckpoint))
	assert.Equal(t, "◌", StateIcon(domain.RunSleeping))

	assert.Equal(t, "→ reviewer", FeedLine(domain.FeedItem{Kind: domain.FeedAgent, Agent: "reviewer"}))
	assert.Equal(t, "developer › shell go test ✔", FeedLine(domain.FeedItem{Kind: domain.FeedTool, Agent: "developer", Tool: "shell", Title: "go test", Status: "ok"}))
	assert.Equal(t, "⤷ helper (math)", FeedLine(domain.FeedItem{Kind: domain.FeedDelegate, Agent: "helper", Title: "math"}))
	assert.Equal(t, "", FeedLine(domain.FeedItem{Kind: domain.FeedUsage, Cost: 1}))
	assert.Equal(t, "· cancelled", FeedLine(domain.FeedItem{Kind: domain.FeedState, Status: "cancelled"}))
}
