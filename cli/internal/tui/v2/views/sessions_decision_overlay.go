package views

import "github.com/datichb/openhub/cli/internal/i18n"

// decisionOverlay is the overlay showing a decision (card, form): it is
// closed when the decision is settled elsewhere — in the tool interface, by
// another oh, by the agent (A37).
type decisionOverlay struct {
	id    string
	token any
}

// trackDecision records the overlay just shown for decision id.
func (v *SessionsView) trackDecision(id string) {
	c, ok := v.shell.(OverlayCloser)
	if !ok {
		return
	}
	v.decOverlay = decisionOverlay{id: id, token: c.CurrentOverlay()}
}

// untrackDecision forgets it (the decision is being answered here).
func (v *SessionsView) untrackDecision() { v.decOverlay = decisionOverlay{} }

// closeSettledDecision closes the overlay of a decision that is no longer
// open (called after each reload of the list).
func (v *SessionsView) closeSettledDecision() {
	o := v.decOverlay
	if o.token == nil || decisionOpen(v.rows, o.id) {
		return
	}
	v.untrackDecision()
	if c, ok := v.shell.(OverlayCloser); ok && c.CloseOverlay(o.token) {
		v.toast(i18n.Tf("tui.inbox.settled_elsewhere", ToolName()), true)
	}
}

func decisionOpen(rows []SessionRow, id string) bool {
	for _, r := range rows {
		for _, d := range r.Decisions {
			if d.ID == id {
				return true
			}
		}
	}
	return false
}
