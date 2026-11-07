package views

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nextFails is a backend whose `next:` steps cannot be read (a workflow that
// no longer validates, a team-state that is not cloned…).
type nextFails struct {
	tabSessions
	nextCalls, lists atomic.Int32
}

func (n *nextFails) NextSteps(context.Context, string) ([]SessionNextStep, error) {
	n.nextCalls.Add(1)
	return nil, errors.New("workflow hub/feature is invalid")
}

func (n *nextFails) List(ctx context.Context, project string, all bool) ([]SessionRow, error) {
	n.lists.Add(1)
	return n.tabSessions.List(ctx, project, all)
}

// The `next:` steps are read without the user asking, for the detail: when
// they cannot be read the detail is shown without them. No red toast at each
// session the cursor crosses, and no reload of the list.
func TestSessionsNextStepsErrorIsQuiet(t *testing.T) {
	frLocale(t)
	be := &nextFails{tabSessions: tabSessions{fakeSessions: fakeSessions{rows: fanoutRows()}}}
	sh := &recordingShell{}
	v := NewSessionsView(SessionsViewConfig{Backend: be})
	v.SetShell(sh)
	onLoop := harness(t, v, be.rows)
	require.Eventually(t, func() bool { return be.lists.Load() >= 1 }, 3*time.Second, 10*time.Millisecond, "the mount reads the list")
	lists := be.lists.Load()

	for _, key := range []string{"s:ses_8f2a01", "s:ses_spec01", "s:ses_91aa01"} {
		onLoop(func() { v.selectKey(key); v.onSelectionChanged() })
	}
	require.Eventually(t, func() bool { return be.nextCalls.Load() >= 2 }, 3*time.Second, 10*time.Millisecond, "the steps are read")
	// Every read started (by the mount or by a selection) is answered and
	// applied: one still in flight would count as a retry below.
	require.Eventually(t, func() bool {
		settled := false
		onLoop(func() {
			settled = len(v.next) >= 3
			for _, n := range v.next {
				if n.loading {
					settled = false
				}
			}
		})
		return settled
	}, 5*time.Second, 10*time.Millisecond, "the steps are all answered")

	sh.mu.Lock()
	toasts := append([]string(nil), sh.toasts...)
	sh.mu.Unlock()
	assert.Empty(t, toasts, "no toast")
	assert.Equal(t, lists, be.lists.Load(), "no reload")
	onLoop(func() {
		assert.Contains(t, v.detail.GetText(true), "terminée", "the detail is there")
		assert.False(t, strings.Contains(v.detail.GetText(true), "Suites proposées"), "without the steps")
	})

	// Not read again for the same session while it does not change.
	calls := be.nextCalls.Load()
	onLoop(func() { v.selectKey("s:ses_8f2a01"); v.onSelectionChanged() })
	onLoop(func() {})
	assert.Equal(t, calls, be.nextCalls.Load(), "no retry loop")
}
