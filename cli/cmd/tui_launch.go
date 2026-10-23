package cmd

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/services/launch"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// TUI wiring of the launch form (P1-T22): the form is generated from the
// resolved workflow; its recap and launch go through launch.Prepare and
// the RunService (cmd/v5_run.go). No launch logic here.

// tuiLaunchRequest opens a launch form.
type tuiLaunchRequest struct {
	WorkflowID string
	ProjectID  string // "" = active project (or a choice)
	Prefill    map[string]string
	Tickets    []string
	AtOptions  bool
	Parent     string // chained session (O7)
	// Mode and Runtime preselect the options ("" = workflow defaults).
	Mode, Runtime string
	// Draft tests the current member's draft (catalogue `t`, local only).
	Draft bool
	// Location preselects a location (an existing worktree path).
	Location string
	// Agent preselects the entry agent of a workflow whose entry is
	// selectable (`libre`: free session with this agent).
	Agent string
	// AliasTarget is the workflow an alias (WorkflowID) opens, chosen in the
	// omnibar (« feature → implement-spec (avec tickets) »); "" = resolved
	// from the inputs given, like oh run.
	AliasTarget string
}

// openLaunchForm resolves the workflow and the project off the event loop,
// then pushes the form.
func openLaunchForm(a *app.App, req tuiLaunchRequest) {
	sh := tuiShell
	if sh == nil {
		return
	}
	withTUIProject(a, req.ProjectID, func(project *domain.Project) {
		go func() {
			cfg, err := launchFormConfig(context.Background(), a, project, req)
			sh.App().QueueUpdateDraw(func() {
				if err != nil {
					sh.ShowToast(err.Error(), shell.ToastError)
					return
				}
				sh.PushView(views.NewLaunchFormView(*cfg))
			})
		}()
	})
}

// withTUIProject runs fn with the project of a launch: the given or active
// one, the only one, or the user's choice.
func withTUIProject(a *app.App, projectID string, fn func(*domain.Project)) {
	ctx := context.Background()
	if projectID != "" {
		if p, err := a.Projects.Get(ctx, projectID); err == nil {
			fn(p)
			return
		}
	}
	if p, err := resolveActiveProject(a); err == nil {
		fn(p)
		return
	}
	projects, _ := a.Projects.List(ctx, domain.ProjectStatusActive)
	if len(projects) == 0 {
		tuiShell.ShowToast(i18n.T("tui.session.no_project_configured"), shell.ToastWarning)
		return
	}
	opts := make([]views.SelectOption, len(projects))
	for i, p := range projects {
		opts[i] = views.SelectOption{Label: p.Name, Value: p.ID}
	}
	tuiShell.ShowSelectModal(i18n.T("tui.session.choose_project"), opts, "", func(id string) {
		for i := range projects {
			if projects[i].ID == id {
				fn(&projects[i])
				return
			}
		}
	})
}

// launchFormConfig builds the form of a workflow for a project.
func launchFormConfig(ctx context.Context, a *app.App, project *domain.Project, req tuiLaunchRequest) (*views.LaunchFormConfig, error) {
	wsvc := newWorkflowService(ctx)
	resolve := wsvc.Resolve
	if req.Draft {
		resolve = wsvc.ResolveDraft
	}
	id, alias := tuiLaunchAlias(ctx, wsvc, project.ID, req)
	res, err := resolve(ctx, workflowsvc.Context{ProjectID: project.ID}, id, workflowsvc.ResolveOpts{})
	if err != nil {
		var invalid *workflowsvc.InvalidError
		if errors.As(err, &invalid) {
			return nil, errors.New(i18n.Tf("cmd.workflow.show.invalid", len(invalid.Diagnostics.Errors())))
		}
		if req.Draft {
			return nil, workflowEditError(io.Discard, err)
		}
		return nil, err
	}
	sp := res.Spec
	origin := string(res.Ref.Layer)
	if sp.Version > 0 {
		origin += " v" + strconv.Itoa(sp.Version)
	}
	if req.Draft {
		origin = "✎ " + i18n.T("tui.catalog.edit.draft") + " · " + origin
	}
	cfg := &views.LaunchFormConfig{
		WorkflowID: sp.ID, Origin: origin + " · " + project.Name, Spec: sp, Lang: i18n.Locale(),
		Prefill: req.Prefill, Tickets: req.Tickets, AtOptions: req.AtOptions,
		Runtimes: tuiRuntimes(ctx, a, sp, project), Locations: preferLocation(tuiLocations(project), req.Location),
		Attach: tuiAttachOptions(), DefaultAttach: attachPreference(a),
		DefaultMode: workflow.CanonicalMode(req.Mode), DefaultRuntime: req.Runtime,
		Alias: alias, AliasWith: aliasRoute(sp, alias), EntryAgents: res.EntryChoices(), Agent: req.Agent,
		FromPreview: tuiFromPreview(a, project, req.Parent, sp), CheckInput: tuiCheckInput(project.Path),
		InputNotes: tuiInputNotes(a, sp),
	}
	if cfg.DefaultRuntime == "" {
		cfg.DefaultRuntime = string(launch.PickRuntime(a.Config, sp, project))
	}
	cfg.RuntimeStatus, cfg.Progress = tuiRuntimeStatus(a, project), &views.LaunchProgress{}
	cfg.ConfigureProvider = func() {
		if tuiShell != nil {
			tuiShell.NavigateTo("provider")
		}
	}
	if beads.Available() == nil && beads.IsInitialized(project.Path) {
		cfg.Beads = views.NewBeadsSource(project.Path)
	}
	runs := &tuiRunCache{}
	opts := func(c views.LaunchChoices) launch.Options {
		return launch.Options{Workflow: sp.ID, Project: project, Agent: c.Agent, Inputs: c.Inputs, Tickets: c.Tickets, Mode: c.Mode,
			Runtime: c.Runtime, Location: c.Location, Attach: c.Attach, ParentSessionID: req.Parent, OneSession: c.OneSession, Draft: req.Draft,
			Dirty: runsvc.DirtyPolicy(c.Dirty), OnExisting: runsvc.OnExisting(c.OnExisting), DiscardBaseChanges: c.DiscardBaseChanges}
	}
	cfg.Recap = func(ctx context.Context, c views.LaunchChoices) (*views.LaunchRecap, error) {
		p, err := runs.get(ctx, a, opts(c))
		if err != nil {
			return nil, tuiLaunchError(err)
		}
		rows, warns := launchRecap(p)
		warns = append(runs.allChanges(), warns...) // hub.toml changed since the TUI read it
		runs.markShown()
		out := &views.LaunchRecap{Warnings: warns, Dirty: planDirty(p.Plan), Existing: tuiExisting(p.Plan), NeedsChoice: p.Plan.NeedsChoice}
		for _, s := range p.Suggestions {
			label := i18n.Tf("cmd.run.precondition_first", s.Workflow)
			if s.Resume {
				label = i18n.Tf("cmd.run.precondition_first_resume", s.Workflow, sp.ID)
			}
			out.Suggestions = append(out.Suggestions, views.LaunchSuggestion{WorkflowID: s.Workflow, Label: label})
		}
		for _, r := range rows {
			out.Rows = append(out.Rows, views.InfoField{Label: r[0], Value: r[1]})
		}
		return out, nil
	}
	var resumed string // existing session continued by the last launch
	cfg.Launch = func(ctx context.Context, c views.LaunchChoices) error {
		resumed = ""
		p, err := runs.get(ctx, a, opts(c))
		if err != nil {
			return tuiLaunchError(err)
		}
		announceConfigChanges(runs) // changed since the recap that was on screen
		runs.reset()                // a plan starts once
		// Image build output, shown by the form while launching.
		p.Plan.Request.Base.Progress = cfg.Progress.Line
		results, err := startRun(ctx, a, p, tuiLaunchUI())
		if tuiStartWiring != nil {
			tuiStartWiring.invalidate() // recents
		}
		if err == nil && len(results) > 0 && results[0].Existing != runsvc.OnExistingUnset {
			resumed = results[0].SessionID
		}
		return tuiLaunchError(err)
	}
	cfg.Launched = func() {
		if resumed != "" && tuiSess != nil {
			tuiSess.open(resumed) // the session continued, not a new one
		}
	}
	cfg.PlanDelete = func(ctx context.Context, id string) (views.DeleteSummary, error) {
		svc, err := newRunService(ctx, a)
		if err != nil {
			return views.DeleteSummary{}, err
		}
		plan, err := svc.PlanDelete(ctx, id, runsvc.DeleteOptions{ProjectPath: project.Path})
		if err != nil {
			return views.DeleteSummary{}, err
		}
		return deleteSummary(plan), nil
	}
	cfg.LaunchFirst = func(ctx context.Context, c views.LaunchChoices, workflowID string) error {
		p, err := runs.get(ctx, a, opts(c))
		if err != nil {
			return err
		}
		announceConfigChanges(runs)
		for _, s := range p.Suggestions {
			if s.Workflow == workflowID {
				_, err = runSuggestedFirst(ctx, a, opts(c), s, tuiLaunchUI())
				if tuiStartWiring != nil {
					tuiStartWiring.invalidate()
				}
				return err
			}
		}
		return nil
	}
	return cfg, nil
}

// preferLocation moves the option of loc first (the form default).
func preferLocation(opts []views.SelectOption, loc string) []views.SelectOption {
	for i, o := range opts {
		if loc != "" && o.Value == loc && i > 0 {
			out := append([]views.SelectOption{o}, opts[:i]...)
			return append(out, opts[i+1:]...)
		}
	}
	return opts
}

// tuiRunCache keeps the prepared launch of the recap for the launch (same
// choices: no second bundle build).
type tuiRunCache struct {
	mu   sync.Mutex
	opts launch.Options
	p    *launch.Prepared
	// snap is the configuration the prepared launch was built with: the cache
	// is dropped when hub.toml changes after it.
	snap    config.Config
	hasSnap bool
	// changes are the launch settings that changed since the form opened (the
	// recap keeps showing them); shown counts those the user has seen.
	changes []string
	shown   int
}

// announceConfigChanges tells, as a notification, the launch settings that
// changed after the last recap the user saw.
func announceConfigChanges(c *tuiRunCache) {
	if news := c.unshown(); len(news) > 0 && tuiShell != nil {
		c.markShown()
		tuiLaunchUI().Notify(strings.Join(news, "\n"), launcher.LevelWarning)
	}
}

func (c *tuiRunCache) get(ctx context.Context, a *app.App, opts launch.Options) (*launch.Prepared, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.p != nil {
		if changed, _ := c.snap.ChangedOnDisk(); changed {
			c.p = nil // the bundle was built with settings that are not the file's any more
		}
	}
	if c.p != nil && reflect.DeepEqual(withoutExistingChoice(c.opts), withoutExistingChoice(opts)) {
		if c.opts.OnExisting != opts.OnExisting || c.opts.DiscardBaseChanges != opts.DiscardBaseChanges {
			// Another choice for the existing session: planned again, no
			// new resolution or bundle build.
			if err := launch.Replan(ctx, c.p, opts.OnExisting, opts.DiscardBaseChanges); err != nil {
				c.p = nil
				return nil, err
			}
			c.opts = opts
		}
		return c.p, nil
	}
	fa, err := launchAppNow(a)
	var baseline *config.Config
	if c.hasSnap {
		baseline = &c.snap
	}
	changes := launchChangeLines(a, fa, err, baseline)
	p, err := launch.Prepare(ctx, launchDeps(fa), opts)
	if err != nil {
		return nil, err
	}
	for _, line := range changes {
		if !slices.Contains(c.changes, line) {
			c.changes = append(c.changes, line)
		}
	}
	c.opts, c.p, c.snap, c.hasSnap = opts, p, *fa.Config, true
	return p, nil
}

func withoutExistingChoice(o launch.Options) launch.Options {
	o.OnExisting, o.DiscardBaseChanges = runsvc.OnExistingUnset, false
	return o
}

// tuiExisting is the comparison block of the recap: each open session of
// the tickets next to its planned session; the choices apply to the
// session targeted by the first planned session.
func tuiExisting(plan *runsvc.RunPlan) []views.LaunchExisting {
	target, _ := existingTarget(plan)
	req := plan.Request
	var out []views.LaunchExisting
	for _, e := range plan.Existing {
		ps := plan.Sessions[e.Planned]
		x := views.LaunchExisting{ID: e.ID, Title: e.Title, Shared: e.Shared, SameWorkflow: e.SameWorkflow, Target: e.ID == target.ID,
			State: string(e.State), StateLabel: stateLabel(e.State), OpenDecisions: e.OpenDecisions,
			Workflow: e.WorkflowID, PlannedWorkflow: req.Workflow.ID, Version: e.WorkflowVersion, PlannedVersion: req.Workflow.Version,
			Runtime: e.Runtime, PlannedRuntime: plannedField(e, runsvc.DiffRuntime, e.Runtime),
			Mode: e.Mode, PlannedMode: plannedField(e, runsvc.DiffMode, e.Mode),
			Model: e.Model, PlannedModel: plannedField(e, runsvc.DiffModel, e.Model),
			Path: e.LaunchPath, PlannedPath: ps.Location.Path, Branch: e.Branch, PlannedBranch: ps.Location.Branch,
			WorkKnown: e.Work.Known, CommitsUnpushed: e.Work.CommitsUnpushed, ChangedFiles: e.Work.ChangedFiles, Cost: e.Cost}
		for _, d := range e.Diffs {
			x.Diffs = append(x.Diffs, d.Field)
		}
		for _, c := range e.Actions {
			x.Actions = append(x.Actions, string(c))
		}
		out = append(out, x)
	}
	return out
}

// allChanges are the launch settings that changed since the form opened.
func (c *tuiRunCache) allChanges() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.changes)
}

// unshown are the changes the recap has not shown yet.
func (c *tuiRunCache) unshown() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.changes[c.shown:])
}

func (c *tuiRunCache) markShown() {
	c.mu.Lock()
	c.shown = len(c.changes)
	c.mu.Unlock()
}

func (c *tuiRunCache) reset() {
	c.mu.Lock()
	c.p = nil
	c.mu.Unlock()
}

// tuiLaunchUI marshals the launch UI calls (toasts, terminal suspension)
// onto the event loop.
func tuiLaunchUI() launcher.LaunchUI {
	sh := tuiShell
	return launcher.NewTUIUI(
		func(fn func() error) error {
			done := make(chan error, 1)
			sh.App().QueueUpdateDraw(func() { done <- sh.SuspendAndExec(fn) })
			return <-done
		},
		func(msg string, ok bool) {
			sh.App().QueueUpdateDraw(func() { sh.ShowToastMsg(msg, ok) })
		},
	)
}

// tuiRuntimes lists the runtimes allowed by the workflow, with their
// availability (remote: tui_launch_remote.go).
func tuiRuntimes(ctx context.Context, a *app.App, sp *workflow.Spec, project *domain.Project) []views.LaunchRuntime {
	var svc *runsvc.Service
	if v5Available(ctx) {
		svc, _ = newRunService(ctx, a)
	}
	var out []views.LaunchRuntime
	for _, r := range sp.AllowedRuntimes() {
		lr := views.LaunchRuntime{Kind: string(r), Label: i18n.T("tui.launch.runtime_" + string(r))}
		switch {
		case r == workflow.RuntimeLocal:
			lr.Available = true
		case r == workflow.RuntimeRemote:
			lr = remoteLaunchRuntime(ctx, a, project, lr)
		case svc == nil:
			lr.Reason = i18n.Tf("cmd.run.requires_v2", toolName())
		default:
			av, err := svc.RuntimeAvailability(ctx, sessionspec.RuntimeKind(r))
			lr.Available = err == nil && av.OK
			switch {
			case err != nil:
				lr.Reason = i18n.T("tui.launch.runtime_not_supported")
			case !av.OK:
				lr.Reason = av.Message()
			default:
				lr.Label += " · " + av.Engine
			}
		}
		out = append(out, lr)
	}
	return out
}

// tuiLocations lists the locations of a project: base, its worktrees, new.
func tuiLocations(project *domain.Project) []views.SelectOption {
	base := i18n.T("tui.launch.location_base")
	if b, err := worktree.CurrentBranch(project.Path); err == nil && b != "" {
		base += " (" + b + ")"
	}
	out := []views.SelectOption{{Label: base, Value: string(runsvc.LocationBase)}}
	if worktree.IsGitRepo(project.Path) {
		if list, err := worktree.List(project.Path); err == nil {
			for _, e := range list {
				if e.IsBare || filepath.Clean(e.Path) == filepath.Clean(project.Path) {
					continue
				}
				out = append(out, views.SelectOption{Label: i18n.Tf("tui.launch.location_worktree", e.Branch, e.Path), Value: e.Path})
			}
		} else {
			slog.Debug("listing worktrees", "error", err)
		}
		out = append(out, views.SelectOption{Label: i18n.T("tui.launch.location_new"), Value: string(runsvc.LocationNew)})
	}
	return out
}

func tuiAttachOptions() []views.SelectOption {
	var out []views.SelectOption
	for _, p := range []sessionspec.AttachPref{sessionspec.AttachAuto, sessionspec.AttachITerm, sessionspec.AttachTerminal,
		sessionspec.AttachTmux, sessionspec.AttachBrowser, sessionspec.AttachSuspend, sessionspec.AttachNone} {
		out = append(out, views.SelectOption{Label: i18n.T("tui.launch.attach_" + string(p)), Value: string(p)})
	}
	return out
}

// tuiLaunchError gives the launch form the message of the CLI (missing
// credentials, refusal of the daemon translated: N12). Missing credentials
// match views.ErrProviderCredentials (« Configure the provider » button).
func tuiLaunchError(err error) error {
	if err == nil {
		return nil
	}
	msg := localizeError(err)
	var pe *ProviderCredentialError
	var api *daemon.APIError
	var te *runsvc.TransferError
	if errors.As(err, &pe) || (errors.As(err, &api) && api.Path == "/grants" && api.Status == 422) {
		return views.ProviderCredentialsError(msg)
	}
	if errors.As(err, &te) {
		return views.TransferFailedError(i18n.Tf("cmd.run.existing.transfer_failed", te.SessionID, localizeError(te.Err)))
	}
	if msg != err.Error() {
		return errors.New(msg)
	}
	return err
}
