package launch

import (
	"context"
	"errors"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// AttachFunc opens the client of a started session when the RunService did
// not (browser, suspension, no terminal): it belongs to the client that
// launches (terminal, TUI). Only the first session of a run may take over
// the current terminal.
type AttachFunc func(ctx context.Context, svc *runsvc.Service, attach sessionspec.AttachPref, res *runsvc.StartResult, first bool) error

// StartOptions are the parts of a start that belong to the client.
type StartOptions struct {
	// Progress receives what the start tells (see Event); nil = nothing.
	Progress func(Event)
	// Attach opens the client of each started session (nil: not opened).
	Attach AttachFunc
}

// ErrDryRun is the refusal to start a launch prepared with Options.DryRun.
func ErrDryRun() error { return errors.New(i18n.T("cmd.run.dry_run.no_start")) }

// Start runs the prepared launch and opens the sessions (attach preference).
func Start(ctx context.Context, p *Prepared, so StartOptions) ([]*runsvc.StartResult, error) {
	if p.Opts.DryRun {
		return nil, ErrDryRun()
	}
	if err := checkTicketRoom(ctx, p); err != nil {
		return nil, err
	}
	if p.Remote != nil {
		return sendRemote(ctx, p, so.Progress)
	}
	d := p.deps
	results, err := p.Svc.Start(ctx, p.Plan)
	if errors.Is(err, runsvc.ErrLaunchInProgress) {
		return results, errors.New(i18n.T("cmd.run.in_progress"))
	}
	var choice *runsvc.ExistingChoiceError
	if errors.As(err, &choice) {
		return results, errors.New(ExistingChoiceMessage(choice.Existing))
	}
	for _, res := range results {
		for _, w := range res.Report.Warnings {
			text := i18n.Tf("cmd.v5.isolation_warning", w)
			if d.IsolationWarning != nil {
				text = d.IsolationWarning(w)
			}
			emit(so.Progress, Event{Kind: EventWarning, Text: text})
		}
	}
	if err != nil {
		return results, BudgetError(err)
	}
	if p.Plan.Request.Base.Fanout != nil && d.SpawnFanoutDriver != nil {
		// The driver of the fan-out runs whatever the client does.
		for _, res := range results {
			if log, err := d.SpawnFanoutDriver(res.SessionID); err != nil {
				emit(so.Progress, Event{Kind: EventWarning, Text: i18n.Tf("cmd.fanout.spawn_failed", res.SessionID, err)})
			} else {
				emit(so.Progress, Event{Kind: EventInfo, Text: i18n.Tf("cmd.fanout.spawned", res.SessionID, log)})
			}
		}
	}
	attach := p.Plan.Request.Base.Attach
	for i, res := range results {
		tellStarted(res, so.Progress)
		if so.Attach == nil {
			continue
		}
		if aerr := so.Attach(ctx, p.Svc, attach, res, i == 0); aerr != nil {
			return results, aerr
		}
	}
	return results, nil
}

// checkTicketRoom refuses a launch that would start more tickets than the
// member may have in progress, before anything is created: no worktree, no
// session, no claim. A session that continues an existing one (resume,
// transfer) starts no ticket.
func checkTicketRoom(ctx context.Context, p *Prepared) error {
	if p.deps.TicketRoom == nil || p.Project == nil {
		return nil
	}
	check := p.deps.TicketRoom(p.Project)
	if check == nil {
		return nil
	}
	var tickets []string
	if p.Remote != nil {
		for _, t := range p.Remote.tickets {
			tickets = append(tickets, t...)
		}
	} else {
		for _, ps := range p.Plan.Sessions {
			if ps.Existing != "" && (ps.Action == runsvc.OnExistingResume || ps.Action == runsvc.OnExistingTransfer) {
				continue
			}
			tickets = append(tickets, ps.Tickets...)
		}
	}
	if len(tickets) == 0 {
		return nil
	}
	return check(ctx, p.Project.ID, tickets)
}

// tellStarted tells what the start of a session did besides starting it.
func tellStarted(res *runsvc.StartResult, progress func(Event)) {
	info := func(text string) { emit(progress, Event{Kind: EventInfo, Text: text}) }
	if res.Stashed != "" {
		info(i18n.Tf("cmd.run.stashed", res.Stashed))
	}
	if d := res.Deleted; d != nil {
		info(DeleteReportText(d))
	}
	if res.Replaced != "" {
		info(i18n.Tf("cmd.run.existing.replaced", res.Replaced, res.SessionID))
	}
	for _, n := range res.Notes {
		info(n)
	}
	if res.Queued {
		info(i18n.Tf("cmd.budget.queued", res.SessionID, res.Ahead))
	}
}
