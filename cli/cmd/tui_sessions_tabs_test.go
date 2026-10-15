package cmd

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

func TestSessionOutputsSorted(t *testing.T) {
	got := sessionOutputs(map[string]any{"tickets": []string{"oh-12", "oh-13"}, "method": "tdd"})
	assert.Equal(t, []views.SessionOutput{{Name: "method", Value: "tdd"}, {Name: "tickets", Value: "oh-12,oh-13"}}, got)
	assert.Nil(t, sessionOutputs(nil))
}

func TestFanoutDetailLines(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("fr")
	t.Cleanup(func() { i18n.SetLocale(prev) })
	parent := &domain.Session{ID: "ses_p", Fanout: &domain.SessionFanout{Role: domain.RoleParent, Status: domain.FanoutRunning,
		Children: []domain.FanoutChild{{Ticket: "oh-12", State: domain.ChildMerged}, {Ticket: "oh-13", State: domain.ChildRunning}}}}
	assert.Equal(t, "⑂ Fan-out : 1/2 enfants mergés · en cours", fanoutLine(parent))
	child := &domain.Session{ID: "ses_c", Fanout: &domain.SessionFanout{Role: domain.RoleChild, Parent: "ses_p", Ticket: "oh-12", Branch: "oh/fanout/p/oh-12"}}
	assert.Contains(t, fanoutLine(child), "ses_p")
	assert.Contains(t, fanoutLine(child), "oh/fanout/p/oh-12")
	assert.Empty(t, fanoutLine(&domain.Session{}))
}

// The end of each child is read from the last child_done event.
func TestChildEnds(t *testing.T) {
	f := &domain.SessionFanout{Events: []domain.FanoutEvent{
		{Kind: domain.FanoutEventChildDone, Ticket: "oh-12", Message: "signal, 0.410 USD"},
		{Kind: domain.FanoutEventMerged, Ticket: "oh-12", Message: "merged"},
		{Kind: domain.FanoutEventChildDone, Ticket: "oh-13", Message: "stopped"},
	}}
	assert.Equal(t, map[string]string{"oh-12": "signal", "oh-13": "stopped"}, childEnds(f))
}

// The driver is started again only when it has work left (as the daemon).
func TestFanoutDriverNeeded(t *testing.T) {
	s := func(st domain.RunState, fs domain.FanoutStatus) *domain.Session {
		return &domain.Session{State: st, Fanout: &domain.SessionFanout{Role: domain.RoleParent, Experimental: true, Status: fs}}
	}
	assert.True(t, fanoutDriverNeeded(s(domain.RunActive, domain.FanoutRunning)))
	assert.True(t, fanoutDriverNeeded(s(domain.RunIdle, domain.FanoutFixing)))
	assert.False(t, fanoutDriverNeeded(s(domain.RunActive, domain.FanoutMerged)))
	assert.False(t, fanoutDriverNeeded(s(domain.RunStopped, domain.FanoutRunning)))
	assert.False(t, fanoutDriverNeeded(&domain.Session{}))
}

func TestStepInputs(t *testing.T) {
	assert.Equal(t, "tickets=oh-1,oh-2 branch=feat/x", stepInputs(workflowsvc.ChainSuggestion{Tickets: []string{"oh-1", "oh-2"}, Prefill: map[string]string{"branch": "feat/x"}}))
}

// A validation refused by the verification is matched by the view (it
// opens the card), with the message of oh.
func TestOverrideRequiredError(t *testing.T) {
	err := overrideRequired{&checkpoint.OverrideError{ID: "cp-2", Result: domain.CheckpointVerify{Status: domain.VerifyFailed}}}
	assert.True(t, errors.Is(err, views.ErrOverrideRequired))
	assert.Contains(t, err.Error(), "cp-2")
}
