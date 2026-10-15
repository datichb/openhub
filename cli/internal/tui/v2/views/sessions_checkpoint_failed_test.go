package views

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingSessions refuses to validate a checkpoint whose verification
// failed, as oh does (only the override passes it).
type failingSessions struct {
	cardSessions
}

func (f *failingSessions) Decide(ctx context.Context, id, choice, message string, answers map[string]string) error {
	if choice == checkpointApprove && f.card.Verify.Failed() {
		return ErrOverrideRequired
	}
	return f.fakeSessions.Decide(ctx, id, choice, message, answers)
}

func failedCard() CheckpointCardView {
	return CheckpointCardView{Checkpoint: "cp-2", Label: "Commit", Summary: "ticket oh-14 prêt",
		Approver: CheckpointApproverView{Expected: "Grace Hopper", Identity: "Ben Datiche", Mismatch: true},
		Verify: CheckpointVerifyView{Status: verifyFail, Summary: "échouée — go test ./... ✗ (1)",
			Excerpt: "--- FAIL: TestTokenRefresh (0.84s)\nFAIL internal/auth"},
		Reports: []CheckpointReport{{ID: "rp-7", Agent: "reviewer", Title: "Revue", Bytes: 4200}},
		Patch:   "diff --git a/x b/x"}
}

// cardText renders a card as the window shows it: the title, the body,
// then the buttons.
func cardText(t *testing.T, title, body string, actions []ModalAction) string {
	t.Helper()
	tv := tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	tv.SetBorder(true).SetTitle(" " + title + " ")
	var labels []string
	for _, a := range actions {
		labels = append(labels, "[ "+a.Label+" ]")
	}
	tv.SetText(body + "\n" + tview.Escape(strings.Join(labels, "  ")))
	return screenText(t, tv, 90, 32)
}

// y on a checkpoint whose verification failed opens its card on
// « Corriger » / « Passer outre… » instead of a toast; the override asks a
// reason, then a confirmation, and the reason goes with the choice.
func TestCheckpointYOnFailedVerification(t *testing.T) {
	frLocale(t)
	rows := checkpointRows()
	be := &failingSessions{cardSessions{fakeSessions: fakeSessions{rows: rows}, card: failedCard()}}
	sh := &dialogShell{}
	v := NewSessionsView(SessionsViewConfig{Backend: be})
	v.SetShell(sh)
	onLoop := harness(t, v, rows)
	onLoop(func() {
		v.selectKey(refKey(decisionRef{id: "checkpoint:ses_a:per_2", session: "ses_a"}))
		v.HandleKey(key('y'))
	})
	require.Eventually(t, func() bool { title, _, _ := sh.modal(); return title != "" }, 3*time.Second, 10*time.Millisecond)
	assert.Empty(t, sh.toasts, "no error toast")
	title, body, actions := sh.modal()
	assertGolden(t, "sessions_checkpoint_verify_failed", cardText(t, title, body, actions))
	require.GreaterOrEqual(t, len(actions), 3)
	assert.Equal(t, "Corriger (consigne à orchestrator-dev)", actions[0].Label)
	assert.Equal(t, "Passer outre…", actions[1].Label)
	assert.True(t, actions[1].Danger)

	// Corriger: the instruction to the agent.
	sh.answer("corrige TestTokenRefresh")
	onLoop(actions[0].Callback)
	require.Eventually(t, func() bool { return len(be.decisions()) == 1 }, 3*time.Second, 10*time.Millisecond)
	assert.Equal(t, "checkpoint:ses_a:per_2=fix:corrige TestTokenRefresh", be.decisions()[0])

	// Passer outre…: an empty reason is refused, then the confirmation.
	sh.answer("  ")
	onLoop(actions[1].Callback)
	sh.cmu.Lock()
	assert.Nil(t, sh.confirm)
	sh.cmu.Unlock()
	sh.answer("flaky connu, ticket oh-20")
	onLoop(actions[1].Callback)
	sh.cmu.Lock()
	confirm, ctitle, cdetail := sh.confirm, sh.ctitle, sh.cdetail
	sh.cmu.Unlock()
	require.NotNil(t, confirm)
	assert.True(t, sh.danger)
	assert.Contains(t, ctitle, "cp-2")
	assert.Contains(t, cdetail, "flaky connu")
	assert.Len(t, be.decisions(), 1, "nothing before the confirmation")
	onLoop(confirm)
	require.Eventually(t, func() bool { return len(be.decisions()) == 2 }, 3*time.Second, 10*time.Millisecond)
	assert.Equal(t, "checkpoint:ses_a:per_2=override:flaky connu, ticket oh-20", be.decisions()[1])
}

// Where oh never overrides (merge checkpoint of a fan-out), the card has
// no « Passer outre… » and the form no override choice.
func TestCheckpointNoOverride(t *testing.T) {
	frLocale(t)
	card := failedCard()
	card.Checkpoint = "cp-merge"
	card.Verify.NoOverride = true
	v := NewSessionsView(SessionsViewConfig{Backend: &fakeSessions{}})
	sh := &dialogShell{}
	v.SetShell(sh)
	d := SessionDecision{ID: "d1", SessionID: "ses_a", Kind: DecisionKindCheckpoint, Summary: "cp-merge"}
	actions := v.verifyFailedActions(&SessionRow{ID: "ses_a", Agent: "orchestrator-dev"}, &d, card)
	require.Len(t, actions, 1)
	assert.Contains(t, actions[0].Label, "Corriger")
	assert.Empty(t, checkpointExtraChoices(card.Verify))
	body := checkpointBody(card)
	assert.Contains(t, body, "jamais outre")
	assert.Nil(t, v.verifyFailedActions(nil, &d, CheckpointCardView{Verify: CheckpointVerifyView{Status: verifyPass}}))
}

// n on a checkpoint: the form shows the expected approver and, out of
// approver, the warning.
func TestCheckpointRefuseShowsApprover(t *testing.T) {
	frLocale(t)
	rows := checkpointRows()
	rows[0].Decisions[0].Approver = "Grace Hopper"
	be := &cardSessions{fakeSessions: fakeSessions{rows: rows}, card: failedCard()}
	sh := &dialogShell{}
	v := NewSessionsView(SessionsViewConfig{Backend: be})
	v.SetShell(sh)
	onLoop := harness(t, v, rows)
	onLoop(func() {
		v.selectKey(refKey(decisionRef{id: "checkpoint:ses_a:per_2", session: "ses_a"}))
		v.HandleKey(key('n'))
	})
	var form *InlineFormConfig
	require.Eventually(t, func() bool {
		sh.mu.Lock()
		defer sh.mu.Unlock()
		form = sh.form
		return form != nil
	}, 3*time.Second, 10*time.Millisecond)
	assert.Equal(t, checkpointFix, form.Fields[0].Default)
	assert.Contains(t, form.Fields[0].Hint, "Valideur attendu : Grace Hopper")
	assert.Contains(t, form.Fields[0].Hint, "Ben Datiche")
	assert.Contains(t, form.Fields[0].Hint, "⚠")

	// Without approver: no line.
	assert.Empty(t, approverHint(CheckpointApproverView{}))
	assert.Contains(t, approverHint(CheckpointApproverView{Expected: "Ada", Identity: "Ada"}), "Vous êtes")
}
