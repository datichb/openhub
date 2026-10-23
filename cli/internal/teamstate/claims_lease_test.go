package teamstate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// claimBefore5_1 is a claim as the versions before the lease wrote it.
const claimBefore5_1 = `claimed_by = 'benjamin'
claimed_at = 2026-07-07T14:30:00Z
worktree = 'feat/SRU-142-user-auth'
status = 'in_progress'
title = 'User auth'
labels = ['agent-reviewed']
`

func TestClaimBefore5_1KeepsItsBytes(t *testing.T) {
	t.Parallel()
	repo := setupTestRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(repo.claimFilePath("T-SRU", "SRU-142")), 0o755))
	require.NoError(t, os.WriteFile(repo.claimFilePath("T-SRU", "SRU-142"), []byte(claimBefore5_1), 0o644))

	c, err := repo.GetClaim("T-SRU", "SRU-142")
	require.NoError(t, err)
	assert.Empty(t, c.LeaseUntil)
	assert.Empty(t, c.Session)

	again, err := toml.Marshal(c)
	require.NoError(t, err)
	assert.Equal(t, claimBefore5_1, string(again), "the new fields are omitted when absent")
}

func TestClaimLeaseFieldsFormat(t *testing.T) {
	t.Parallel()
	c := Claim{
		ClaimedBy:  "benjamin",
		ClaimedAt:  time.Date(2026, 7, 7, 14, 30, 0, 0, time.UTC),
		Status:     ClaimStatusInProgress,
		LeaseUntil: InstantOf(time.Date(2026, 7, 8, 14, 30, 0, 0, time.UTC)),
		Session:    "ses_abc",
	}
	data, err := toml.Marshal(c)
	require.NoError(t, err)
	assert.Equal(t, `claimed_by = 'benjamin'
claimed_at = 2026-07-07T14:30:00Z
status = 'in_progress'
lease_until = '2026-07-08T14:30:00Z'
session = 'ses_abc'
`, string(data))

	var back Claim
	require.NoError(t, toml.Unmarshal(data, &back))
	assert.Equal(t, c.LeaseUntil, back.LeaseUntil)
	assert.Equal(t, "ses_abc", back.Session)
}

func TestClaimIsExpired(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	cfg := &TeamConfig{Takeover: TakeoverConfig{StaleDays: 2}}

	tests := []struct {
		name  string
		claim Claim
		cfg   *TeamConfig
		want  bool
	}{
		{"lease in the future", Claim{LeaseUntil: InstantOf(now.Add(time.Minute))}, cfg, false},
		{"lease past", Claim{LeaseUntil: InstantOf(now.Add(-time.Second))}, cfg, true},
		{"lease ending now", Claim{LeaseUntil: InstantOf(now)}, cfg, false},
		{"the lease wins over a stale activity", Claim{LeaseUntil: InstantOf(now.Add(time.Hour)), LastActivity: now.Add(-30 * 24 * time.Hour)}, cfg, false},
		{"the lease wins over a recent activity", Claim{LeaseUntil: InstantOf(now.Add(-time.Hour)), LastActivity: now}, cfg, true},
		{"no lease, recent activity", Claim{LastActivity: now.Add(-24 * time.Hour)}, cfg, false},
		{"no lease, stale activity", Claim{LastActivity: now.Add(-3 * 24 * time.Hour)}, cfg, true},
		{"no lease, claimed_at when no activity", Claim{ClaimedAt: now.Add(-3 * 24 * time.Hour)}, cfg, true},
		{"no lease, default stale days without config", Claim{LastActivity: now.Add(-2 * 24 * time.Hour)}, nil, false},
		{"no lease, default stale days exceeded", Claim{LastActivity: now.Add(-4 * 24 * time.Hour)}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.claim.IsExpired(now, tt.cfg))
		})
	}
}

func TestClaimIsExpiredFallbackIsIsStale(t *testing.T) {
	t.Parallel()
	repo := setupTestRepo(t)
	for _, ago := range []time.Duration{time.Hour, 2 * 24 * time.Hour, 4 * 24 * time.Hour, 10 * 24 * time.Hour} {
		c := Claim{LastActivity: time.Now().Add(-ago)}
		cfg := &TeamConfig{Takeover: TakeoverConfig{StaleDays: 3}}
		assert.Equal(t, repo.IsStale(&c, 3), c.IsExpired(time.Now(), cfg), ago.String())
	}
}

// ── Lease set on creation and takeover ──────────────────────────────────────

func TestCreateClaimSetsLease(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	before := time.Now().UTC()
	_, err := repo.CreateClaim(ctx, Claim{TicketID: "SRU-1", Project: "T-SRU", ClaimedBy: "benjamin", Session: "ses_1"})
	require.NoError(t, err)

	got, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.Equal(t, "ses_1", got.Session)
	assert.WithinDuration(t, before.Add(24*time.Hour), got.LeaseUntil.Time(), time.Minute)
	assert.False(t, got.IsExpired(time.Now(), nil))
}

func TestCreateClaimLeaseFollowsTheConfiguration(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	require.NoError(t, repo.SaveConfig(ctx, &TeamConfig{Claim: ClaimConfig{LeaseHours: 6, HeartbeatMinutes: 30}}))

	before := time.Now().UTC()
	_, err := repo.CreateClaim(ctx, Claim{TicketID: "SRU-1", Project: "T-SRU", ClaimedBy: "benjamin"})
	require.NoError(t, err)
	got, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.WithinDuration(t, before.Add(6*time.Hour), got.LeaseUntil.Time(), time.Minute)
	assert.Empty(t, got.Session)
}

func TestCreateClaimKeepsAGivenLease(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	lease := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := repo.CreateClaim(context.Background(), Claim{TicketID: "SRU-1", Project: "T-SRU", ClaimedBy: "benjamin", LeaseUntil: InstantOf(lease)})
	require.NoError(t, err)
	got, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.True(t, lease.Equal(got.LeaseUntil.Time()))
}

func TestClaimsWithoutWorkHaveNoLease(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	_, err := repo.CreateClaim(ctx, Claim{TicketID: "SRU-1", Project: "T-SRU", ClaimedBy: "benjamin", Status: ClaimStatusPlanned})
	require.NoError(t, err)
	_, err = repo.CreateClaim(ctx, Claim{TicketID: "SRU-2", Project: "T-SRU", Status: ClaimStatusInProgress}) // pool claim
	require.NoError(t, err)
	_, err = repo.CreateClaim(ctx, Claim{TicketID: "SRU-3", Project: "T-SRU", ClaimedBy: "benjamin", Status: ClaimStatusDone})
	require.NoError(t, err)

	for _, id := range []string{"SRU-1", "SRU-2", "SRU-3"} {
		got, err := repo.GetClaim("T-SRU", id)
		require.NoError(t, err)
		assert.Empty(t, got.LeaseUntil, id)
	}
}

func TestCreateClaimLocalSetsLease(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	before := time.Now().UTC()
	err := repo.WithWriteLock(ctx, func(ctx context.Context) error {
		rel, err := repo.CreateClaimLocal(Claim{TicketID: "SRU-1", Project: "T-SRU", ClaimedBy: "benjamin", Session: "ses_1"})
		if err != nil {
			return err
		}
		return repo.CommitAndPush(ctx, "claims", rel)
	})
	require.NoError(t, err)
	got, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.WithinDuration(t, before.Add(24*time.Hour), got.LeaseUntil.Time(), time.Minute)
	assert.Equal(t, "ses_1", got.Session)
}

func TestTransferClaimRenewsTheLease(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	old := time.Now().UTC().Add(-48 * time.Hour)
	_, err := repo.CreateClaim(ctx, Claim{TicketID: "SRU-1", Project: "T-SRU", ClaimedBy: "alice", Session: "ses_alice", LeaseUntil: InstantOf(old)})
	require.NoError(t, err)

	// Without a session: the one of the previous owner does not stay.
	before := time.Now().UTC()
	require.NoError(t, repo.TransferClaim(ctx, "T-SRU", "SRU-1", "benjamin"))
	got, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.Equal(t, "benjamin", got.ClaimedBy)
	assert.Empty(t, got.Session)
	assert.WithinDuration(t, before.Add(24*time.Hour), got.LeaseUntil.Time(), time.Minute)

	// With the session of the new owner.
	require.NoError(t, repo.TransferClaim(WithClaimSession(ctx, "ses_carol"), "T-SRU", "SRU-1", "carol"))
	got, err = repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.Equal(t, "carol", got.ClaimedBy)
	assert.Equal(t, "ses_carol", got.Session)
}

func TestClaimPoolTicketSetsLease(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	_, err := repo.CreateClaim(ctx, Claim{TicketID: "SRU-1", Project: "T-SRU", Status: ClaimStatusPlanned})
	require.NoError(t, err)

	before := time.Now().UTC()
	require.NoError(t, repo.ClaimPoolTicket(WithClaimSession(ctx, "ses_1"), "T-SRU", "SRU-1", "benjamin", ""))
	got, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.Equal(t, ClaimStatusInProgress, got.Status)
	assert.Equal(t, "ses_1", got.Session)
	assert.WithinDuration(t, before.Add(24*time.Hour), got.LeaseUntil.Time(), time.Minute)
}

func TestOtherWritesKeepTheLease(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	_, err := repo.CreateClaim(ctx, Claim{TicketID: "SRU-1", Project: "T-SRU", ClaimedBy: "benjamin", Session: "ses_1"})
	require.NoError(t, err)
	before, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)

	require.NotEmpty(t, before.LeaseUntil)
	require.NoError(t, repo.AddClaimLabel(ctx, "T-SRU", "SRU-1", "x"))
	require.NoError(t, repo.UpdateClaimStatus(ctx, "T-SRU", "SRU-1", ClaimStatusReview))
	after, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.Equal(t, before.LeaseUntil, after.LeaseUntil)
	assert.Equal(t, "ses_1", after.Session)
}

func TestInstant(t *testing.T) {
	t.Parallel()
	d := time.Date(2026, 7, 8, 16, 30, 0, 999, time.FixedZone("CEST", 2*3600))
	assert.Equal(t, Instant("2026-07-08T14:30:00Z"), InstantOf(d), "UTC, to the second")
	assert.True(t, d.Truncate(time.Second).Equal(InstantOf(d).Time()))
	assert.Equal(t, Instant(""), InstantOf(time.Time{}))
	assert.True(t, Instant("").Time().IsZero())
	assert.True(t, Instant("not a date").Time().IsZero())
}

func TestMalformedLeaseIsJudgedLikeNoLease(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	c := Claim{LeaseUntil: "tomorrow", LastActivity: now.Add(-10 * 24 * time.Hour)}
	assert.True(t, c.IsExpired(now, nil))
}
