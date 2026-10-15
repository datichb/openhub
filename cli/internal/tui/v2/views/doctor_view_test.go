package views

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// f « corriger » on a check with a repair: its plan in a confirmation
// (danger), the repair only after it, then the checks again and the report.
func TestDoctorViewFix(t *testing.T) {
	var applied, runs atomic.Int32
	fix := &DoctorFix{
		Plan: func(context.Context) (string, error) { return "retirer le hook post-commit de demo", nil },
		Apply: func(context.Context) (string, error) {
			applied.Add(1)
			return "1 projet corrigé", nil
		},
	}
	v := NewDoctorView(nil)
	v.run = func() []DoctorCheck {
		runs.Add(1)
		return []DoctorCheck{{Name: "Beads", Detail: "1 problème", Fix: fix}, {Name: "git", OK: true, Detail: "2.49"}}
	}
	sh := &deleteShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	require.Eventually(t, func() bool {
		var n int
		onLoop(app, func() { n = len(v.checks) })
		return n == 2
	}, 2*time.Second, 10*time.Millisecond)
	assert.Contains(t, v.StatusHints(), "f "+i18n.T("tui.doctor.fix"))
	onLoop(app, func() { v.list.SetCurrentItem(0) })

	onLoop(app, func() { v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModNone)) })
	var c recordedConfirm
	require.Eventually(t, func() bool {
		var ok bool
		c, ok = sh.lastConfirm()
		return ok
	}, 2*time.Second, 10*time.Millisecond)
	assert.True(t, c.danger)
	assert.Equal(t, i18n.Tf("tui.doctor.fix_title", "Beads"), c.title)
	assert.Contains(t, c.detail, "retirer le hook post-commit de demo")
	assert.Zero(t, applied.Load(), "not before the confirmation")

	onLoop(app, c.onYes)
	require.Eventually(t, func() bool {
		sh.cmu.Lock()
		defer sh.cmu.Unlock()
		return strings.Contains(sh.body, "1 projet corrigé")
	}, 2*time.Second, 10*time.Millisecond, "report shown")
	assert.Equal(t, int32(1), applied.Load())
	assert.Equal(t, int32(2), runs.Load(), "checked again")
}

// A check without a repair says so; DoctorFixFor gives the repairs of the
// command layer.
func TestDoctorViewNoFixAndFixFor(t *testing.T) {
	prev := DoctorFixFor
	t.Cleanup(func() { DoctorFixFor = prev })
	DoctorFixFor = func(c DoctorCheck) *DoctorFix {
		if c.Name == "orphans" {
			return &DoctorFix{Open: func() {}}
		}
		return nil
	}
	v := NewDoctorView(nil)
	v.run = func() []DoctorCheck { return []DoctorCheck{{Name: "git", OK: true}, {Name: "orphans", Detail: "2"}} }
	checks := v.collectChecks()
	assert.Nil(t, checks[0].Fix)
	require.NotNil(t, checks[1].Fix)

	sh := &recordingShell{}
	v.shell = sh
	v.summary, v.list = tview.NewTextView(), widgets.NewSectionedList()
	v.checks = checks
	v.render()
	v.list.SetCurrentItem(0)
	v.fixSelected()
	assert.Equal(t, []string{i18n.T("tui.doctor.no_fix")}, sh.toasts)
	assert.Contains(t, v.list.GetItems()[1].MainText, i18n.T("tui.doctor.fix"), "marked as fixable")
}

// o switches the offline mode of the checks (oh doctor --offline).
func TestDoctorViewOffline(t *testing.T) {
	prev := DoctorOffline
	t.Cleanup(func() { DoctorOffline = prev })
	on := false
	DoctorOffline = &DoctorOfflineSwitch{On: func() bool { return on }, Set: func(b bool) { on = b }}
	v := NewDoctorView(nil)
	v.run = func() []DoctorCheck { return []DoctorCheck{{Name: "git", OK: true}} }
	assert.Contains(t, v.StatusHints(), "o "+i18n.T("tui.doctor.offline"))
	v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModNone))
	assert.True(t, on)
	v.summary, v.list = tview.NewTextView(), widgets.NewSectionedList()
	v.checks = v.collectChecks()
	v.render()
	assert.Contains(t, v.summary.GetText(true), i18n.T("tui.doctor.offline_on"))
}
