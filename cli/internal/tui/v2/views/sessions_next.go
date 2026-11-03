package views

import (
	"context"
	"fmt"
	"log/slog"
)

// `next:` steps of the selected session in the Détail tab: read off
// the event loop on selection, kept while the session (state, outputs)
// does not change. The steps are an addition to the detail, read without
// the user asking: when they cannot be read the detail is shown without
// them, quietly (no toast, no reload of the list).

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
	if v.app == nil {
		return cur.steps
	}
	id := r.ID
	v.next[id] = sessionNext{sig: sig, steps: cur.steps, loading: true}
	load(v.app, v.ctx, &loadSlot{}, func(ctx context.Context) ([]SessionNextStep, error) {
		return nb.NextSteps(ctx, id)
	}, func(steps []SessionNextStep, err error) {
		if v.next == nil {
			return
		}
		if err != nil {
			slog.Debug("next steps of a session unreadable", "session", id, "error", err)
			steps = nil
		}
		v.next[id] = sessionNext{sig: sig, steps: steps}
		if sel, _ := v.selected(); sel != nil && sel.ID == id {
			v.renderDetail(sel)
		}
	})
	return cur.steps
}
