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
	data, err := marshalClaim(&Claim{ClaimedBy: "benjamin", ClaimedAt: at, Status: ClaimStatusInProgress, LastActivity: InstantOf(at.Add(time.Hour))})
	require.NoError(t, err)
	assert.Equal(t, `claimed_by = 'benjamin'
claimed_at = 2026-07-08T14:30:00Z
status = 'in_progress'
last_activity = 2026-07-08T15:30:00Z
`, string(data), "a native date-time, not a string")

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
	data, err := marshalClaim(&c)
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

func TestLastActivityReadsAQuotedString(t *testing.T) {
	t.Parallel()
	var c Claim
	require.NoError(t, toml.Unmarshal([]byte("claimed_by = 'b'\nlast_activity = '2026-07-08T14:30:00Z'\n"), &c))
	assert.Equal(t, time.Date(2026, 7, 8, 14, 30, 0, 0, time.UTC), c.LastActivity.Time())
	data, err := marshalClaim(&c)
	require.NoError(t, err)
	assert.Contains(t, string(data), "last_activity = 2026-07-08T14:30:00Z\n", "rewritten as a date-time")
}

func TestUnreadableLastActivityIsNotWritten(t *testing.T) {
	t.Parallel()
	data, err := marshalClaim(&Claim{ClaimedBy: "b", LastActivity: "yesterday"})
	require.NoError(t, err)
	assert.NotContains(t, string(data), "last_activity")
}

// claimV50 is the Claim of oh v5.0: a time.Time last_activity, no lease and
// no session. A claim written by this version is read by it unchanged.
type claimV50 struct {
	ClaimedBy     string       `toml:"claimed_by"`
	ClaimedAt     time.Time    `toml:"claimed_at"`
	Worktree      string       `toml:"worktree,omitempty"`
	Status        string       `toml:"status"`
	LastActivity  time.Time    `toml:"last_activity,omitempty"`
	Title         string       `toml:"title,omitempty"`
	Description   string       `toml:"description,omitempty"`
	TrackerStatus string       `toml:"tracker_status,omitempty"`
	Labels        []string     `toml:"labels,omitempty"`
	ExternalIID   int          `toml:"external_iid,omitempty"`
	MRURL         string       `toml:"mr_url,omitempty"`
	Remote        *ClaimRemote `toml:"remote,omitempty"`
}

func TestClaimWrittenNowIsReadByV50(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 7, 8, 14, 30, 0, 0, time.UTC)
	data, err := marshalClaim(&Claim{
		ClaimedBy: "benjamin", ClaimedAt: at, Status: ClaimStatusReview,
		LastActivity: InstantOf(at.Add(time.Hour)), LeaseUntil: InstantOf(at.Add(24 * time.Hour)), Session: "ses_1",
		Title: "t", Labels: []string{"x"}, Remote: &ClaimRemote{Session: "r", Status: "sent", UpdatedAt: at},
	})
	require.NoError(t, err)

	var old claimV50
	require.NoError(t, toml.Unmarshal(data, &old), string(data))
	assert.Equal(t, at.Add(time.Hour), old.LastActivity)
	assert.Equal(t, "benjamin", old.ClaimedBy)
	assert.Equal(t, ClaimStatusReview, old.Status)
	assert.Equal(t, []string{"x"}, old.Labels)
	require.NotNil(t, old.Remote)

	// Without activity the key is absent: nothing for v5.0 to choke on.
	data, err = marshalClaim(&Claim{ClaimedBy: "b", ClaimedAt: at, Status: ClaimStatusReview, LeaseUntil: InstantOf(at)})
	require.NoError(t, err)
	var none claimV50
	assert.NoError(t, toml.Unmarshal(data, &none))
	assert.True(t, none.LastActivity.IsZero())
}

func TestEveryWritingOperationWritesANativeLastActivity(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	_, err := repo.CreateClaim(ctx, Claim{TicketID: "SRU-1", Project: "T-SRU", ClaimedBy: "benjamin", Session: "ses_1"})
	require.NoError(t, err)
	require.NoError(t, repo.AddClaimLabel(ctx, "T-SRU", "SRU-1", "x"))
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpStatus, Project: "T-SRU", TicketID: "SRU-1", ClaimedBy: "benjamin", Session: "ses_1", Status: ClaimStatusReview}))
	_, _, err = repo.FlushClaimQueue(ctx)
	require.NoError(t, err)
	_, err = repo.TouchClaims(ctx, []ClaimRef{{Project: "T-SRU", TicketID: "SRU-1"}}, "benjamin", "ses_1", time.Now().Add(20*time.Hour))
	require.NoError(t, err)

	raw, err := os.ReadFile(repo.claimFilePath("T-SRU", "SRU-1"))
	require.NoError(t, err)
	assert.Regexp(t, `(?m)^last_activity = \d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ$`, string(raw))
	var old claimV50
	require.NoError(t, toml.Unmarshal(raw, &old), string(raw))
	assert.WithinDuration(t, time.Now(), old.LastActivity, time.Minute)
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
	assert.True(t, strings.Contains(string(raw), "last_activity = 20"), string(raw))
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
