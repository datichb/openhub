package views

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// M1: an instruction to a session that has not started yet (queued for a
// free slot, preparing) is not asked for: the view says so, as for a session
// that is over, instead of opening the input and failing after the text is
// typed.
func TestSessionsSendToASessionNotStartedIsRefused(t *testing.T) {
	frLocale(t)
	at := time.Now().Add(-time.Minute)
	rows := []SessionRow{
		{ID: "ses_queued", Agent: "developer", State: "queued", StateIcon: "…", StateLabel: "en file", Started: at},
		{ID: "ses_prep", Agent: "developer", State: "preparing", StateIcon: "◓", StateLabel: "préparation", Started: at},
		{ID: "ses_run", Agent: "developer", State: "active", StateIcon: "●", StateLabel: "en cours", Started: at},
	}
	be := &actionSessions{fakeSessions: fakeSessions{rows: rows}}
	sh := &dialogShell{}
	sh.answer("relis les tests")
	v := NewSessionsView(SessionsViewConfig{Backend: be})
	v.SetShell(sh)
	onLoop := harness(t, v, be.rows)

	for _, id := range []string{"ses_queued", "ses_prep"} {
		onLoop(func() { v.selectKey("s:" + id); v.onSelectionChanged() })
		onLoop(func() { assert.Nil(t, v.HandleKey(key('m'))) })
	}
	sh.cmu.Lock()
	toasts := append([]string(nil), sh.toasts...)
	sh.cmu.Unlock()
	want := "La session n'a pas encore démarré : la consigne ne serait pas envoyée. Réessaie quand elle sera active."
	assert.Equal(t, []string{want, want}, toasts)
	assert.Empty(t, sh.inputs, "no input is offered")
	assert.Empty(t, be.got())

	onLoop(func() { v.selectKey("s:ses_run"); v.onSelectionChanged() })
	onLoop(func() { assert.Nil(t, v.HandleKey(key('m'))) })
	require.Eventually(t, func() bool { return len(be.got()) == 1 }, 3*time.Second, 10*time.Millisecond)
}
