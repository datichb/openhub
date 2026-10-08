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
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Headless launches (`oh run --headless`, takeover-brief enrich): the
// sessions start without client, oh waits for the end of their turn, reads
// what the assistant wrote and stops them. A session that raises a decision
// is left running (nobody can answer it here): `oh session approve <id>`.

// errHeadlessCheckpoints refuses a headless launch whose workflow may wait
// for the user.
func headlessCheck(p *preparedRun) error {
	return headlessCheckpointsError(p.resolution.Spec, p.resolution.Mode)
}

// headlessCheckpointsError explains why a workflow cannot run headless in a
// mode, and whether another allowed mode can (A25: a mandatory checkpoint
// that pauses in every mode cannot be passed without interface).
func headlessCheckpointsError(spec *workflow.Spec, mode string) error {
	ids := spec.WaitingCheckpoints(mode)
	if len(ids) == 0 {
		return nil
	}
	var modes []string
	for _, m := range spec.AllowedModes() {
		if m != mode && len(spec.WaitingCheckpoints(m)) == 0 {
			modes = append(modes, m)
		}
	}
	if len(modes) == 0 {
		return errors.New(i18n.Tf("cmd.run.headless_checkpoints_always", spec.ID, strings.Join(ids, ", ")))
	}
	return errors.New(i18n.Tf("cmd.run.headless_checkpoints", spec.ID, mode, strings.Join(ids, ", "), strings.Join(modes, ", ")))
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
