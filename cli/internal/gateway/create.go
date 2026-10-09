package gateway

// OpCreate is the creation of tickets (`create`, `create-form`, `q`): the
// workflow guard refuses it before the checkpoint that unlocks `create`
// (N9). Kept apart from ticketOp, whose argument parsing belongs to the
// closing and claiming guard.
const OpCreate = "create"

var createCommands = map[string]bool{"create": true, "create-form": true, "q": true}

// createOp reports a ticket creation (c: parsed by checkBeads, through the
// bd argument reader of bdargs: aliases and attached forms are canonical).
func createOp(c command) (BeadsOp, bool) {
	if c.Name() == "" || c.Help() || !createCommands[c.Key()] {
		return BeadsOp{}, false
	}
	return BeadsOp{Kind: OpCreate}, true
}
