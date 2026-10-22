package views

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gc"
)

// slowFirstPlan answers the full plan to its first call, after release, and
// a plan of one element to the next ones.
func slowFirstPlan() (plan func(context.Context) (gc.Report, error), started, release chan struct{}) {
	var calls atomic.Int32
	started, release = make(chan struct{}), make(chan struct{})
	plan = func(context.Context) (gc.Report, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
			return gcTestPlan(), nil
		}
		return gc.Report{Items: gcTestPlan().Items[:1]}, nil
	}
	return
}

func gcPlanItems(app *tview.Application, v *GCView) (n int, loaded bool) {
	onLoop(app, func() { n, loaded = len(v.plan.Items), v.loaded })
	return
}

// Two plans in flight (r pressed again): the plan of the last request stays,
// even when the first one answers after it.
func TestGCViewStalePlanIsIgnored(t *testing.T) {
	plan, started, release := slowFirstPlan()
	v := NewGCView(GCViewConfig{Bytes: gcBytes, Plan: plan})
	v.SetShell(&deleteShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	<-started
	onLoop(app, func() { v.HandleKey(key('r')) })
	require.Eventually(t, func() bool { n, loaded := gcPlanItems(app, v); return loaded && n == 1 }, 2*time.Second, 10*time.Millisecond)

	close(release)
	time.Sleep(50 * time.Millisecond)
	n, loaded := gcPlanItems(app, v)
	assert.True(t, loaded)
	assert.Equal(t, 1, n, "the full plan of the first request does not come back")
}

// The plan of a previous mount does not reach the new one.
func TestGCViewPlanAfterRemountIsIgnored(t *testing.T) {
	plan, started, release := slowFirstPlan()
	v := NewGCView(GCViewConfig{Bytes: gcBytes, Plan: plan})
	v.SetShell(&deleteShell{})
	app := runApp(t, tview.NewFlex())
	onLoop(app, func() { v.Mount(tview.NewFlex(), app) })
	<-started
	onLoop(app, v.Unmount)
	onLoop(app, func() { v.Mount(tview.NewFlex(), app) })
	require.Eventually(t, func() bool { n, loaded := gcPlanItems(app, v); return loaded && n == 1 }, 2*time.Second, 10*time.Millisecond)

	close(release)
	time.Sleep(50 * time.Millisecond)
	n, _ := gcPlanItems(app, v)
	assert.Equal(t, 1, n)
}

// A removal started while a plan is in flight wins over that plan: the old
// plan would show elements that are being removed.
func TestGCViewRemovalSupersedesThePendingPlan(t *testing.T) {
	plan, started, release := slowFirstPlan()
	ran := make(chan struct{})
	v := NewGCView(GCViewConfig{Bytes: gcBytes, Plan: plan,
		Run: func(_ context.Context, only gc.Report) (gc.Report, error) {
			close(ran)
			return gc.Report{Items: only.Items, Applied: true}, nil
		}})
	v.SetShell(&deleteShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	<-started
	onLoop(app, func() { v.run(gc.Report{Items: gcTestPlan().Items[:1]}) })
	<-ran
	require.Eventually(t, func() bool { n, loaded := gcPlanItems(app, v); return loaded && n == 1 }, 2*time.Second, 10*time.Millisecond)
	close(release)
	time.Sleep(50 * time.Millisecond)
	n, _ := gcPlanItems(app, v)
	assert.Equal(t, 1, n, "the plan computed before the removal is dropped")
}
