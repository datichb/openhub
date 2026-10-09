package gateway

// OpCreate is the creation of tickets (`create`, `create-form`, `q`): the
// workflow guard refuses it before the checkpoint that unlocks `create`
// (N9). Kept apart from ticketOp, whose argument parsing belongs to the
// closing and claiming guard.
const OpCreate = "create"

var createCommands = map[string]bool{"create": true, "create-form": true, "q": true}

// createOp reports a ticket creation (c: parsed by checkBeads).
func createOp(c command) (BeadsOp, bool) {
	if c.At < 0 || !createCommands[c.Name] {
		return BeadsOp{}, false
	}
	return BeadsOp{Kind: OpCreate}, true
}
