package views

import (
	"context"
	"fmt"
)

// `next:` steps of the selected session in the Détail tab (R5): read off
// the event loop on selection, kept while the session (state, outputs)
// does not change.

type sessionNext struct {
	sig     string
	steps   []SessionNextStep
	loading bool
}

func nextSignature(r *SessionRow) string {
	return fmt.Sprintf("%s|%d|%t", r.State, len(r.Outputs), r.Finished)
}

// nextSteps returns the steps known for r, and loads them when they are
// not (the detail is drawn again once they are).
func (v *SessionsView) nextSteps(r *SessionRow) []SessionNextStep {
	nb, ok := v.cfg.Backend.(SessionNexter)
	if !ok || r == nil || r.Workflow == "" || v.next == nil {
		return nil
	}
	sig := nextSignature(r)
	cur, known := v.next[r.ID]
	if known && (cur.sig == sig || cur.loading) {
		return cur.steps
	}
	id := r.ID
	v.next[id] = sessionNext{sig: sig, steps: cur.steps, loading: true}
	var steps []SessionNextStep
	v.async(func(ctx context.Context) (string, error) {
		var err error
		steps, err = nb.NextSteps(ctx, id)
		return "", err
	}, func(string) {
		if v.next == nil {
			return
		}
		v.next[id] = sessionNext{sig: sig, steps: steps}
		if sel, _ := v.selected(); sel != nil && sel.ID == id {
			v.renderDetail(sel)
		}
	})
	return cur.steps
}
