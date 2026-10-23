package views

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gatedSessions is a backend whose List waits for its gate, as a slow daemon
// does; each call takes the next gate and the next answer.
type gatedSessions struct {
	fakeSessions
	calls   atomic.Int32
	started chan struct{}
	gates   []chan struct{}
	rows    [][]SessionRow
	errs    []error
}

func (g *gatedSessions) List(ctx context.Context, _ string, _ bool) ([]SessionRow, error) {
	n := int(g.calls.Add(1)) - 1
	if n < len(g.gates) && g.gates[n] != nil {
		if n == 0 && g.started != nil {
			close(g.started)
		}
		<-g.gates[n]
	}
	var err error
	if n < len(g.errs) {
		err = g.errs[n]
	}
	if n < len(g.rows) {
		return g.rows[n], err
	}
	return nil, err
}

func sessionsText(loop *tview.Application, v *SessionsView) (txt string) {
	onLoop(loop, func() {
		if v.list == nil {
			return
		}
		var parts []string
		for _, it := range v.list.GetItems() {
			parts = append(parts, it.MainText)
		}
		txt = strings.Join(parts, "\n")
	})
	return
}

func mountedSessions(t *testing.T, be SessionsBackend) (*SessionsView, *tview.Application, *tview.Flex) {
	t.Helper()
	v := NewSessionsView(SessionsViewConfig{Backend: be})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)
	onLoop(loop, func() { v.Mount(content, loop) })
	return v, loop, content
}

// The mount returns with the loading text while the sessions are read, and
// the list comes when the backend answers.
func TestSessionsViewMountDoesNotBlockOnTheBackend(t *testing.T) {
	frLocale(t)
	gate := make(chan struct{})
	be := &gatedSessions{gates: []chan struct{}{gate}, rows: [][]SessionRow{{{ID: "ses_b", Agent: "reviewer", State: "active", StateIcon: "●"}}}}
	v, loop, _ := mountedSessions(t, be)

	require.Eventually(t, func() bool { return be.calls.Load() == 1 }, 2*time.Second, 5*time.Millisecond)
	assert.Contains(t, sessionsText(loop, v), "Chargement")
	close(gate)
	require.Eventually(t, func() bool { return strings.Contains(sessionsText(loop, v), "reviewer") }, 2*time.Second, 10*time.Millisecond)
	assert.NotContains(t, sessionsText(loop, v), "Chargement")
}

// A read that fails when nothing was ever read says so in the list, instead
// of leaving « Chargement… » for ever; r reads again.
func TestSessionsViewFirstReadFailureIsShown(t *testing.T) {
	frLocale(t)
	be := &gatedSessions{
		errs: []error{errors.New("daemon down")},
		rows: [][]SessionRow{nil, {{ID: "ses_b", Agent: "reviewer", State: "active", StateIcon: "●"}}},
	}
	v, loop, _ := mountedSessions(t, be)
	require.Eventually(t, func() bool {
		return strings.Contains(sessionsText(loop, v), "Impossible de charger : daemon down")
	}, 2*time.Second, 10*time.Millisecond)

	onLoop(loop, v.reload)
	require.Eventually(t, func() bool { return strings.Contains(sessionsText(loop, v), "reviewer") }, 2*time.Second, 10*time.Millisecond)
}

// The sessions read by a previous mount, slow to come back, are applied to
// neither the unmounted view nor the new mount (run with -race).
func TestSessionsViewStaleReadAfterRemountIsIgnored(t *testing.T) {
	frLocale(t)
	gate := make(chan struct{})
	started := make(chan struct{})
	be := &gatedSessions{
		started: started,
		gates:   []chan struct{}{gate},
		rows: [][]SessionRow{
			{{ID: "ses_old", Agent: "oldagent", State: "active", StateIcon: "●"}},
			{{ID: "ses_new", Agent: "newagent", State: "active", StateIcon: "●"}},
		},
	}
	v := NewSessionsView(SessionsViewConfig{Backend: be})
	v.SetShell(&mockShell{})
	first, second := tview.NewFlex(), tview.NewFlex()
	loop := runApp(t, first)

	onLoop(loop, func() { v.Mount(first, loop) })
	<-started
	onLoop(loop, v.Unmount)
	onLoop(loop, func() { v.Mount(second, loop) })
	require.Eventually(t, func() bool { return strings.Contains(sessionsText(loop, v), "newagent") }, 2*time.Second, 10*time.Millisecond)

	close(gate) // the first read answers now
	time.Sleep(50 * time.Millisecond)
	txt := sessionsText(loop, v)
	assert.Contains(t, txt, "newagent")
	assert.NotContains(t, txt, "oldagent", "the answer of the previous mount is dropped")

}

// failingTrace is a backend whose trace cannot be read.
type failingTrace struct{ fakeSessions }

func (*failingTrace) SessionTrace(context.Context, string) (string, error) {
	return "", errors.New("trace unreadable")
}

// A trace that cannot be read is shown in the tab with the common words,
// not left on « loading ».
func TestSessionsTraceFailureIsShownInTheTab(t *testing.T) {
	frLocale(t)
	be := &failingTrace{fakeSessions: fakeSessions{rows: fanoutRows()}}
	v, loop, _ := mountedSessions(t, be)
	require.Eventually(t, func() bool { return strings.Contains(sessionsText(loop, v), "implement-spec") }, 2*time.Second, 10*time.Millisecond)
	onLoop(loop, func() { v.selectKey("s:ses_8f2a01"); v.onSelectionChanged(); v.showTab(tabTrace) })
	require.Eventually(t, func() bool {
		var txt string
		onLoop(loop, func() { txt = v.tabs.trace.text.GetText(true) })
		return strings.Contains(txt, "Impossible de charger : trace unreadable")
	}, 2*time.Second, 10*time.Millisecond)
}
