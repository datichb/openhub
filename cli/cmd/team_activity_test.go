package cmd

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/glyph"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// oh team activity draws each event with the symbol of glyph.Events and
// the phrase of Event.Describe, the same as the Activity view of the TUI.
func TestTeamActivityUsesTheCommonEventTable(t *testing.T) {
	i18ntest.Set(t, "fr")
	_, repo, stdout := setupClaimTestApp(t)
	ctx := context.Background()
	for _, e := range []teamstate.Event{
		{Actor: "alice", Type: teamstate.EventClaimTaken, Project: "api", Ticket: "bd-7"},
		{Actor: "bob", Type: teamstate.EventWikiRejected, Project: "api", Data: map[string]interface{}{"page": "decisions", "reason": "déjà couvert"}},
		{Actor: "carol", Type: teamstate.EventReviewApproved, Project: "api", Ticket: "bd-7"},
	} {
		require.NoError(t, repo.AppendEvent(ctx, e))
	}

	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	cmd.Flags().Bool("today", true, "")
	cmd.Flags().Bool("week", false, "")
	cmd.Flags().String("project", "", "")
	cmd.Flags().String("member", "", "")
	cmd.Flags().Int("limit", 50, "")
	require.NoError(t, runTeamActivity(cmd, nil))

	out := stdout.String()
	assert.Contains(t, out, glyph.ForEvent(teamstate.EventClaimTaken).Glyph+" alice a pris api/bd-7")
	assert.Contains(t, out, glyph.ForEvent(teamstate.EventWikiRejected).Glyph+" bob proposition wiki decisions rejetée : déjà couvert")
	assert.Contains(t, out, glyph.ForEvent(teamstate.EventReviewApproved).Glyph+" carol a approuvé la revue de api/bd-7", "an event the old table did not know")
}

// oh team activity and oh team status name the project and the member, and keep
// the ID of one the registry or the team does not know.
func TestTeamStatusAndActivityShowNames(t *testing.T) {
	i18ntest.Set(t, "en")
	a, repo, stdout := setupClaimTestApp(t)
	a.Projects = &mockProjectStore{projects: []domain.Project{{ID: claimsProject, Name: "Alpha"}}}
	ctx := context.Background()
	claimsCreate(t, repo, "bd-7", "testuser", "")
	_, err := repo.CreateClaim(ctx, teamstate.Claim{TicketID: "bd-8", Project: "deadbeef", ClaimedBy: "testuser", Status: teamstate.ClaimStatusInProgress})
	require.NoError(t, err)
	require.NoError(t, repo.AppendEvent(ctx, teamstate.Event{Actor: "testuser", Type: teamstate.EventClaimTaken, Project: claimsProject, Ticket: "bd-7"}))

	status := &cobra.Command{}
	status.SetContext(ctx)
	status.Flags().Bool("detail", false, "")
	require.NoError(t, runTeamStatus(status, nil))
	out := stdout.String()
	assert.Contains(t, out, "Alpha/bd-7")
	assert.NotContains(t, out, claimsProject+"/bd-7")
	assert.Contains(t, out, "deadbeef/bd-8", "a project nobody registered stays readable")

	stdout.Reset()
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	cmd.Flags().Bool("today", true, "")
	cmd.Flags().Bool("week", false, "")
	cmd.Flags().String("project", "", "")
	cmd.Flags().String("member", "", "")
	cmd.Flags().Int("limit", 50, "")
	require.NoError(t, runTeamActivity(cmd, nil))
	assert.Contains(t, stdout.String(), "Test User")
	assert.Contains(t, stdout.String(), "claimed Alpha/bd-7")
}

// oh team status counts the tickets by the roles of the columns of the board
// of the team, lists the reserve and the tickets of a member who left (they are
// counted, so they are shown).
func TestTeamStatusFollowsTheBoardAndShowsTheReserve(t *testing.T) {
	i18ntest.Set(t, "en")
	_, repo, stdout := setupClaimTestApp(t)
	ctx := context.Background()
	team, err := repo.LoadConfig(ctx)
	require.NoError(t, err)
	team.Board.Columns = []teamstate.BoardColumnConfig{
		{ID: "backlog", Name: "Backlog", Role: teamstate.ColumnRoleInitial},
		{ID: "dev", Name: "Dev", Role: teamstate.ColumnRoleActive},
		{ID: "qa", Name: "QA", Role: teamstate.ColumnRoleActive},
		{ID: "mr", Name: "MR", Role: teamstate.ColumnRoleReview},
		{ID: "stuck", Name: "Stuck", Role: teamstate.ColumnRoleBlocked},
		{ID: "shipped", Name: "Shipped", Role: teamstate.ColumnRoleTerminal},
	}
	require.NoError(t, repo.SaveConfig(ctx, team))
	for _, c := range []teamstate.Claim{
		{TicketID: "T-1", Project: "p", ClaimedBy: "testuser", Status: "dev"},
		{TicketID: "T-2", Project: "p", ClaimedBy: "testuser", Status: "qa"},
		{TicketID: "T-3", Project: "p", ClaimedBy: "testuser", Status: "mr"},
		{TicketID: "T-4", Project: "p", ClaimedBy: "testuser", Status: "stuck"},
		{TicketID: "POOL-1", Project: "p", Status: "backlog"},
		{TicketID: "GONE-1", Project: "p", ClaimedBy: "departed", Status: "dev"},
	} {
		_, err := repo.CreateClaim(ctx, c)
		require.NoError(t, err)
	}

	status := &cobra.Command{}
	status.SetContext(ctx)
	status.Flags().Bool("detail", false, "")
	require.NoError(t, runTeamStatus(status, nil))
	out := stdout.String()
	assert.Contains(t, out, "3 active tickets")
	assert.Contains(t, out, "1 in review")
	assert.Contains(t, out, "1 blocked")
	assert.Contains(t, out, "(reserve)")
	assert.Contains(t, out, "p/POOL-1")
	assert.Contains(t, out, "departed (no longer in the team)")
	assert.Contains(t, out, "p/GONE-1")
}
