package teamstate

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

// createUnder creates a claim under a project ID of its own.
func createUnder(t *testing.T, repo *Repo, project string, c Claim) {
	t.Helper()
	c.Project = project
	_, err := repo.CreateClaim(context.Background(), c)
	require.NoError(t, err)
}

// Bug 7 (K4a): five claims of a member under the ID the project had before it
// was created again are attached to the new ID in one commit, and a second
// run does nothing.
func TestRehomeClaimsMovesTheClaimsOfTheMemberInOneCommit(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := context.Background()
	for _, id := range []string{"SRU-1", "SRU-2", "SRU-3", "SRU-4", "SRU-5"} {
		createUnder(t, repo, "t-sru-b267fbf1", Claim{TicketID: id, ClaimedBy: "benjamin", Status: ClaimStatusReview, Labels: []string{"keep"}})
	}
	createUnder(t, repo, "t-sru-b267fbf1", Claim{TicketID: "SRU-9", ClaimedBy: "alice"})
	before := commitCount(t, bare)
	raw, err := os.ReadFile(repo.claimFilePath("t-sru-b267fbf1", "SRU-1"))
	require.NoError(t, err)

	res, err := repo.RehomeClaims(ctx, "benjamin", []ClaimMove{{From: "t-sru-b267fbf1", To: "t-sru-0a1b2c3d"}})
	require.NoError(t, err)
	assert.Len(t, res.Moved, 5)
	assert.Empty(t, res.Conflicts)
	assert.Equal(t, before+1, commitCount(t, bare), "one commit for the five claims")

	moved, err := repo.ListClaims("t-sru-0a1b2c3d")
	require.NoError(t, err)
	assert.Len(t, moved, 5)
	for _, c := range moved {
		assert.Equal(t, "benjamin", c.ClaimedBy)
		assert.Equal(t, "t-sru-0a1b2c3d", c.Project)
	}
	got, err := os.ReadFile(repo.claimFilePath("t-sru-0a1b2c3d", "SRU-1"))
	require.NoError(t, err)
	assert.Equal(t, string(raw), string(got), "the file is moved as it is, not rewritten")
	left, err := repo.ListClaims("t-sru-b267fbf1")
	require.NoError(t, err)
	require.Len(t, left, 1, "the claim of another member stays where it is")
	assert.Equal(t, "alice", left[0].ClaimedBy)
	assert.Equal(t, "", gitStatus(t, repo.path), "nothing left uncommitted")

	res, err = repo.RehomeClaims(ctx, "benjamin", []ClaimMove{{From: "t-sru-b267fbf1", To: "t-sru-0a1b2c3d"}})
	require.NoError(t, err)
	assert.Empty(t, res.Moved)
	assert.Equal(t, before+1, commitCount(t, bare), "idempotent: no second commit")
}

func TestRehomeClaimsKeepsTheClaimOfTheNewID(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	createUnder(t, repo, "old-12345678", Claim{TicketID: "SRU-1", ClaimedBy: "benjamin"})
	createUnder(t, repo, "old-12345678", Claim{TicketID: "SRU-2", ClaimedBy: "benjamin"})
	createUnder(t, repo, "new-87654321", Claim{TicketID: "SRU-1", ClaimedBy: "benjamin", Status: ClaimStatusReview})
	before := commitCount(t, bare)

	res, err := repo.RehomeClaims(context.Background(), "benjamin", []ClaimMove{{From: "old-12345678", To: "new-87654321"}})
	require.NoError(t, err)
	assert.Equal(t, []RehomedClaim{{From: "old-12345678", To: "new-87654321", TicketID: "SRU-2"}}, res.Moved)
	assert.Equal(t, []RehomedClaim{{From: "old-12345678", To: "new-87654321", TicketID: "SRU-1"}}, res.Conflicts)
	c, err := repo.GetClaim("new-87654321", "SRU-1")
	require.NoError(t, err)
	assert.Equal(t, ClaimStatusReview, c.Status, "the claim of the new ID is not overwritten")
	_, err = repo.GetClaim("old-12345678", "SRU-1")
	assert.NoError(t, err, "the conflicting claim stays under the old ID")
	assert.Equal(t, before+1, commitCount(t, bare))
}

func TestRehomeClaimsRefusesUnsafeNames(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	for _, m := range []ClaimMove{{From: "../x", To: "ok-12345678"}, {From: "ok-12345678", To: ""}, {From: "same", To: "same"}} {
		_, err := repo.RehomeClaims(ctx, "benjamin", []ClaimMove{m})
		assert.Error(t, err, "%+v", m)
	}
	_, err := repo.RehomeClaims(ctx, "", []ClaimMove{{From: "a-12345678", To: "b-12345678"}})
	assert.Error(t, err, "no member")
}

func gitStatus(t *testing.T, dir string) string {
	t.Helper()
	return gittest.Run(t, dir, "status", "--porcelain")
}
