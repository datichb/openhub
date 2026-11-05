package views

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

const circuitTitleFR = "Coupe-circuit : 3 délégations d'affilée sans intervention"

func circuitRows() []SessionRow {
	now := time.Now()
	return []SessionRow{{ID: "ses_a", Project: "openhub", Workflow: "ticket", Agent: "orchestrator-dev", State: "waiting", Started: now,
		Decisions: []SessionDecision{{ID: "circuit:ses_a:1", SessionID: "ses_a", Kind: DecisionKindCircuit, Icon: "⊘", Summary: circuitTitleFR, Created: now}}}}
}

// The title of a circuit decision is shown whole in the list, in the detail
// pane and in the card: the count filled in, once, in the colour of the alerts.
func TestSessionsViewCircuitDecisionTitle(t *testing.T) {
	frLocale(t)
	sh := &cardShell{}
	v := NewSessionsView(SessionsViewConfig{Backend: &fakeSessions{rows: circuitRows()}})
	v.SetShell(sh)
	v.Mount(tview.NewFlex(), tview.NewApplication())
	v.rows = circuitRows()
	v.render()

	t.Run("list", func(t *testing.T) {
		var main string
		for _, it := range v.list.GetItems() {
			if _, ok := it.Reference.(decisionRef); ok {
				main = it.MainText
			}
		}
		require.NotEmpty(t, main, "the decision has a row")
		assert.Equal(t, 1, strings.Count(main, circuitTitleFR), "the whole title, once")
		assert.NotContains(t, main, "%")
		assert.Contains(t, main, theme.Paint(theme.ErrorHex, "⊘"), "icon in the colour of the alerts")
	})

	t.Run("detail", func(t *testing.T) {
		text := v.detail.GetText(true)
		assert.Equal(t, 1, strings.Count(text, circuitTitleFR), "the whole title, once")
	})

	t.Run("card", func(t *testing.T) {
		r, d := v.selected()
		require.NotNil(t, d)
		v.openDecision(r, d)
		title, body, actions := sh.modal()
		assert.Equal(t, circuitTitleFR, body, "the card says the title, nothing else")
		assert.True(t, strings.HasPrefix(title, "⊘ "), title)
		require.NotEmpty(t, actions)
		assert.Equal(t, i18n.T("tui.inbox.dismiss"), actions[0].Label)
	})

	t.Run("a long title is not cut by the view", func(t *testing.T) {
		long := circuitTitleFR + " (" + strings.Repeat("très long ", 20) + ")"
		item := decisionItem(SessionDecision{ID: "x", Kind: DecisionKindCircuit, Icon: "⊘", Summary: long})
		assert.Contains(t, item.MainText, long)
	})

	t.Run("x archives it", func(t *testing.T) {
		be := &fakeSessions{rows: circuitRows()}
		v2 := NewSessionsView(SessionsViewConfig{Backend: be})
		v2.SetShell(&mockShell{})
		v2.Mount(tview.NewFlex(), tview.NewApplication())
		v2.rows = circuitRows()
		v2.render()
		_, d := v2.selected()
		require.NotNil(t, d)
		assert.Nil(t, v2.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'x', 0)))
		require.Eventually(t, func() bool { return len(be.decisions()) == 1 }, time.Second, 10*time.Millisecond)
		assert.Equal(t, "circuit:ses_a:1=dismiss", be.decisions()[0])
	})
}
