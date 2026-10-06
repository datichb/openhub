package cmd

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Headless launches (`oh run --headless`, takeover-brief enrich): the
// sessions start without client, oh waits for the end of their turn, reads
// what the assistant wrote and stops them. A session that raises a decision
// is left running (nobody can answer it here): `oh session approve <id>`.

// errHeadlessCheckpoints refuses a headless launch whose workflow may wait
// for the user.
func headlessCheck(p *preparedRun) error {
	if ids := p.resolution.WaitingCheckpoints(); len(ids) > 0 {
		return errors.New(i18n.Tf("cmd.run.headless_checkpoints", p.resolution.Spec.ID, p.resolution.Mode, strings.Join(ids, ", ")))
	}
	return nil
}

// runHeadless starts a prepared launch without client and awaits every
// session (timeout 0 = none).
func runHeadless(ctx context.Context, a *app.App, p *preparedRun, ui launcher.LaunchUI, timeout time.Duration) ([]*runsvc.HeadlessResult, error) {
	if err := headlessCheck(p); err != nil {
		return nil, err
	}
	p.plan.Request.Base.Attach, p.plan.Request.Base.Headless = sessionspec.AttachNone, true
	started, err := p.start(ctx, a, ui)
	if err != nil {
		return nil, err
	}
	wctx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		wctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	var (
		out  []*runsvc.HeadlessResult
		errs []error
	)
	for _, s := range started {
		r, err := p.svc.AwaitTurn(wctx, s.SessionID)
		switch {
		case errors.Is(err, runsvc.ErrDecisionPending):
			errs = append(errs, errors.New(i18n.Tf("cmd.run.headless_decision", s.SessionID)))
			continue // left running: the decision can still be answered
		case err != nil:
			errs = append(errs, err)
		default:
			out = append(out, r)
		}
		if serr := p.svc.StopSession(context.Background(), s.SessionID); serr != nil {
			errs = append(errs, serr)
		}
	}
	return out, errors.Join(errs...)
}
