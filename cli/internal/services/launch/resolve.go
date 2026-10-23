package launch

import (
	"context"
	"errors"
	"strings"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// ResolveLaunch resolves the workflow of a launch with its inputs: the
// ticket input and the ticket of each session (multi-tickets). What it tells
// (alias resolved, notes of the computed inputs, findings of an invalid
// workflow) goes to opts.Progress.
func ResolveLaunch(ctx context.Context, d Deps, opts *Options) (res *workflowsvc.Resolution, ticketInput string, perSession []string, err error) {
	wsvc := d.Workflows(ctx)
	resolve := wsvc.Resolve
	if opts.Draft {
		resolve = wsvc.ResolveDraft
	}
	progress := opts.Progress
	// R5: an alias (`aliases:`) launches its target; `with` routes it by
	// the inputs given (feature --tickets → implement-spec).
	asked := opts.Workflow
	if target, ok := wsvc.ResolveAlias(ctx, workflowsvc.Context{ProjectID: opts.Project.ID}, opts.Workflow, func(input string) bool {
		return strings.TrimSpace(opts.Inputs[input]) != "" || (len(opts.Tickets) > 0 && AliasTicketInput(ctx, wsvc, opts.Project.ID, opts.Workflow, input))
	}); ok {
		emit(progress, Event{Kind: EventNotice, Text: i18n.Tf("cmd.run.alias", opts.Workflow, target)})
		opts.Workflow = target
	}
	// Resolve once to learn the inputs (ticket input), then with the inputs.
	// No session options here: they would check the required inputs, not
	// known yet (every workflow with a required input was refused).
	probe, err := resolve(ctx, workflowsvc.Context{ProjectID: opts.Project.ID}, opts.Workflow, workflowsvc.ResolveOpts{})
	if err != nil {
		return nil, "", nil, ExplainResolveError(opts.Workflow, err, showDiagnostics(progress))
	}
	if _, declared := probe.Spec.Inputs.Get(RequestInput); opts.Message && !declared {
		return nil, "", nil, messageWithoutRequest(asked, probe.Spec)
	}
	inputs := LaunchInputs(probe.Spec, *opts)
	if opts.Runtime == "" {
		opts.Runtime = string(PickRuntime(d.Config, probe.Spec, opts.Project))
	}
	ticketInput, perSession, err = LaunchTickets(probe.Spec, opts, inputs, d.ticketSource(opts.Project))
	if err != nil {
		return nil, "", nil, err
	}
	// Inputs computed by oh (`from:`, e.g. the discussions of a merge
	// request) when they are not given.
	wsvc.InputSources = d.InputSources(func(line string) { emit(progress, Event{Kind: EventNotice, Text: line}) })
	if err := wsvc.ComputeInputs(ctx, workflowsvc.Context{ProjectID: opts.Project.ID, FromSession: opts.ParentSessionID}, probe.Spec, inputs); err != nil {
		return nil, "", nil, err
	}
	res, err = resolve(ctx, workflowsvc.Context{ProjectID: opts.Project.ID}, opts.Workflow, workflowsvc.ResolveOpts{
		Session: &workflow.SessionOptions{Mode: opts.Mode, Runtime: workflow.Runtime(opts.Runtime), Inputs: inputs, EntryAgent: opts.Agent}})
	if err != nil {
		return nil, "", nil, ExplainResolveError(opts.Workflow, err, showDiagnostics(progress))
	}

	return res, ticketInput, perSession, nil
}

func showDiagnostics(progress func(Event)) func(workflow.Diagnostics) {
	return func(ds workflow.Diagnostics) { emit(progress, Event{Kind: EventDiagnostics, Diagnostics: ds}) }
}

// ExplainResolveError explains a resolution failure; show receives the
// findings of an invalid workflow.
func ExplainResolveError(id string, err error, show func(workflow.Diagnostics)) error {
	if errors.Is(err, workflowsvc.ErrUnknownWorkflow) {
		return errors.New(i18n.Tf("cmd.workflow.show.unknown", id))
	}
	var invalid *workflowsvc.InvalidError
	if errors.As(err, &invalid) {
		show(invalid.Diagnostics)
		return errors.New(i18n.Tf("cmd.workflow.show.invalid", len(invalid.Diagnostics.Errors())))
	}
	return err
}

// AliasTicketInput reports whether input is the ticket input of a target
// of alias id (--tickets fills it).
func AliasTicketInput(ctx context.Context, wsvc *workflowsvc.Service, projectID, id, input string) bool {
	for _, target := range wsvc.AliasTargetIDs(ctx, workflowsvc.Context{ProjectID: projectID}, id) {
		res, _ := wsvc.Resolve(ctx, workflowsvc.Context{ProjectID: projectID}, target, workflowsvc.ResolveOpts{})
		if res == nil {
			continue
		}
		if k, _ := workflowsvc.TicketInput(res.Spec); k == input {
			return true
		}
	}
	return false
}

// LaunchTickets puts the tickets of a launch in the ticket input: one
// session per ticket (perSession) when the input allows several sessions
// (an epic then stands for its open children, A24), else a list or a single
// ticket. opts.Tickets and opts.Epics receive the expanded tickets.
func LaunchTickets(sp *workflow.Spec, opts *Options, inputs map[string]any, src workflowsvc.TicketSource) (ticketInput string, perSession []string, err error) {
	ticketInput, multi := workflowsvc.TicketInput(sp)
	if len(opts.Tickets) == 0 {
		return ticketInput, nil, nil
	}
	if multi {
		if opts.Tickets, opts.Epics, err = workflowsvc.ExpandEpics(src, opts.Tickets); err != nil {
			var empty *workflowsvc.EmptyEpicError
			if errors.As(err, &empty) {
				return "", nil, errors.New(i18n.Tf("cmd.run.empty_epic", empty.Epic))
			}
			return "", nil, err
		}
	}
	in, _ := sp.Inputs.Get(ticketInput)
	switch {
	case ticketInput == "":
		return "", nil, errors.New(i18n.Tf("cmd.run.no_ticket_input", opts.Workflow))
	case multi && opts.OneSession:
		inputs[ticketInput] = strings.Join(opts.Tickets, ",")
	case multi:
		perSession = opts.Tickets
		inputs[ticketInput] = opts.Tickets[0]
	case in.Type == workflow.InputBeadsIDs:
		inputs[ticketInput] = strings.Join(opts.Tickets, ",")
	case len(opts.Tickets) > 1:
		return "", nil, errors.New(i18n.Tf("cmd.run.single_ticket", opts.Workflow))
	default:
		inputs[ticketInput] = opts.Tickets[0]
	}
	return ticketInput, perSession, nil
}

// LaunchInputs merges the explicit inputs, the alias inputs declared by the
// workflow and the free text (first text input not set otherwise).
func LaunchInputs(sp *workflow.Spec, opts Options) map[string]any {
	inputs := map[string]any{}
	for k, v := range opts.Inputs {
		inputs[k] = v
	}
	for k, v := range opts.LooseInputs {
		if _, declared := sp.Inputs.Get(k); declared && v != "" {
			if _, set := inputs[k]; !set {
				inputs[k] = v
			}
		}
	}
	if opts.Text != "" {
		if k := textInput(sp); k != "" {
			if _, set := inputs[k]; !set {
				inputs[k] = opts.Text
			}
		}
	}
	return inputs
}

// messageWithoutRequest refuses -m for a workflow (alias resolved) that has
// no request input, naming the inputs it has.
func messageWithoutRequest(asked string, sp *workflow.Spec) error {
	name := sp.ID
	if asked != "" && asked != sp.ID {
		name = asked + " → " + sp.ID
	}
	inputs := strings.Join(sp.Inputs.Keys(), ", ")
	if inputs == "" {
		return errors.New(i18n.Tf("cmd.run.message_no_request_none", name))
	}
	return errors.New(i18n.Tf("cmd.run.message_no_request", name, inputs))
}

// textInput is the input receiving free text: the first `text` input, else
// the first `string` one.
func textInput(sp *workflow.Spec) string {
	if k := workflowsvc.FirstInput(sp, workflow.InputText); k != "" {
		return k
	}
	return workflowsvc.FirstInput(sp, workflow.InputString)
}

func (d Deps) ticketSource(project *domain.Project) workflowsvc.TicketSource {
	if d.Tickets != nil {
		return d.Tickets(project)
	}
	return BeadsTicketSource(project)
}

// BeadsTicketSource reads the tickets of a project from Beads (nil when the
// project has no Beads database).
func BeadsTicketSource(project *domain.Project) workflowsvc.TicketSource {
	if project == nil || beads.Available() != nil || !beads.IsInitialized(project.Path) {
		return nil
	}
	return beadsTickets(project.Path)
}

type beadsTickets string

func (b beadsTickets) IsEpic(id string) (bool, error) {
	d, err := beads.Show(string(b), id)
	if err != nil {
		return false, err
	}
	return d.Type == "epic", nil
}

func (b beadsTickets) OpenChildren(epic string) ([]string, error) {
	children, err := beads.DevPickableChildren(string(b), epic)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(children))
	for i, c := range children {
		out[i] = c.ID
	}
	return out, nil
}
