package bdargs

import "strings"

// readCommands are the bd commands (canonical paths) that never write the
// Beads database. The gateway runs them with --readonly (bd then refuses
// any write, a second line behind the allow-list), a session whose
// allow-list names only them is read-only, and a read-only workflow
// (risk: read) may allow nothing else. Closed list: a command missing here
// counts as a write.
var readCommands = map[string]bool{
	"show": true, "list": true, "ready": true, "search": true, "query": true, "count": true,
	"children": true, "blocked": true, "stale": true, "lint": true, "history": true,
	"status": true, "statuses": true, "types": true, "where": true, "info": true, "context": true,
	"graph": true, "graph check": true, "diff": true, "find-duplicates": true,
	"comments": true, "comments list": true,
	"dep list": true, "dep tree": true, "dep cycles": true,
	"label list": true, "label list-all": true,
	"epic status":    true,
	"provenance log": true, "provenance by-ref": true,
	"memories": true, "recall": true,
	"kv get": true, "kv list": true,
	"config get": true, "config list": true, "config show": true,
	"state": true, "state list": true,
	"gate list": true, "gate show": true,
	"mol show": true, "mol current": true, "mol progress": true, "mol ready": true, "mol stale": true,
	"swarm list": true, "swarm status": true, "swarm validate": true,
	"formula list": true, "formula show": true, "formula schema": true,
	"todo list": true, "human list": true, "hooks list": true,
	"conflicts list": true, "conflicts show": true,
	"dolt show": true, "dolt status": true, "backup status": true,
	"schema": true, "prime": true, "onboard": true, "quickstart": true, "version": true,
}

// writeFlags turn a read command into a write.
var writeFlags = map[string][]string{
	"ready": {"claim"},
}

// IsRead reports whether a parsed command never writes the Beads database
// (help requests and bd alone are not commands: false).
func IsRead(c Cmd) bool {
	if len(c.Path) == 0 || c.Help() || !readCommands[c.Key()] {
		return false
	}
	for _, f := range writeFlags[c.Key()] {
		if c.Bool(f) {
			return false
		}
	}
	return true
}

// EntryIsRead reports whether an allow-list entry ("show", "view" alias,
// "dep tree", "comments") names a read command. An entry naming a read
// command still grants its write forms (`comments add`, `ready --claim`)
// to a session that may write; a session whose entries are all read
// commands is read-only and only runs commands for which IsRead holds.
func EntryIsRead(entry string) bool {
	w := strings.Fields(entry)
	if len(w) == 0 || len(w) > 2 {
		return false
	}
	p, ok := Known(w[0])
	if !ok {
		return false
	}
	if len(w) == 2 {
		if sub, ok := Known(p[0], w[1]); ok {
			return readCommands[strings.Join(sub, " ")]
		}
	}
	return readCommands[p[0]]
}

// ReadOnly reports whether every entry of an allow-list names a read
// command (an empty list grants nothing: not read-only, nothing runs).
func ReadOnly(allow []string) bool {
	if len(allow) == 0 {
		return false
	}
	for _, e := range allow {
		if !EntryIsRead(e) {
			return false
		}
	}
	return true
}
