package cmd

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

func memberRemoveCmd(t *testing.T, flags map[string]string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "remove"}
	cmd.Flags().String("to", "", "")
	cmd.Flags().Bool("release", false, "")
	addYesFlag(cmd)
	require.NoError(t, cmd.Flags().Set("yes", "true"))
	for k, v := range flags {
		require.NoError(t, cmd.Flags().Set(k, v))
	}
	cmd.SetContext(context.Background())
	return cmd
}

// teamWithLead is a team of lea (lead), testuser (the member of the machine,
// dev), alice and bob; alice holds two open claims.
func teamWithLead(t *testing.T) *teamstate.Repo {
	t.Helper()
	i18ntest.Set(t, "en")
	_, repo, _ := setupClaimTestApp(t)
	ctx := context.Background()
	require.NoError(t, repo.AddMember(ctx, teamstate.Member{ID: "lea", DisplayName: "Léa", Role: teamstate.RoleLead}))
	addTeamMembers(t, repo, "alice", "bob")
	claimsCreate(t, repo, "A-1", "alice", "")
	claimsCreate(t, repo, "A-2", "alice", "")
	terminalIs(t, false)
	return repo
}

// Only a lead removes another member, and the team says so before asking.
func TestTeamMemberRemoveNeedsALead(t *testing.T) {
	repo := teamWithLead(t)

	err := runTeamMemberRemove(memberRemoveCmd(t, map[string]string{"release": "true"}), []string{"alice"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "testuser is not a lead")
	_, gerr := repo.GetMember(context.Background(), "alice")
	require.NoError(t, gerr, "nothing was removed")
	assert.Equal(t, "alice", claimsOnDisk(t, repo, "A-1").ClaimedBy)
}

func TestTeamMemberRemoveHandsTheClaimsOver(t *testing.T) {
	repo := teamWithLead(t)
	ctx := context.Background()
	require.NoError(t, repo.PatchMember(ctx, "testuser", func(m *teamstate.Member) error { m.Role = teamstate.RoleLead; return nil }))

	// the claims of the member need an answer
	err := runTeamMemberRemove(memberRemoveCmd(t, nil), []string{"alice"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--to <member>")
	assert.Equal(t, ExitUsage, ExitCode(err))
	_, gerr := repo.GetMember(ctx, "alice")
	require.NoError(t, gerr)

	require.NoError(t, runTeamMemberRemove(memberRemoveCmd(t, map[string]string{"to": "bob"}), []string{"alice"}))
	_, gerr = repo.GetMember(ctx, "alice")
	require.ErrorIs(t, gerr, teamstate.ErrMemberNotFound)
	assert.Equal(t, "bob", claimsOnDisk(t, repo, "A-1").ClaimedBy)
	assert.Equal(t, "bob", claimsOnDisk(t, repo, "A-2").ClaimedBy)
	events := eventsOn(t, repo, teamstate.TeamEventsProject)
	require.Len(t, events, 1)
	assert.Equal(t, teamstate.EventMemberRemoved, events[0].Type)
	assert.Equal(t, "testuser", events[0].Actor)
	assert.EqualValues(t, 2, events[0].Data["claims_transferred"])
	assert.Equal(t, "bob", events[0].Data["to"])
	for _, e := range eventsOn(t, repo, claimsProject) {
		assert.Equal(t, teamstate.EventClaimTransferred, e.Type)
		assert.Equal(t, "testuser", e.Actor)
	}
}

func TestTeamMemberRemoveCanReleaseTheClaims(t *testing.T) {
	repo := teamWithLead(t)
	ctx := context.Background()
	require.NoError(t, repo.PatchMember(ctx, "testuser", func(m *teamstate.Member) error { m.Role = teamstate.RoleLead; return nil }))

	require.NoError(t, runTeamMemberRemove(memberRemoveCmd(t, map[string]string{"release": "true"}), []string{"alice"}))
	left, err := repo.ListClaims(ctx, claimsProject)
	require.NoError(t, err)
	assert.Empty(t, left)
	assert.True(t, repo.ClaimRemovedOnPurpose(ctx, claimsProject, "A-1"))
}

func TestTeamMemberRemoveRefusesYourselfAndUnknownMembers(t *testing.T) {
	repo := teamWithLead(t)
	ctx := context.Background()
	require.NoError(t, repo.PatchMember(ctx, "testuser", func(m *teamstate.Member) error { m.Role = teamstate.RoleLead; return nil }))

	err := runTeamMemberRemove(memberRemoveCmd(t, nil), []string{"testuser"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oh teams leave test-team --remove-member")

	err = runTeamMemberRemove(memberRemoveCmd(t, nil), []string{"ghost"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost is not a card of the team")

	err = runTeamMemberRemove(memberRemoveCmd(t, map[string]string{"to": "alice", "release": "true"}), []string{"bob"})
	require.Error(t, err)
	assert.Equal(t, ExitUsage, ExitCode(err))

	err = runTeamMemberRemove(memberRemoveCmd(t, map[string]string{"to": "alice"}), []string{"alice"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot go to the member being removed")
	_, gerr := repo.GetMember(ctx, "alice")
	require.NoError(t, gerr)
}

// A team with no lead is not locked: any member removes another (the same rule
// as for the roles and the enforced settings).
func TestTeamMemberRemoveInATeamWithoutALead(t *testing.T) {
	i18ntest.Set(t, "en")
	_, repo, _ := setupClaimTestApp(t)
	addTeamMembers(t, repo, "alice")
	terminalIs(t, false)

	require.NoError(t, runTeamMemberRemove(memberRemoveCmd(t, nil), []string{"alice"}))
	_, err := repo.GetMember(context.Background(), "alice")
	assert.ErrorIs(t, err, teamstate.ErrMemberNotFound)
}
