package launch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/runsvc"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// The workflows of the tests: a writer with a ticket input (and an alias), a
// writer without inputs, a read-only one that runs in a container only, and
// a document that does not validate.
var testWorkflows = map[string]string{
	"ticket.yaml": `apiVersion: oh/v1
kind: Workflow
id: ticket
risk: write
aliases: [tk]
entry: { agent: developer }
inputs:
  ticket: { type: beads-id, required: true, picker: { multi: true } }
  branch: { type: branch, default: "feat/{{ .ticket }}" }
prompt:
  text: "Work on {{ .ticket }} ({{ .branch }})"
agents:
  developer: { role: workflow, mode: primary }
runtime: { default: local, allowed: [local, container] }
`,
	"quick.yaml": `apiVersion: oh/v1
kind: Workflow
id: quick
risk: write
entry: { agent: developer }
prompt:
  text: "Go"
agents:
  developer: { role: workflow, mode: primary }
`,
	"far.yaml": `apiVersion: oh/v1
kind: Workflow
id: far
risk: write
entry: { agent: developer }
prompt:
  text: "Go"
agents:
  developer: { role: workflow, mode: primary }
runtime: { default: remote, allowed: [remote] }
`,
	"ask.yaml": `apiVersion: oh/v1
kind: Workflow
id: ask
risk: write
entry: { agent: developer }
inputs:
  request: { type: text }
prompt:
  text: |
    tracker={{ .oh.tracker }} project={{ .oh.tracker_project }} mcp={{ .oh.gitlab_mcp }}
    {{- range .oh.gitlab_refs }}
    ref {{ .text }} -> {{ .tool }} project={{ .project }} iid={{ .iid }}
    {{- end }}
agents:
  developer: { role: workflow, mode: primary }
`,
	"broken.yaml": `apiVersion: oh/v1
kind: Workflow
id: broken
risk: nonsense
`,
}

type fakeGit struct{ dirty bool }

func (g fakeGit) IsRepo(string) bool                         { return true }
func (g fakeGit) IsDirty(string) bool                        { return g.dirty }
func (g fakeGit) CreateWorktree(p, b string) (string, error) { return filepath.Join(p, "..", b), nil }
func (g fakeGit) Stash(string, string) (string, error)       { return "abc1234", nil }

// fakes records what the launch asked of its dependencies.
type fakes struct {
	project    *domain.Project
	planOnly   []bool
	provider   string
	claimsHook int
	credential int
	note       string
	bundleErr  error
	credErr    error
	toolErr    error
}

func newDeps(t *testing.T) (Deps, *fakes) {
	t.Helper()
	i18ntest.Set(t, "en")
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	require.NoError(t, err)
	if _, err := os.Stat(filepath.Join(root, "agents")); err != nil {
		t.Skip("hub content not found")
	}
	dir := t.TempDir()
	for name, body := range testWorkflows {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	}
	f := &fakes{project: &domain.Project{ID: "p1", Name: "proj", Path: t.TempDir()}}
	locks := t.TempDir()
	d := Deps{
		Config:      &config.Config{},
		RequireTool: func(context.Context) error { return f.toolErr },
		Workflows: func(context.Context) *workflowsvc.Service {
			return &workflowsvc.Service{HubDir: root, HubWorkflowsDir: dir}
		},
		InputSources: func(note func(string)) map[string]workflowsvc.InputSource {
			if f.note != "" {
				note(f.note)
			}
			return nil
		},
		Tickets: func(*domain.Project) workflowsvc.TicketSource { return nil },
		Credentials: func(context.Context, *domain.Project, string) error {
			f.credential++
			return f.credErr
		},
		NewRunService: func(_ context.Context, planOnly bool) (*runsvc.Service, error) {
			f.planOnly = append(f.planOnly, planOnly)
			return &runsvc.Service{Git: fakeGit{}, LaunchLocksDir: locks}, nil
		},
		TicketClaims: func(*domain.Project) func(context.Context, string, []string) []string {
			f.claimsHook++
			return func(context.Context, string, []string) []string { return nil }
		},
		BuildBundle: func(_ *domain.Project, _ *workflowsvc.Resolution, prov string) (*bundle.Bundle, []string, error) {
			f.provider = prov
			return &bundle.Bundle{}, []string{"jira"}, f.bundleErr
		},
		Request: func(p *domain.Project, _ string) runsvc.StartRequest {
			return runsvc.StartRequest{ProjectID: p.ID, Provider: "bedrock"}
		},
		Now: func() time.Time { return time.Date(2026, 10, 23, 14, 5, 9, 0, time.UTC) },
	}
	return d, f
}

func collect() (func(Event), *[]Event) {
	var events []Event
	return func(e Event) { events = append(events, e) }, &events
}

func kinds(events []Event, kind EventKind) []string {
	var out []string
	for _, e := range events {
		if e.Kind == kind {
			out = append(out, e.Text)
		}
	}
	return out
}

func TestPrepareDryRunPlansWithoutStarting(t *testing.T) {
	d, f := newDeps(t)
	progress, events := collect()
	p, err := Prepare(t.Context(), d, Options{Workflow: "quick", Project: f.project, Inputs: map[string]string{}, DryRun: true, Progress: progress})
	require.NoError(t, err)

	assert.Equal(t, []bool{true}, f.planOnly, "the service only plans")
	assert.Zero(t, f.claimsHook, "a dry run claims no ticket")
	assert.Equal(t, "bedrock", f.provider, "the hub default provider")
	require.Len(t, p.Plan.Sessions, 1)
	assert.Contains(t, p.Plan.Sessions[0].Prompt, "Go")
	assert.Equal(t, "proj · quick", p.Plan.Request.Base.Title)
	assert.Equal(t, f.project.Path, p.Plan.Sessions[0].Location.Path)
	assert.Contains(t, p.Plan.Warnings, runsvc.Warning{Code: "mcp_missing", Args: []any{"jira"}}, "MCP servers the project lacks are warned")
	assert.Empty(t, *events, "nothing to tell")

	_, err = Start(t.Context(), p, StartOptions{})
	assert.ErrorContains(t, err, "never starts a session")
}

func TestPrepareClaimsHookOnlyOutsideDryRun(t *testing.T) {
	d, f := newDeps(t)
	_, err := Prepare(t.Context(), d, Options{Workflow: "quick", Project: f.project})
	require.NoError(t, err)
	assert.Equal(t, 1, f.claimsHook)
	assert.Equal(t, []bool{false}, f.planOnly)
	assert.Equal(t, 1, f.credential, "credentials are checked for a local launch")
}

func TestPrepareOneSessionPerTicket(t *testing.T) {
	d, f := newDeps(t)
	p, err := Prepare(t.Context(), d, Options{Workflow: "ticket", Project: f.project, Tickets: []string{"bd-1", "bd-2"}})
	require.NoError(t, err)
	require.Len(t, p.Plan.Sessions, 2)
	assert.Equal(t, "bd-1", p.Plan.Sessions[0].Label)
	assert.Equal(t, "proj · ticket · bd-2", p.Plan.Sessions[1].Title)
	assert.Contains(t, p.Plan.Sessions[1].Prompt, "Work on bd-2 (feat/bd-2)")
	assert.Equal(t, []string{"bd-1"}, p.Plan.Sessions[0].Tickets)
}

func TestPrepareAliasAndComputedInputsAreReported(t *testing.T) {
	d, f := newDeps(t)
	f.note = "computed input"
	progress, events := collect()
	p, err := Prepare(t.Context(), d, Options{Workflow: "tk", Project: f.project, Tickets: []string{"bd-9"}, Progress: progress})
	require.NoError(t, err)
	assert.Equal(t, "ticket", p.Resolution.Spec.ID)
	assert.Equal(t, []string{i18n.Tf("cmd.run.alias", "tk", "ticket"), "computed input"}, kinds(*events, EventNotice))
}

func TestPrepareBranchOfASessionWithoutTicketUsesTheClock(t *testing.T) {
	d, f := newDeps(t)
	p, err := Prepare(t.Context(), d, Options{Workflow: "quick", Project: f.project})
	require.NoError(t, err)
	assert.Equal(t, "oh/quick-20261023-140509", p.Plan.Request.Sessions[0].Branch)
}

func TestPrepareRefusals(t *testing.T) {
	t.Run("no project", func(t *testing.T) {
		d, _ := newDeps(t)
		_, err := Prepare(t.Context(), d, Options{Workflow: "quick"})
		assert.EqualError(t, err, i18n.T("cmd.run.no_project"))
	})
	t.Run("no tool, nothing else is asked", func(t *testing.T) {
		d, f := newDeps(t)
		f.toolErr = errors.New("no tool")
		_, err := Prepare(t.Context(), d, Options{Workflow: "quick", Project: f.project})
		assert.EqualError(t, err, "no tool")
		assert.Empty(t, f.planOnly)
		assert.Zero(t, f.credential)
	})
	t.Run("unknown workflow", func(t *testing.T) {
		d, f := newDeps(t)
		_, err := Prepare(t.Context(), d, Options{Workflow: "nope", Project: f.project})
		assert.EqualError(t, err, i18n.Tf("cmd.workflow.show.unknown", "nope"))
	})
	t.Run("invalid workflow shows its findings", func(t *testing.T) {
		d, f := newDeps(t)
		progress, events := collect()
		_, err := Prepare(t.Context(), d, Options{Workflow: "broken", Project: f.project, Progress: progress})
		require.Error(t, err)
		require.Len(t, *events, 1)
		assert.Equal(t, EventDiagnostics, (*events)[0].Kind)
		assert.NotEmpty(t, (*events)[0].Diagnostics)
		assert.Contains(t, err.Error(), i18n.Tf("cmd.workflow.show.invalid", len((*events)[0].Diagnostics.Errors())))
	})
	t.Run("-m on a workflow without request", func(t *testing.T) {
		d, f := newDeps(t)
		_, err := Prepare(t.Context(), d, Options{Workflow: "quick", Project: f.project, Message: true, Inputs: map[string]string{RequestInput: "x"}})
		assert.EqualError(t, err, i18n.Tf("cmd.run.message_no_request_none", "quick"))
	})
	t.Run("credentials", func(t *testing.T) {
		d, f := newDeps(t)
		f.credErr = errors.New("no key")
		_, err := Prepare(t.Context(), d, Options{Workflow: "quick", Project: f.project})
		assert.EqualError(t, err, "no key")
		assert.Empty(t, f.planOnly, "refused before the service is wired")
	})
	t.Run("runtime unavailable", func(t *testing.T) {
		d, f := newDeps(t)
		_, err := Prepare(t.Context(), d, Options{Workflow: "ticket", Project: f.project, Tickets: []string{"bd-1"}, Runtime: "container"})
		assert.ErrorContains(t, err, i18n.Tf("cmd.run.runtime_unavailable", "container", ""))
	})
	t.Run("dry run of a remote launch", func(t *testing.T) {
		d, f := newDeps(t)
		_, err := Prepare(t.Context(), d, Options{Workflow: "far", Project: f.project, DryRun: true})
		assert.EqualError(t, err, i18n.T("cmd.run.dry_run.remote"))
		assert.Empty(t, f.planOnly)
	})
	t.Run("bundle", func(t *testing.T) {
		d, f := newDeps(t)
		f.bundleErr = errors.New("boom")
		_, err := Prepare(t.Context(), d, Options{Workflow: "quick", Project: f.project})
		assert.EqualError(t, err, fmt.Errorf(i18n.T("cmd.v5_run.building_session_bundle"), f.bundleErr).Error())
	})
	t.Run("ticket on a workflow without ticket input", func(t *testing.T) {
		d, f := newDeps(t)
		_, err := Prepare(t.Context(), d, Options{Workflow: "quick", Project: f.project, Tickets: []string{"bd-1"}})
		assert.EqualError(t, err, i18n.Tf("cmd.run.no_ticket_input", "quick"))
	})
}

func TestPrepareFanoutNeedsTheConfiguration(t *testing.T) {
	assert.EqualError(t, CheckFanoutLaunch(&config.Config{}, sessionspec.RuntimeLocal, nil), i18n.T("cmd.fanout.disabled"))
	on := &config.Config{}
	on.Fanout.Experimental = true
	assert.NoError(t, CheckFanoutLaunch(on, sessionspec.RuntimeContainer, nil))
	assert.Error(t, CheckFanoutLaunch(on, sessionspec.RuntimeRemote, nil))
	assert.Error(t, CheckFanoutLaunch(on, sessionspec.RuntimeLocal, []string{"bd-1"}), "one parent session for every ticket")
}

func TestReplanKeepsTheChoice(t *testing.T) {
	d, f := newDeps(t)
	p, err := Prepare(t.Context(), d, Options{Workflow: "quick", Project: f.project})
	require.NoError(t, err)
	svc, prompt := p.Svc, p.Plan.Sessions[0].Prompt

	require.NoError(t, Replan(t.Context(), p, runsvc.OnExistingParallel, true))
	assert.Equal(t, runsvc.OnExistingParallel, p.Opts.OnExisting)
	assert.True(t, p.Opts.DiscardBaseChanges)
	assert.Equal(t, runsvc.OnExistingParallel, p.Plan.Request.OnExisting)
	assert.Same(t, svc, p.Svc, "no new service, resolution or bundle")
	assert.Equal(t, prompt, p.Plan.Sessions[0].Prompt)
	assert.Equal(t, []bool{false}, f.planOnly)
}

func TestSessionLimitsReadTheHubConfiguration(t *testing.T) {
	cfg := &config.Config{}
	cfg.Limits.SessionBudgetUSD = 3
	got := SessionLimits(t.Context(), cfg, nil, &domain.Project{ID: "p1"}, nil)
	assert.InDelta(t, 3, got.SessionBudgetUSD, 0.001)
}

func TestTellStarted(t *testing.T) {
	i18ntest.Set(t, "en")
	progress, events := collect()
	tellStarted(&runsvc.StartResult{SessionID: "ses_1", Stashed: "abc1234", Replaced: "ses_0", Notes: []string{"a note"}, Queued: true, Ahead: 2}, progress)
	assert.Equal(t, []string{
		i18n.Tf("cmd.run.stashed", "abc1234"),
		i18n.Tf("cmd.run.existing.replaced", "ses_0", "ses_1"),
		"a note",
		i18n.Tf("cmd.budget.queued", "ses_1", 2),
	}, kinds(*events, EventInfo))
	tellStarted(&runsvc.StartResult{SessionID: "ses_2"}, nil)
}

func TestBudgetError(t *testing.T) {
	i18ntest.Set(t, "en")
	plain := errors.New("x")
	assert.Same(t, plain, BudgetError(plain))
	err := BudgetError(&runsvc.DailyBudgetError{Spent: 5, Allowance: 4, Scope: "hub"})
	assert.EqualError(t, err, i18n.Tf("cmd.budget.daily_spent", 5.0, 4.0, "hub"))
}

func TestRemoteIsOffByDefault(t *testing.T) {
	i18ntest.Set(t, "en")
	assert.False(t, RemoteExperimental(nil))
	assert.EqualError(t, RequireRemoteExperimental(&config.Config{}), i18n.T("cmd.remote.experimental.disabled"))
	on := &config.Config{}
	on.Remote.Experimental = true
	assert.NoError(t, RequireRemoteExperimental(on))
}
