package teamstate

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdoptClaimTakesTheSession(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := context.Background()
	mustCreate(t, repo, Claim{TicketID: "SRU-1", ClaimedBy: "benjamin", Session: "ses_old"})
	require.NoError(t, repo.AddClaimLabel(ctx, "T-SRU", "SRU-1", LabelSessionEnded))
	require.NoError(t, repo.AddClaimLabel(ctx, "T-SRU", "SRU-1", "keep-me"))
	before := commitCount(t, bare)

	adopted, err := repo.AdoptClaim(ctx, "T-SRU", "SRU-1", "benjamin", "ses_new")
	require.NoError(t, err)
	assert.True(t, adopted)
	c, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.Equal(t, "ses_new", c.Session)
	assert.Equal(t, []string{"keep-me"}, c.Labels, "the label of the previous session is removed")
	assert.True(t, c.LeaseUntil.Time().After(time.Now().Add(23*time.Hour)), "the lease starts again")
	assert.Equal(t, before+1, commitCount(t, bare))

	adopted, err = repo.AdoptClaim(ctx, "T-SRU", "SRU-1", "benjamin", "ses_new")
	require.NoError(t, err)
	assert.False(t, adopted, "already its session: nothing to do")
	assert.Equal(t, before+1, commitCount(t, bare), "no commit")
}

func TestAdoptClaimWithoutSession(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	mustCreate(t, repo, Claim{TicketID: "SRU-1", ClaimedBy: "benjamin"})

	adopted, err := repo.AdoptClaim(ctx, "T-SRU", "SRU-1", "benjamin", "ses_1")
	require.NoError(t, err)
	assert.True(t, adopted)
	c, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.Equal(t, "ses_1", c.Session)
}

func TestAdoptClaimLeavesOthersAlone(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := context.Background()
	mustCreate(t, repo, Claim{TicketID: "SRU-1", ClaimedBy: "alice", Session: "ses_alice"})
	mustCreate(t, repo, Claim{TicketID: "POOL-1"})
	before := commitCount(t, bare)

	for _, id := range []string{"SRU-1", "POOL-1"} {
		adopted, err := repo.AdoptClaim(ctx, "T-SRU", id, "benjamin", "ses_1")
		require.NoError(t, err)
		assert.False(t, adopted, id)
	}
	c, _ := repo.GetClaim("T-SRU", "SRU-1")
	assert.Equal(t, "ses_alice", c.Session)
	assert.Equal(t, before, commitCount(t, bare))

	_, err := repo.AdoptClaim(ctx, "T-SRU", "NOPE-1", "benjamin", "ses_1")
	assert.ErrorIs(t, err, ErrClaimNotFound)
	adopted, err := repo.AdoptClaim(ctx, "T-SRU", "SRU-1", "", "ses_1")
	require.NoError(t, err)
	assert.False(t, adopted, "no member: nothing adopted")
}
