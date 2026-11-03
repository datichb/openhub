package views

import (
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// The keys of the boards besides the convention and the columns (colPrevKey,
// colNextKey).
var (
	// boardActionsKey opens the workflows that can start on the card.
	boardActionsKey = keys.Own("board.actions", 'a', "tui.hints.actions")
	// boardInitKey sets Beads up for the project (board not initialised).
	boardInitKey = keys.Own("board.init", 'i', "tui.hints.init_beads")
	// boardLinkKey links a card to a tracker reference.
	boardLinkKey = keys.Own("board.link", 'L', "tui.hints.link")

	// The team board: claim, transfer, change the status of the card, and
	// move between the projects (release is releaseKey).
	teamClaimKey    = keys.Own("teamboard.claim", 'c', "tui.hints.claim")
	teamTransferKey = keys.Own("teamboard.transfer", 't', "tui.hints.transfer")
	teamStatusKey   = keys.Own("teamboard.status", 's', "tui.hints.status")
	projectPrevKey  = keys.Own("teamboard.project_prev", '[', "tui.hints.projects")
	projectNextKey  = keys.Own("teamboard.project_next", ']', "tui.hints.projects")
	// boardColumnsKey opens the editor of the columns of the board and of the
	// labels mapped to them (the « Colonnes du board » entry of the Team mode).
	boardColumnsKey = keys.Own("teamboard.columns", 'C', "tui.hints.board_columns")
)

// projectsHint is the hint of the keys that move between the projects of the
// team board (« [/] projets »); the bracket is escaped for the tview tags.
func projectsHint() string {
	return tview.Escape(projectPrevKey.Keys()) + "/" + projectNextKey.Keys() + " " + projectPrevKey.Label()
}
