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
