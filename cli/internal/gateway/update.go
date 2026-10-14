package gateway

import (
	"github.com/datichb/openhub/cli/internal/gateway/bdargs"
)

// OpUpdate is a change to the Beads database that neither creates nor
// closes a ticket (r9b, triage): `update` (priority, labels, type…),
// `label add`, `dep add`, `comments add`, `reopen`… The workflow guard
// refuses it before the checkpoint that unlocks `update`; a workflow that
// declares no such checkpoint locks nothing. Closing and claiming keep
// their own operation (ticketOp), creating too (createOp).
const OpUpdate = "update"

// updateOp reports a write that is neither a creation nor a closing or a
// claim (c: parsed by checkBeads). IDs are the tickets named on the command
// line, where bd expects them.
func updateOp(c command) (BeadsOp, bool) {
	if c.Name() == "" || c.Help() || bdargs.IsRead(c.Cmd) {
		return BeadsOp{}, false
	}
	if _, ok := ticketOp(c); ok {
		return BeadsOp{}, false
	}
	if _, ok := createOp(c); ok {
		return BeadsOp{}, false
	}
	return BeadsOp{Kind: OpUpdate, IDs: updatedIDs(c)}, true
}

// updatedArgs says where each ticket command of bd 1.3 takes its tickets:
// all its arguments (-1), all but the last one (-2: label add <id>…
// <label>), or the first n.
var updatedArgs = map[string]int{
	"update": -1, "edit": 1, "reopen": -1, "defer": -1, "undefer": -1, "unclaim": -1, "promote": -1,
	"label add": -2, "label remove": -2, "tag": -2,
	"dep add": 2, "dep remove": 2, "dep relate": 2, "dep unrelate": 2, "link": 2,
	"comments add": 1, "comment": 1, "note": 1, "assign": 1, "priority": 1, "set-state": 1, "rename": 1,
}

// updatedIDs returns the tickets a write names (none for a command that is
// not about given tickets: config, remember…).
func updatedIDs(c command) []string {
	n, ok := updatedArgs[c.Key()]
	if !ok {
		return nil
	}
	args := c.Args
	switch {
	case n == -2:
		args = args[:max(0, len(args)-1)]
	case n > 0:
		args = args[:min(n, len(args))]
	}
	var ids []string
	for _, a := range cleanIDs(args) {
		if reTicketID.MatchString(a) {
			ids = append(ids, a)
		}
	}
	return ids
}
