package cmd

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/runsvc"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// TUI wiring of the launch form (P1-T22): the form is generated from the
// resolved workflow; its recap and launch go through prepareWorkflowRun and
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
	res, err := resolve(ctx, workflowsvc.Context{ProjectID: project.ID}, req.WorkflowID, workflowsvc.ResolveOpts{})
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
		DefaultMode: req.Mode, DefaultRuntime: req.Runtime,
	}
	if cfg.DefaultRuntime == "" {
		cfg.DefaultRuntime = string(sp.PickRuntime(runtimePrefs(a, project)...))
	}
	cfg.RuntimeStatus, cfg.Progress = tuiRuntimeStatus(a, project), &views.LaunchProgress{}
	if beads.Available() == nil && beads.IsInitialized(project.Path) {
		cfg.Beads = views.NewBeadsSource(project.Path)
	}
	runs := &tuiRunCache{}
	opts := func(c views.LaunchChoices) runOptions {
		return runOptions{Workflow: sp.ID, Project: project, Inputs: c.Inputs, Tickets: c.Tickets, Mode: c.Mode,
			Runtime: c.Runtime, Location: c.Location, Attach: c.Attach, ParentSessionID: req.Parent, OneSession: c.OneSession, Draft: req.Draft}
	}
	cfg.Recap = func(ctx context.Context, c views.LaunchChoices) (*views.LaunchRecap, error) {
		p, err := runs.get(ctx, a, opts(c))
		if err != nil {
			return nil, err
		}
		rows, warns := runRecap(p)
		out := &views.LaunchRecap{Warnings: warns}
		for _, s := range p.suggestions {
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
	cfg.Launch = func(ctx context.Context, c views.LaunchChoices) error {
		p, err := runs.get(ctx, a, opts(c))
		if err != nil {
			return err
		}
		runs.reset() // a plan starts once
		// Image build output, shown by the form while launching.
		p.plan.Request.Base.Progress = cfg.Progress.Line
		_, err = p.start(ctx, a, tuiLaunchUI())
		if tuiStartWiring != nil {
			tuiStartWiring.invalidate() // recents
		}
		return err
	}
	cfg.LaunchFirst = func(ctx context.Context, c views.LaunchChoices, workflowID string) error {
		p, err := runs.get(ctx, a, opts(c))
		if err != nil {
			return err
		}
		for _, s := range p.suggestions {
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
	opts runOptions
	p    *preparedRun
}

func (c *tuiRunCache) get(ctx context.Context, a *app.App, opts runOptions) (*preparedRun, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.p != nil && reflect.DeepEqual(c.opts, opts) {
		return c.p, nil
	}
	p, err := prepareWorkflowRun(ctx, a, opts, io.Discard)
	if err != nil {
		return nil, err
	}
	c.opts, c.p = opts, p
	return p, nil
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
			lr.Reason = i18n.T("cmd.run.requires_v2")
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
