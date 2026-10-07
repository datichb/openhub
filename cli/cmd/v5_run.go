package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/runsvc"
	remotesvc "github.com/datichb/openhub/cli/internal/services/remote"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Workflow launches (oh run, aliases, TUI launch form): resolve the
// workflow, compile its bundle, plan the sessions (locations, warnings),
// render their prompts, then start them through the RunService.

// runOptions are the choices of a workflow launch.
type runOptions struct {
	Workflow string
	Project  *domain.Project
	Provider string // --provider ("" = project / hub default)
	Inputs   map[string]string
	// Tickets starts one session per ticket when the ticket input accepts
	// several (picker.multi), else fills a beads-ids input.
	Tickets         []string
	Mode            string
	Runtime         string
	Location        string // base | new | <worktree path>
	Attach          string // "" = configured preference
	ParentSessionID string
	// LooseInputs are kept only when the workflow declares them (aliases of
	// the former commands map their flags this way).
	LooseInputs map[string]string
	// Text goes to the first free text input (text, else string) that is
	// not set otherwise (former --prompt, --issue…).
	Text string
	// Branch names the worktree branch when the workflow has no branch input.
	Branch string
	// OneSession gives every ticket to a single session (the workflow
	// handles several tickets) instead of one session per ticket.
	OneSession bool
	// Agent replaces the entry agent of a workflow whose entry is selectable
	// (`libre`, former `oh start --agent`).
	Agent string
	// Draft resolves the workflow with the member's drafts (local only, may
	// not loosen the published version).
	Draft bool
	// Progress receives preparation output (container image build).
	Progress func(line string)
}

// preparedRun is a launch ready to start: its plan can be shown (recap).
type preparedRun struct {
	opts       runOptions
	project    *domain.Project
	resolution *workflowsvc.Resolution
	bundle     *bundle.Bundle
	svc        *runsvc.Service
	plan       *runsvc.RunPlan
	// suggestions are the failed `suggest` preconditions (run another
	// workflow first).
	suggestions []preconditionSuggestion
	// remote is set for a remote launch (sessions sent to GitLab CI).
	remote *remotePrep
}

// prepareWorkflowRun resolves, compiles and plans a launch.
func prepareWorkflowRun(ctx context.Context, a *app.App, opts runOptions, errOut io.Writer) (*preparedRun, error) {
	if err := requireV2(ctx); err != nil {
		return nil, err
	}
	if opts.Project == nil {
		return nil, errors.New(i18n.T("cmd.run.no_project"))
	}
	res, ticketInput, perSession, err := resolveLaunch(ctx, a, &opts, errOut)
	if err != nil {
		return nil, err
	}

	svc, err := newRunService(ctx, a)
	if err != nil {
		return nil, err
	}
	svc.OnTicketsStarted = startTicketClaims(a, opts.Project)
	kind := sessionspec.RuntimeKind(res.Runtime)
	var rp *remotePrep
	if kind == sessionspec.RuntimeRemote {
		if rp, err = prepareRemote(ctx, a, opts.Project, res); err != nil {
			return nil, err
		}
	} else if av, err := svc.RuntimeAvailability(ctx, kind); err != nil || !av.OK {
		reason := av.Message()
		if err != nil {
			reason = err.Error()
		}
		return nil, errors.New(i18n.Tf("cmd.run.runtime_unavailable", string(kind), reason))
	}

	prov := provider.ResolveProvider(opts.Provider, opts.Project.Provider, a.Config.Opencode.DefaultProvider)
	b, missingMCP, err := buildWorkflowBundle(a, opts.Project, res, prov)
	if err != nil {
		return nil, fmt.Errorf("building session bundle: %w", err)
	}

	base := v5Request(a, opts.Project, opts.Provider)
	base.Bundle, base.Mode, base.Runtime, base.Progress = b, res.Mode, kind, opts.Progress
	base.Title = sessionTitle(opts.Project, res.Spec.ID)
	base.Limits = sessionLimits(ctx, a, opts.Project, res.Spec)
	if res.Spec.Beads != nil {
		base.BeadsAllow = append([]string{}, res.Spec.Beads.Allow...) // non-nil: declared
	}
	if kind == sessionspec.RuntimeContainer {
		if id := gitIdentityEnv(opts.Project.Path); len(id) > 0 {
			base.SessionEnv = id // commits from the container shell
		}
	}
	if opts.Attach != "" {
		base.Attach = sessionspec.AttachPref(opts.Attach)
	}
	req := runsvc.RunRequest{
		Base: base, ProjectPath: opts.Project.Path, Location: runsvc.ParseLocation(opts.Location),
		Workflow:        runsvc.WorkflowRef{ID: res.Spec.ID, Layer: string(res.Ref.Layer), Version: res.Spec.Version, Risk: string(res.Spec.Risk)},
		ParentSessionID: opts.ParentSessionID,
	}
	if req.Workflow.Risk == "" {
		req.Workflow.Risk = string(workflow.RiskWrite)
	}
	// One resolution per session: the ticket input changes the defaults
	// (branch) and the prompt.
	sessions := []*workflowsvc.Resolution{res}
	labels := []string{""}
	if len(perSession) > 0 {
		sessions, labels = nil, nil
		for _, t := range perSession {
			in := map[string]any{}
			for k, v := range res.Inputs {
				in[k] = v
			}
			in[ticketInput] = t
			sessions = append(sessions, res.WithInputs(in))
			labels = append(labels, t)
		}
	}
	values := make([]map[string]any, len(sessions))
	for i, r := range sessions {
		v, err := r.Values(workflowsvc.PromptContext{Project: opts.Project.Name, Lang: i18n.Locale()})
		if err != nil {
			return nil, err
		}
		values[i] = v
		label := labels[i]
		if label == "" && ticketInput != "" {
			if t, ok := v[ticketInput].(string); ok {
				label = t
			}
		}
		title := base.Title
		if label != "" {
			title += " · " + label
		}
		req.Sessions = append(req.Sessions, runsvc.PlannedInput{Label: label, Title: title, Branch: sessionBranch(res.Spec, v, label, opts.Branch),
			Tickets: remoteTickets(v, ticketInput)})
	}
	preconds, suggestions, err := preconditionWarnings(res.Spec, opts.Project.Path)
	if err != nil {
		return nil, err
	}
	plan, err := svc.Plan(ctx, req)
	if err != nil {
		return nil, runPlanError(err)
	}
	plan.Warnings = append(preconds, plan.Warnings...)
	for _, m := range missingMCP {
		plan.Warnings = append(plan.Warnings, runsvc.Warning{Code: "mcp_missing", Args: []any{m}})
	}
	if rp != nil {
		// Remote: the job clones the project; no local location or warning.
		plan.Warnings = preconds
		for i := range plan.Sessions {
			rp.tickets = append(rp.tickets, remoteTickets(values[i], ticketInput))
			rp.inputs = append(rp.inputs, sessions[i].Inputs)
		}
	}
	for i := range plan.Sessions {
		loc := plan.Sessions[i].Location.Path
		branch := plan.Sessions[i].Location.WorkBranch(opts.Project.Path)
		if rp != nil {
			loc = remotesvc.WorkDir(opts.Project.Name)
		}
		p, err := sessions[i].RenderPrompt(workflowsvc.PromptContext{Project: opts.Project.Name,
			Location: loc, Branch: branch, Lang: i18n.Locale()})
		if err != nil {
			return nil, err
		}
		plan.Sessions[i].Prompt = p
	}
	return &preparedRun{opts: opts, project: opts.Project, resolution: res, bundle: b, svc: svc, plan: plan, suggestions: suggestions, remote: rp}, nil
}

// resolveLaunch resolves the workflow of a launch with its inputs: the
// ticket input and the ticket of each session (multi-tickets).
func resolveLaunch(ctx context.Context, a *app.App, opts *runOptions, errOut io.Writer) (res *workflowsvc.Resolution, ticketInput string, perSession []string, err error) {
	wsvc := newWorkflowService(ctx)
	resolve := wsvc.Resolve
	if opts.Draft {
		resolve = wsvc.ResolveDraft
	}
	// Resolve once to learn the inputs (ticket input), then with the inputs.
	// No session options here: they would check the required inputs, not
	// known yet (every workflow with a required input was refused).
	probe, err := resolve(ctx, workflowsvc.Context{ProjectID: opts.Project.ID}, opts.Workflow, workflowsvc.ResolveOpts{})
	if err != nil {
		return nil, "", nil, workflowError(errOut, opts.Workflow, err)
	}
	inputs := launchInputs(probe.Spec, *opts)
	if opts.Runtime == "" {
		opts.Runtime = string(probe.Spec.PickRuntime(runtimePrefs(a, opts.Project)...))
	}
	ticketInput, multi := workflowsvc.TicketInput(probe.Spec)
	if len(opts.Tickets) > 0 {
		in, _ := probe.Spec.Inputs.Get(ticketInput)
		switch {
		case ticketInput == "":
			return nil, "", nil, errors.New(i18n.Tf("cmd.run.no_ticket_input", opts.Workflow))
		case multi && opts.OneSession:
			inputs[ticketInput] = strings.Join(opts.Tickets, ",")
		case multi:
			perSession = opts.Tickets
			inputs[ticketInput] = opts.Tickets[0]
		case in.Type == workflow.InputBeadsIDs:
			inputs[ticketInput] = strings.Join(opts.Tickets, ",")
		case len(opts.Tickets) > 1:
			return nil, "", nil, errors.New(i18n.Tf("cmd.run.single_ticket", opts.Workflow))
		default:
			inputs[ticketInput] = opts.Tickets[0]
		}
	}
	// Inputs computed by oh (`from:`, e.g. the discussions of a merge
	// request) when they are not given.
	wsvc.InputSources = gitlabInputSources(a)
	if err := wsvc.ComputeInputs(ctx, workflowsvc.Context{ProjectID: opts.Project.ID}, probe.Spec, inputs); err != nil {
		return nil, "", nil, err
	}
	res, err = resolve(ctx, workflowsvc.Context{ProjectID: opts.Project.ID}, opts.Workflow, workflowsvc.ResolveOpts{
		Session: &workflow.SessionOptions{Mode: opts.Mode, Runtime: workflow.Runtime(opts.Runtime), Inputs: inputs, EntryAgent: opts.Agent}})
	if err != nil {
		return nil, "", nil, workflowError(errOut, opts.Workflow, err)
	}

	return res, ticketInput, perSession, nil
}

// preconditionWarnings evaluates the workflow preconditions on the project
// directory: a failed `block` refuses the launch, a failed `suggest` is a
// warning naming the workflow to run first.
func preconditionWarnings(sp *workflow.Spec, dir string) ([]runsvc.Warning, []preconditionSuggestion, error) {
	var (
		out         []runsvc.Warning
		suggestions []preconditionSuggestion
	)
	for _, r := range workflow.FailedPreconditions(workflow.EvaluatePreconditions(sp, os.DirFS(dir))) {
		label := r.Label.Text(i18n.Locale())
		if label == "" {
			label = r.ID
		}
		switch {
		case r.Block:
			return nil, nil, errors.New(i18n.Tf("cmd.run.precondition_block", label))
		case r.Suggest != "":
			out = append(out, runsvc.Warning{Code: "precondition", Args: []any{label, r.Suggest}})
			suggestions = append(suggestions, preconditionSuggestion{Label: label, Workflow: r.Suggest, Resume: r.Resume})
		default:
			out = append(out, runsvc.Warning{Code: "precondition_failed", Args: []any{label}})
		}
	}
	return out, suggestions, nil
}

// launchInputs merges the explicit inputs, the alias inputs declared by the
// workflow and the free text (first text input not set otherwise).
func launchInputs(sp *workflow.Spec, opts runOptions) map[string]any {
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

// sessionBranch is the branch of a session worktree: the workflow `branch`
// input, else oh/<workflow>-<ticket|time>.
func sessionBranch(sp *workflow.Spec, values map[string]any, label, fallback string) string {
	if k := workflowsvc.FirstInput(sp, workflow.InputBranch); k != "" {
		if b, ok := values[k].(string); ok && b != "" {
			return b
		}
	}
	if fallback != "" {
		return fallback
	}
	suffix := label
	if suffix == "" {
		suffix = time.Now().Format("20060102-150405")
	}
	return "oh/" + sp.ID + "-" + suffix
}

// textInput is the input receiving free text: the first `text` input, else
// the first `string` one.
func textInput(sp *workflow.Spec) string {
	if k := workflowsvc.FirstInput(sp, workflow.InputText); k != "" {
		return k
	}
	return workflowsvc.FirstInput(sp, workflow.InputString)
}

func runPlanError(err error) error {
	switch {
	case errors.Is(err, runsvc.ErrNotGitRepo):
		return errors.New(i18n.T("cmd.run.not_git"))
	case errors.Is(err, runsvc.ErrNoBranch):
		return errors.New(i18n.T("cmd.run.no_branch"))
	}
	return err
}

// warningText localizes a plan warning.
func warningText(w runsvc.Warning) string {
	return i18n.Tf("cmd.run.warn."+w.Code, w.Args...)
}

// start runs the prepared launch and opens the sessions (attach
// preference). Single-session browser / suspend attaches are handled here.
func (p *preparedRun) start(ctx context.Context, a *app.App, ui launcher.LaunchUI) ([]*runsvc.StartResult, error) {
	if p.remote != nil {
		return p.sendRemote(ctx, a, ui)
	}
	results, err := p.svc.Start(ctx, p.plan)
	if errors.Is(err, runsvc.ErrLaunchInProgress) {
		return results, errors.New(i18n.T("cmd.run.in_progress"))
	}
	for _, res := range results {
		for _, w := range res.Report.Warnings {
			ui.Notify(i18n.Tf("cmd.v5.isolation_warning", w), launcher.LevelWarning)
		}
	}
	if err != nil {
		return results, budgetError(err)
	}
	attach := p.plan.Request.Base.Attach
	for i, res := range results {
		if aerr := afterStart(ctx, a, p.svc, ui, attach, res, i == 0); aerr != nil {
			return results, aerr
		}
	}
	return results, nil
}

// afterStart opens the client of a started session when the RunService did
// not (browser, suspension, no terminal). Only the first session of a run
// may take over the current terminal.
func afterStart(ctx context.Context, a *app.App, svc *runsvc.Service, ui launcher.LaunchUI, attach sessionspec.AttachPref, res *runsvc.StartResult, first bool) error {
	for _, n := range res.Notes {
		ui.Notify(n, launcher.LevelInfo)
	}
	if res.Queued {
		ui.Notify(i18n.Tf("cmd.budget.queued", res.SessionID, res.Ahead), launcher.LevelInfo)
	}
	if first && inProcessDaemonRunning() {
		// Windows: the proxy lives in this process; leaving oh puts the
		// session to sleep.
		ui.Notify(i18n.T("cmd.daemon.inprocess_notice"), launcher.LevelWarning)
	}
	switch {
	case attach == sessionspec.AttachNone:
		ui.Notify(i18n.Tf("cmd.run.started", res.SessionID), launcher.LevelSuccess)
	case attach == sessionspec.AttachBrowser:
		url, err := svc.PairURL(ctx, res.SessionID, pairURL)
		if err != nil {
			return err
		}
		ui.Notify(i18n.Tf("cmd.v5.opened_browser", url), launcher.LevelSuccess)
		return openURL(url)
	case attach == sessionspec.AttachSuspend || res.AttachErr != nil:
		if res.AttachErr != nil {
			ui.Notify(i18n.T("cmd.session.no_terminal"), launcher.LevelWarning)
		}
		if !first || (ui.SuspendAndExec() == nil && !stdinIsTerminal()) {
			// No terminal to take over (oh run from a script, an IDE…): the
			// client would block this process.
			ui.Notify(i18n.Tf("cmd.run.attach_later", res.SessionID), launcher.LevelInfo)
			return nil
		}
		return runAttachInline(ctx, a, svc, ui, res.SessionID)
	default:
		ui.Notify(i18n.Tf("cmd.v5.opened_in", string(res.AttachMethod), res.SessionID), launcher.LevelSuccess)
	}
	return nil
}

// runRecap is what a launch will do: labelled rows and warnings (oh run
// --recap, TUI launch form).
func runRecap(p *preparedRun) (rows [][2]string, warnings []string) {
	r := bundle.Show(p.bundle)
	add := func(label, value string) { rows = append(rows, [2]string{label, value}) }
	add(i18n.T("cmd.run.recap.mode"), p.resolution.Mode)
	add(i18n.T("cmd.run.recap.runtime"), string(p.resolution.Runtime))
	agents := make([]string, len(r.Agents))
	for i, ag := range r.Agents {
		agents[i] = ag.ID
		if ag.Entry {
			agents[i] += " (" + i18n.T("cmd.bundle.show.entry") + ")"
		}
	}
	add(i18n.Tf("cmd.bundle.show.agents", len(r.Agents)), strings.Join(agents, " · "))
	add(i18n.Tf("cmd.bundle.show.skills", len(r.Skills)), i18n.Tf("cmd.bundle.show.initial_value", r.Budget.Initial, r.Budget.EntryAgent, r.Budget.SkillCatalog))
	if len(r.MCP) > 0 {
		add("MCP", strings.Join(r.MCP, ", "))
	}
	isolation := string(r.Isolation)
	if r.StrictIsolation {
		isolation += " · strict"
	}
	add(i18n.T("cmd.bundle.show.isolation"), isolation)
	add(i18n.T("cmd.run.recap.sessions"), i18n.Tf("cmd.run.recap.sessions_value", len(p.plan.Sessions), p.plan.Worktrees()))
	for _, s := range p.plan.Sessions {
		loc := s.Location.Path
		switch {
		case s.Location.Create:
			loc += " (" + i18n.Tf("cmd.run.recap.new_worktree", s.Location.Branch) + ")"
		case s.Location.Kind == runsvc.LocationBase:
			loc += " (" + i18n.T("cmd.run.recap.base") + ")"
		}
		label := s.Label
		if label == "" {
			label = theme.IconArrow
		}
		add("  "+label, loc)
	}
	for _, w := range p.plan.Warnings {
		warnings = append(warnings, warningText(w))
	}
	return rows, warnings
}

// printRunRecap prints what a launch will do (oh run --recap).
func printRunRecap(w io.Writer, p *preparedRun) {
	fmt.Fprintln(w, theme.Title.Render(i18n.Tf("cmd.run.recap.title", p.resolution.Spec.ID, p.resolution.Ref.String(), p.project.Name)))
	rows, warns := runRecap(p)
	for _, r := range rows {
		fmt.Fprintf(w, "  %-14s %s\n", r[0], r[1])
	}
	for _, wr := range warns {
		fmt.Fprintf(w, "  %s %s\n", theme.WarningStyle.Render(theme.IconWarning), wr)
	}
}

func printRunWarnings(w io.Writer, warns []runsvc.Warning) {
	for _, wr := range warns {
		fmt.Fprintf(w, "  %s %s\n", theme.WarningStyle.Render(theme.IconWarning), warningText(wr))
	}
}
