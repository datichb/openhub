package fanout

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// sweep-fanout: a fan-out of tasks. The children are the tasks oh recorded
// at cp-plan (the checkpoint state), never a list of the session; each
// child gets its task in its inputs and record, no ticket; oh closes
// nothing at the merge.
func tasksFixture(t *testing.T, tasks []domain.PlanTask) *fixture {
	t.Helper()
	f := newFixture(t) // no tickets
	f.wf.ID = "sweep"
	f.wf.Fanout.Child, f.wf.Fanout.Source, f.wf.Fanout.Delegates = "sweep-task", sessionspec.FanoutSourceTasks, []string{"cp-commit"}
	f.wf.Checkpoints[0].Tasks = &sessionspec.TasksCheckDef{Max: 12}
	f.driver.Workflow = func(context.Context, string) (*sessionspec.WorkflowRuntime, domain.CheckpointState, error) {
		st := domain.CheckpointState{Tasks: tasks}
		if f.passed {
			st.Passed = map[string]time.Time{"cp-plan": time.Now()}
		}
		return f.wf, st, nil
	}
	f.driver.Ticket = func(context.Context, string, string) ([]string, bool, error) {
		t.Error("a task has no ticket: bd is never asked")
		return nil, false, nil
	}
	p, err := f.store.Get(context.Background(), "ses_parent")
	require.NoError(t, err)
	p.Fanout.ChildInputs = map[string]string{"goal": "migrer"}
	require.NoError(t, f.store.SetSessionFanout(context.Background(), "ses_parent", p.Fanout))
	return f
}

func TestDriverTasksBecomeChildren(t *testing.T) {
	tasks := []domain.PlanTask{
		{ID: "t1", Title: "Soustraction", Description: "ajouter Sub", Files: []string{"sub.go"}, Domain: "backend"},
		{ID: "t2", Title: "Multiplication", Files: []string{"mul.go"}},
	}
	f := tasksFixture(t, tasks)
	f.launcher.work["t1"] = [2]string{"sub.go", "package calc\n\nfunc Sub(a, b int) int { return a - b }\n"}
	f.launcher.work["t2"] = [2]string{"mul.go", "package calc\n\nfunc Mul(a, b int) int { return a * b }\n"}
	fo := f.runDriver(t)
	require.Equal(t, domain.FanoutReady, fo.Status)
	require.Len(t, fo.Children, 2)
	for i, c := range fo.Children {
		assert.Equal(t, domain.ChildMerged, c.State, c.Ticket)
		require.NotNil(t, c.Task)
		assert.Equal(t, tasks[i], *c.Task)
		assert.Equal(t, ChildBranch("ses_parent", tasks[i].ID), c.Branch)
	}
	require.Len(t, f.launcher.started, 2)
	for _, r := range f.launcher.started {
		assert.Equal(t, "sweep-task", r.Workflow)
		require.NotNil(t, r.Task, "the task is given by oh")
		require.NotNil(t, r.Record.Task, "and recorded on the child (its scope)")
		assert.Equal(t, "migrer", r.Inputs["goal"], "the inputs of the parent")
		if r.Ticket == "t1" {
			assert.Equal(t, "Soustraction\n\najouter Sub", r.Inputs["task"])
			assert.Equal(t, "sub.go", r.Inputs["files"])
			assert.Equal(t, "backend", r.Inputs["domain"])
		} else {
			assert.Equal(t, "Multiplication", r.Inputs["task"])
			assert.NotContains(t, r.Inputs, "domain")
		}
	}
	require.Len(t, f.launcher.told, 1)
	assert.Contains(t, f.launcher.told[0], "| Tâche |")
	assert.Contains(t, f.launcher.told[0], "t1 Soustraction")

	in := &Integrator{Sessions: f.store, Close: func(context.Context, string, string, string) error {
		t.Error("a task has no ticket to close")
		return nil
	}, Record: func(context.Context, string, []string) { t.Error("no tickets output") }}
	p, _ := f.store.Get(context.Background(), "ses_parent")
	msg, err := in.Integrate(context.Background(), p, "cp-merge")
	require.NoError(t, err)
	assert.Contains(t, msg, "t1, t2")
	assert.FileExists(t, filepath.Join(f.project, "mul.go"))
}

// No task recorded at cp-plan (the checkpoint passed without a breakdown):
// no child, the fan-out stops on an error.
func TestDriverTasksNoneRecorded(t *testing.T) {
	f := tasksFixture(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.Error(t, f.driver.Run(ctx, "ses_parent"))
	p, _ := f.store.Get(context.Background(), "ses_parent")
	assert.Equal(t, domain.FanoutFailed, p.Fanout.Status)
	assert.Empty(t, f.launcher.started)
}
