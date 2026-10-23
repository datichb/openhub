package teamstate

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLastActivityIsWrittenAndReadBack(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 7, 8, 14, 30, 0, 0, time.UTC)
	data, err := toml.Marshal(&Claim{ClaimedBy: "benjamin", ClaimedAt: at, Status: ClaimStatusInProgress, LastActivity: InstantOf(at.Add(time.Hour))})
	require.NoError(t, err)
	assert.Contains(t, string(data), "last_activity = '2026-07-08T15:30:00Z'")

	var got Claim
	require.NoError(t, toml.Unmarshal(data, &got))
	assert.Equal(t, at.Add(time.Hour), got.LastActivity.Time())
}

func TestClaimWithoutLastActivityIsRewrittenIdentically(t *testing.T) {
	t.Parallel()
	old := "claimed_by = 'benjamin'\nclaimed_at = 2026-07-08T14:30:00Z\nstatus = 'in_progress'\n"
	var c Claim
	require.NoError(t, toml.Unmarshal([]byte(old), &c))
	assert.True(t, c.LastActivity.IsZero())
	data, err := toml.Marshal(&c)
	require.NoError(t, err)
	assert.Equal(t, old, string(data))
}

func TestLastActivityReadsAnUnquotedDatetime(t *testing.T) {
	t.Parallel()
	for name, raw := range map[string]string{
		"utc":      "2026-07-08T14:30:00Z",
		"fraction": "2026-07-08T14:30:00.123456Z",
		"offset":   "2026-07-08T16:30:00+02:00",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var c Claim
			src := "claimed_by = 'b'\nclaimed_at = 2026-07-01T00:00:00Z\nlast_activity = " + raw + "\nlease_until = " + raw + "\n"
			require.NoError(t, toml.Unmarshal([]byte(src), &c))
			want := time.Date(2026, 7, 8, 14, 30, 0, 0, time.UTC)
			assert.WithinDuration(t, want, c.LastActivity.Time(), time.Second)
			assert.WithinDuration(t, want, c.LeaseUntil.Time(), time.Second)
		})
	}
}

func TestIsStaleUsesLastActivityFromTheFile(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	// claimed 10 days ago, in progress again just now: not stale.
	_, err := repo.CreateClaim(ctx, Claim{TicketID: "SRU-1", Project: "T-SRU", ClaimedBy: "benjamin", ClaimedAt: time.Now().Add(-10 * 24 * time.Hour)})
	require.NoError(t, err)
	before, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.True(t, before.LastActivity.IsZero())
	assert.True(t, repo.IsStale(before, 3), "no activity: judged from claimed_at")

	require.NoError(t, repo.AddClaimLabel(ctx, "T-SRU", "SRU-1", "touched"))
	got, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.False(t, got.LastActivity.IsZero(), "last_activity is on disk")
	assert.WithinDuration(t, time.Now(), got.LastActivity.Time(), time.Minute)
	assert.False(t, repo.IsStale(got, 3), "the activity wins over claimed_at")

	raw, err := os.ReadFile(repo.claimFilePath("T-SRU", "SRU-1"))
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(raw), "last_activity = '"), string(raw))
}

func TestLastActivityIsSetByTheWritingOperations(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	old := time.Now().Add(-10 * 24 * time.Hour)
	create := func(id string, c Claim) {
		t.Helper()
		c.TicketID, c.Project, c.ClaimedAt = id, "T-SRU", old
		_, err := repo.CreateClaim(ctx, c)
		require.NoError(t, err)
	}
	active := func(id string) bool {
		t.Helper()
		c, err := repo.GetClaim("T-SRU", id)
		require.NoError(t, err)
		return !c.LastActivity.IsZero() && time.Since(c.LastActivity.Time()) < time.Minute
	}

	create("S", Claim{ClaimedBy: "benjamin"})
	require.NoError(t, repo.UpdateClaimStatus(ctx, "T-SRU", "S", ClaimStatusReview))
	assert.True(t, active("S"), "status")

	create("L", Claim{ClaimedBy: "benjamin"})
	require.NoError(t, repo.AddClaimLabel(ctx, "T-SRU", "L", "x"))
	assert.True(t, active("L"), "add label")

	create("R", Claim{ClaimedBy: "benjamin", Labels: []string{"x"}})
	require.NoError(t, repo.RemoveClaimLabel(ctx, "T-SRU", "R", "x"))
	assert.True(t, active("R"), "remove label")

	create("T", Claim{ClaimedBy: "benjamin"})
	require.NoError(t, repo.UpdateClaimStatusFromTracker(ctx, "T-SRU", "T", ClaimStatusReview))
	assert.True(t, active("T"), "status from the tracker")

	create("X", Claim{ClaimedBy: "benjamin"})
	require.NoError(t, repo.TransferClaim(ctx, "T-SRU", "X", "alice"))
	assert.True(t, active("X"), "takeover")

	create("P", Claim{})
	require.NoError(t, repo.ClaimPoolTicket(ctx, "T-SRU", "P", "alice", ""))
	assert.True(t, active("P"), "pool claim")

	create("M", Claim{ClaimedBy: "benjamin"})
	require.NoError(t, repo.UpdateClaimMetadata(ctx, "T-SRU", "M", "title", "", ""))
	assert.False(t, active("M"), "tracker metadata is not an activity of the owner")

	create("K", Claim{ClaimedBy: "benjamin", Session: "ses_1"})
	_, err := repo.TouchClaims(ctx, []ClaimRef{{Project: "T-SRU", TicketID: "K"}}, "benjamin", "ses_1", time.Now().Add(20*time.Hour))
	require.NoError(t, err)
	assert.False(t, active("K"), "a renewal is not an activity")
}
