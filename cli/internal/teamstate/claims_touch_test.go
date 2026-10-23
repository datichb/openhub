package teamstate

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

func commitCount(t *testing.T, dir string) int {
	t.Helper()
	n, err := strconv.Atoi(gittest.Run(t, dir, "rev-list", "--count", "HEAD"))
	require.NoError(t, err)
	return n
}

func ref(ticket string) ClaimRef { return ClaimRef{Project: "T-SRU", TicketID: ticket} }

func mustCreate(t *testing.T, repo *Repo, c Claim) {
	t.Helper()
	c.Project = "T-SRU"
	_, err := repo.CreateClaim(context.Background(), c)
	require.NoError(t, err)
}

func TestTouchClaimsNoCommitWhileTheLeasesAreLong(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := context.Background()
	mustCreate(t, repo, Claim{TicketID: "SRU-1", ClaimedBy: "benjamin", Session: "ses_1"})
	mustCreate(t, repo, Claim{TicketID: "SRU-2", ClaimedBy: "benjamin", Session: "ses_1"})
	before := commitCount(t, repo.path)
	beforeBare := commitCount(t, bare)

	// 11h59 after the creation: 12h01 left, more than half of 24 h.
	now := time.Now().UTC().Add(11*time.Hour + 59*time.Minute)
	touched, err := repo.TouchClaims(ctx, []ClaimRef{ref("SRU-1"), ref("SRU-2")}, "benjamin", "ses_1", now)
	require.NoError(t, err)
	assert.Empty(t, touched)
	assert.Equal(t, before, commitCount(t, repo.path))
	assert.Equal(t, beforeBare, commitCount(t, bare))
}

func TestTouchClaimsOneCommitForSeveralTickets(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := context.Background()
	for _, id := range []string{"SRU-1", "SRU-2", "SRU-3"} {
		mustCreate(t, repo, Claim{TicketID: id, ClaimedBy: "benjamin", Session: "ses_1"})
	}
	before := commitCount(t, repo.path)
	created, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)

	// 13 h after the creation: 11 h left, less than half.
	now := time.Now().UTC().Add(13 * time.Hour)
	touched, err := repo.TouchClaims(ctx, []ClaimRef{ref("SRU-1"), ref("SRU-2"), ref("SRU-3")}, "benjamin", "ses_1", now)
	require.NoError(t, err)
	assert.Equal(t, []ClaimRef{ref("SRU-1"), ref("SRU-2"), ref("SRU-3")}, touched)

	assert.Equal(t, before+1, commitCount(t, repo.path), "one commit for the three tickets")
	assert.Equal(t, before+1, commitCount(t, bare), "pushed")
	assert.Contains(t, gittest.Run(t, repo.path, "log", "-1", "--format=%s"), "lease: benjamin renews 3 claim(s)")

	for _, id := range []string{"SRU-1", "SRU-2", "SRU-3"} {
		got, err := repo.GetClaim("T-SRU", id)
		require.NoError(t, err)
		assert.Equal(t, InstantOf(now.Add(24*time.Hour)), got.LeaseUntil, id)
		assert.Equal(t, "ses_1", got.Session)
		assert.Equal(t, "benjamin", got.ClaimedBy)
		assert.Equal(t, ClaimStatusInProgress, got.Status)
		assert.Equal(t, created.LastActivity, got.LastActivity, "a renewal is not an activity")
	}

	// Renewed: the next call finds long leases.
	touched, err = repo.TouchClaims(ctx, []ClaimRef{ref("SRU-1")}, "benjamin", "ses_1", now.Add(time.Minute))
	require.NoError(t, err)
	assert.Empty(t, touched)
	assert.Equal(t, before+1, commitCount(t, repo.path))
}

func TestTouchClaimsOnlyOnesOwnClaims(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	mustCreate(t, repo, Claim{TicketID: "SRU-1", ClaimedBy: "benjamin", Session: "ses_1"})
	mustCreate(t, repo, Claim{TicketID: "SRU-2", ClaimedBy: "alice", Session: "ses_alice"})
	mustCreate(t, repo, Claim{TicketID: "SRU-3", ClaimedBy: "benjamin", Session: "ses_other"}) // same member, another session
	other, err := repo.GetClaim("T-SRU", "SRU-2")
	require.NoError(t, err)
	before := commitCount(t, repo.path)

	now := time.Now().UTC().Add(20 * time.Hour)
	touched, err := repo.TouchClaims(ctx, []ClaimRef{ref("SRU-1"), ref("SRU-2"), ref("SRU-3")}, "benjamin", "ses_1", now)
	require.NoError(t, err)
	assert.Equal(t, []ClaimRef{ref("SRU-1"), ref("SRU-3")}, touched, "the claim of another member is ignored")
	assert.Equal(t, before+1, commitCount(t, repo.path))

	got, err := repo.GetClaim("T-SRU", "SRU-2")
	require.NoError(t, err)
	assert.Equal(t, other.LeaseUntil, got.LeaseUntil, "alice's lease is untouched")

	got, err = repo.GetClaim("T-SRU", "SRU-3")
	require.NoError(t, err)
	assert.Equal(t, "ses_other", got.Session, "the session of another session of the member stays")
	assert.Equal(t, InstantOf(now.Add(24*time.Hour)), got.LeaseUntil)
}

func TestTouchClaimsOnlyOthersAndNothingKnown(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	mustCreate(t, repo, Claim{TicketID: "SRU-2", ClaimedBy: "alice", Session: "ses_alice"})
	before := commitCount(t, repo.path)

	now := time.Now().UTC().Add(23 * time.Hour)
	touched, err := repo.TouchClaims(ctx, []ClaimRef{ref("SRU-2"), ref("NOPE-1"), {Project: "UNKNOWN", TicketID: "X-1"}, {Project: "../x", TicketID: "y"}, ref("a/../b")}, "benjamin", "ses_1", now)
	require.NoError(t, err)
	assert.Empty(t, touched)
	assert.Equal(t, before, commitCount(t, repo.path))

	touched, err = repo.TouchClaims(ctx, nil, "benjamin", "ses_1", now)
	require.NoError(t, err)
	assert.Empty(t, touched)
}

func TestTouchClaimsMatchesTheSession(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	mustCreate(t, repo, Claim{TicketID: "SRU-1", ClaimedBy: "alice", Session: "ses_shared"})

	now := time.Now().UTC().Add(20 * time.Hour)
	touched, err := repo.TouchClaims(context.Background(), []ClaimRef{ref("SRU-1")}, "", "ses_shared", now)
	require.NoError(t, err)
	assert.Equal(t, []ClaimRef{ref("SRU-1")}, touched)
}

func TestTouchClaimsAdoptsAClaimWithoutLease(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := context.Background()
	dir := filepath.Dir(repo.claimFilePath("T-SRU", "SRU-142"))
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(repo.claimFilePath("T-SRU", "SRU-142"), []byte(claimBefore5_1), 0o644))
	gittest.Run(t, repo.path, "add", "projects")
	gittest.Run(t, repo.path, "commit", "-q", "-m", "claim from an older oh")
	gittest.Run(t, repo.path, "push", "-q")
	before := commitCount(t, repo.path)

	now := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)
	touched, err := repo.TouchClaims(ctx, []ClaimRef{ref("SRU-142")}, "benjamin", "ses_1", now)
	require.NoError(t, err)
	assert.Equal(t, []ClaimRef{ref("SRU-142")}, touched)
	assert.Equal(t, before+1, commitCount(t, repo.path))
	assert.Equal(t, before+1, commitCount(t, bare))

	got, err := repo.GetClaim("T-SRU", "SRU-142")
	require.NoError(t, err)
	assert.Equal(t, Instant("2026-07-11T08:00:00Z"), got.LeaseUntil)
	assert.Equal(t, "ses_1", got.Session)
	assert.Equal(t, "User auth", got.Title)
	assert.Equal(t, []string{"agent-reviewed"}, got.Labels)
}

func TestTouchClaimsRenewsAnExpiredLease(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	mustCreate(t, repo, Claim{TicketID: "SRU-1", ClaimedBy: "benjamin", Session: "ses_1", LeaseUntil: InstantOf(time.Now().Add(-5 * time.Hour))})

	touched, err := repo.TouchClaims(context.Background(), []ClaimRef{ref("SRU-1")}, "benjamin", "ses_1", time.Now())
	require.NoError(t, err)
	assert.Len(t, touched, 1)
	got, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.False(t, got.IsExpired(time.Now(), nil))
}

func TestTouchClaimsIgnoresClaimsWithoutWork(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	mustCreate(t, repo, Claim{TicketID: "SRU-1", ClaimedBy: "benjamin", Status: ClaimStatusPlanned})
	mustCreate(t, repo, Claim{TicketID: "SRU-2", ClaimedBy: "benjamin", Status: ClaimStatusDone})
	mustCreate(t, repo, Claim{TicketID: "SRU-3", Status: ClaimStatusInProgress}) // pool
	before := commitCount(t, repo.path)

	touched, err := repo.TouchClaims(context.Background(), []ClaimRef{ref("SRU-1"), ref("SRU-2"), ref("SRU-3")}, "benjamin", "ses_1", time.Now().Add(72*time.Hour))
	require.NoError(t, err)
	assert.Empty(t, touched)
	assert.Equal(t, before, commitCount(t, repo.path))
}

func TestTouchClaimsFollowsLeaseHours(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	require.NoError(t, repo.SaveConfig(ctx, &TeamConfig{Claim: ClaimConfig{LeaseHours: 4, HeartbeatMinutes: 30}}))
	mustCreate(t, repo, Claim{TicketID: "SRU-1", ClaimedBy: "benjamin"})
	before := commitCount(t, repo.path)

	// 1 h in: 3 h left of 4 h, long.
	touched, err := repo.TouchClaims(ctx, []ClaimRef{ref("SRU-1")}, "benjamin", "", time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Empty(t, touched)
	assert.Equal(t, before, commitCount(t, repo.path))

	// 2 h 30 in: 1 h 30 left, under half.
	now := time.Now().UTC().Add(150 * time.Minute)
	touched, err = repo.TouchClaims(ctx, []ClaimRef{ref("SRU-1")}, "benjamin", "", now)
	require.NoError(t, err)
	assert.Len(t, touched, 1)
	got, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.Equal(t, InstantOf(now.Add(4*time.Hour)), got.LeaseUntil)
}

// An older oh reads the new fields as unknown keys and, if it rewrites the
// claim, drops them: the claim then falls back on IsStale.
func TestOlderStructIgnoresTheLeaseFields(t *testing.T) {
	t.Parallel()
	type claim5_0 struct {
		ClaimedBy string    `toml:"claimed_by"`
		ClaimedAt time.Time `toml:"claimed_at"`
		Status    string    `toml:"status"`
	}
	data, err := toml.Marshal(Claim{
		ClaimedBy: "benjamin", ClaimedAt: time.Date(2026, 7, 7, 14, 30, 0, 0, time.UTC), Status: ClaimStatusInProgress,
		LeaseUntil: InstantOf(time.Now()), Session: "ses_1",
	})
	require.NoError(t, err)
	var old claim5_0
	require.NoError(t, toml.Unmarshal(data, &old))
	assert.Equal(t, "benjamin", old.ClaimedBy)

	rewritten, err := toml.Marshal(old)
	require.NoError(t, err)
	var back Claim
	require.NoError(t, toml.Unmarshal(rewritten, &back))
	assert.Empty(t, back.LeaseUntil)
	assert.Empty(t, back.Session)
}

func TestTouchClaimsConcurrentWithAnotherWriter(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	other := NewRepo(repo.remote, repo.path) // a second Repo value on the same clone
	ctx := context.Background()
	for _, id := range []string{"SRU-1", "SRU-2"} {
		mustCreate(t, repo, Claim{TicketID: id, ClaimedBy: "benjamin", Session: "ses_1"})
	}

	now := time.Now().UTC().Add(20 * time.Hour)
	errs := make(chan error, 2)
	go func() {
		_, err := repo.TouchClaims(ctx, []ClaimRef{ref("SRU-1")}, "benjamin", "ses_1", now)
		errs <- err
	}()
	go func() {
		errs <- other.AddClaimLabel(ctx, "T-SRU", "SRU-2", "x")
	}()
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)

	got, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.Equal(t, InstantOf(now.Add(24*time.Hour)), got.LeaseUntil)
	got, err = repo.GetClaim("T-SRU", "SRU-2")
	require.NoError(t, err)
	assert.Equal(t, []string{"x"}, got.Labels)
}
