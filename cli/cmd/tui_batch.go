package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Validation by batch in the TUI (R6, CP-R6): the same selection and the
// same answers as `oh session approve --batch` (session_batch.go).

var _ views.SessionBatcher = (*tuiSessions)(nil)

// batchSelection is the batch of a session: its fan-out when it is in one,
// else every checkpoint waiting.
func (t *tuiSessions) batchSelection(ctx context.Context, svc *sessionsvc.Service, sessionID string) sessionsvc.BatchSelection {
	var sel sessionsvc.BatchSelection
	if sessionID == "" {
		return sel
	}
	if sess, err := svc.Sessions.Get(ctx, sessionID); err == nil && sess.Fanout != nil {
		sel.Sessions = fanoutFamily(ctx, t.a, sess)
	}
	return sel
}

func (t *tuiSessions) BatchItems(ctx context.Context, sessionID string) ([]views.BatchDecision, error) {
	svc, err := t.service(ctx)
	if err != nil {
		return nil, err
	}
	items, err := svc.Inbox(ctx, domain.DecisionFilter{})
	if err != nil {
		return nil, err
	}
	var out []views.BatchDecision
	for _, it := range sessionsvc.SelectBatch(items, t.batchSelection(ctx, svc, sessionID)) {
		out = append(out, batchDecisionView(ctx, svc, it))
	}
	return out, nil
}

func batchDecisionView(ctx context.Context, svc *sessionsvc.Service, it sessionsvc.BatchItem) views.BatchDecision {
	ac := approverOf(ctx, svc, it.Decision)
	return views.BatchDecision{ID: it.Decision.ID, SessionID: it.Decision.SessionID, Checkpoint: it.Checkpoint,
		Summary: sessionsvc.DecisionSummary(it.Decision), Verify: batchVerify(ctx, svc, it),
		Approver: ac.ApproverText(), Identity: ac.Identity.String(), ApproverMismatch: ac.Mismatch}
}

func (t *tuiSessions) ApplyBatch(ctx context.Context, choices []views.BatchChoice) ([]views.BatchResult, error) {
	svc, err := t.service(ctx)
	if err != nil {
		return nil, err
	}
	items, err := svc.Inbox(ctx, domain.DecisionFilter{})
	if err != nil {
		return nil, err
	}
	batch := choicesBatch(items, choices)
	batch = sessionsvc.ApplyBatch(ctx, batch, func(ctx context.Context, r sessionsvc.Reply) error { return decisionError(svc.Decide(ctx, r)) })
	answered := map[string]sessionsvc.BatchItem{}
	for _, it := range batch {
		answered[it.Decision.ID] = it
	}
	out := make([]views.BatchResult, 0, len(choices))
	for _, c := range choices {
		it, ok := answered[c.ID]
		if !ok {
			out = append(out, views.BatchResult{BatchDecision: views.BatchDecision{ID: c.ID}, Action: c.Action, Error: i18n.T("tui.batch.gone")})
			continue
		}
		out = append(out, views.BatchResult{BatchDecision: batchDecisionView(ctx, svc, it), Action: string(it.Action), Error: it.Error})
	}
	return out, nil
}

// choicesBatch is the batch of the choices made in the TUI, among the
// decisions still waiting (one answered meanwhile is left out).
func choicesBatch(items []sessionsvc.InboxItem, choices []views.BatchChoice) []sessionsvc.BatchItem {
	sel := sessionsvc.BatchSelection{Reject: map[string]string{}}
	chosen := map[string]bool{}
	for _, c := range choices {
		chosen[c.ID] = true
		switch c.Action {
		case views.BatchReject:
			sel.Reject[c.ID] = c.Message
		case views.BatchApprove:
		default:
			sel.Except = append(sel.Except, c.ID)
		}
	}
	var batch []sessionsvc.BatchItem
	for _, it := range sessionsvc.SelectBatch(items, sel) {
		if chosen[it.Decision.ID] {
			batch = append(batch, it)
		}
	}
	return batch
}
