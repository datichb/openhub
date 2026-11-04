package cmd

import (
	"strings"
	"sync"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// cardImport is the Beads ticket a launch from a card runs on when the card is
// a GitLab issue. The form and its recap work on the ticket the issue is when
// it is already linked (looked up, nothing written), else on a stand-in named
// like its link (gitlab-623); the ticket is created, or found, only when the
// launch is confirmed, so that a form closed leaves nothing in Beads.
type cardImport struct {
	a      *app.App
	ticket views.TicketContext

	mu       sync.Mutex
	standIn  string
	id       string // the ticket the form runs on: standIn until the launch resolves it
	existing bool   // the issue was already linked when the form opened
}

// newCardImport reads, without writing, whether the issue of a card already
// has its Beads ticket.
func newCardImport(a *app.App, ticket views.TicketContext) *cardImport {
	c := &cardImport{a: a, ticket: ticket, standIn: beads.BuildExternalRef(beads.ProviderGitLab, ticket.ID)}
	c.id = c.standIn
	if path := boardCardPath(a, ticket); path != "" && beads.Available() == nil && beads.IsInitialized(path) {
		if linked, err := beads.FindByExternalRef(path, c.standIn); err == nil && linked != nil {
			c.id, c.existing = linked.ID, true
		}
	}
	return c
}

// current is the ticket the launch runs on.
func (c *cardImport) current() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.id
}

// tickets are the tickets of the form with the stand-in replaced by the ticket
// the launch runs on.
func (c *cardImport) tickets(in []string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(in))
	for i, t := range in {
		out[i] = t
		if t == c.standIn {
			out[i] = c.id
		}
	}
	return out
}

// noClaim are the tickets the session does not claim: the card is the claim.
func (c *cardImport) noClaim() []string { return []string{c.current()} }

// resolve returns the Beads ticket of the issue, created when there is none
// (idempotent: a second confirmation, or a launch that failed after the
// import, finds the ticket again). It is called when the launch is confirmed.
func (c *cardImport) resolve() (id string, created bool, err error) {
	id, created, err = importCardIssue(c.a, c.ticket)
	if err != nil {
		return "", false, err
	}
	c.mu.Lock()
	c.id = id
	c.mu.Unlock()
	return id, created, nil
}

// recapRow tells, in the recap of the form, which Beads ticket the issue is or
// will be.
func (c *cardImport) recapRow() views.InfoField {
	label := i18n.T("v53f.card.import_row_label")
	ref := "#" + strings.TrimSpace(c.ticket.ID)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.existing {
		return views.InfoField{Label: label, Value: i18n.Tf("v53f.card.import_row_linked", ref, c.id)}
	}
	return views.InfoField{Label: label, Value: i18n.Tf("v53f.card.import_row_new", ref, c.standIn)}
}
