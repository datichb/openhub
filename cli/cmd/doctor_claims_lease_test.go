package cmd

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

func TestExpiredClaims(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) teamstate.Instant { return teamstate.InstantOf(now.Add(d)) }
	cfg := &teamstate.TeamConfig{}
	claims := []teamstate.Claim{
		{TicketID: "lease-past", ClaimedBy: "me", Status: teamstate.ClaimStatusInProgress, LeaseUntil: at(-time.Hour)},
		{TicketID: "lease-ok", ClaimedBy: "me", Status: teamstate.ClaimStatusInProgress, LeaseUntil: at(5 * time.Hour)},
		{TicketID: "legacy-stale", ClaimedBy: "me", Status: teamstate.ClaimStatusInProgress, LastActivity: at(-5 * 24 * time.Hour)},
		{TicketID: "legacy-fresh", ClaimedBy: "me", Status: teamstate.ClaimStatusInProgress, LastActivity: at(-time.Hour)},
		{TicketID: "ended", ClaimedBy: "me", Status: teamstate.ClaimStatusInProgress, LeaseUntil: at(5 * time.Hour), Labels: []string{teamstate.LabelSessionEnded}},
		{TicketID: "bob", ClaimedBy: "bob", Status: teamstate.ClaimStatusInProgress, LeaseUntil: at(-time.Hour)},
		{TicketID: "done", ClaimedBy: "me", Status: teamstate.ClaimStatusDone, LeaseUntil: at(-time.Hour)},
		{TicketID: "planned", ClaimedBy: "me", Status: teamstate.ClaimStatusPlanned, LastActivity: at(-9 * 24 * time.Hour)},
	}
	got := expiredClaims("p1", "me", claims, cfg, now)
	reasons := map[string]string{}
	for _, c := range got {
		reasons[c.Ticket] = c.Reason
	}
	assert.Equal(t, map[string]string{
		"lease-past": "lease", "legacy-stale": "inactive", "ended": "ended",
	}, reasons, "the member's own claims only, none without a lease (planned, done), none still valid")
}

func TestExpiredClaimsResult(t *testing.T) {
	i18ntest.Set(t, "fr")
	ok := expiredClaimsResult(nil)
	assert.True(t, ok.OK)
	assert.False(t, ok.Warn)
	assert.Equal(t, doctorIDClaimsExpired, ok.ID)

	r := expiredClaimsResult([]expiredClaim{{Project: "p1", Ticket: "T-1", Reason: "lease"}, {Project: "p1", Ticket: "T-2", Reason: "ended"}})
	assert.Equal(t, doctorIDClaimsExpired, r.ID)
	assert.True(t, r.OK && r.Warn, "a warning, the doctor stays green")
	assert.Contains(t, r.Detail, "p1/T-1 (bail dépassé)")
	assert.Contains(t, r.Detail, "p1/T-2 (session terminée)")
	assert.Contains(t, r.Detail, "oh team release T-1 -p p1", "the command")
}

func TestClaimQueueResult(t *testing.T) {
	i18ntest.Set(t, "fr")
	now := time.Now()
	assert.Empty(t, claimQueueResults(nil, now))
	assert.Empty(t, claimQueueResults([]claimQueueState{{Repo: "/r", Count: 2, Oldest: now.Add(-time.Minute)}}, now), "a queue being emptied is not a finding")

	rs := claimQueueResults([]claimQueueState{{Repo: "/r", Count: 2, Oldest: now.Add(-3 * time.Hour)}}, now)
	require.Len(t, rs, 1)
	assert.Equal(t, doctorIDClaimsQueue, rs[0].ID)
	assert.True(t, rs[0].OK && rs[0].Warn)
	assert.Contains(t, rs[0].Detail, "2 opération(s)")
	assert.Contains(t, rs[0].Detail, "3 h", "the age of the oldest")
	assert.Contains(t, rs[0].Detail, "oh doctor --fix")

	rs = claimQueueResults([]claimQueueState{{Repo: "/r", Corrupt: &teamstate.ClaimQueueCorruptError{Path: "/r/.git/q", SetAside: "/r/.git/q.corrupt", Cause: assert.AnError}}}, now)
	require.Len(t, rs, 1)
	assert.True(t, rs[0].Warn)
	assert.Contains(t, rs[0].Detail, "illisible")
}

// The check reads the claims of the member in the team-state and the queue of
// the clone.
func TestClaimsLeaseDoctorChecks(t *testing.T) {
	a, repo := setupBeat(t)
	claimsCreate(t, repo, "pt-a", "testuser", "ses_1")
	claimsCreate(t, repo, "pt-bob", "bob", "ses_b")
	ctx := context.Background()

	rs := claimsLeaseResults(ctx, a, time.Now())
	require.Len(t, rs, 1)
	assert.True(t, rs[0].OK)
	assert.False(t, rs[0].Warn, "no claim expired yet")

	rs = claimsLeaseResults(ctx, a, time.Now().Add(48*time.Hour))
	require.Len(t, rs, 1)
	assert.Equal(t, doctorIDClaimsExpired, rs[0].ID)
	assert.True(t, rs[0].Warn)
	assert.Contains(t, rs[0].Detail, claimsProject+"/pt-a")
	assert.NotContains(t, rs[0].Detail, "pt-bob", "somebody else's claim")

	// A queue that has been waiting for a long time.
	online := goOffline(t, repo)
	require.NoError(t, repo.EnqueueClaimOp(teamstate.ClaimOp{Kind: teamstate.ClaimOpRelease, Project: claimsProject, TicketID: "pt-a", ClaimedBy: "testuser", Session: "ses_1"}))
	rs = claimsLeaseResults(ctx, a, time.Now().Add(2*time.Hour))
	require.Len(t, rs, 2)
	assert.Equal(t, doctorIDClaimsQueue, rs[1].ID)
	online()

	// oh doctor --fix applies it.
	var out bytes.Buffer
	require.NoError(t, repairClaimsQueue(ctx, &out, alwaysConfirm))
	claimsGone(t, repo, "pt-a")
	assert.Contains(t, out.String(), "1")
	ops, err := repo.ClaimQueue()
	require.NoError(t, err)
	assert.Empty(t, ops)
}
