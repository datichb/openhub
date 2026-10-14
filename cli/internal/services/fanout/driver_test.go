package fanout

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func TestMain(m *testing.M) { os.Exit(gittest.Main(m)) }

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%v: %s", args, out)
	return strings.TrimSpace(string(out))
}

func commit(t *testing.T, dir, file, content, msg string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644))
	run(t, dir, "git", "add", file)
	run(t, dir, "git", "commit", "-q", "-m", msg)
}

// quiet runs a command of a fake session (in a goroutine that may outlive
// the test: errors are ignored).
func quiet(dir string, args ...string) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x")
	_ = cmd.Run()
}

// fakeLauncher does the work of each session in its directory: a child
// commits its file (work[ticket]); a merger writes resolve and commits.
type fakeLauncher struct {
	t       *testing.T
	store   *sqlite.SessionStore
	mu      sync.Mutex
	n       int
	work    map[string][2]string // ticket → file, content
	resolve string               // merger: content of the conflicted file ("" = gives up)
	started []LaunchRequest
	told    []string
	cost    float64
	maxSeen int
	running int
	hold    time.Duration
	ended   bool // the sessions complete instead of going idle
}

func (l *fakeLauncher) Start(ctx context.Context, req LaunchRequest) (string, error) {
	l.mu.Lock()
	l.n++
	id := fmt.Sprintf("ses_%s_%d", req.Record.Role, l.n)
	l.started = append(l.started, req)
	l.running++
	l.maxSeen = max(l.maxSeen, l.running)
	l.mu.Unlock()
	require.NoError(l.t, l.store.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, State: domain.RunActive,
		LaunchPath: req.Dir, Cost: l.cost, Fanout: req.Record, ParentSessionID: req.Parent.ID}))
	go func() {
		time.Sleep(l.hold)
		switch req.Record.Role {
		case domain.RoleChild:
			if w, ok := l.work[req.Ticket]; ok {
				_ = os.WriteFile(filepath.Join(req.Dir, w[0]), []byte(w[1]), 0o644)
				quiet(req.Dir, "git", "add", w[0])
				quiet(req.Dir, "git", "commit", "-q", "-m", "feat: "+req.Ticket)
			}
		case domain.RoleMerger:
			if l.resolve != "" {
				for _, f := range strings.Fields(req.Inputs["files"]) {
					_ = os.WriteFile(filepath.Join(req.Dir, f), []byte(l.resolve), 0o644)
					quiet(req.Dir, "git", "add", f)
				}
				quiet(req.Dir, "git", "commit", "-q", "--no-edit")
			}
		}
		s, err := l.store.Get(context.Background(), id)
		if err != nil {
			return // the test is over
		}
		s.State = domain.RunIdle
		if l.ended {
			s.State = domain.RunCompleted
		}
		_ = l.store.Update(context.Background(), s)
		l.mu.Lock()
		l.running--
		l.mu.Unlock()
	}()
	return id, nil
}

func (l *fakeLauncher) Status(ctx context.Context, id string) (SessionStatus, error) {
	s, err := l.store.Get(ctx, id)
	if err != nil {
		return SessionStatus{}, err
	}
	return SessionStatus{Ended: s.State == domain.RunStopped || s.State == domain.RunCompleted, Idle: s.State == domain.RunIdle}, nil
}

func (l *fakeLauncher) Stop(ctx context.Context, id string) error {
	s, err := l.store.Get(ctx, id)
	if err != nil {
		return err
	}
	if s.State != domain.RunIdle {
		s.State = domain.RunStopped
	}
	return l.store.Update(ctx, s)
}

func (l *fakeLauncher) Tell(_ context.Context, id, text string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.told = append(l.told, id+": "+text)
	return nil
}

type fixture struct {
	store    *sqlite.SessionStore
	project  string
	launcher *fakeLauncher
	driver   *Driver
	passed   bool
	wf       *sessionspec.WorkflowRuntime
	deps     map[string][]string
	closed   map[string]bool
}

func newFixture(t *testing.T, tickets ...string) *fixture {
	t.Helper()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	root := t.TempDir()
	project := filepath.Join(root, "proj")
	require.NoError(t, os.MkdirAll(project, 0o755))
	run(t, project, "git", "init", "-q", "-b", "main")
	commit(t, project, "calc.go", "package calc\n", "init")
	f := &fixture{store: sqlite.NewSessionStore(st), project: project, passed: true, deps: map[string][]string{}, closed: map[string]bool{}}
	f.wf = &sessionspec.WorkflowRuntime{ID: "implement-spec", Checkpoints: []sessionspec.CheckpointDef{{ID: "cp-plan", Mandatory: true}, {ID: "cp-merge", Mandatory: true}},
		Fanout: &sessionspec.FanoutDef{Child: "ticket", Merge: "cp-merge", Parallel: 3, Merger: "merge-conflicts", Delegates: []string{"cp-2"}}}
	f.launcher = &fakeLauncher{t: t, store: f.store, work: map[string][2]string{}}
	f.driver = &Driver{Sessions: f.store, Launcher: f.launcher, Poll: 20 * time.Millisecond, IdleTimeout: 400 * time.Millisecond,
		Workflow: func(context.Context, string) (*sessionspec.WorkflowRuntime, domain.CheckpointState, error) {
			st := domain.CheckpointState{}
			if f.passed {
				st.Passed = map[string]time.Time{"cp-plan": time.Now()}
			}
			return f.wf, st, nil
		},
		Ticket: func(_ context.Context, _, ticket string) ([]string, bool, error) {
			return f.deps[ticket], f.closed[ticket], nil
		},
	}
	require.NoError(t, f.store.Create(context.Background(), &domain.Session{ID: "ses_parent", ProjectID: "p1", Status: domain.SessionStatusRunning, State: domain.RunIdle,
		LaunchPath: project, Tickets: tickets, Fanout: &domain.SessionFanout{Role: domain.RoleParent, Experimental: true, Status: domain.FanoutWaiting}}))
	return f
}

func (f *fixture) runDriver(t *testing.T) *domain.SessionFanout {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, f.driver.Run(ctx, "ses_parent"))
	p, err := f.store.Get(context.Background(), "ses_parent")
	require.NoError(t, err)
	return p.Fanout
}

func TestDriverMergesIndependentChildren(t *testing.T) {
	f := newFixture(t, "ev-1", "ev-2")
	f.launcher.work["ev-1"] = [2]string{"sub.go", "package calc\n\nfunc Sub(a, b int) int { return a - b }\n"}
	f.launcher.work["ev-2"] = [2]string{"mul.go", "package calc\n\nfunc Mul(a, b int) int { return a * b }\n"}
	fo := f.runDriver(t)
	require.Equal(t, domain.FanoutReady, fo.Status)
	require.Len(t, fo.Children, 2)
	for _, c := range fo.Children {
		assert.Equal(t, domain.ChildMerged, c.State, c.Ticket)
		assert.NotEmpty(t, c.Merge)
		assert.True(t, strings.HasPrefix(c.Branch, BranchPrefix))
	}
	ig := fo.Integration
	head := run(t, ig.Path, "git", "rev-parse", "HEAD")
	assert.Equal(t, head, ig.Head)
	assert.FileExists(t, filepath.Join(ig.Path, "sub.go"))
	assert.FileExists(t, filepath.Join(ig.Path, "mul.go"))
	assert.Equal(t, "2", run(t, ig.Path, "git", "rev-list", "--count", "--merges", ig.Base+"..HEAD"), "one merge commit per child")
	require.Len(t, f.launcher.told, 1, "the parent is told once")
	assert.Contains(t, f.launcher.told[0], "ev-1")
	assert.Contains(t, f.launcher.told[0], "cp-merge")
	// The children were launched with their record (child of the parent).
	for _, r := range f.launcher.started {
		assert.Equal(t, domain.RoleChild, r.Record.Role)
		assert.Equal(t, "ses_parent", r.Record.Parent)
		assert.Equal(t, "ticket", r.Workflow)
	}
	// The project directory is untouched until the merge checkpoint.
	assert.NoFileExists(t, filepath.Join(f.project, "sub.go"))
	kinds := map[string]int{}
	for _, e := range fo.Events {
		kinds[e.Kind]++
	}
	assert.Equal(t, 2, kinds[domain.FanoutEventMerged])
	assert.Equal(t, 1, kinds[domain.FanoutEventReady])

	// The merge by oh, at the merge checkpoint: fast forward, tickets closed.
	var closed []string
	in := &Integrator{Sessions: f.store, Close: func(_ context.Context, dir, ticket, reason string) error {
		assert.Equal(t, f.project, dir)
		assert.Contains(t, reason, "Merged by oh")
		closed = append(closed, ticket)
		return nil
	}, Record: func(_ context.Context, id string, tickets []string) {
		assert.Equal(t, []string{"ev-1", "ev-2"}, tickets)
	}}
	p, _ := f.store.Get(context.Background(), "ses_parent")
	msg, err := in.Integrate(context.Background(), p, "cp-merge")
	require.NoError(t, err)
	assert.Contains(t, msg, "ev-1, ev-2")
	assert.Equal(t, []string{"ev-1", "ev-2"}, closed)
	assert.FileExists(t, filepath.Join(f.project, "sub.go"))
	assert.Equal(t, head, run(t, f.project, "git", "rev-parse", "HEAD"), "fast forward")
	p, _ = f.store.Get(context.Background(), "ses_parent")
	assert.Equal(t, domain.FanoutMerged, p.Fanout.Status)
}

func TestDriverParallelCap(t *testing.T) {
	f := newFixture(t, "ev-1", "ev-2", "ev-3", "ev-4")
	for i := 1; i <= 4; i++ {
		f.launcher.work[fmt.Sprintf("ev-%d", i)] = [2]string{fmt.Sprintf("f%d.go", i), "package calc\n"}
	}
	f.launcher.hold = 150 * time.Millisecond
	f.driver.Parallel = 2
	fo := f.runDriver(t)
	assert.Equal(t, domain.FanoutReady, fo.Status)
	assert.LessOrEqual(t, f.launcher.maxSeen, 2)
	assert.GreaterOrEqual(t, f.launcher.maxSeen, 2, "children run at the same time")

	g := newFixture(t, "ev-1", "ev-2")
	g.launcher.work["ev-1"] = [2]string{"a.go", "package calc\n"}
	g.launcher.work["ev-2"] = [2]string{"b.go", "package calc\n"}
	g.launcher.hold = 100 * time.Millisecond
	g.wf.Fanout.Parallel = 1
	g.runDriver(t)
	assert.Equal(t, 1, g.launcher.maxSeen, "sequential")
}

func TestDriverDependencies(t *testing.T) {
	f := newFixture(t, "ev-2", "ev-1")
	f.deps["ev-2"] = []string{"ev-1"}
	f.launcher.work["ev-1"] = [2]string{"a.go", "package calc\n"}
	f.launcher.work["ev-2"] = [2]string{"b.go", "package calc\n"}
	fo := f.runDriver(t)
	require.Equal(t, domain.FanoutReady, fo.Status)
	require.Len(t, f.launcher.started, 2)
	assert.Equal(t, "ev-1", f.launcher.started[0].Ticket, "the blocker first")
	c, _ := fo.Child("ev-2")
	assert.True(t, IsAncestor(context.Background(), fo.Integration.Path, fo.Children[1].Merge, c.Merge) || c.State == domain.ChildMerged)
}

// Two children started from the same integration head (in parallel) both
// change calc.go: the second merge conflicts.
func TestDriverConflictResolvedByMerger(t *testing.T) {
	f := newFixture(t, "ev-1", "ev-2")
	f.launcher.hold = 150 * time.Millisecond
	f.launcher.work["ev-1"] = [2]string{"calc.go", "package calc\n\nfunc Sub() {}\n"}
	f.launcher.work["ev-2"] = [2]string{"calc.go", "package calc\n\nfunc Mul() {}\n"}
	f.launcher.resolve = "package calc\n\nfunc Sub() {}\n\nfunc Mul() {}\n"
	fo := f.runDriver(t)
	require.Equal(t, domain.FanoutReady, fo.Status)
	c := conflicted(t, fo)
	assert.Equal(t, domain.ChildMerged, c.State)
	assert.Equal(t, []string{"calc.go"}, c.Conflicts)
	assert.NotEmpty(t, c.Merger)
	data, _ := os.ReadFile(filepath.Join(fo.Integration.Path, "calc.go"))
	assert.Contains(t, string(data), "Mul")
	assert.Contains(t, string(data), "Sub")
	last := f.launcher.started[len(f.launcher.started)-1]
	assert.Equal(t, domain.RoleMerger, last.Record.Role)
	assert.Equal(t, "merge-conflicts", last.Workflow)
	assert.Equal(t, fo.Integration.Path, last.Dir)
}

func TestDriverConflictUnresolved(t *testing.T) {
	f := newFixture(t, "ev-1", "ev-2")
	f.launcher.hold = 150 * time.Millisecond
	f.launcher.work["ev-1"] = [2]string{"calc.go", "package calc\n\nfunc Sub() {}\n"}
	f.launcher.work["ev-2"] = [2]string{"calc.go", "package calc\n\nfunc Mul() {}\n"}
	f.launcher.resolve = "" // the merger gives up
	fo := f.runDriver(t)
	c := conflicted(t, fo)
	assert.Equal(t, domain.ChildConflict, c.State)
	assert.False(t, MergeInProgress(context.Background(), fo.Integration.Path), "integration left clean")
	head := run(t, fo.Integration.Path, "git", "rev-parse", "HEAD")
	assert.Equal(t, fo.Integration.Head, head)
	assert.Equal(t, domain.FanoutReady, fo.Status, "the merged children can still be validated")

	// A merger that commits markers is refused too.
	g := newFixture(t, "ev-1", "ev-2")
	g.launcher.hold = 150 * time.Millisecond
	g.launcher.work = f.launcher.work
	g.launcher.resolve = "<<<<<<< HEAD\nx\n=======\ny\n>>>>>>> b\n"
	fo = g.runDriver(t)
	c = conflicted(t, fo)
	assert.Equal(t, domain.ChildConflict, c.State)
	assert.NotContains(t, readFile(t, filepath.Join(fo.Integration.Path, "calc.go")), "<<<<<<<")
}

// conflicted is the child whose merge conflicted (the second one merged).
func conflicted(t *testing.T, fo *domain.SessionFanout) domain.FanoutChild {
	t.Helper()
	for _, c := range fo.Children {
		if len(c.Conflicts) > 0 {
			return c
		}
	}
	t.Fatalf("no conflict: %+v", fo.Children)
	return domain.FanoutChild{}
}

func readFile(t *testing.T, p string) string {
	data, err := os.ReadFile(p)
	require.NoError(t, err)
	return string(data)
}

func TestDriverSkipsClosedTicket(t *testing.T) {
	f := newFixture(t, "ev-1", "ev-2")
	f.closed["ev-1"] = true
	f.launcher.work["ev-2"] = [2]string{"b.go", "package calc\n"}
	fo := f.runDriver(t)
	c, _ := fo.Child("ev-1")
	assert.Equal(t, domain.ChildFailed, c.State)
	assert.Empty(t, c.Session, "no child for a closed ticket")
	require.Len(t, f.launcher.started, 1)
	assert.Equal(t, "ev-2", f.launcher.started[0].Ticket)
}

func TestDriverChildWithoutCommit(t *testing.T) {
	f := newFixture(t, "ev-1")
	fo := f.runDriver(t)
	c, _ := fo.Child("ev-1")
	assert.Equal(t, domain.ChildFailed, c.State)
	assert.Equal(t, domain.FanoutReady, fo.Status)
}

func TestDriverGlobalBudget(t *testing.T) {
	f := newFixture(t, "ev-1", "ev-2")
	f.wf.Fanout.Parallel = 1
	f.launcher.work["ev-1"] = [2]string{"a.go", "package calc\n"}
	f.launcher.work["ev-2"] = [2]string{"b.go", "package calc\n"}
	f.launcher.cost = 0.8
	p, _ := f.store.Get(context.Background(), "ses_parent")
	p.Fanout.BudgetUSD = 0.5
	require.NoError(t, f.store.SetSessionFanout(context.Background(), p.ID, p.Fanout))
	fo := f.runDriver(t)
	first, _ := fo.Child("ev-1")
	second, _ := fo.Child("ev-2")
	assert.Equal(t, domain.ChildBudget, second.State, "not launched once the global budget is spent")
	assert.Contains(t, []domain.FanoutChildState{domain.ChildBudget, domain.ChildMerged}, first.State)
	assert.InDelta(t, 0.5, f.launcher.started[0].BudgetUSD, 1e-9, "the child gets what remains of the budget")
	assert.Len(t, f.launcher.started, 1)
	var budget bool
	for _, e := range fo.Events {
		budget = budget || e.Kind == domain.FanoutEventBudget
	}
	assert.True(t, budget, "traced")
	assert.Contains(t, f.launcher.told[0], "0.50")
}

func TestDriverTamperedIntegration(t *testing.T) {
	f := newFixture(t, "ev-1", "ev-2")
	f.wf.Fanout.Parallel = 1
	f.launcher.work["ev-1"] = [2]string{"a.go", "package calc\n"}
	f.launcher.work["ev-2"] = [2]string{"b.go", "package calc\n"}
	// The first child also moves the integration branch by hand.
	orig := f.launcher.work
	f.launcher.work = map[string][2]string{"ev-1": orig["ev-1"]}
	go func() {
		for {
			p, err := f.store.Get(context.Background(), "ses_parent")
			if err == nil && p.Fanout.Integration != nil {
				if c, ok := p.Fanout.Child("ev-1"); ok && c.Session != "" {
					commit(t, p.Fanout.Integration.Path, "evil.go", "package calc\n", "evil")
					return
				}
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
	f.launcher.hold = 200 * time.Millisecond
	fo := f.runDriver(t)
	assert.Equal(t, domain.FanoutFailed, fo.Status)
	var tampered bool
	for _, e := range fo.Events {
		tampered = tampered || e.Kind == domain.FanoutEventTampered
	}
	assert.True(t, tampered)

	// And the merge checkpoint refuses it as well.
	p, _ := f.store.Get(context.Background(), "ses_parent")
	p.Fanout.Status = domain.FanoutReady
	_, err := (&Integrator{Sessions: f.store}).Integrate(context.Background(), p, "cp-merge")
	assert.Error(t, err)
}

func TestDriverRefusals(t *testing.T) {
	f := newFixture(t, "ev-1")
	p, _ := f.store.Get(context.Background(), "ses_parent")
	p.Fanout.Experimental = false
	require.NoError(t, f.store.SetSessionFanout(context.Background(), p.ID, p.Fanout))
	assert.ErrorIs(t, f.driver.Run(context.Background(), "ses_parent"), ErrDisabled)
	require.NoError(t, f.store.SetSessionFanout(context.Background(), p.ID, nil))
	assert.ErrorIs(t, f.driver.Run(context.Background(), "ses_parent"), ErrNotFanout)

	// The parent ends before cp-plan: the fan-out stops, nothing launched.
	g := newFixture(t, "ev-1")
	g.passed = false
	q, _ := g.store.Get(context.Background(), "ses_parent")
	q.State = domain.RunCompleted
	require.NoError(t, g.store.Update(context.Background(), q))
	fo := g.runDriver(t)
	assert.Equal(t, domain.FanoutStopped, fo.Status)
	assert.Empty(t, g.launcher.started)
}

func TestIntegrateRefusesDirtyParent(t *testing.T) {
	f := newFixture(t, "ev-1")
	f.launcher.work["ev-1"] = [2]string{"a.go", "package calc\n"}
	f.runDriver(t)
	require.NoError(t, os.WriteFile(filepath.Join(f.project, "calc.go"), []byte("package calc // edited\n"), 0o644))
	p, _ := f.store.Get(context.Background(), "ses_parent")
	_, err := (&Integrator{Sessions: f.store, Close: func(context.Context, string, string, string) error { return nil }}).Integrate(context.Background(), p, "cp-merge")
	assert.Error(t, err)
}

func TestHasMarkers(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a"), []byte("x\n<<<<<<< HEAD\ny\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b"), []byte("x // <<<<<<< not at the start\n"), 0o644))
	assert.True(t, HasMarkers(dir, []string{"a"}))
	assert.False(t, HasMarkers(dir, []string{"b", "missing"}))
}

// doneReason is the reason of the child_done event of a ticket.
func doneReason(fo *domain.SessionFanout, ticket string) string {
	for _, e := range fo.Events {
		if e.Kind == domain.FanoutEventChildDone && e.Ticket == ticket {
			return strings.SplitN(e.Message, ",", 2)[0]
		}
	}
	return ""
}

// The end of a child is its end of work seen by oh, not its idleness: a
// child idle with a commit but whose commit checkpoint did not pass is
// only stopped by the guard; once it passed, the signal ends it at once.
func TestDriverChildEndSignal(t *testing.T) {
	childWf := &sessionspec.WorkflowRuntime{ID: "ticket", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-2", Mandatory: true, Unlocks: []string{sessionspec.UnlockCommit}}}}
	for _, tc := range []struct {
		name   string
		passed bool
		work   bool
		want   string
	}{
		{"signal", true, true, DoneSignal},
		{"checkpoint not passed", false, true, DoneIdle},
		{"no commit", true, false, DoneIdle},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, "ev-1")
			f.driver.IdleTimeout = 2 * time.Second
			if tc.work {
				f.launcher.work["ev-1"] = [2]string{"a.go", "package calc\n"}
			}
			parentWf := f.wf
			f.driver.Workflow = func(_ context.Context, id string) (*sessionspec.WorkflowRuntime, domain.CheckpointState, error) {
				st := domain.CheckpointState{Passed: map[string]time.Time{"cp-plan": time.Now()}}
				if id == "ses_parent" {
					return parentWf, st, nil
				}
				if tc.passed {
					st.Passed["cp-2"] = time.Now()
				}
				return childWf, st, nil
			}
			start := time.Now()
			fo := f.runDriver(t)
			assert.Equal(t, tc.want, doneReason(fo, "ev-1"))
			if tc.want == DoneSignal {
				assert.Less(t, time.Since(start), 1500*time.Millisecond, "no wait for the guard")
			} else {
				assert.GreaterOrEqual(t, time.Since(start), 2*time.Second, "stopped by the guard only")
			}
		})
	}
}

// A session that ends is done whatever its work.
func TestDriverChildEnded(t *testing.T) {
	f := newFixture(t, "ev-1")
	f.driver.IdleTimeout = time.Hour
	f.launcher.ended = true
	fo := f.runDriver(t)
	assert.Equal(t, DoneEnded, doneReason(fo, "ev-1"))
}
