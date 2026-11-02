package cmd

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/glyph"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// v5.1: oh team activity draws each event with the symbol of glyph.Events and
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
