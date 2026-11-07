package teamstate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/workflow"
)

// The local reads (queue of the claims, workflows, drafts, history, files of
// the clone) stop when their context is done, like the other reads.
func TestLocalReadsStopWithTheirContext(t *testing.T) {
	t.Parallel()
	repo := NewRepo("", t.TempDir())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := repo.ClaimQueue(ctx)
	require.ErrorIs(t, err, context.Canceled)
	_, err = repo.ReadWorkflowLock(ctx)
	require.ErrorIs(t, err, context.Canceled)
	_, err = repo.WorkflowProjects(ctx)
	require.ErrorIs(t, err, context.Canceled)
	_, err = repo.ListDrafts(ctx, "alice", TeamScope())
	require.ErrorIs(t, err, context.Canceled)
	_, err = repo.ListHistory(ctx, TeamScope(), "feature")
	require.ErrorIs(t, err, context.Canceled)
	_, err = repo.QueueAdoptClaim(ctx, "p", "t", "alice", "s1")
	require.ErrorIs(t, err, context.Canceled)
	_, err = repo.QueueCreateClaim(ctx, "p", "t", "alice", "s1", "in_progress")
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, repo.EnqueueClaimOp(ctx, ClaimOp{Kind: ClaimOpRelease, Project: "p", TicketID: "t", ClaimedBy: "alice"}), context.Canceled)

	_, queued := repo.QueuedAdoption(ctx, "p", "t", "alice")
	assert.False(t, queued)
	assert.False(t, repo.HasConfig(ctx))
	assert.False(t, repo.HasPolicies(ctx))
	assert.False(t, repo.IsStale(ctx, &Claim{}, 1))

	diags := repo.LoadWorkflowScopes(ctx, workflow.NewMemCatalog(), TeamScope())
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, context.Canceled.Error())
	_, ds := repo.LoadDrafts(ctx, workflow.NewMemCatalog(), "alice", TeamScope())
	assert.True(t, len(ds) > 0)
}
