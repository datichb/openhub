// Package launch is the launch of a workflow (oh run, aliases, TUI launch
// form, headless runs, fan-out children, evaluations): it resolves the
// workflow, compiles its bundle, plans the sessions (locations, warnings),
// renders their prompts (Prepare, Replan), then starts them through the
// RunService (Start).
//
// The service reaches the rest of oh through Deps (stores, configuration,
// daemon client and the few wirings that belong to the command line) and tells
// what it does through Progress events: the command line and the TUI render
// them, the package writes nowhere.
package launch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon/client"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	remotesvc "github.com/datichb/openhub/cli/internal/services/remote"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// RequestInput is the input -m fills (oh run -m).
const RequestInput = "request"

// Options are the choices of a workflow launch.
type Options struct {
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
	// Message is true when Inputs[request] comes from -m: the resolved
	// workflow must declare a request input.
	Message bool
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
	// Dirty is what a writing session does in a directory with uncommitted
	// changes: a new worktree (default), --allow-dirty or --stash.
	Dirty runsvc.DirtyPolicy
	// Epics are the epics of Tickets replaced by their open children (set
	// by the launch).
	Epics []workflowsvc.EpicExpansion
	// Progress receives what the preparation tells (see Event); nil = nothing
	// is reported.
	Progress func(Event)
	// OnExisting is what is done with an open session already working on a
	// ticket of the launch (--on-existing; "" = ask, or refused for the
	// same workflow).
	OnExisting runsvc.OnExisting
	// DiscardBaseChanges lets OnExistingDelete discard the uncommitted
	// changes of a session that worked in the project directory.
	DiscardBaseChanges bool
	// DryRun prepares the launch to show it (oh run --dry-run) without any
	// effect on the project or the team: no ticket claim, no remote
	// preparation, no shell files of local sessions. The one thing written
	// is the session bundle, content-addressed in ~/.oh/bundles (shared by
	// every launch of the same content, removed by `oh gc` when unused).
	DryRun bool
}

// Suggestion is a failed `suggest` precondition of a launch (run another
// workflow first).
type Suggestion struct {
	Label    string
	Workflow string
	Resume   bool
}

// Prepared is a launch ready to start: its plan can be shown (recap).
type Prepared struct {
	Opts       Options
	Project    *domain.Project
	Resolution *workflowsvc.Resolution
	Bundle     *bundle.Bundle
	Svc        *runsvc.Service
	Plan       *runsvc.RunPlan
	// Suggestions are the failed `suggest` preconditions.
	Suggestions []Suggestion
	// Remote is set for a remote launch (sessions sent to GitLab CI).
	Remote *RemotePrep

	// What planning again needs (Replan: another choice for the existing
	// sessions): the resolution and prompt values of each session.
	deps        Deps
	sessions    []*workflowsvc.Resolution
	values      []map[string]any
	ticketInput string
	preconds    []runsvc.Warning
	missingMCP  []string
	// tracker is the tracker of the project; gitlabRefs the GitLab tickets
	// named by the inputs of each session.
	tracker    TrackerInfo
	gitlabRefs [][]workflowsvc.GitLabRef
}

// Deps are what the launch needs from the rest of oh. Config, Secrets and
// Preferences are the data it reads; the functions are the wirings of the
// command line (the composition root), each one required unless said
// otherwise.
type Deps struct {
	Config      *config.Config
	Secrets     domain.SecretStore     // trigger key of a remote launch (nil: not checked)
	Preferences domain.PreferenceStore // project restrictions (nil: none)

	// RequireTool refuses when no supported tool is installed (nil: no check).
	RequireTool func(ctx context.Context) error
	// Workflows is the WorkflowService on the hub content.
	Workflows func(ctx context.Context) *workflowsvc.Service
	// InputSources are the sources of computed inputs (`from:`); note receives
	// what they tell while they run.
	InputSources func(note func(string)) map[string]workflowsvc.InputSource
	// Tickets reads the tickets of a project (nil: from Beads).
	Tickets func(project *domain.Project) workflowsvc.TicketSource
	// Tracker says where the tickets of a project live (nil: unknown, so
	// #<iid> is not read as a GitLab issue).
	Tracker func(ctx context.Context, project *domain.Project) TrackerInfo
	// Credentials refuses a launch whose provider has no credentials (nil: none).
	Credentials func(ctx context.Context, project *domain.Project, providerFlag string) error
	// NewRunService wires the RunService; planOnly (dry run) is a service
	// that can plan but not start.
	NewRunService func(ctx context.Context, planOnly bool) (*runsvc.Service, error)
	// TicketClaims is the hook run when a session starts on tickets (nil
	// without a team).
	TicketClaims func(project *domain.Project) func(ctx context.Context, projectID string, tickets []string) []string
	// BuildBundle compiles the bundle of a resolved workflow; missing lists the
	// MCP servers the workflow asks for but the project does not provide.
	BuildBundle func(project *domain.Project, res *workflowsvc.Resolution, prov string) (b *bundle.Bundle, missing []string, err error)
	// Request is the provider, team and attach settings of a project session.
	Request func(project *domain.Project, providerFlag string) runsvc.StartRequest

	// Start.
	// Daemon returns the daemon client, starting the daemon when needed (nil:
	// not started for the tracking of a remote launch).
	Daemon func(ctx context.Context) (*client.Client, error)
	// RemoteSender wires the remote service for sending.
	RemoteSender func() *remotesvc.Service
	// TeamRepo returns the team-state clone of the project.
	TeamRepo func(ctx context.Context, project *domain.Project) (*teamstate.Repo, error)
	// SpawnFanoutDriver starts the driver of a fan-out parent and returns its
	// log path (nil: no driver).
	SpawnFanoutDriver func(sessionID string) (string, error)
	// IsolationWarning localizes a warning of the launch attestation (nil: the
	// generic text).
	IsolationWarning func(w string) string

	// Now is the clock (nil: time.Now).
	Now func() time.Time
}

func (d Deps) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

// EventKind says what an Event is about and how a client shows it.
type EventKind int

const (
	// EventBuild is a line of the preparation output (container image build).
	EventBuild EventKind = iota
	// EventNotice is a note on the way a launch is understood (alias
	// resolved, what a computed input did).
	EventNotice
	// EventDiagnostics are the findings of an invalid workflow.
	EventDiagnostics
	// EventInfo, EventSuccess and EventWarning are the messages of the start,
	// from the most neutral to the one to read.
	EventInfo
	EventSuccess
	EventWarning
)

// Event is what a launch tells while it runs.
type Event struct {
	Kind EventKind
	// Text is the message, already translated (not set for EventDiagnostics).
	Text string
	// Diagnostics are the findings of EventDiagnostics.
	Diagnostics workflow.Diagnostics
}

func emit(f func(Event), e Event) {
	if f != nil {
		f(e)
	}
}

// Prepare resolves, compiles and plans a launch.
func Prepare(ctx context.Context, d Deps, opts Options) (*Prepared, error) {
	if d.RequireTool != nil {
		if err := d.RequireTool(ctx); err != nil {
			return nil, err
		}
	}
	if opts.Project == nil {
		return nil, errors.New(i18n.T("cmd.run.no_project"))
	}
	res, ticketInput, perSession, err := ResolveLaunch(ctx, d, &opts)
	if err != nil {
		return nil, err
	}
	kind := sessionspec.RuntimeKind(res.Runtime)
	if opts.DryRun && kind == sessionspec.RuntimeRemote {
		return nil, errors.New(i18n.T("cmd.run.dry_run.remote"))
	}
	if res.Fanout {
		// R6: experimental, local, one parent session for every ticket.
		if err := CheckFanoutLaunch(d.Config, kind, perSession); err != nil {
			return nil, err
		}
	}
	// Credentials checked before anything is built or asked of the daemon
	// (N12). A remote session gets its own from the runner.
	if kind != sessionspec.RuntimeRemote && d.Credentials != nil {
		if err := d.Credentials(ctx, opts.Project, opts.Provider); err != nil {
			return nil, err
		}
	}

	svc, err := d.NewRunService(ctx, opts.DryRun)
	if err != nil {
		return nil, err
	}
	if !opts.DryRun && d.TicketClaims != nil {
		svc.OnTicketsStarted = d.TicketClaims(opts.Project)
	}
	var rp *RemotePrep
	if kind == sessionspec.RuntimeRemote {
		if rp, err = PrepareRemote(ctx, d, opts.Project, res); err != nil {
			return nil, err
		}
	} else if av, err := svc.RuntimeAvailability(ctx, kind); err != nil || !av.OK {
		reason := av.Message()
		if err != nil {
			reason = err.Error()
		}
		return nil, errors.New(i18n.Tf("cmd.run.runtime_unavailable", string(kind), reason))
	}

	prov := provider.ResolveProvider(opts.Provider, opts.Project.Provider, d.Config.LLM.DefaultProvider)
	b, missingMCP, err := d.BuildBundle(opts.Project, res, prov)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("cmd.v5_run.building_session_bundle"), err)
	}

	base := d.Request(opts.Project, opts.Provider)
	base.Bundle, base.Mode, base.Runtime = b, res.Mode, kind
	if opts.Progress != nil {
		progress := opts.Progress
		base.Progress = func(line string) { progress(Event{Kind: EventBuild, Text: line}) }
	}
	base.Title = sessionTitle(opts.Project, res.Spec.ID)
	base.Limits = SessionLimits(ctx, d.Config, d.Preferences, opts.Project, res.Spec)
	if res.Spec.Beads != nil {
		base.BeadsAllow = append([]string{}, res.Spec.Beads.Allow...) // non-nil: declared
	}
	// p6-debts: a file scope given at launch (sweep-task by hand) is checked
	// now, as oh checks the tasks of a breakdown.
	if id := res.Spec.ScopeInput(); id != "" {
		if raw, _ := res.Inputs[id].(string); strings.TrimSpace(raw) != "" {
			dir := opts.Project.Path
			if l := runsvc.ParseLocation(opts.Location); l.Kind == runsvc.LocationWorktree {
				dir = l.Path
			}
			if _, problems := checkpoint.CheckScope(ctx, dir, id, raw); len(problems) > 0 {
				return nil, errors.New(i18n.Tf("cmd.run.scope_refused", id, "- "+strings.Join(problems, "\n- ")))
			}
		}
	}
	// The inputs passed on by `next:` (the method of spec), recorded now.
	base.Outputs = res.Spec.RecordedInputs(res.Inputs)
	if res.Fanout {
		base.Fanout = FanoutParentRecord(d.Config)
		// The inputs the workflow gives to the children (fanout.inputs:
		// method, instructions), when the session got them.
		base.Fanout.ChildInputs = res.Spec.ChildInputs(res.Inputs)
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
		ParentSessionID: opts.ParentSessionID, Dirty: opts.Dirty,
		OnExisting: opts.OnExisting, DiscardBaseChanges: opts.DiscardBaseChanges,
	}
	if kind == sessionspec.RuntimeRemote {
		req.Dirty = runsvc.DirtyAllow // the job clones the project
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
		req.Sessions = append(req.Sessions, runsvc.PlannedInput{Label: label, Title: title, Branch: SessionBranch(res.Spec, v, label, opts.Branch, d.now()),
			Tickets: RemoteTickets(v, ticketInput), ReadBranch: ReadBranch(res.Spec, v)})
	}
	if rp != nil {
		if err := checkRemoteReview(opts.Project.Path, req.Sessions); err != nil {
			return nil, err
		}
	}
	p := &Prepared{Opts: opts, Project: opts.Project, Resolution: res, Bundle: b, Svc: svc, Remote: rp, deps: d,
		sessions: sessions, values: values, ticketInput: ticketInput, missingMCP: missingMCP,
		tracker: d.trackerOf(ctx, opts.Project)}
	for _, r := range sessions {
		p.gitlabRefs = append(p.gitlabRefs, GitLabRefs(res.Spec, r.Inputs, p.tracker))
	}
	if err := p.planRun(ctx, req); err != nil {
		return nil, err
	}
	return p, nil
}

// Replan plans the launch again with another choice for the open sessions
// of its tickets (no new resolution or bundle build).
func Replan(ctx context.Context, p *Prepared, choice runsvc.OnExisting, discardBase bool) error {
	req := p.Plan.Request
	req.OnExisting, req.DiscardBaseChanges = choice, discardBase
	p.Opts.OnExisting, p.Opts.DiscardBaseChanges = choice, discardBase
	return p.planRun(ctx, req)
}

// planRun plans the sessions of the launch and renders their prompts.
func (p *Prepared) planRun(ctx context.Context, req runsvc.RunRequest) error {
	opts, rp := p.Opts, p.Remote
	plan, err := p.Svc.Plan(ctx, req)
	if err != nil {
		return runPlanError(err)
	}
	// p6-debts: the preconditions are read where the session works (its
	// worktree with --location new), not in the project directory.
	fsys, probe := preconditionTarget(opts.Project.Path, plan.Sessions, rp != nil)
	if p.preconds, p.Suggestions, err = preconditionWarningsIn(p.Resolution.Spec, fsys, probe); err != nil {
		return err
	}
	plan.Warnings = append(append([]runsvc.Warning{}, p.preconds...), plan.Warnings...)
	for _, m := range p.missingMCP {
		plan.Warnings = append(plan.Warnings, runsvc.Warning{Code: "mcp_missing", Args: []any{m}})
	}
	plan.Warnings = append(plan.Warnings, p.gitlabMCPWarnings()...)
	if rp != nil {
		// Remote: the job clones the project; no local location, warning or
		// existing session to continue.
		plan.Warnings = p.preconds
		plan.Existing, plan.NeedsChoice = nil, false
		rp.tickets, rp.inputs = nil, nil
		for i := range plan.Sessions {
			plan.Sessions[i].Existing, plan.Sessions[i].Action = "", ""
			rp.tickets = append(rp.tickets, RemoteTickets(p.values[i], p.ticketInput))
			rp.inputs = append(rp.inputs, p.sessions[i].Inputs)
		}
	}
	memories := ""
	if beads.IsInitialized(opts.Project.Path) {
		if m, err := beads.Memories(opts.Project.Path); err == nil {
			memories = m
		}
	}
	for i := range plan.Sessions {
		loc := plan.Sessions[i].Location.Path
		branch := plan.Sessions[i].Location.WorkBranch(opts.Project.Path)
		if rp != nil {
			loc = remotesvc.WorkDir(opts.Project.Name)
		}
		reviewRef := ""
		if l := plan.Sessions[i].Location; l.Temp {
			reviewRef = worktree.ReadTarget{Ref: l.Ref}.Name()
		}
		prompt, err := p.sessions[i].RenderPrompt(workflowsvc.PromptContext{Project: opts.Project.Name,
			Location: loc, Branch: branch, Lang: i18n.Locale(), Memories: memories, ReviewRef: reviewRef,
			Tracker: p.tracker.Type, TrackerProject: p.tracker.Project, GitLabMCP: hasMCP(p.Bundle, gitlabMCPServer),
			GitLabRefs: p.gitlabRefs[i]})
		if err != nil {
			return err
		}
		plan.Sessions[i].Prompt = prompt
	}
	p.Plan = plan
	return nil
}

func runPlanError(err error) error {
	var rb *runsvc.ReadBranchError
	switch {
	case errors.As(err, &rb):
		return errors.New(i18n.Tf("cmd.run.review_branch_missing", rb.Branch))
	case errors.Is(err, runsvc.ErrNotGitRepo):
		return errors.New(i18n.T("cmd.run.not_git"))
	case errors.Is(err, runsvc.ErrNoBranch):
		return errors.New(i18n.T("cmd.run.no_branch"))
	}
	return err
}

func sessionTitle(p *domain.Project, entry string) string {
	return p.Name + " · " + entry
}

// SessionBranch is the branch of a session worktree: the workflow `branch`
// input, else oh/<workflow>-<ticket|time>.
func SessionBranch(sp *workflow.Spec, values map[string]any, label, fallback string, now time.Time) string {
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
		suffix = now.Format("20060102-150405")
	}
	return "oh/" + sp.ID + "-" + suffix
}

// ReadBranch is the branch a read-only workflow reviews: its `branch` input.
// Empty for a workflow that may write (it works on a branch of its own, see
// SessionBranch), or without a branch given.
func ReadBranch(sp *workflow.Spec, values map[string]any) string {
	if runsvc.RiskWrites(string(sp.Risk)) {
		return ""
	}
	if k := workflowsvc.FirstInput(sp, workflow.InputBranch); k != "" {
		b, _ := values[k].(string)
		return b
	}
	return ""
}

// checkRemoteReview refuses the remote review of a branch other than the
// current one: the job clones the project at the commit of the current
// branch, so it would read the wrong files (a temporary worktree is a local
// notion).
func checkRemoteReview(projectPath string, sessions []runsvc.PlannedInput) error {
	for _, in := range sessions {
		if in.ReadBranch == "" {
			continue
		}
		t, err := worktree.ResolveReadTarget(projectPath, in.ReadBranch)
		if err != nil {
			return runPlanError(&runsvc.ReadBranchError{Branch: in.ReadBranch})
		}
		if !t.Current {
			return errors.New(i18n.Tf("cmd.run.review_remote_other_branch", in.ReadBranch))
		}
	}
	return nil
}
