package gc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/filelock"
)

type fakeStore struct {
	sessions []domain.Session
	servers  []domain.Server
	deleted  []string
	history  History
	purged   bool
}

func (f *fakeStore) Sessions(context.Context) ([]domain.Session, error) { return f.sessions, nil }
func (f *fakeStore) Servers(context.Context) ([]domain.Server, error)   { return f.servers, nil }
func (f *fakeStore) DeleteServer(_ context.Context, gk string) error {
	f.deleted = append(f.deleted, gk)
	return nil
}
func (f *fakeStore) CountHistory(context.Context, Cutoffs) (History, error) {
	if f.purged {
		return History{}, nil
	}
	return f.history, nil
}
func (f *fakeStore) DeleteHistory(context.Context, Cutoffs) (History, error) {
	f.purged = true
	return f.history, nil
}

var now = time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

func opts() Options {
	return Options{SessionAge: 30 * 24 * time.Hour, HistoryAge: 180 * 24 * time.Hour, UsageAge: 400 * 24 * time.Hour, BundleGrace: 24 * time.Hour, AutoWorktrees: true}
}

type fixture struct {
	env   *Env
	store *fakeStore
	root  string
}

func newFixture(t *testing.T) *fixture {
	root := t.TempDir()
	st := &fakeStore{}
	env := &Env{
		HubDir: root, BundlesDir: filepath.Join(root, "bundles"), ServersDir: filepath.Join(root, "servers"),
		SessionsDir: filepath.Join(root, "sessions"), RunDir: filepath.Join(root, "run"), Store: st,
		Now: func() time.Time { return now },
	}
	return &fixture{env: env, store: st, root: root}
}

// mkdir creates dir with a file and sets its age.
func mkdir(t *testing.T, dir string, age time.Duration) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f"), []byte("12345"), 0o644))
	at := now.Add(-age)
	require.NoError(t, os.Chtimes(dir, at, at))
}

func ids(r Report, k Kind) []string {
	var out []string
	for _, it := range r.ByKind()[k] {
		out = append(out, it.ID)
	}
	sort.Strings(out)
	return out
}

const day = 24 * time.Hour

// N16: only stopped sessions older than the retention lose their files;
// idle, sleeping, completed, failed and active ones keep them.
func TestPlanSessionStates(t *testing.T) {
	f := newFixture(t)
	oldEnd := now.Add(-40 * day)
	recentEnd := now.Add(-2 * day)
	for _, c := range []struct {
		id    string
		state domain.RunState
		ended time.Time
	}{
		{"idle", domain.RunIdle, oldEnd}, {"sleeping", domain.RunSleeping, oldEnd}, {"completed", domain.RunCompleted, oldEnd},
		{"failed", domain.RunFailed, oldEnd}, {"active", domain.RunActive, oldEnd},
		{"stopped-old", domain.RunStopped, oldEnd}, {"stopped-recent", domain.RunStopped, recentEnd},
	} {
		e := c.ended
		f.store.sessions = append(f.store.sessions, domain.Session{ID: c.id, State: c.state, StartedAt: e.Add(-time.Hour), EndedAt: &e,
			BundleHash: "b-" + c.id, GroupKey: "g-" + c.id})
		mkdir(t, filepath.Join(f.env.BundlesDir, "b-"+c.id), 40*day)
		mkdir(t, filepath.Join(f.env.ServersDir, "g-"+c.id), 40*day)
		mkdir(t, filepath.Join(f.env.SessionsDir, c.id), 40*day)
	}
	r, err := Plan(context.Background(), f.env, opts())
	require.NoError(t, err)
	assert.Equal(t, []string{"b-stopped-old"}, ids(r, KindBundles))
	assert.Equal(t, []string{"g-stopped-old"}, ids(r, KindServers))
	assert.Equal(t, []string{"stopped-old"}, ids(r, KindSessions))
	for _, it := range r.Items {
		assert.Positive(t, it.Bytes, it.ID)
	}
	assert.FileExists(t, filepath.Join(f.env.BundlesDir, "b-stopped-old", "f"), "a plan removes nothing")
}

// A bundle shared with a live session, or used less than the grace period
// ago, is kept; a live server holds its bundle and its group.
func TestPlanBundlesSharedAndGrace(t *testing.T) {
	f := newFixture(t)
	oldEnd := now.Add(-40 * day)
	f.store.sessions = []domain.Session{
		{ID: "s1", State: domain.RunStopped, EndedAt: &oldEnd, BundleHash: "shared"},
		{ID: "s2", State: domain.RunCompleted, EndedAt: &oldEnd, BundleHash: "shared"},
		{ID: "s3", State: domain.RunStopped, EndedAt: &oldEnd, BundleHash: "graced"},
	}
	f.store.servers = []domain.Server{{GroupKey: "g-live", BundleHash: "served", Status: domain.ServerReady, PID: os.Getpid()}}
	mkdir(t, filepath.Join(f.env.BundlesDir, "shared"), 40*day)
	mkdir(t, filepath.Join(f.env.BundlesDir, "graced"), time.Hour)
	mkdir(t, filepath.Join(f.env.BundlesDir, "served"), 40*day)
	mkdir(t, filepath.Join(f.env.BundlesDir, "orphan"), 40*day)
	mkdir(t, filepath.Join(f.env.BundlesDir, ".build-123"), 2*time.Hour)
	mkdir(t, filepath.Join(f.env.BundlesDir, ".fetch-456"), 10*time.Minute)
	mkdir(t, filepath.Join(f.env.ServersDir, "g-live"), 40*day)
	r, err := Plan(context.Background(), f.env, opts())
	require.NoError(t, err)
	assert.Equal(t, []string{"orphan"}, ids(r, KindBundles))
	assert.Equal(t, []string{".build-123"}, ids(r, KindTemp))
	assert.Empty(t, ids(r, KindServers))
}

// Run removes what is planned; a session folder keeps its results until the
// history goes; a group whose lock is held is left (the error is reported);
// the row of a removed group is deleted.
func TestRunApplies(t *testing.T) {
	f := newFixture(t)
	oldEnd := now.Add(-40 * day)
	ancient := now.Add(-200 * day)
	f.store.sessions = []domain.Session{
		{ID: "s-old", State: domain.RunStopped, EndedAt: &oldEnd, BundleHash: "b1", GroupKey: "g1"},
		{ID: "s-ancient", State: domain.RunStopped, EndedAt: &ancient, GroupKey: "g2"},
	}
	f.store.history = History{Sessions: 3, UsageSessions: 2}
	mkdir(t, filepath.Join(f.env.BundlesDir, "b1"), 40*day)
	// A read-only bundle (made so once built) is removed too.
	require.NoError(t, os.Chmod(filepath.Join(f.env.BundlesDir, "b1"), 0o555))
	mkdir(t, filepath.Join(f.env.ServersDir, "g1", "data"), 40*day)
	mkdir(t, filepath.Join(f.env.ServersDir, "g2"), 40*day)
	mkdir(t, filepath.Join(f.env.SessionsDir, "s-old", "results"), 40*day)
	mkdir(t, filepath.Join(f.env.SessionsDir, "s-ancient", "results"), 200*day)
	require.NoError(t, os.WriteFile(filepath.Join(f.env.SessionsDir, "s-old", "context.json"), []byte("{}"), 0o600))
	// g2 is being resumed by another oh.
	unlock, err := filelock.Lock(filepath.Join(f.env.ServersDir, "g2", "lock"))
	require.NoError(t, err)
	defer unlock()
	at := now.Add(-40 * day)
	require.NoError(t, os.Chtimes(filepath.Join(f.env.ServersDir, "g1"), at, at))
	require.NoError(t, os.Chtimes(filepath.Join(f.env.ServersDir, "g2"), at, at))

	done, err := Run(context.Background(), f.env, opts(), nil)
	require.NoError(t, err)
	assert.True(t, done.Applied)
	assert.NoDirExists(t, filepath.Join(f.env.BundlesDir, "b1"))
	assert.NoDirExists(t, filepath.Join(f.env.ServersDir, "g1"))
	assert.DirExists(t, filepath.Join(f.env.ServersDir, "g2"), "locked group skipped")
	assert.Equal(t, []string{"g1"}, f.store.deleted)
	assert.DirExists(t, filepath.Join(f.env.SessionsDir, "s-old", "results"), "results kept until the history goes")
	assert.NoFileExists(t, filepath.Join(f.env.SessionsDir, "s-old", "context.json"))
	assert.NoDirExists(t, filepath.Join(f.env.SessionsDir, "s-ancient"))
	assert.Len(t, done.Errors, 1)
	assert.Contains(t, done.Errors[0], "g2")
	assert.Equal(t, 5, done.History.Total())

	again, err := Plan(context.Background(), f.env, opts())
	require.NoError(t, err)
	assert.Equal(t, []string{"g2"}, ids(again, KindServers), "only the busy group is left")
	assert.Empty(t, ids(again, KindSessions), "a folder left with its results only is not planned again")
}

// Run limited to a confirmed plan removes nothing else; the global lock
// refuses a second run.
func TestRunOnlyTheConfirmedPlan(t *testing.T) {
	f := newFixture(t)
	mkdir(t, filepath.Join(f.env.BundlesDir, "a"), 40*day)
	plan, err := Plan(context.Background(), f.env, opts())
	require.NoError(t, err)
	mkdir(t, filepath.Join(f.env.BundlesDir, "b"), 40*day) // appeared after the plan
	done, err := Run(context.Background(), f.env, opts(), &plan)
	require.NoError(t, err)
	assert.Equal(t, []string{"a"}, ids(done, KindBundles))
	assert.DirExists(t, filepath.Join(f.env.BundlesDir, "b"))

	unlock, err := Lock(f.env.RunDir)
	require.NoError(t, err)
	_, err = Run(context.Background(), f.env, opts(), nil)
	assert.ErrorIs(t, err, ErrBusy)
	unlock()
}

func TestOnlyKinds(t *testing.T) {
	f := newFixture(t)
	oldEnd := now.Add(-40 * day)
	f.store.sessions = []domain.Session{{ID: "s", State: domain.RunStopped, EndedAt: &oldEnd}}
	mkdir(t, filepath.Join(f.env.BundlesDir, "a"), 40*day)
	mkdir(t, filepath.Join(f.env.SessionsDir, "s"), 40*day)
	f.store.history = History{AgentEvents: 1}
	o := opts()
	o.Only = []Kind{KindSessions}
	r, err := Plan(context.Background(), f.env, o)
	require.NoError(t, err)
	assert.Equal(t, []string{"s"}, ids(r, KindSessions))
	assert.Empty(t, ids(r, KindBundles))
	assert.Zero(t, r.History.Total())
}

type fakeLister struct {
	procs   []adapters.ServerProcess
	stopped []int
}

func (l *fakeLister) ServerProcesses(context.Context) ([]adapters.ServerProcess, error) {
	return l.procs, nil
}

func (l *fakeLister) StopServerProcess(_ context.Context, pid int) error {
	l.stopped = append(l.stopped, pid)
	return nil
}

// N18: oh gc stops the orphan servers too.
func TestOrphansAreStopped(t *testing.T) {
	f := newFixture(t)
	tmp := t.TempDir()
	l := &fakeLister{procs: []adapters.ServerProcess{
		{PID: 11, PPID: 1, WorkDir: filepath.Join(tmp, "TestX", "w")},
		{PID: 12, PPID: 77, WorkDir: filepath.Join(tmp, "TestY", "w")},
	}}
	f.env.Processes, f.env.TmpDir = l, tmp
	done, err := Run(context.Background(), f.env, opts(), nil)
	require.NoError(t, err)
	assert.Equal(t, []int{11}, l.stopped)
	assert.Equal(t, []string{"11"}, ids(done, KindOrphans))
}

// --- worktrees, on real repositories ---

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=maintenance.auto", "GIT_CONFIG_VALUE_0=false")
	out, err := c.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return strings.TrimSpace(string(out))
}

// N16: a worktree goes only when oh created it, nobody uses it, it is clean,
// unlocked and pushed; its branch only when merged.
func TestWorktrees(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	f := newFixture(t)
	bare := t.TempDir()
	git(t, bare, "init", "-q", "--bare", "-b", "main")
	repo := filepath.Join(t.TempDir(), "proj")
	git(t, filepath.Dir(repo), "clone", "-q", bare, repo)
	git(t, repo, "commit", "-q", "--allow-empty", "-m", "init")
	git(t, repo, "push", "-q", "origin", "HEAD:main")
	f.env.ProjectPath = func(context.Context, string) string { return repo }
	f.env.BaseBranch = func(string) string { return "main" }

	oldEnd := now.Add(-40 * day)
	wts := map[string]string{}
	add := func(name string, auto bool, state domain.RunState) string {
		p := filepath.Join(filepath.Dir(repo), "wt-"+name)
		git(t, repo, "worktree", "add", "-q", "-b", "feat/"+name, p)
		wts[name] = p
		f.store.sessions = append(f.store.sessions, domain.Session{ID: "s-" + name, ProjectID: "p", State: state, EndedAt: &oldEnd,
			LaunchPath: p, WorktreeAuto: auto, WorktreeBranch: "feat/" + name})
		return p
	}
	add("clean", true, domain.RunStopped) // no own commit: removed, branch (fresh) kept
	merged := add("merged", true, domain.RunStopped)
	git(t, merged, "commit", "-q", "--allow-empty", "-m", "work")
	git(t, repo, "merge", "-q", "--no-ff", "-m", "merge", "feat/merged")
	pushed := add("pushed", true, domain.RunStopped)
	git(t, pushed, "commit", "-q", "--allow-empty", "-m", "pushed work")
	git(t, pushed, "push", "-q", "origin", "feat/pushed")
	dirty := add("dirty", true, domain.RunStopped)
	require.NoError(t, os.WriteFile(filepath.Join(dirty, "x.txt"), []byte("x"), 0o644)) // untracked
	unpushed := add("unpushed", true, domain.RunStopped)
	git(t, unpushed, "commit", "-q", "--allow-empty", "-m", "local only")
	add("locked", true, domain.RunStopped)
	git(t, repo, "worktree", "lock", wts["locked"])
	add("used", true, domain.RunCompleted)
	add("manual", false, domain.RunStopped)

	plan, err := Plan(context.Background(), f.env, opts())
	require.NoError(t, err)
	assert.Equal(t, []string{"feat/clean", "feat/merged", "feat/pushed"}, ids(plan, KindWorktrees))

	o := opts()
	o.AutoWorktrees = false
	off, err := Plan(context.Background(), f.env, o)
	require.NoError(t, err)
	assert.Empty(t, ids(off, KindWorktrees), "[worktree] auto_cleanup = false")

	_, err = Run(context.Background(), f.env, opts(), nil)
	require.NoError(t, err)
	for _, gone := range []string{"clean", "merged", "pushed"} {
		assert.NoDirExists(t, wts[gone], gone)
	}
	for _, kept := range []string{"dirty", "unpushed", "locked", "used", "manual"} {
		assert.DirExists(t, wts[kept], kept)
	}
	branches := git(t, repo, "branch", "--format=%(refname:short)")
	assert.NotContains(t, branches, "feat/merged", "merged branch deleted")
	assert.Contains(t, branches, "feat/clean", "a fresh branch is not merged: kept")
	assert.Contains(t, branches, "feat/pushed", "pushed but not merged: kept")
}
