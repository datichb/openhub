package views

import (
	"context"
	"sync"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// Runtime option of the launch form (P4-T11, 10 §4.3): a status line under
// the options for the selected runtime outside the machine (engine, image
// cached or build to expect), computed off the event loop, and the
// preparation output (image build) shown while launching.

// launchProgressLines is the number of preparation lines kept on screen.
const launchProgressLines = 4

// LaunchProgress relays preparation output (image build) to the form; its
// Line method may be called from any goroutine.
type LaunchProgress struct {
	mu   sync.Mutex
	sink func(line string)
}

// Line shows a preparation line (ignored when no form listens).
func (p *LaunchProgress) Line(line string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	sink := p.sink
	p.mu.Unlock()
	if sink != nil {
		sink(line)
	}
}

func (p *LaunchProgress) attach(sink func(string)) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.sink = sink
	p.mu.Unlock()
}

// runtimeState is the status line state of the form.
type runtimeState struct {
	tv    *tview.TextView
	gen   int
	cache map[string]string // by runtime kind
	lines []string          // preparation output while launching
}

// runtimeStatusLine adds the status line of the selected runtime (options
// step) and loads it when needed.
func (v *LaunchFormView) runtimeStatusLine() {
	if v.cfg.RuntimeStatus == nil || len(v.cfg.Runtimes) == 0 {
		return
	}
	tv := tview.NewTextView().SetDynamicColors(true)
	tv.SetBackgroundColor(theme.BgPanel)
	v.rt.tv = tv
	v.body.AddItem(tv, 1, 0, false)
	v.loadRuntimeStatus()
}

// loadRuntimeStatus shows the status of the selected runtime, computing it
// off the event loop the first time (kept per runtime for the form).
func (v *LaunchFormView) loadRuntimeStatus() {
	if v.rt.tv == nil || v.cfg.RuntimeStatus == nil {
		return
	}
	kind := v.m.runtime
	if kind == "" || kind == "local" {
		v.rt.tv.SetText("")
		return
	}
	if v.rt.cache == nil {
		v.rt.cache = map[string]string{}
	}
	if text, ok := v.rt.cache[kind]; ok {
		v.rt.tv.SetText(runtimeStatusText(text))
		return
	}
	v.rt.tv.SetText(runtimeStatusText("⠋ " + i18n.T("tui.launch.runtime.checking")))
	v.rt.gen++
	gen, app, ctx, choices := v.rt.gen, v.app, v.ctx, v.m.choices()
	go func() {
		text := v.cfg.RuntimeStatus(ctx, choices)
		if app == nil || ctx.Err() != nil {
			return
		}
		app.QueueUpdateDraw(func() {
			v.rt.cache[kind] = text
			if v.rt.gen == gen && v.rt.tv != nil && v.m.runtime == kind {
				v.rt.tv.SetText(runtimeStatusText(text))
			}
		})
	}()
}

func runtimeStatusText(s string) string {
	if s == "" {
		return ""
	}
	return "  [" + theme.TextMutedHex + "]" + tview.Escape(s) + "[-]"
}

// listenProgress shows the preparation output while launching.
func (v *LaunchFormView) listenProgress() {
	v.rt.lines = nil
	app := v.app
	v.cfg.Progress.attach(func(line string) {
		if app == nil {
			return
		}
		app.QueueUpdateDraw(func() {
			v.rt.lines = append(v.rt.lines, line)
			if n := len(v.rt.lines); n > launchProgressLines {
				v.rt.lines = v.rt.lines[n-launchProgressLines:]
			}
			if v.recapTV != nil {
				v.recapTV.SetText(v.recapText())
			}
		})
	})
}

func (v *LaunchFormView) stopProgress() { v.cfg.Progress.attach(nil) }

// progressText renders the last preparation lines.
func (v *LaunchFormView) progressText() string {
	out := ""
	for _, l := range v.rt.lines {
		out += "  [" + theme.TextMutedHex + "]" + tview.Escape(l) + "[-]\n"
	}
	return out
}

// RuntimeStatusFunc computes the status line of a runtime (off the event
// loop): engine, image cached or build to expect.
type RuntimeStatusFunc func(ctx context.Context, c LaunchChoices) string
