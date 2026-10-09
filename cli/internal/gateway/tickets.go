package gateway

import "strings"

// Ticket operations seen by the workflow guard (A17, A38): closing a ticket
// waits for the checkpoint that unlocks it; claimed and closed tickets tell
// oh when the workflow is over.

// Ticket operation kinds.
const (
	OpClose = "close"
	OpClaim = "claim"
)

// BeadsOp is a bd command that changes the state of tickets.
type BeadsOp struct {
	Kind string
	IDs  []string // tickets named on the command line (may be empty)
	// Status is a status of the project's own configuration set by
	// `update --status` (the gateway asks bd whether it closes the ticket).
	Status string
}

// Statuses of bd 1.3 that do not close a ticket (category active, wip,
// frozen); closed is the built-in done status.
var openStatuses = map[string]bool{
	"open": true, "in_progress": true, "blocked": true, "deferred": true, "pinned": true, "hooked": true,
}

// closingCommands close tickets whatever their flags (bd 1.3): the
// duplicate and the superseded issues are closed by bd, an import or a
// batch can change any status, raw SQL anything.
var closingCommands = map[string]bool{
	"close": true, "duplicate": true, "supersede": true, "import": true, "batch": true, "sql": true,
	"todo done": true, "gate resolve": true, "mol squash": true, "mol burn": true,
}

// ticketOp returns the ticket operation of a checked command, if any:
// closing commands (close and its alias done, duplicate, supersede, import,
// batch…), `update --status` with a done status (closing), `update
// --claim`, `ready --claim` or `--status in_progress` (claiming).
func ticketOp(c command) (BeadsOp, bool) {
	if c.Name() == "" || c.Help() {
		return BeadsOp{}, false
	}
	key := c.Key()
	switch {
	case closingCommands[key]:
		ids := c.Args
		switch key {
		case "duplicate", "supersede":
			ids = c.Args[:min(1, len(c.Args))] // the second issue stays open
		case "import", "batch", "sql":
			ids = nil
		}
		return BeadsOp{Kind: OpClose, IDs: cleanIDs(ids)}, true
	case key == "update":
		if v, ok := c.Value("status"); ok {
			st := strings.ToLower(strings.TrimSpace(v))
			switch {
			case st == "closed" || st == "done":
				return BeadsOp{Kind: OpClose, IDs: cleanIDs(c.Args)}, true
			case st == "in_progress":
				return BeadsOp{Kind: OpClaim, IDs: cleanIDs(c.Args)}, true
			case !openStatuses[st]:
				return BeadsOp{Kind: OpClose, IDs: cleanIDs(c.Args), Status: st}, true
			}
		}
		if c.Bool("claim") {
			return BeadsOp{Kind: OpClaim, IDs: cleanIDs(c.Args)}, true
		}
	case key == "ready" && c.Bool("claim"):
		return BeadsOp{Kind: OpClaim}, true
	}
	return BeadsOp{}, false
}

func cleanIDs(args []string) []string {
	var ids []string
	for _, a := range args {
		if a != "" && a != "-" && !strings.ContainsAny(a, " \t\n") {
			ids = append(ids, a)
		}
	}
	return ids
}
