package views

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gc"
)

// mountedDeploy mounts the clean-up view on its section of the former
// deployments.
func mountedDeploy(t *testing.T, cfg *DeployCleanupConfig) (v *GCView, loop *tview.Application, sh *trustShell) {
	t.Helper()
	v = NewGCView(GCViewConfig{Deploy: cfg, Section: CleanupDeploy,
		Plan: func(context.Context) (gc.Report, error) { return gcTestPlan(), nil }})
	sh = &trustShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	loop = runApp(t, content)
	onLoop(loop, func() { v.Mount(content, loop) })
	return v, loop, sh
}

func deployText(loop *tview.Application, v *GCView) (txt string) {
	onLoop(loop, func() { txt = v.deploy.text.GetText(true) })
	return
}

func deployLoaded(loop *tview.Application, v *GCView) (n int, loaded bool) {
	onLoop(loop, func() { n, loaded = v.deploy.plan.Projects, v.deploy.loaded })
	return
}

// The panel says it is scanning while the projects are read, with the common
// words of the views.
func TestCleanupDeployShowsTheScanInProgress(t *testing.T) {
	frLocale(t)
	started, release := make(chan struct{}), make(chan struct{})
	v, loop, _ := mountedDeploy(t, &DeployCleanupConfig{
		Scan: func(context.Context) (DeployCleanupPlan, error) {
			close(started)
			<-release
			return DeployCleanupPlan{Projects: 1, Text: "demo\n"}, nil
		},
	})
	<-started
	assert.Contains(t, deployText(loop, v), "Recherche des anciens déploiements")
	close(release)
	require.Eventually(t, func() bool { n, ok := deployLoaded(loop, v); return ok && n == 1 }, 2*time.Second, 10*time.Millisecond)
}

// A scan that panics is shown in the panel instead of taking the program down.
func TestCleanupDeployScanFailureIsShown(t *testing.T) {
	frLocale(t)
	v, loop, _ := mountedDeploy(t, &DeployCleanupConfig{
		Scan: func(context.Context) (DeployCleanupPlan, error) { panic("boom") },
	})
	require.Eventually(t, func() bool {
		return strings.Contains(deployText(loop, v), "Impossible de charger : panic: boom")
	}, 2*time.Second, 10*time.Millisecond)
}

// Two scans in flight (r pressed again): the last one stays, even when the
// first answers after it.
func TestCleanupDeployStaleScanIsIgnored(t *testing.T) {
	frLocale(t)
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	v, loop, _ := mountedDeploy(t, &DeployCleanupConfig{
		Scan: func(context.Context) (DeployCleanupPlan, error) {
			if calls.Add(1) == 1 {
				close(started)
				<-release
				return DeployCleanupPlan{Projects: 1, Text: "first\n"}, nil
			}
			return DeployCleanupPlan{Projects: 2, Text: "second\n"}, nil
		},
	})
	<-started
	onLoop(loop, func() { v.HandleKey(key('r')) })
	require.Eventually(t, func() bool { n, ok := deployLoaded(loop, v); return ok && n == 2 }, 2*time.Second, 10*time.Millisecond)

	close(release)
	time.Sleep(50 * time.Millisecond)
	n, ok := deployLoaded(loop, v)
	assert.True(t, ok)
	assert.Equal(t, 2, n, "the scan of the first request does not come back")
}

// The scan of a mount that was left does not reach the next one.
func TestCleanupDeployScanAfterRemountIsIgnored(t *testing.T) {
	frLocale(t)
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	v, loop, _ := mountedDeploy(t, &DeployCleanupConfig{
		Scan: func(context.Context) (DeployCleanupPlan, error) {
			if calls.Add(1) == 1 {
				close(started)
				<-release
				return DeployCleanupPlan{Projects: 1, Text: "first\n"}, nil
			}
			return DeployCleanupPlan{Projects: 2, Text: "second\n"}, nil
		},
	})
	<-started
	var old *deployPane
	onLoop(loop, func() { old = v.deploy })
	onLoop(loop, v.Unmount)
	onLoop(loop, func() { v.Mount(tview.NewFlex(), loop) })
	require.Eventually(t, func() bool { n, ok := deployLoaded(loop, v); return ok && n == 2 }, 2*time.Second, 10*time.Millisecond)

	close(release)
	time.Sleep(50 * time.Millisecond)
	onLoop(loop, func() {
		assert.False(t, old.loaded, "the pane of the previous mount keeps waiting")
		assert.Equal(t, 2, v.deploy.plan.Projects)
	})
}

// A clean-up that panics says so in a toast, and the projects are scanned
// again; the panel can be used again.
func TestCleanupDeployApplyFailureIsToasted(t *testing.T) {
	frLocale(t)
	v, loop, sh := mountedDeploy(t, &DeployCleanupConfig{
		Scan: func(context.Context) (DeployCleanupPlan, error) {
			return DeployCleanupPlan{Projects: 1, Text: "demo\n"}, nil
		},
		Apply: func(context.Context) (string, bool) { panic("boom") },
	})
	require.Eventually(t, func() bool { n, ok := deployLoaded(loop, v); return ok && n == 1 }, 2*time.Second, 10*time.Millisecond)
	onLoop(loop, func() { v.deploy.apply() })
	require.Eventually(t, func() bool {
		ts := settingsToasts(loop, sh)
		return len(ts) == 1 && strings.Contains(ts[0], "panic: boom")
	}, 2*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		var running, loaded bool
		onLoop(loop, func() { running, loaded = v.deploy.running, v.deploy.loaded })
		return !running && loaded
	}, 2*time.Second, 10*time.Millisecond)
}

// The result of a clean-up of a view that was left is not shown.
func TestCleanupDeployApplyAfterUnmountIsIgnored(t *testing.T) {
	frLocale(t)
	applying, release := make(chan struct{}), make(chan struct{})
	v, loop, sh := mountedDeploy(t, &DeployCleanupConfig{
		Scan: func(context.Context) (DeployCleanupPlan, error) {
			return DeployCleanupPlan{Projects: 1, Text: "demo\n"}, nil
		},
		Apply: func(context.Context) (string, bool) {
			close(applying)
			<-release
			return "cleaned", true
		},
	})
	require.Eventually(t, func() bool { n, ok := deployLoaded(loop, v); return ok && n == 1 }, 2*time.Second, 10*time.Millisecond)
	onLoop(loop, func() { v.deploy.apply() })
	<-applying
	onLoop(loop, v.Unmount)
	close(release)
	time.Sleep(50 * time.Millisecond)
	assert.Empty(t, settingsToasts(loop, sh))
}
