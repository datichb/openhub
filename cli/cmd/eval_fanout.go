package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/evalrun"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// Fan-out in an evaluation pass (R6): the driver runs in the pass process
// (it holds the credential of the pass), the children get the token cap of
// the pass, their decisions are answered like those of the parent, the
// pass does not settle while they run, and their cost counts.

type evalFanout struct {
	a      *app.App
	parent string
	cancel context.CancelFunc
	done   chan struct{}
	mu     sync.Mutex
	err    error
}

// startEvalFanout starts the driver of the fan-out of the parent session.
func startEvalFanout(ctx context.Context, a *app.App, parent string, spec evalPassSpec, log io.Writer) *evalFanout {
	f := &evalFanout{a: a, parent: parent, done: make(chan struct{})}
	dctx, cancel := context.WithCancel(ctx)
	f.cancel = cancel
	sess, err := a.Sessions.Get(ctx, parent)
	if err != nil {
		f.err = err
		close(f.done)
		return f
	}
	d, err := newFanoutDriver(ctx, a, sess, log, func(p *preparedRun) {
		p.Plan.Request.Base.MaxTokens = evalMaxTokens(spec.Model, spec.BudgetUSD)
	})
	if err != nil {
		f.err = err
		close(f.done)
		return f
	}
	d.Poll = 3 * time.Second
	go func() {
		defer close(f.done)
		if err := d.Run(dctx, parent); err != nil {
			f.mu.Lock()
			f.err = err
			f.mu.Unlock()
			fmt.Fprintln(log, "eval pass: fan-out:", err)
		}
	}()
	return f
}

// stop ends the driver and stops the children still running.
func (f *evalFanout) stop() {
	if f == nil {
		return
	}
	f.cancel()
	<-f.done
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if rs, err := newRunService(ctx, f.a); err == nil {
		for _, id := range fanoutChildren(ctx, f.a, f.parent) {
			_ = rs.StopSession(ctx, id)
		}
	}
}

// sessions are the children and mergers launched so far.
func (f *evalFanout) sessions(ctx context.Context) []string {
	if f == nil {
		return nil
	}
	return fanoutChildren(ctx, f.a, f.parent)
}

// running reports whether children still run (the driver did not end).
func (f *evalFanout) running(ctx context.Context) bool {
	if f == nil {
		return false
	}
	select {
	case <-f.done:
		return false
	default:
	}
	return fanoutRunning(ctx, f.a, f.parent)
}

// cost is what the children and mergers spent.
func (f *evalFanout) cost(ctx context.Context) float64 {
	total := 0.0
	for _, id := range f.sessions(ctx) {
		if s, err := f.a.Sessions.Get(ctx, id); err == nil {
			total += s.Cost
		}
	}
	return total
}

// collect adds the cost and tokens of the children to the pass and saves
// their traces next to the one of the parent (trace-<session>.jsonl).
func (f *evalFanout) collect(ctx context.Context, ssvc *sessionsvc.Service, res *evalrun.PassOutput, dir, token string) {
	if f == nil {
		return
	}
	for _, id := range f.sessions(ctx) {
		s, err := f.a.Sessions.Get(ctx, id)
		if err != nil {
			continue
		}
		res.CostUSD += s.Cost
		res.TokensIn += s.TokensIn
		res.TokensOut += s.TokensOut
		tr, err := ssvc.Trace(ctx, id, sessionsvc.TraceOptions{Usage: sqlite.NewUsageStore(store), Secrets: []string{token}})
		if err != nil {
			continue
		}
		if out, err := os.Create(filepath.Join(dir, "trace-"+id+".jsonl")); err == nil {
			_ = tr.WriteJSONL(out)
			_ = out.Close()
		}
	}
}
