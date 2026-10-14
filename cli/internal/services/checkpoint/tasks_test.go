package checkpoint

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// sweep-fanout: the breakdown of cp-plan is read and checked by oh, shown on
// the card as oh read it, recorded once passed; a child of a fan-out of
// tasks never commits outside the scope of its task. Each way around it
// has its case.

func tasksWF() *sessionspec.WorkflowRuntime {
	return &sessionspec.WorkflowRuntime{ID: "sweep", DefaultMode: "semi-auto",
		Outputs: []sessionspec.OutputDef{{ID: "tasks", Type: "text", Record: RecordTasks}},
		Checkpoints: []sessionspec.CheckpointDef{
			{ID: "cp-plan", Mandatory: true, Behaviors: allPause(), Tasks: &sessionspec.TasksCheckDef{Max: 3, Domains: []string{"backend", "tests"}}},
			{ID: "cp-recap", Behaviors: allPause()},
		}}
}

var tasksRepo = []string{"api/a.go", "api/b.go", "api/b_test.go", "web/x.ts", "docs/r.md"}

func newTasksService(t *testing.T, fanout *domain.SessionFanout) (*Service, context.Context) {
	t.Helper()
	svc, ctx := newTestService(t)
	writeBundle(t, svc.BundlesDir, "h_sweep", tasksWF())
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "api"), 0o755))
	svc.Files = func(context.Context, string) ([]string, error) { return tasksRepo, nil }
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_s", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_sweep", Mode: "semi-auto", State: domain.RunActive, LaunchPath: dir, Fanout: fanout}))
	return svc, ctx
}

func task(title, domain string, parallel bool, files ...any) map[string]any {
	return map[string]any{"title": title, "description": "d", "domain": domain, "parallel": parallel, "files": files}
}

func askTasks(t *testing.T, svc *Service, ctx context.Context, tasks []any) (*domain.Decision, bool) {
	t.Helper()
	d, auto, _, err := svc.Asked(ctx, "ses_s", "ses_s", adapters.PendingDecision{ID: "per_plan",
		Call: &adapters.ToolCall{Input: map[string]any{"id": "cp-plan", "summary": "plan", "tasks": tasks}}})
	require.NoError(t, err)
	return d, auto
}

func TestTasksRecordedAsOhReadThem(t *testing.T) {
	svc, ctx := newTasksService(t, nil)
	plan := []any{task("Handlers", "backend", true, "api/a.go", "./api/b.go"), task("Tests", "tests", true, "api/new_test.go"), task("Docs", "tests", false, "docs/")}
	d, auto := askTasks(t, svc, ctx, plan)
	require.False(t, auto)
	require.NotNil(t, d)
	assert.Contains(t, d.Payload.Message, "t1 [backend] Handlers — api/a.go, api/b.go (parallel)", "the card shows the breakdown as oh read it")
	assert.Contains(t, d.Payload.Message, "t2 [tests] Tests — api/new_test.go", "a new file in an existing folder")
	eff, err := svc.Evaluate(ctx, "ses_s", bundle.CheckpointAction(), map[string]any{"id": "cp-plan", "tasks": plan})
	require.NoError(t, err)
	assert.Equal(t, "ask", eff, "a breakdown oh takes waits for the user")

	tool := &fakeTool{}
	require.NoError(t, svc.Resolve(ctx, tool, *d, ChoiceApprove, ""))
	res, err := svc.Reached(ctx, "ses_s", Call{ID: "cp-plan", Tasks: plan})
	require.NoError(t, err)
	assert.Contains(t, res.Tasks, "t3 [tests] Docs — docs/")
	st, err := svc.State(ctx, "ses_s")
	require.NoError(t, err)
	require.Len(t, st.Tasks, 3)
	assert.Equal(t, domain.PlanTask{ID: "t1", Title: "Handlers", Description: "d", Files: []string{"api/a.go", "api/b.go"}, Domain: "backend", Parallel: true}, st.Tasks[0])
	assert.Empty(t, st.ShownTasks)
	out, err := svc.Outputs(ctx, "ses_s")
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, "tasks", out[0].ID)
	assert.Contains(t, out[0].Value, "t2 [tests] Tests")
	// Recorded by oh only: an agent cannot declare it.
	assert.ErrorContains(t, svc.Declare(ctx, "ses_s", Output{ID: "tasks", Type: "text", Value: "t1 other"}), "recorded by oh")
}

func TestTasksRefusedBeforeAnyCard(t *testing.T) {
	cases := map[string][]any{
		"none":                {},
		"too many":            {task("a", "backend", false, "api/a.go"), task("b", "backend", false, "api/b.go"), task("c", "tests", false, "web/x.ts"), task("d", "tests", false, "docs/r.md")},
		"no title":            {task("", "backend", false, "api/a.go")},
		"unknown domain":      {task("a", "frontend", false, "web/x.ts")},
		"no scope":            {task("a", "backend", false)},
		"outside the repo":    {task("a", "backend", false, "../etc/passwd")},
		"absolute":            {task("a", "backend", false, "/etc/passwd")},
		"git folder":          {task("a", "backend", false, ".git/config")},
		"option":              {task("a", "backend", false, "--all")},
		"unknown file":        {task("a", "backend", false, "lib/z.go")},
		"pattern of nothing":  {task("a", "backend", false, "lib/**")},
		"parallel overlap":    {task("a", "backend", true, "api/"), task("b", "tests", true, "api/*_test.go")},
		"not a task":          {"api/a.go"},
		"multi-line title":    {task("a\nb", "backend", false, "api/a.go")},
		"repository as scope": {task("a", "backend", false, ".")},
	}
	for name, plan := range cases {
		t.Run(name, func(t *testing.T) {
			svc, ctx := newTasksService(t, nil)
			d, auto := askTasks(t, svc, ctx, plan)
			assert.True(t, auto, "no card for a breakdown oh refuses")
			assert.Nil(t, d)
			eff, err := svc.Evaluate(ctx, "ses_s", bundle.CheckpointAction(), map[string]any{"id": "cp-plan", "tasks": plan})
			require.NoError(t, err)
			assert.Equal(t, "allow", eff, "the call runs, to be refused with the problems")
			_, err = svc.Reached(ctx, "ses_s", Call{ID: "cp-plan", Tasks: plan})
			assert.ErrorIs(t, err, ErrTasksInvalid)
			st, _ := svc.State(ctx, "ses_s")
			assert.False(t, st.HasPassed("cp-plan"))
			assert.Empty(t, st.Tasks)
		})
	}
	// Outside a fan-out, two tasks that run one after the other may share
	// files; in a fan-out, every pair of tasks may run at the same time.
	seq := []any{task("a", "backend", false, "api/"), task("b", "tests", true, "api/b_test.go")}
	svc, ctx := newTasksService(t, nil)
	_, auto := askTasks(t, svc, ctx, seq)
	assert.False(t, auto, "sequential tasks may share files")
	svc, ctx = newTasksService(t, &domain.SessionFanout{Role: domain.RoleParent, Experimental: true, Status: domain.FanoutWaiting})
	_, auto = askTasks(t, svc, ctx, seq)
	assert.True(t, auto, "in a fan-out the scopes are disjoint")
}

// The breakdown validated on the card is the one recorded: a call that
// brings another one (the agent changed its mind after the validation, or
// a forged call) is refused, nothing passes.
func TestTasksChangedAfterTheCardRefused(t *testing.T) {
	svc, ctx := newTasksService(t, nil)
	plan := []any{task("Handlers", "backend", false, "api/a.go")}
	d, _ := askTasks(t, svc, ctx, plan)
	require.NotNil(t, d)
	require.NoError(t, svc.Resolve(ctx, &fakeTool{}, *d, ChoiceApprove, ""))
	other := []any{task("Handlers", "backend", false, "api/")}
	_, err := svc.Reached(ctx, "ses_s", Call{ID: "cp-plan", Tasks: other})
	assert.ErrorIs(t, err, ErrTasksInvalid)
	assert.ErrorContains(t, err, "differs")
	_, err = svc.Reached(ctx, "ses_s", Call{ID: "cp-plan"})
	assert.ErrorIs(t, err, ErrTasksInvalid, "no breakdown in the call")
	st, _ := svc.State(ctx, "ses_s")
	assert.False(t, st.HasPassed("cp-plan"))
	_, err = svc.Reached(ctx, "ses_s", Call{ID: "cp-plan", Tasks: plan})
	require.NoError(t, err)
}

// A child of a fan-out of tasks: its commit checkpoint is refused while it
// changed a file outside the scope of its task (recorded by oh).
func TestTaskScopeOfTheChild(t *testing.T) {
	svc, ctx, _, _ := newVerifyService(t, &sessionspec.VerifyDef{Cmd: []string{"make check"}, Expect: "pass", TimeoutSec: 60})
	sess, err := svc.Sessions.Get(ctx, "ses_v")
	require.NoError(t, err)
	child := &domain.SessionFanout{Role: domain.RoleChild, Parent: "ses_p", Ticket: "t1",
		Task: &domain.PlanTask{ID: "t1", Title: "x", Files: []string{"api/", "go.sum"}}}
	sess.Fanout = child
	svc.Changed = func(context.Context, string, string) ([]string, error) {
		return []string{"api/a.go", "api/x/y.go", "go.sum"}, nil
	}
	r, ok := svc.scopeCheck(ctx, sess)
	assert.True(t, ok, "%v", r)

	svc.Changed = func(context.Context, string, string) ([]string, error) {
		return []string{"api/a.go", "go.sum", "web/x.ts"}, nil
	}
	r, ok = svc.scopeCheck(ctx, sess)
	assert.False(t, ok)
	assert.Contains(t, r.Reason, "web/x.ts")
	assert.NotContains(t, r.Reason, "api/a.go,")

	// Not a child of tasks: no scope.
	sess.Fanout = &domain.SessionFanout{Role: domain.RoleChild, Parent: "ses_p", Ticket: "ev-1"}
	_, ok = svc.scopeCheck(ctx, sess)
	assert.True(t, ok)
	sess.Fanout = nil
	_, ok = svc.scopeCheck(ctx, sess)
	assert.True(t, ok)
}

func TestInScope(t *testing.T) {
	for _, c := range []struct {
		p, e string
		in   bool
	}{
		{"api/a.go", "api/a.go", true}, {"api/a.go", "api", true}, {"api/a.go", "api/", true}, {"apix/a.go", "api", false},
		{"api/a_test.go", "api/*_test.go", true}, {"api/x/a.go", "api/**", true}, {"web/a.go", "api/**", false}, {"api/a.go", "api/b.go", false},
	} {
		assert.Equal(t, c.in, InScope(c.p, c.e), "%s in %s", c.p, c.e)
	}
}
