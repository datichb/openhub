package gateway

import (
	"bufio"
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

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

// Ticket ids in the output of a creation (R5: oh records the tickets a
// session created). Forms of bd 1.3: « ✓ Created issue: ev-1 — title »,
// « ✓ Created 2 issues from plan.md: » then « ev-1: title » lines, a bare id
// per line (`q`, `--silent`), JSON (an object or a list, `--json`).
var (
	reCreatedOne  = regexp.MustCompile(`Created issue:\s+([A-Za-z0-9][A-Za-z0-9._-]*)`)
	reCreatedMany = regexp.MustCompile(`Created \d+ issues?\b`)
	reCreatedItem = regexp.MustCompile(`^\s+([A-Za-z0-9][A-Za-z0-9._-]*-[A-Za-z0-9._]+):\s`)
	reTicketID    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*-[A-Za-z0-9._]+$`)
	rePlaceholder = regexp.MustCompile(`^pending-\d+$`)
)

// CreatedIDs returns the tickets a successful creation printed, in order,
// without duplicates; none when the output has no known form.
func CreatedIDs(stdout []byte) []string {
	var out []string
	add := func(id string) {
		id = strings.TrimRight(id, ".,;:")
		if !reTicketID.MatchString(id) || rePlaceholder.MatchString(id) {
			return // a placeholder of a remote job is not a ticket yet
		}
		for _, o := range out {
			if o == id {
				return
			}
		}
		out = append(out, id)
	}
	trimmed := bytes.TrimSpace(stdout)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		var one struct {
			ID string `json:"id"`
		}
		var many []struct {
			ID string `json:"id"`
		}
		switch {
		case json.Unmarshal(trimmed, &many) == nil:
			for _, m := range many {
				add(m.ID)
			}
		case json.Unmarshal(trimmed, &one) == nil:
			add(one.ID)
		}
		return out
	}
	var lines []string
	sc := bufio.NewScanner(bytes.NewReader(stdout))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	bare := len(lines) > 0
	for _, l := range lines {
		if t := strings.TrimSpace(l); t != "" && !reTicketID.MatchString(t) {
			bare = false
		}
	}
	if bare {
		for _, l := range lines {
			add(strings.TrimSpace(l))
		}
		return out
	}
	many := false
	for _, l := range lines {
		if m := reCreatedOne.FindStringSubmatch(l); m != nil {
			add(m[1])
			continue
		}
		if reCreatedMany.MatchString(l) {
			many = true
			continue
		}
		if m := reCreatedItem.FindStringSubmatch(l); many && m != nil {
			add(m[1])
		}
	}
	return out
}
