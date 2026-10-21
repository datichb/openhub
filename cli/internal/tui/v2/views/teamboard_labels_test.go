package views

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// Ticket labels and the assignee must stay visible on both team boards (the
// embedded view and the full-screen `oh team board`): the card renderer reads
// "[word]" as a colour tag, so every literal bracket has to be escaped.
func TestTeamBoards_TicketLabelsAlwaysVisible(t *testing.T) {
	t.Parallel()
	const workflowLabel = "workflow-doing"
	mapping := map[string]string{workflowLabel: "in_progress"}
	ticket := TeamTicket{
		ID: "T-1", Title: "Fix the board", Status: "in_progress", Assignee: "[dev]",
		Labels: []string{
			teamstate.LabelAgentReviewed, teamstate.LabelNeedsHumanReview,
			"bug", "needs-review", workflowLabel,
		},
	}

	render := func(card widgets.Card) string {
		col := widgets.NewCardColumn("Col", theme.Accent)
		col.AddCard(card)
		return screenText(t, col, 80, 12)
	}

	embedded := NewTeamBoardView(TeamBoardViewConfig{LabelStatusMapping: mapping})
	screens := map[string]string{
		"vue intégrée":      render(embedded.buildTeamCard(ticket)),
		"board plein écran": render(buildFullScreenCard(ticket, mapping)),
	}

	for name, screen := range screens {
		assert.Contains(t, screen, "[AI]", "%s : le badge « revu par l'IA » doit être visible", name)
		assert.Contains(t, screen, "[!]", "%s : le badge « revue humaine requise » doit être visible", name)
		assert.Contains(t, screen, "bug", "%s : l'étiquette bug doit être visible", name)
		assert.Contains(t, screen, "needs-review", "%s : l'étiquette needs-review doit être visible", name)
		assert.Contains(t, screen, "@[dev]", "%s : l'assigné [dev] doit être visible", name)
		assert.NotContains(t, screen, workflowLabel, "%s : l'étiquette de workflow doit être masquée", name)
		for _, raw := range []string{"[-]", "[green]", "[#", "[gray]", "[yellow]"} {
			assert.NotContains(t, screen, raw, "%s : aucune balise brute (%s) dans le texte visible", name, raw)
		}
	}
}
