package views

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLaunchFormRuntimeStatus(t *testing.T) {
	cfg := launchCfg(t)
	cfg.Tickets = []string{"bd-1"}
	cfg.Runtimes[1].Available, cfg.Runtimes[1].Reason = true, ""
	cfg.AtOptions = true
	var mu sync.Mutex
	calls := map[string]int{}
	cfg.RuntimeStatus = func(_ context.Context, c LaunchChoices) string {
		mu.Lock()
		calls[c.Runtime]++
		mu.Unlock()
		return "colima 28 ✔ · image oh-dev/app:9f3c en cache"
	}
	v := NewLaunchFormView(cfg)
	v.SetShell(&recordingShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })

	text := func() string {
		var s string
		onLoop(app, func() {
			if v.rt.tv != nil {
				s = v.rt.tv.GetText(true)
			}
		})
		return s
	}
	assert.Empty(t, text(), "local: no status line")

	onLoop(app, func() { v.m.runtime = "container"; v.loadRuntimeStatus() })
	require.Eventually(t, func() bool { return strings.Contains(text(), "en cache") }, 2*time.Second, 10*time.Millisecond)

	// Back to local, then container again: the status is computed once.
	onLoop(app, func() { v.m.runtime = "local"; v.loadRuntimeStatus() })
	assert.Empty(t, text())
	onLoop(app, func() { v.m.runtime = "container"; v.render() })
	assert.Contains(t, text(), "en cache")
	mu.Lock()
	assert.Equal(t, 1, calls["container"])
	assert.Zero(t, calls["local"], "never asked for local")
	mu.Unlock()
}

func TestLaunchFormShowsPreparation(t *testing.T) {
	cfg := launchCfg(t)
	cfg.Tickets = []string{"bd-1"}
	cfg.Progress = &LaunchProgress{}
	release := make(chan struct{})
	cfg.Launch = func(context.Context, LaunchChoices) error {
		for i := 1; i <= 6; i++ {
			cfg.Progress.Line("Step " + string(rune('0'+i)))
		}
		<-release
		return nil
	}
	v := NewLaunchFormView(cfg)
	v.SetShell(&recordingShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	onLoop(app, func() { v.launch() })

	require.Eventually(t, func() bool {
		var txt string
		onLoop(app, func() { txt = v.recapTV.GetText(true) })
		return strings.Contains(txt, "Step 6")
	}, 2*time.Second, 10*time.Millisecond)
	var txt string
	onLoop(app, func() { txt = v.recapTV.GetText(true) })
	assert.NotContains(t, txt, "Step 2", "only the last lines are kept")
	assert.Contains(t, txt, "Step 3")
	close(release)

	require.Eventually(t, func() bool {
		var launching bool
		onLoop(app, func() { launching = v.launching })
		return !launching
	}, 2*time.Second, 10*time.Millisecond)
	cfg.Progress.Line("late") // no form listening any more: ignored
}

func TestLaunchProgressNil(t *testing.T) {
	var p *LaunchProgress
	p.Line("x") // no panic
	p.attach(func(string) {})
}
