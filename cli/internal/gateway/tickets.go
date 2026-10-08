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
}

// Boolean flags of `bd close` and `bd update` (bd 1.3): any other flag
// takes a value.
var ticketBoolFlags = map[string]bool{
	"--claim": true, "--claim-next": true, "--continue": true, "-f": true, "--force": true, "--no-auto": true,
	"--suggest-next": true, "--allow-empty-description": true, "--ephemeral": true, "--history": true,
	"--no-history": true, "--persistent": true, "--stdin": true,
}

// ticketOp returns the ticket operation of a checked command (c: parsed by
// checkBeads), if any: `close` (alias `done`), `update --status closed|done`
// (closing), `update --claim` or `--status in_progress` (claiming).
func ticketOp(argv []string, c command) (BeadsOp, bool) {
	if c.At < 0 || (c.Name != "close" && c.Name != "done" && c.Name != "update") {
		return BeadsOp{}, false
	}
	var ids []string
	status, claim := "", false
	args := argv[c.At+1:]
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			ids = append(ids, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			if !strings.ContainsAny(a, " \t\n") {
				ids = append(ids, a)
			}
			continue
		}
		name, value, hasValue := strings.Cut(a, "=")
		if globalBoolFlags[name] || ticketBoolFlags[name] {
			claim = claim || name == "--claim" || name == "--claim-next"
			continue
		}
		if !hasValue && i+1 < len(args) {
			i++
			value = args[i]
		}
		if name == "-s" || name == "--status" {
			status = strings.ToLower(strings.TrimSpace(value))
		}
	}
	switch {
	case c.Name != "update":
		return BeadsOp{Kind: OpClose, IDs: ids}, true
	case status == "closed" || status == "done":
		return BeadsOp{Kind: OpClose, IDs: ids}, true
	case claim || status == "in_progress":
		return BeadsOp{Kind: OpClaim, IDs: ids}, true
	}
	return BeadsOp{}, false
}
