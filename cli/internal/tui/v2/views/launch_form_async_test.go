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

	"github.com/datichb/openhub/cli/internal/beads"
)

func recapContent(loop *tview.Application, v *LaunchFormView) (txt string) {
	onLoop(loop, func() {
		if v.recapTV != nil {
			txt = v.recapTV.GetText(true)
		}
	})
	return
}

// The recap is prepared off the event loop: the mount returns with « Préparation… »,
// and a failure is shown with the common words of the views.
func TestLaunchFormRecapStatesAreShown(t *testing.T) {
	frLocale(t)
	cfg := launchCfg(t)
	cfg.Tickets = []string{"bd-1"}
	started, release := make(chan struct{}), make(chan struct{})
	cfg.Recap = func(context.Context, LaunchChoices) (*LaunchRecap, error) {
		close(started)
		<-release
		return nil, errors.New("bd absent")
	}
	v := NewLaunchFormView(cfg)
	v.SetShell(&recordingShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)
	onLoop(loop, func() { v.Mount(content, loop); v.next(); v.next() })
	<-started
	assert.Contains(t, recapContent(loop, v), "Préparation")
	close(release)
	require.Eventually(t, func() bool {
		return strings.Contains(recapContent(loop, v), "Impossible de charger : bd absent")
	}, 2*time.Second, 10*time.Millisecond)
}

// A recap slow to come back, asked before another one (the choice of the
// dirty directory changed), is dropped; so is the one of an unmounted form,
// and the form mounted again asks its own (run with -race).
func TestLaunchFormStaleRecapIsIgnored(t *testing.T) {
	frLocale(t)
	cfg := launchCfg(t)
	cfg.Tickets = []string{"bd-1"}
	var calls atomic.Int32
	first, release := make(chan struct{}), make(chan struct{})
	cfg.Recap = func(_ context.Context, c LaunchChoices) (*LaunchRecap, error) {
		if calls.Add(1) == 1 {
			close(first)
			<-release
			return &LaunchRecap{Rows: []InfoField{{Label: "Etat", Value: "ancien"}}, Dirty: true}, nil
		}
		return &LaunchRecap{Rows: []InfoField{{Label: "Etat", Value: "recent"}}, Dirty: true}, nil
	}
	v := NewLaunchFormView(cfg)
	v.SetShell(&recordingShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)
	onLoop(loop, func() { v.Mount(content, loop); v.next(); v.next() })
	<-first
	onLoop(loop, func() { v.loadRecap() }) // asked again before the first one answers
	require.Eventually(t, func() bool { return strings.Contains(recapContent(loop, v), "recent") }, 2*time.Second, 10*time.Millisecond)
	close(release)
	time.Sleep(50 * time.Millisecond)
	assert.NotContains(t, recapContent(loop, v), "ancien")

	// Unmounted while a recap is read, then mounted again.
	slow := make(chan struct{})
	var n atomic.Int32
	v.cfg.Recap = func(context.Context, LaunchChoices) (*LaunchRecap, error) {
		if n.Add(1) == 1 {
			<-slow
			return &LaunchRecap{Rows: []InfoField{{Label: "Etat", Value: "orphelin"}}}, nil
		}
		return &LaunchRecap{Rows: []InfoField{{Label: "Etat", Value: "remonte"}}}, nil
	}
	onLoop(loop, func() { v.loadRecap() })
	onLoop(loop, v.Unmount)
	close(slow)
	time.Sleep(50 * time.Millisecond)
	onLoop(loop, func() { assert.Nil(t, v.recap, "the answer of the unmounted form is not applied") })
	onLoop(loop, func() { v.Mount(content, loop); v.loadRecap() })
	require.Eventually(t, func() bool { return strings.Contains(recapContent(loop, v), "remonte") }, 2*time.Second, 10*time.Millisecond)
}

// The tickets of the picker are read in load(): a failure is shown with the
// common words, and the answer of a picker whose context ended is dropped.
func TestBeadsPickerLoadStates(t *testing.T) {
	frLocale(t)
	src := sampleBeads()
	src.err = errors.New("bd absent")
	loop := runApp(t, tview.NewBox())
	var p *BeadsPicker
	onLoop(loop, func() {
		p = NewBeadsPicker(context.Background(), loop, BeadsPickerConfig{Source: src})
		p.Load()
	})
	require.Eventually(t, func() bool {
		var ok bool
		onLoop(loop, func() { ok = p.loadErr != nil && !p.loading })
		return ok
	}, 2*time.Second, 10*time.Millisecond)
	onLoop(loop, func() { assert.Contains(t, drawPicker(t, p), "Impossible de charger : bd absent") })
}

type gatedBeads struct {
	fakeBeads
	started chan struct{}
	release chan struct{}
}

func (g *gatedBeads) Tickets(ctx context.Context) ([]beads.Ticket, error) {
	close(g.started)
	<-g.release
	return g.tickets, nil
}

func TestBeadsPickerStaleLoadAfterCancelIsIgnored(t *testing.T) {
	frLocale(t)
	g := &gatedBeads{fakeBeads: *sampleBeads(), started: make(chan struct{}), release: make(chan struct{})}
	loop := runApp(t, tview.NewBox())
	ctx, cancel := context.WithCancel(context.Background())
	var p *BeadsPicker
	onLoop(loop, func() {
		p = NewBeadsPicker(ctx, loop, BeadsPickerConfig{Source: g})
		p.Load()
	})
	<-g.started
	onLoop(loop, func() { assert.True(t, p.loading, "the tickets are being read") })
	cancel() // the form is unmounted
	close(g.release)
	time.Sleep(50 * time.Millisecond)
	onLoop(loop, func() {
		assert.True(t, p.loading, "the answer after the end of the context is dropped")
		assert.Equal(t, 0, p.m.ticketCount())
	})
}
