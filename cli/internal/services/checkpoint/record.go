package checkpoint

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
)

// Outputs recorded by oh (R5): an output declared with `record:` is filled
// by oh from what it saw the session do, never by an agent (Declare refuses
// it): tickets created or closed through the Beads gateway, specs checked
// at a `verify.spec` checkpoint, reports deposited with workflow_report,
// files written under paths (written.go).

// Record kinds (workflow.OutputRecord).
const (
	RecordCreated = "created"
	RecordClosed  = "closed"
	RecordSpec    = "spec"
	RecordReport  = "report"
	RecordWritten = "written" // written.go
)

// outputBeadsIDs is the output type of a list of tickets.
const outputBeadsIDs = "beads-ids"

// recordMu serializes the read-modify-write of the lists (tickets created
// by two commands at once).
var recordMu sync.Mutex

// Record adds values to the output of the session recorded with kind (a
// list for beads-ids, appended without duplicates; else the last value
// replaces the previous one). Nothing is recorded when the workflow has no
// output of that kind. It reports whether the output changed.
func (s *Service) Record(ctx context.Context, sessionID, kind string, values ...string) (bool, error) {
	var clean []string
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			clean = append(clean, v)
		}
	}
	if len(clean) == 0 {
		return false, nil
	}
	sess, wf, err := s.session(ctx, sessionID)
	if errors.Is(err, ErrNoWorkflow) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	def, ok := wf.Recorded(kind)
	if !ok {
		return false, nil
	}
	if s.SessionOutputs == nil {
		return false, errors.New("checkpoint: no output store")
	}
	recordMu.Lock()
	defer recordMu.Unlock()
	if def.Type != outputBeadsIDs {
		v := strings.Join(clean, ",")
		if fmt.Sprint(sess.Outputs[def.ID]) == v {
			return false, nil
		}
		return true, s.SessionOutputs.SetSessionOutput(ctx, sessionID, def.ID, v)
	}
	// Re-read the session: another command may have recorded tickets since.
	if cur, err := s.Sessions.Get(ctx, sessionID); err == nil {
		sess = cur
	}
	list := ticketValues(sess.Outputs[def.ID])
	changed := false
	for _, v := range clean {
		if !slices.Contains(list, v) {
			list = append(list, v)
			changed = true
		}
	}
	if !changed {
		return false, nil
	}
	return true, s.SessionOutputs.SetSessionOutput(ctx, sessionID, def.ID, list)
}

// ticketValues reads a stored list of tickets.
func ticketValues(v any) []string {
	var out []string
	switch x := v.(type) {
	case nil:
	case []string:
		out = append(out, x...)
	case []any:
		for _, e := range x {
			out = append(out, fmt.Sprint(e))
		}
	default:
		for _, p := range strings.Split(fmt.Sprint(x), ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}
