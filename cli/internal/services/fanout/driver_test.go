package fanout

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

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
	costOf  map[string]float64 // ticket → cost of its child, instead of cost
	maxSeen int
	running int
	hold    time.Duration
	ended   bool // the sessions complete instead of going idle
	// together: a child does its work once that many sessions started
	// (children at the same time, by condition, not by hold).
	together int
	all      chan struct{}
	// onWork runs in a session before its work (nil = none).
	onWork func(LaunchRequest)
	// fix is what a fixer commits (file, content); fixMerge: it commits a
	// merge instead.
	fix      [2]string
	fixMerge bool
	// onTell runs when the parent is told (nil = nothing), out of the lock.
	onTell func()
}

func (l *fakeLauncher) Start(ctx context.Context, req LaunchRequest) (string, error) {
	l.mu.Lock()
	l.n++
	id := fmt.Sprintf("ses_%s_%d", req.Record.Role, l.n)
	l.started = append(l.started, req)
	l.running++
	l.maxSeen = max(l.maxSeen, l.running)
	if l.all == nil {
		l.all = make(chan struct{})
	}
	if l.n == l.together {
		close(l.all)
	}
	all := l.all
	cost := l.cost
	if c, ok := l.costOf[req.Ticket]; ok && req.Record.Role == domain.RoleChild {
		cost = c
	}
	l.mu.Unlock()
	require.NoError(l.t, l.store.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, State: domain.RunActive,
		LaunchPath: req.Dir, Cost: cost, Fanout: req.Record, ParentSessionID: req.Parent.ID}))
	go func() {
		if l.together > 0 && req.Record.Role == domain.RoleChild {
			select {
			case <-all:
			case <-time.After(runGuard):
			}
		}
		if l.onWork != nil {
			l.onWork(req)
		}
		time.Sleep(l.hold)
		switch req.Record.Role {
		case domain.RoleChild:
			if w, ok := l.work[req.Ticket]; ok {
				_ = os.WriteFile(filepath.Join(req.Dir, w[0]), []byte(w[1]), 0o644)
				quiet(req.Dir, "git", "add", w[0])
				quiet(req.Dir, "git", "commit", "-q", "-m", "feat: "+req.Ticket)
			}
		case domain.RoleFixer:
			if l.fixMerge {
				quiet(req.Dir, "git", "checkout", "-q", "-b", "side")
				_ = os.WriteFile(filepath.Join(req.Dir, "side.go"), []byte("package calc\n"), 0o644)
				quiet(req.Dir, "git", "add", "side.go")
				quiet(req.Dir, "git", "commit", "-q", "-m", "side")
				quiet(req.Dir, "git", "checkout", "-q", req.Record.Branch)
				quiet(req.Dir, "git", "merge", "-q", "--no-ff", "-m", "merge side", "side")
			} else if l.fix[0] != "" {
				_ = os.WriteFile(filepath.Join(req.Dir, l.fix[0]), []byte(l.fix[1]), 0o644)
				quiet(req.Dir, "git", "add", l.fix[0])
				quiet(req.Dir, "git", "commit", "-q", "-m", "fix: "+req.Inputs["request"])
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
	s.State = domain.RunStopped
	return l.store.Update(ctx, s)
}

func (l *fakeLauncher) Tell(_ context.Context, id, text string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.told = append(l.told, id+": "+text)
	if l.onTell != nil {
		l.mu.Unlock()
		l.onTell()
		l.mu.Lock()
	}
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
	st := sqlitetest.New(t)
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	project := newProject(t)
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
	fo, err := f.runDriverErr()
	require.NoError(t, err)
	return fo
}

// runGuard bounds a test that waits for a driver, a status or a goroutine of
// a fake. It is a guard against a hang, not a duration a test expects: each
// one returns as soon as the driver ends, so a loaded machine slows the test
// down without failing it.
const runGuard = 2 * time.Minute

// runDriverErr runs the driver to its end and returns the record of the
// parent; usable out of the test goroutine.
func (f *fixture) runDriverErr() (*domain.SessionFanout, error) {
	ctx, cancel := context.WithTimeout(context.Background(), runGuard)
	defer cancel()
	if err := f.driver.Run(ctx, "ses_parent"); err != nil {
		return nil, err
	}
	p, err := f.store.Get(context.Background(), "ses_parent")
	if err != nil {
		return nil, err
	}
	return p.Fanout, nil
}

func TestDriverMergesIndependentChildren(t *testing.T) {
	t.Parallel()
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
	head := gittest.Run(t, ig.Path, "rev-parse", "HEAD")
	assert.Equal(t, head, ig.Head)
	assert.FileExists(t, filepath.Join(ig.Path, "sub.go"))
	assert.FileExists(t, filepath.Join(ig.Path, "mul.go"))
	assert.Equal(t, "2", gittest.Run(t, ig.Path, "rev-list", "--count", "--merges", ig.Base+"..HEAD"), "one merge commit per child")
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
	assert.Equal(t, head, gittest.Run(t, f.project, "rev-parse", "HEAD"), "fast forward")
	p, _ = f.store.Get(context.Background(), "ses_parent")
	assert.Equal(t, domain.FanoutMerged, p.Fanout.Status)
}

// stopStore stops the driver (cancel) as the merge of ev-1 is recorded, and
// fails once the write of the end of work of ev-1.
type stopStore struct {
	Store
	cancel   context.CancelFunc
	stopped  atomic.Bool
	failed   atomic.Bool
	errWrite error
}

func (s *stopStore) SetSessionFanout(ctx context.Context, id string, fo *domain.SessionFanout) error {
	if c, ok := fo.Child("ev-1"); ok && c.State == domain.ChildMerged && s.stopped.CompareAndSwap(false, true) {
		s.cancel()
	}
	if n := len(fo.Events); n > 0 && fo.Events[n-1].Kind == domain.FanoutEventChildDone && fo.Events[n-1].Ticket == "ev-1" && s.failed.CompareAndSwap(false, true) {
		return s.errWrite
	}
	return s.Store.SetSessionFanout(ctx, id, fo)
}

// The driver stopped right after git merged a child: the merge is still
// recorded, so the driver started again goes on instead of finding the
// integration branch « modified outside oh ».
func TestDriverRecordSurvivesStopAfterMerge(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "ev-1", "ev-2")
	f.launcher.work["ev-1"] = [2]string{"sub.go", "package calc\n\nfunc Sub(a, b int) int { return a - b }\n"}
	f.launcher.work["ev-2"] = [2]string{"mul.go", "package calc\n\nfunc Mul(a, b int) int { return a * b }\n"}
	f.wf.Fanout.Parallel = 1
	ctx, cancel := context.WithTimeout(context.Background(), runGuard)
	defer cancel()
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	errWrite := errors.New("database is locked")
	f.driver.Sessions = &stopStore{Store: f.store, cancel: stop, errWrite: errWrite}
	var logMu sync.Mutex
	var journal []string
	f.driver.Log = func(format string, args ...any) {
		logMu.Lock()
		defer logMu.Unlock()
		journal = append(journal, fmt.Sprintf(format, args...))
	}

	err := f.driver.Run(ctx, "ses_parent")
	require.ErrorIs(t, err, context.Canceled, "the first driver stops with its context")
	p, err := f.store.Get(context.Background(), "ses_parent")
	require.NoError(t, err)
	ig := p.Fanout.Integration
	require.NotNil(t, ig)
	c, _ := p.Fanout.Child("ev-1")
	require.NotNil(t, c)
	assert.Equal(t, domain.ChildMerged, c.State, "the merge of ev-1 is recorded though the driver was stopped")
	assert.Equal(t, gittest.Run(t, ig.Path, "rev-parse", "HEAD"), ig.Head, "the recorded head is the one of git")

	fo := f.runDriver(t)
	require.Equal(t, domain.FanoutReady, fo.Status, "the driver started again goes on: %+v", fo.Events)
	require.Len(t, fo.Children, 2)
	for _, c := range fo.Children {
		assert.Equal(t, domain.ChildMerged, c.State, "%s: %s", c.Ticket, c.Error)
	}
	for _, e := range fo.Events {
		assert.NotEqual(t, domain.FanoutEventTampered, e.Kind, "the integration branch is not seen as modified: %s", e.Message)
	}
	ig = fo.Integration
	assert.Equal(t, gittest.Run(t, ig.Path, "rev-parse", "HEAD"), ig.Head)
	assert.Equal(t, "2", gittest.Run(t, ig.Path, "rev-list", "--count", "--merges", ig.Base+"..HEAD"), "one merge commit per child")
	require.Len(t, f.launcher.told, 1, "the parent is told once")
	logMu.Lock()
	defer logMu.Unlock()
	assert.Contains(t, strings.Join(journal, "\n"), errWrite.Error(), "the write that failed is in the journal")
	assert.NoFileExists(t, filepath.Join(f.project, "sub.go"), "the project directory is untouched")
}

// failStore fails once the write of the first record fail accepts.
type failStore struct {
	Store
	fail     func(fo *domain.SessionFanout) bool
	failed   atomic.Bool
	errWrite error
}

func (s *failStore) SetSessionFanout(ctx context.Context, id string, fo *domain.SessionFanout) error {
	if s.fail(fo) && s.failed.CompareAndSwap(false, true) {
		return s.errWrite
	}
	return s.Store.SetSessionFanout(ctx, id, fo)
}

// stopRecorder keeps the sessions the driver stopped.
type stopRecorder struct {
	Launcher
	mu      sync.Mutex
	stopped []string
}

func (r *stopRecorder) Stop(ctx context.Context, id string) error {
	r.mu.Lock()
	r.stopped = append(r.stopped, id)
	r.mu.Unlock()
	return r.Launcher.Stop(ctx, id)
}

func (r *stopRecorder) stoppedIDs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.stopped...)
}

// A record the driver cannot write, once git moved or a session started,
// stops the driver: it goes on from a record that is behind git otherwise.
func TestDriverStopsOnACriticalRecordItCannotWrite(t *testing.T) {
	t.Parallel()
	conflict := func(f *fixture) {
		f.launcher.together = 2
		f.launcher.work["ev-1"] = [2]string{"calc.go", "package calc\n\nfunc Sub() {}\n"}
		f.launcher.work["ev-2"] = [2]string{"calc.go", "package calc\n\nfunc Mul() {}\n"}
	}
	lastFix := func(fo *domain.SessionFanout) (x domain.FanoutFix, ok bool) {
		if f, found := fo.LastFix(); found {
			return *f, true
		}
		return x, false
	}
	// outcome is what the driver left once it returned.
	type outcome struct {
		f        *fixture
		launcher *stopRecorder
		stored   *domain.SessionFanout
	}
	notInProgress := func(t *testing.T, o outcome) {
		assert.False(t, MergeInProgress(context.Background(), o.stored.Integration.Path), "no merge left in progress in the integration worktree")
	}
	for _, tc := range []struct {
		name    string
		tickets []string
		setup   func(f *fixture)
		fix     bool // the failure is in a fix round, asked once the children are merged
		fail    func(fo *domain.SessionFanout) bool
		check   func(t *testing.T, o outcome)
	}{
		{"merge recorded", []string{"ev-1"},
			func(f *fixture) { f.launcher.work["ev-1"] = [2]string{"sub.go", "package calc\n"} },
			false,
			func(fo *domain.SessionFanout) bool {
				c, ok := fo.Child("ev-1")
				return ok && c.State == domain.ChildMerged
			},
			func(t *testing.T, o outcome) {
				ig := o.stored.Integration
				c, _ := o.stored.Child("ev-1")
				assert.NotEqual(t, domain.ChildMerged, c.State, "the child is not recorded as merged")
				assert.Equal(t, "1", gittest.Run(t, ig.Path, "rev-list", "--count", "--merges", ig.Base+"..HEAD"), "git did merge the child")
				assert.NotEqual(t, ig.Head, gittest.Run(t, ig.Path, "rev-parse", "HEAD"), "the record is the one behind git")
				notInProgress(t, o)
			}},
		{"conflict left unresolved", []string{"ev-1", "ev-2"},
			func(f *fixture) { conflict(f); f.wf.Fanout.Merger = "" },
			false,
			func(fo *domain.SessionFanout) bool {
				for _, c := range fo.Children {
					if c.State == domain.ChildConflict {
						return true
					}
				}
				return false
			},
			func(t *testing.T, o outcome) {
				ig := o.stored.Integration
				assert.Equal(t, ig.Head, gittest.Run(t, ig.Path, "rev-parse", "HEAD"), "the merge was undone: git and the record agree")
				assert.NotContains(t, readFile(t, filepath.Join(ig.Path, "calc.go")), "<<<<<<<", "no marker left")
				notInProgress(t, o)
			}},
		{"merger started", []string{"ev-1", "ev-2"},
			conflict,
			false,
			func(fo *domain.SessionFanout) bool {
				for _, c := range fo.Children {
					if c.Merger != "" {
						return true
					}
				}
				return false
			},
			func(t *testing.T, o outcome) {
				ig := o.stored.Integration
				var merger string
				for i, r := range o.f.launcher.started {
					if r.Record.Role == domain.RoleMerger {
						merger = fmt.Sprintf("ses_merger_%d", i+1)
					}
				}
				require.NotEmpty(t, merger, "the merger was started")
				assert.Contains(t, o.launcher.stoppedIDs(), merger, "the merger no one follows is stopped")
				assert.Equal(t, ig.Head, gittest.Run(t, ig.Path, "rev-parse", "HEAD"), "the merge was undone: git and the record agree")
				notInProgress(t, o)
			}},
		{"parent stopped", []string{"ev-1"},
			func(f *fixture) {
				p, err := f.store.Get(context.Background(), "ses_parent")
				require.NoError(t, err)
				p.State = domain.RunStopped
				require.NoError(t, f.store.Update(context.Background(), p))
			},
			false,
			func(fo *domain.SessionFanout) bool { return fo.Status == domain.FanoutStopped },
			func(t *testing.T, o outcome) {
				assert.Equal(t, domain.FanoutRunning, o.stored.Status, "the fan-out is not recorded as stopped")
				assert.Empty(t, o.f.launcher.started, "no child started")
				notInProgress(t, o)
			}},
		{"fixer about to start", []string{"ev-1"},
			func(f *fixture) {
				f.launcher.work["ev-1"] = [2]string{"a.go", "package calc\n"}
				f.wf.Fanout.Fix, f.wf.Fanout.FixInput = "quick", "request"
				f.wf.MaxFixRounds = 2
			},
			true,
			func(fo *domain.SessionFanout) bool { x, ok := lastFix(fo); return ok && x.State == domain.ChildRunning },
			func(t *testing.T, o outcome) {
				ig := o.stored.Integration
				assert.Len(t, o.f.launcher.started, 1, "only the child was started: no fixer")
				assert.Equal(t, ig.Head, gittest.Run(t, ig.Path, "rev-parse", "HEAD"), "the integration branch did not move")
				x, _ := o.stored.LastFix()
				assert.Equal(t, domain.ChildPending, x.State, "the round is not recorded as running")
				notInProgress(t, o)
			}},
		{"fixer started", []string{"ev-1"},
			func(f *fixture) {
				f.launcher.work["ev-1"] = [2]string{"a.go", "package calc\n"}
				f.wf.Fanout.Fix, f.wf.Fanout.FixInput = "quick", "request"
				f.wf.MaxFixRounds = 2
			},
			true,
			func(fo *domain.SessionFanout) bool { x, ok := lastFix(fo); return ok && x.Session != "" },
			func(t *testing.T, o outcome) {
				ig := o.stored.Integration
				require.Len(t, o.f.launcher.started, 2, "the child, then the fixer")
				assert.Equal(t, domain.RoleFixer, o.f.launcher.started[1].Record.Role)
				assert.Contains(t, o.launcher.stoppedIDs(), "ses_fixer_2", "the fixer no one follows is stopped")
				x, _ := o.stored.LastFix()
				assert.Empty(t, x.Session, "the fixer is not in the record")
				assert.Equal(t, ig.Head, gittest.Run(t, ig.Path, "rev-parse", "HEAD"), "the integration branch did not move")
				notInProgress(t, o)
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, tc.tickets...)
			tc.setup(f)
			errWrite := errors.New("database is locked")
			f.driver.Sessions = &failStore{Store: f.store, fail: tc.fail, errWrite: errWrite}
			rec := &stopRecorder{Launcher: f.launcher}
			f.driver.Launcher = rec
			var logMu sync.Mutex
			var journal []string
			f.driver.Log = func(format string, args ...any) {
				logMu.Lock()
				defer logMu.Unlock()
				journal = append(journal, fmt.Sprintf(format, args...))
			}

			ctx, cancel := context.WithTimeout(context.Background(), runGuard)
			defer cancel()
			var err error
			if tc.fix {
				done := make(chan error, 1)
				go func() { done <- f.driver.Run(ctx, "ses_parent") }()
				f.waitStatus(t, domain.FanoutReady)
				require.NoError(t, f.requestFix(t, "fix it"))
				select {
				case err = <-done:
				case <-ctx.Done():
					t.Fatal("the driver did not stop")
				}
			} else {
				err = f.driver.Run(ctx, "ses_parent")
			}
			require.Error(t, err, "the driver stops")
			assert.ErrorIs(t, err, errWrite, "with the error of the write")
			assert.ErrorIs(t, err, errRecord, "as a record it could not write")
			p, gerr := f.store.Get(context.Background(), "ses_parent")
			require.NoError(t, gerr)
			logMu.Lock()
			defer logMu.Unlock()
			assert.Contains(t, strings.Join(journal, "\n"), "record not written: "+errWrite.Error(), "the write that failed is in the journal")
			tc.check(t, outcome{f: f, launcher: rec, stored: p.Fanout})
		})
	}
}

func TestDriverParallelCap(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "ev-1", "ev-2", "ev-3", "ev-4")
	for i := 1; i <= 4; i++ {
		f.launcher.work[fmt.Sprintf("ev-%d", i)] = [2]string{fmt.Sprintf("f%d.go", i), "package calc\n"}
	}
	f.launcher.hold = 150 * time.Millisecond
	f.launcher.together = 2
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
	t.Parallel()
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
	t.Parallel()
	f := newFixture(t, "ev-1", "ev-2")
	f.launcher.together = 2
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
	t.Parallel()
	f := newFixture(t, "ev-1", "ev-2")
	f.launcher.together = 2
	f.launcher.work["ev-1"] = [2]string{"calc.go", "package calc\n\nfunc Sub() {}\n"}
	f.launcher.work["ev-2"] = [2]string{"calc.go", "package calc\n\nfunc Mul() {}\n"}
	f.launcher.resolve = "" // the merger gives up
	fo := f.runDriver(t)
	c := conflicted(t, fo)
	assert.Equal(t, domain.ChildConflict, c.State)
	assert.False(t, MergeInProgress(context.Background(), fo.Integration.Path), "integration left clean")
	head := gittest.Run(t, fo.Integration.Path, "rev-parse", "HEAD")
	assert.Equal(t, fo.Integration.Head, head)
	assert.Equal(t, domain.FanoutReady, fo.Status, "the merged children can still be validated")

	// A merger that commits markers is refused too.
	g := newFixture(t, "ev-1", "ev-2")
	g.launcher.together = 2
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
	t.Parallel()
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

// How a lone child ends when it did not signal its end of work: failed when
// it has not committed, put aside after the idle timeout, or ended with its
// session.
func TestDriverChildOutcome(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		setup func(f *fixture)
		check func(t *testing.T, fo *domain.SessionFanout)
	}{
		{"no commit", func(*fixture) {}, func(t *testing.T, fo *domain.SessionFanout) {
			c, _ := fo.Child("ev-1")
			assert.Equal(t, domain.ChildFailed, c.State)
			assert.Equal(t, domain.FanoutReady, fo.Status)
		}},
		{"idle past the timeout", func(f *fixture) { f.driver.IdleTimeout = 50 * time.Millisecond }, func(t *testing.T, fo *domain.SessionFanout) {
			assert.Equal(t, DoneIdle, doneReason(fo, "ev-1"))
		}},
		{"session ended", func(f *fixture) {
			f.driver.IdleTimeout = time.Hour
			f.launcher.ended = true
		}, func(t *testing.T, fo *domain.SessionFanout) {
			assert.Equal(t, DoneEnded, doneReason(fo, "ev-1"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, "ev-1")
			tc.setup(f)
			tc.check(t, f.runDriver(t))
		})
	}
}

// slowStore is a store slow to read the parent: the driver works on a
// record read a while before (a loaded machine).
type slowStore struct {
	Store
	delay time.Duration
}

func (s slowStore) Get(ctx context.Context, id string) (*domain.Session, error) {
	p, err := s.Store.Get(ctx, id)
	if id == "ses_parent" {
		time.Sleep(s.delay)
	}
	return p, err
}

// A child seen running in a record read before its follower ended is not
// followed (and merged) a second time (p8-fixes: TestDriverConflictResolvedByMerger,
// a merged child marked failed, « no commit on the branch of the child »).
func TestDriverChildFollowedOnce(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "ev-1", "ev-2")
	f.driver.Sessions = slowStore{Store: f.store, delay: 30 * time.Millisecond}
	f.launcher.together = 2
	f.launcher.work["ev-1"] = [2]string{"calc.go", "package calc\n\nfunc Sub() {}\n"}
	f.launcher.work["ev-2"] = [2]string{"calc.go", "package calc\n\nfunc Mul() {}\n"}
	f.launcher.resolve = "package calc\n\nfunc Sub() {}\n\nfunc Mul() {}\n"
	fo := f.runDriver(t)
	require.Equal(t, domain.FanoutReady, fo.Status)
	for _, c := range fo.Children {
		assert.Equal(t, domain.ChildMerged, c.State, "%s: %s", c.Ticket, c.Error)
	}
	done := map[string]int{}
	for _, e := range fo.Events {
		if e.Kind == domain.FanoutEventChildDone && e.Session != "" && !strings.HasPrefix(e.Session, "ses_merger") {
			done[e.Session]++
		}
	}
	for s, n := range done {
		assert.Equal(t, 1, n, "%s followed once: %+v", s, fo.Events)
	}

	// Deterministic: the record says merged, the stale copy running.
	p, err := f.store.Get(context.Background(), "ses_parent")
	require.NoError(t, err)
	stale := p.Fanout.Children[0]
	stale.State = domain.ChildRunning
	_, ok := f.driver.resumable(context.Background(), "ses_parent", stale)
	assert.False(t, ok, "merged since: not followed again")
	_, err = f.driver.update(context.Background(), "ses_parent", func(fo *domain.SessionFanout) { fo.Children[0].State = domain.ChildRunning })
	require.NoError(t, err)
	c, ok := f.driver.resumable(context.Background(), "ses_parent", stale)
	assert.True(t, ok, "still running (launched by a previous driver): followed")
	assert.Equal(t, stale.Session, c.Session)
}

func TestDriverGlobalBudget(t *testing.T) {
	t.Parallel()
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
	assert.InDelta(t, 0.25, f.launcher.started[0].BudgetUSD, 1e-9, "the child gets its share of what remains (two children left: half)")
	assert.Len(t, f.launcher.started, 1)
	var budget bool
	for _, e := range fo.Events {
		budget = budget || e.Kind == domain.FanoutEventBudget
	}
	assert.True(t, budget, "traced")
	assert.Contains(t, f.launcher.told[0], "0.50")
}

func TestDriverTamperedIntegration(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "ev-1", "ev-2")
	f.wf.Fanout.Parallel = 1
	f.launcher.work["ev-1"] = [2]string{"a.go", "package calc\n"}
	f.launcher.work["ev-2"] = [2]string{"b.go", "package calc\n"}
	// The first child also moves the integration branch by hand.
	orig := f.launcher.work
	f.launcher.work = map[string][2]string{"ev-1": orig["ev-1"]}
	f.launcher.onWork = func(req LaunchRequest) {
		if req.Ticket == "ev-1" {
			quiet(req.Parent.Fanout.Integration.Path, "git", "commit", "-q", "--allow-empty", "-m", "evil")
		}
	}
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
	t.Parallel()
	f := newFixture(t, "ev-1")
	p, _ := f.store.Get(context.Background(), "ses_parent")
	p.Fanout.Experimental = false
	require.NoError(t, f.store.SetSessionFanout(context.Background(), p.ID, p.Fanout))
	assert.ErrorIs(t, f.driver.Run(context.Background(), "ses_parent"), ErrDisabled)
	require.NoError(t, f.store.SetSessionFanout(context.Background(), p.ID, nil))
	assert.ErrorIs(t, f.driver.Run(context.Background(), "ses_parent"), ErrNotFanout)

	// The parent ends its step before cp-plan: nothing launched, the
	// fan-out waits for the daemon to start the driver again (p6-debts);
	// stopped for good, the fan-out stops.
	for state, want := range map[domain.RunState]domain.FanoutStatus{domain.RunCompleted: domain.FanoutWaiting, domain.RunStopped: domain.FanoutStopped} {
		g := newFixture(t, "ev-1")
		g.passed = false
		q, _ := g.store.Get(context.Background(), "ses_parent")
		q.State = state
		require.NoError(t, g.store.Update(context.Background(), q))
		fo := g.runDriver(t)
		assert.Equal(t, want, fo.Status, state)
		assert.Empty(t, g.launcher.started)
	}
}

func TestIntegrateRefusesDirtyParent(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "ev-1")
	f.launcher.work["ev-1"] = [2]string{"a.go", "package calc\n"}
	f.runDriver(t)
	require.NoError(t, os.WriteFile(filepath.Join(f.project, "calc.go"), []byte("package calc // edited\n"), 0o644))
	p, _ := f.store.Get(context.Background(), "ses_parent")
	_, err := (&Integrator{Sessions: f.store, Close: func(context.Context, string, string, string) error { return nil }}).Integrate(context.Background(), p, "cp-merge")
	assert.Error(t, err)
}

func TestHasMarkers(t *testing.T) {
	t.Parallel()
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
// child idle with a commit but whose commit checkpoint did not pass, or
// idle without a commit, is not done; once the missing part is there, the
// signal ends it. Waits by condition, not by time: the guard is an hour
// in these cases (it cannot decide), the driver is seen checking the idle
// child several times without ending it, then the test completes the work.
func TestDriverChildEndSignal(t *testing.T) {
	t.Parallel()
	childWf := &sessionspec.WorkflowRuntime{ID: "ticket", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-2", Mandatory: true, Unlocks: []string{sessionspec.UnlockCommit}}}}
	for _, tc := range []struct {
		name   string
		passed bool // cp-2 passed from the start
		work   bool // the child commits its work
	}{
		{"signal", true, true},
		{"checkpoint not passed", false, true},
		{"no commit", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, "ev-1")
			f.driver.IdleTimeout = time.Hour
			if tc.work {
				f.launcher.work["ev-1"] = [2]string{"a.go", "package calc\n"}
			}
			var passed atomic.Bool
			passed.Store(tc.passed)
			var checks atomic.Int64 // end-of-work checks of the idle child
			parentWf := f.wf
			f.driver.Workflow = func(_ context.Context, id string) (*sessionspec.WorkflowRuntime, domain.CheckpointState, error) {
				st := domain.CheckpointState{Passed: map[string]time.Time{"cp-plan": time.Now()}}
				if id == "ses_parent" {
					return parentWf, st, nil
				}
				checks.Add(1)
				if passed.Load() {
					st.Passed["cp-2"] = time.Now()
				}
				return childWf, st, nil
			}
			done := make(chan driverEnd, 1)
			go func() {
				fo, err := f.runDriverErr()
				done <- driverEnd{fo, err}
			}()
			if !tc.passed || !tc.work {
				const seen = 5
				waitFor(t, done, func() bool { return checks.Load() >= seen }, "the driver checks the idle child")
				p, err := f.store.Get(context.Background(), "ses_parent")
				require.NoError(t, err)
				require.Empty(t, doneReason(p.Fanout, "ev-1"), "idle without its end of work: not done")
				if !tc.passed {
					passed.Store(true)
				} else {
					require.Len(t, p.Fanout.Children, 1)
					c := p.Fanout.Children[0]
					gittest.Run(t, c.Path, "commit", "-q", "--allow-empty", "-m", "feat: ev-1")
				}
			}
			select {
			case end := <-done:
				require.NoError(t, end.err)
				assert.Equal(t, DoneSignal, doneReason(end.fo, "ev-1"))
			case <-t.Context().Done():
				t.Fatal("the driver did not end")
			}
		})
	}
}

// driverEnd is how a driver run in a goroutine ended.
type driverEnd struct {
	fo  *domain.SessionFanout
	err error
}

// waitFor waits until cond holds, failing when the driver ends first.
func waitFor(t *testing.T, done <-chan driverEnd, cond func() bool, what string) {
	t.Helper()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for !cond() {
		select {
		case end := <-done:
			if end.err != nil {
				t.Fatalf("%s: the driver failed first: %v", what, end.err)
			}
			t.Fatalf("%s: the driver ended first (%s)", what, doneReason(end.fo, "ev-1"))
		case <-t.Context().Done():
			t.Fatalf("%s: never", what)
		case <-tick.C:
		}
	}
}
