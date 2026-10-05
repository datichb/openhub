package cmd

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/platform"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// v5Platform decorates the legacy opencode platform so that parallel runs
// (oh start --parallel, sweeps) and headless runs (brief enrichment, sweep
// single task, LLM completions) use the v5 runtime when opencode V2 is
// installed: session bundles, shared server groups, credential proxy.
type v5Platform struct {
	platform.SessionPlatform
	app *app.App
}

func wrapV5Platform(a *app.App, p platform.SessionPlatform) platform.SessionPlatform {
	return &v5Platform{SessionPlatform: p, app: a}
}

func (p *v5Platform) Capabilities() platform.Capabilities {
	c := p.SessionPlatform.Capabilities()
	if v5Available(context.Background()) {
		c.Parallel, c.Events = true, true
	}
	return c
}

func (p *v5Platform) RequiresDeploy() bool {
	if v5Available(context.Background()) {
		return false
	}
	return p.SessionPlatform.RequiresDeploy()
}

// NewParallelRunner implements platform.SessionPlatform.
func (p *v5Platform) NewParallelRunner(opts platform.ParallelRunnerOpts) (platform.ParallelRunner, error) {
	if !v5Available(context.Background()) {
		return p.SessionPlatform.NewParallelRunner(opts)
	}
	project, err := p.app.Projects.Get(context.Background(), opts.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("loading project: %w", err)
	}
	svc, err := newRunService(context.Background(), p.app)
	if err != nil {
		return nil, err
	}
	return &v5ParallelRunner{app: p.app, svc: svc, project: project, provider: opts.Provider,
		bundles: map[string]*bundle.Bundle{}, tasks: map[string]string{}}, nil
}

// RunHeadless implements platform.SessionPlatform: a one-shot session on the
// v5 runtime (no interactive client), stopped once its turn is over.
func (p *v5Platform) RunHeadless(ctx context.Context, opts platform.HeadlessOpts) (*platform.HeadlessResult, error) {
	if !v5Available(ctx) || opts.ProjectID == "" {
		return p.SessionPlatform.RunHeadless(ctx, opts)
	}
	project, err := p.app.Projects.Get(ctx, opts.ProjectID)
	if err != nil {
		return p.SessionPlatform.RunHeadless(ctx, opts)
	}
	svc, err := newRunService(ctx, p.app)
	if err != nil {
		return nil, err
	}
	entry := opts.Agent
	if entry == "" {
		entry = "orchestrator"
	}
	req := v5Request(p.app, project, opts.Provider)
	b, err := buildSessionBundle(p.app, project, config.ResolveTeamForProject(p.app.Config, project), entry, req.Provider)
	if err != nil {
		return nil, err
	}
	location := opts.ProjectPath
	if location == "" {
		location = project.Path
	}
	req.Bundle, req.Location, req.EntryAgent, req.Prompt = b, location, entry, opts.Prompt
	req.Title, req.WorkflowID, req.Attach = sessionTitle(project, entry)+" (headless)", entry, sessionspec.AttachNone
	res, err := svc.StartSession(ctx, req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = svc.StopSession(context.Background(), res.SessionID) }()

	c := opencodev2.NewClient(res.Server.URL, res.Server.Password)
	if err := c.Wait(ctx, res.SessionID); err != nil {
		return nil, fmt.Errorf("waiting for the headless session: %w", err)
	}
	text, err := c.AssistantText(ctx, res.SessionID)
	if err != nil {
		return nil, err
	}
	out := &platform.HeadlessResult{Content: text, RawOutput: text}
	if s, err := c.GetSession(ctx, res.SessionID); err == nil {
		out.Cost, out.TokensIn, out.TokensOut, out.TokensReasoning = s.Cost, s.Tokens.Input, s.Tokens.Output, s.Tokens.Reasoning
		if s.Model != nil {
			out.Model = s.Model.ProviderID + "/" + s.Model.ID
		}
	}
	return out, nil
}

// v5ParallelRunner runs parallel tasks as sessions of one server group (one
// tool server for every worktree of the run).
type v5ParallelRunner struct {
	app      *app.App
	svc      *runsvc.Service
	project  *domain.Project
	provider string

	mu      sync.Mutex
	bundles map[string]*bundle.Bundle // entry agent → bundle
	tasks   map[string]string         // task id → session id
	groups  map[string]bool
}

var _ platform.ParallelRunner = (*v5ParallelRunner)(nil)

func (r *v5ParallelRunner) bundleFor(agent, prov string) (*bundle.Bundle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if b, ok := r.bundles[agent]; ok {
		return b, nil
	}
	b, err := buildSessionBundle(r.app, r.project, config.ResolveTeamForProject(r.app.Config, r.project), agent, prov)
	if err != nil {
		return nil, err
	}
	r.bundles[agent] = b
	return b, nil
}

func (r *v5ParallelRunner) LaunchTask(ctx context.Context, opts platform.TaskOpts) (platform.TaskHandle, error) {
	agent := opts.Agent
	if agent == "" {
		agent = "orchestrator-dev"
	}
	req := v5Request(r.app, r.project, r.provider)
	b, err := r.bundleFor(agent, req.Provider)
	if err != nil {
		return platform.TaskHandle{}, err
	}
	req.Bundle, req.Location, req.EntryAgent, req.Prompt = b, opts.WorktreePath, agent, opts.Prompt
	req.Title, req.WorkflowID, req.Attach = opts.Title, agent, sessionspec.AttachNone
	if req.Title == "" {
		req.Title = opts.TaskID
	}
	res, err := r.svc.StartSession(ctx, req)
	if err != nil {
		return platform.TaskHandle{}, err
	}
	r.mu.Lock()
	r.tasks[opts.TaskID] = res.SessionID
	if r.groups == nil {
		r.groups = map[string]bool{}
	}
	r.groups[res.GroupKey] = true
	r.mu.Unlock()
	return platform.TaskHandle{TaskID: opts.TaskID, SessionID: res.SessionID}, nil
}

func (r *v5ParallelRunner) session(taskID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sid, ok := r.tasks[taskID]
	if !ok {
		return "", fmt.Errorf("unknown task %s", taskID)
	}
	return sid, nil
}

// GetAllStatuses maps v5 session states (kept up to date by the daemon) to
// the coordinator statuses.
func (r *v5ParallelRunner) GetAllStatuses(ctx context.Context) (map[string]platform.TaskStatus, error) {
	r.mu.Lock()
	tasks := make(map[string]string, len(r.tasks))
	for k, v := range r.tasks {
		tasks[k] = v
	}
	r.mu.Unlock()
	out := make(map[string]platform.TaskStatus, len(tasks))
	for task, sid := range tasks {
		ts := platform.TaskStatus{SessionID: sid, Status: "running"}
		if s, err := r.svc.Session(ctx, sid); err == nil {
			ts.Status = taskStatus(s.State)
		}
		out[task] = ts
	}
	return out, nil
}

func taskStatus(s domain.RunState) string {
	switch s {
	case domain.RunIdle, domain.RunSleeping:
		return "idle"
	case domain.RunCompleted, domain.RunStopped:
		return "completed"
	case domain.RunFailed:
		return "failed"
	}
	return "running"
}

func (r *v5ParallelRunner) GetModifiedFiles(ctx context.Context, taskID string) ([]platform.FileChange, error) {
	sid, err := r.session(taskID)
	if err != nil {
		return nil, err
	}
	res, err := r.results(ctx, sid)
	if err != nil {
		return nil, err
	}
	out := make([]platform.FileChange, 0, len(res.Changes))
	for _, c := range res.Changes {
		op := "modified"
		switch c.Status {
		case "added":
			op = "created"
		case "deleted", "removed":
			op = "deleted"
		}
		out = append(out, platform.FileChange{Path: c.File, Operation: op})
	}
	return out, nil
}

func (r *v5ParallelRunner) results(ctx context.Context, sid string) (adapters.SessionResult, error) {
	sess, err := r.svc.Session(ctx, sid)
	if err != nil {
		return adapters.SessionResult{}, err
	}
	srv, err := r.svc.Servers.Get(ctx, sess.GroupKey)
	if err != nil {
		return adapters.SessionResult{}, err
	}
	return r.svc.Adapter.Results(ctx, adapters.ServerHandle{URL: srv.URL, Password: srv.Password, PID: srv.PID}, sid)
}

func (r *v5ParallelRunner) control(ctx context.Context, taskID string, op adapters.ControlOp) error {
	sid, err := r.session(taskID)
	if err != nil {
		return err
	}
	sess, err := r.svc.Session(ctx, sid)
	if err != nil {
		return err
	}
	srv, err := r.svc.Servers.Get(ctx, sess.GroupKey)
	if err != nil {
		return err
	}
	return r.svc.Adapter.Control(ctx, adapters.ServerHandle{URL: srv.URL, Password: srv.Password, PID: srv.PID}, sid, op)
}

func (r *v5ParallelRunner) SendMessage(ctx context.Context, taskID, message string) error {
	return r.control(ctx, taskID, adapters.ControlOp{Kind: "prompt", Text: message})
}

func (r *v5ParallelRunner) AbortTask(ctx context.Context, taskID string) error {
	sid, err := r.session(taskID)
	if err != nil {
		return err
	}
	return r.svc.StopSession(ctx, sid)
}

func (r *v5ParallelRunner) AttachTask(ctx context.Context, taskID string) error {
	sid, err := r.session(taskID)
	if err != nil {
		return err
	}
	return runAttachChild(ctx, r.app, r.svc, sid)
}

// Cleanup lets the sessions finish their current step, then sleep (v5
// sessions are resumable; they are not killed when the monitor exits).
func (r *v5ParallelRunner) Cleanup(ctx context.Context) {
	r.mu.Lock()
	groups := make([]string, 0, len(r.groups))
	for g := range r.groups {
		groups = append(groups, g)
	}
	r.mu.Unlock()
	if len(groups) == 0 {
		return
	}
	dc, _, err := ensureDaemon(ctx)
	if err != nil {
		return
	}
	for _, g := range groups {
		if err := dc.SetPolicy(ctx, g, daemon.PolicySleepWhenIdle); err != nil && !errors.Is(err, daemon.ErrNotRunning) {
			continue
		}
	}
}
