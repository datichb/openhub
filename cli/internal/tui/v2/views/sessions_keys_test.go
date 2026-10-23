package views

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// actionSessions records what a, m and i ask of the backend.
type actionSessions struct {
	fakeSessions
	amu   sync.Mutex
	calls []string
}

func (s *actionSessions) record(c string) {
	s.amu.Lock()
	defer s.amu.Unlock()
	s.calls = append(s.calls, c)
}
func (s *actionSessions) Attach(id, how string) { s.record("attach " + id + " [" + how + "]") }
func (s *actionSessions) Send(_ context.Context, id, text string) error {
	s.record("send " + id + " " + text)
	return nil
}
func (s *actionSessions) Interrupt(_ context.Context, id string) error {
	s.record("interrupt " + id)
	return nil
}
func (s *actionSessions) got() []string {
	s.amu.Lock()
	defer s.amu.Unlock()
	return append([]string(nil), s.calls...)
}

// a attaches, m sends an instruction, i interrupts: keys of the view
// declared with keys.Own, matched with Matches (modernisation tui-fanout).
func TestSessionsOwnKeysAttachSendInterrupt(t *testing.T) {
	frLocale(t)
	be := &actionSessions{fakeSessions: fakeSessions{rows: fanoutRows()}}
	sh := &dialogShell{}
	sh.answer("  ", "relis les tests")
	v := NewSessionsView(SessionsViewConfig{Backend: be})
	v.SetShell(sh)
	onLoop := harness(t, v, be.rows)
	onLoop(func() { v.selectKey("s:ses_spec01"); v.onSelectionChanged() })
	onLoop(func() {
		assert.Nil(t, v.HandleKey(key('a')))
		assert.Nil(t, v.HandleKey(key('m')), "a blank instruction is not sent")
		assert.Nil(t, v.HandleKey(key('m')))
		assert.Nil(t, v.HandleKey(key('i')))
		assert.NotNil(t, v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModAlt)), "Alt+a is not a")
	})
	require.Eventually(t, func() bool { return len(be.got()) == 3 }, 3*time.Second, 10*time.Millisecond)
	assert.ElementsMatch(t, []string{"attach ses_spec01 []", "send ses_spec01 relis les tests", "interrupt ses_spec01"}, be.got())
}

// U-07: the hints bar of the list holds the essential keys; the help (?)
// lists all of them. A tab or the batch has its own line in both.
func TestSessionsHintsAreEssentialAndHelpListsAll(t *testing.T) {
	frLocale(t)
	v := NewSessionsView(SessionsViewConfig{Backend: &fakeSessions{}})
	hints := v.StatusHints()
	assert.LessOrEqual(t, len(strings.Split(hints, " · ")), 10)
	assert.True(t, strings.HasSuffix(hints, "? aide"), hints)
	all := v.HelpKeys()
	for _, k := range []string{"A ouvrir avec", "t suivre", "i interr.", "M modèle", "e enchaîner", "B lot", "d supprimer", "R reprendre", "o MR", "w navigateur", "$ relever", "x classer"} {
		assert.Contains(t, all, k)
		assert.NotContains(t, hints, k, "%s belongs to the help, not to the bar", k)
	}
	assert.Contains(t, all, "Entrée décider/suivre")
}

// A6: an instruction to a session that is over is not asked for: nothing
// reads it. The view says so instead of opening the input and, after the
// text is typed, failing (or worse, saying « Consigne envoyée »).
func TestSessionsSendToAnEndedSessionIsRefused(t *testing.T) {
	frLocale(t)
	be := &actionSessions{fakeSessions: fakeSessions{rows: fanoutRows()}}
	sh := &dialogShell{}
	sh.answer("relis les tests")
	v := NewSessionsView(SessionsViewConfig{Backend: be})
	v.SetShell(sh)
	onLoop := harness(t, v, be.rows)

	onLoop(func() { v.selectKey("s:ses_91aa01"); v.onSelectionChanged() })
	onLoop(func() { assert.Nil(t, v.HandleKey(key('m'))) })
	sh.cmu.Lock()
	toasts := append([]string(nil), sh.toasts...)
	sh.cmu.Unlock()
	assert.Equal(t, []string{"La session est terminée : la consigne ne serait pas remise. Pour continuer, ouvre-la (a)."}, toasts)
	assert.Empty(t, sh.inputs, "no input is offered for a session that is over")
	assert.Empty(t, be.got())

	// An open session still takes it.
	onLoop(func() { v.selectKey("s:ses_91ab01"); v.onSelectionChanged() })
	onLoop(func() { assert.Nil(t, v.HandleKey(key('m'))) })
	require.Eventually(t, func() bool { return len(be.got()) == 1 }, 3*time.Second, 10*time.Millisecond)
	assert.Equal(t, []string{"send ses_91ab01 relis les tests"}, be.got())
}
