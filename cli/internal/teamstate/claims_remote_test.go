package teamstate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetClaimRemote(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	err := repo.SetClaimRemote(ctx, "T-SRU", "SRU-1", ClaimRemote{Session: "ses_x", Status: RemoteSent})
	assert.ErrorIs(t, err, ErrClaimNotFound)

	_, err = repo.CreateClaim(ctx, Claim{TicketID: "SRU-1", Project: "T-SRU", ClaimedBy: "alice"})
	require.NoError(t, err)
	require.NoError(t, repo.SetClaimRemote(ctx, "T-SRU", "SRU-1", ClaimRemote{Session: "ses_x", Target: "acme", Pipeline: 812, Status: RemoteSent}))
	require.NoError(t, repo.SetClaimRemote(ctx, "T-SRU", "SRU-1", ClaimRemote{Session: "ses_x", Target: "acme", Pipeline: 812, Status: RemoteMRReady, MRURL: "https://g/mr/1"}))

	got, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	require.NotNil(t, got.Remote)
	assert.Equal(t, RemoteMRReady, got.Remote.Status)
	assert.Equal(t, int64(812), got.Remote.Pipeline)
	assert.Equal(t, "https://g/mr/1", got.MRURL)
	assert.False(t, got.Remote.UpdatedAt.IsZero())
	assert.Equal(t, "alice", got.ClaimedBy)
}
