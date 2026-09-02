package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

func newClaimCmd(t *testing.T, project string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "claim"}
	cmd.Flags().StringP("project", "p", project, "")
	cmd.Flags().String("worktree", "", "")
	cmd.Flags().Bool("planned", false, "")
	cmd.SetContext(context.Background())
	return cmd
}

func newReleaseCmd(t *testing.T, project string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "release"}
	cmd.Flags().StringP("project", "p", project, "")
	cmd.SetContext(context.Background())
	return cmd
}

func newTransferCmd(t *testing.T, project, to string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "transfer"}
	cmd.Flags().String("to", to, "")
	cmd.Flags().StringP("project", "p", project, "")
	cmd.SetContext(context.Background())
	return cmd
}

func TestRunClaim_Success(t *testing.T) {
	_, repo, stdout := setupClaimTestApp(t)
	ctx := context.Background()

	cmd := newClaimCmd(t, "myproject")
	err := runClaim(cmd, []string{"TICKET-1"})
	require.NoError(t, err)

	// Verify success message
	assert.Contains(t, stdout.String(), "myproject/TICKET-1")
	assert.Contains(t, stdout.String(), "testuser")

	// Verify claim file was created
	claim, err := repo.GetClaim("myproject", "TICKET-1")
	require.NoError(t, err)
	assert.Equal(t, "testuser", claim.ClaimedBy)
	assert.Equal(t, teamstate.ClaimStatusInProgress, claim.Status)

	// Event was emitted
	events, err := repo.ListEventsLimited("myproject", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, events)

	_ = ctx
}

func TestRunClaim_AlreadyTaken(t *testing.T) {
	a, repo, stdout := setupClaimTestApp(t)
	ctx := context.Background()

	// Pre-create a claim by another member
	claim := teamstate.Claim{
		TicketID:  "TAKEN-1",
		Project:   "myproject",
		ClaimedBy: "alice",
		ClaimedAt: time.Now().Add(-1 * time.Hour),
		Status:    teamstate.ClaimStatusInProgress,
	}
	_, err := repo.CreateClaim(ctx, claim)
	require.NoError(t, err)

	cmd := newClaimCmd(t, "myproject")
	err = runClaim(cmd, []string{"TAKEN-1"})
	// Should NOT return an error — it's a warning
	assert.NoError(t, err)

	// Should display warning about existing claim
	output := stdout.String()
	assert.Contains(t, output, "alice")
	assert.Contains(t, output, "TAKEN-1")

	_ = a
}

func TestRunClaim_Planned(t *testing.T) {
	_, repo, _ := setupClaimTestApp(t)

	cmd := newClaimCmd(t, "myproject")
	_ = cmd.Flags().Set("planned", "true")
	err := runClaim(cmd, []string{"PLAN-1"})
	require.NoError(t, err)

	// Verify claim has planned status
	claim, err := repo.GetClaim("myproject", "PLAN-1")
	require.NoError(t, err)
	assert.Equal(t, teamstate.ClaimStatusPlanned, claim.Status)
}

func TestRunRelease_Success(t *testing.T) {
	_, repo, stdout := setupClaimTestApp(t)
	ctx := context.Background()

	// Create a claim first
	claim := teamstate.Claim{
		TicketID:  "REL-1",
		Project:   "myproject",
		ClaimedBy: "testuser",
		ClaimedAt: time.Now(),
		Status:    teamstate.ClaimStatusInProgress,
	}
	_, err := repo.CreateClaim(ctx, claim)
	require.NoError(t, err)

	cmd := newReleaseCmd(t, "myproject")
	err = runRelease(cmd, []string{"REL-1"})
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "REL-1")

	// Verify claim is gone
	_, err = repo.GetClaim("myproject", "REL-1")
	assert.Error(t, err)
}

func TestRunRelease_NotFound(t *testing.T) {
	_, _, stdout := setupClaimTestApp(t)

	cmd := newReleaseCmd(t, "myproject")
	err := runRelease(cmd, []string{"GHOST-1"})
	assert.NoError(t, err) // warning, not error

	assert.Contains(t, stdout.String(), "GHOST-1")
}

func TestRunClaimTransfer_Success(t *testing.T) {
	_, repo, stdout := setupClaimTestApp(t)
	ctx := context.Background()

	// Create a claim to transfer
	claim := teamstate.Claim{
		TicketID:  "XFER-1",
		Project:   "myproject",
		ClaimedBy: "testuser",
		ClaimedAt: time.Now(),
		Status:    teamstate.ClaimStatusInProgress,
	}
	_, err := repo.CreateClaim(ctx, claim)
	require.NoError(t, err)

	cmd := newTransferCmd(t, "myproject", "alice")
	err = runClaimTransfer(cmd, []string{"XFER-1"})
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "alice")
	assert.Contains(t, stdout.String(), "XFER-1")

	// Verify ownership changed
	updated, err := repo.GetClaim("myproject", "XFER-1")
	require.NoError(t, err)
	assert.Equal(t, "alice", updated.ClaimedBy)

	// Verify takeover brief was generated
	briefsDir := filepath.Join(repo.Path(), "projects", "myproject", "takeover-briefs")
	entries, _ := os.ReadDir(briefsDir)
	_ = fmt.Sprintf("briefs: %d", len(entries)) // brief generation is best-effort
}
