// Package gc cleans up what oh leaves behind (N16, DC15): session bundles,
// data of the server groups, session folders, automatic worktrees, temporary
// build folders, orphan tool servers and the old history of the database.
//
// Plan only reads; Run takes the global lock, plans again and applies.
// Invariants:
//   - only stopped sessions are cleaned up (completed and failed ones can be
//     resumed): a bundle, a server group, a session folder or a worktree used
//     by a session that is not stopped is kept;
//   - files of a stopped session are kept SessionAge, its results and the
//     history HistoryAge, the usage ledger UsageAge;
//   - a bundle used less than BundleGrace ago is kept (Build touches it);
//   - a server group is removed with its group lock held, its server dead;
//   - a worktree is removed only when oh created it, clean, unlocked, with
//     no commit that exists nowhere else; its branch only when merged.
package gc

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/filelock"
)

// Kind is a category of what oh gc removes.
type Kind string

// Kinds, in the order of the report.
const (
	KindBundles   Kind = "bundles"
	KindServers   Kind = "servers"
	KindSessions  Kind = "sessions"
	KindWorktrees Kind = "worktrees"
	KindTemp      Kind = "temp"
	KindOrphans   Kind = "orphans"
	KindHistory   Kind = "history"
)

// Kinds lists every kind.
var Kinds = []Kind{KindBundles, KindServers, KindSessions, KindWorktrees, KindTemp, KindOrphans, KindHistory}

// ErrBusy: another clean-up holds the lock.
var ErrBusy = errors.New("another oh gc is running")

// tempAge is the age of a temporary build folder left by a killed process.
const tempAge = time.Hour

// Item is one thing to remove.
type Item struct {
	Kind   Kind   `json:"kind"`
	ID     string `json:"id,omitempty"`   // bundle hash, group key, session id, branch
	Path   string `json:"path,omitempty"` // removed path
	Bytes  int64  `json:"bytes"`
	Reason string `json:"reason,omitempty"`
	// KeepResults: a session folder whose results/ stay (younger than the
	// history).
	KeepResults bool `json:"keep_results,omitempty"`
	// Repo is the project of a worktree; PID the process of an orphan.
	Repo string `json:"repo,omitempty"`
	PID  int    `json:"pid,omitempty"`
}

func (it Item) key() string { return string(it.Kind) + "\x00" + it.ID + "\x00" + it.Path }

// History counts the rows of the database past their retention.
type History struct {
	Sessions      int `json:"sessions"`
	AgentEvents   int `json:"agent_events"`
	Decisions     int `json:"decisions"`
	UsageSessions int `json:"usage_sessions"`
	UsageProxy    int `json:"usage_proxy"`
	BudgetExtra   int `json:"budget_extra"`
}

// Total is the number of rows.
func (h History) Total() int {
	return h.Sessions + h.AgentEvents + h.Decisions + h.UsageSessions + h.UsageProxy + h.BudgetExtra
}

// Cutoffs are the dates before which history rows go.
type Cutoffs struct {
	History time.Time // stopped sessions, agent events, settled decisions
	Usage   time.Time // usage ledger, budget raises
}

// Store is the database side of the clean-up.
type Store interface {
	Sessions(ctx context.Context) ([]domain.Session, error)
	Servers(ctx context.Context) ([]domain.Server, error)
	// DeleteServer removes the row of a server group and revokes its grants.
	DeleteServer(ctx context.Context, groupKey string) error
	CountHistory(ctx context.Context, c Cutoffs) (History, error)
	DeleteHistory(ctx context.Context, c Cutoffs) (History, error)
}

// Env is where the clean-up looks.
type Env struct {
	HubDir      string
	BundlesDir  string
	ServersDir  string
	SessionsDir string
	RunDir      string // gc.lock
	TmpDir      string // test servers (default os.TempDir)
	Store       Store
	// ProjectPath gives the repository of a project ("" unknown).
	ProjectPath func(ctx context.Context, projectID string) string
	// BaseBranch is the branch a worktree branch is merged into.
	BaseBranch func(repo string) string
	// GroupLockPath is the lock of a server group.
	GroupLockPath func(group string) string
	// Processes finds the orphan tool servers (nil: none looked for).
	Processes adapters.ServerProcessLister
	// Git runs the git checks of the worktrees (default: real git).
	Git Git
	Now func() time.Time
}

// Options tune a clean-up.
type Options struct {
	Only          []Kind // empty: every kind
	SessionAge    time.Duration
	HistoryAge    time.Duration
	UsageAge      time.Duration
	BundleGrace   time.Duration
	AutoWorktrees bool // [worktree] auto_cleanup
}

func (o Options) wants(k Kind) bool {
	if len(o.Only) == 0 {
		return true
	}
	for _, x := range o.Only {
		if x == k {
			return true
		}
	}
	return false
}

// Report is a plan, or what a run did.
type Report struct {
	Items   []Item   `json:"items"`
	History History  `json:"history"`
	Errors  []string `json:"errors,omitempty"`
	Applied bool     `json:"applied"`
}

// Bytes is the space of the items.
func (r Report) Bytes() int64 {
	var n int64
	for _, it := range r.Items {
		n += it.Bytes
	}
	return n
}

// Empty reports a plan with nothing to remove.
func (r Report) Empty() bool { return len(r.Items) == 0 && r.History.Total() == 0 }

// ByKind groups the items.
func (r Report) ByKind() map[Kind][]Item {
	out := map[Kind][]Item{}
	for _, it := range r.Items {
		out[it.Kind] = append(out[it.Kind], it)
	}
	return out
}

func (e *Env) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func (e *Env) git() Git {
	if e.Git != nil {
		return e.Git
	}
	return realGit{}
}

// endedAt is when a session stopped (else its last change, else its start).
func endedAt(s domain.Session) time.Time {
	switch {
	case s.EndedAt != nil:
		return *s.EndedAt
	case s.StateChangedAt != nil:
		return *s.StateChangedAt
	}
	return s.StartedAt
}

// usage indexes what the sessions and servers hold.
type usage struct {
	sessions     map[string]domain.Session
	liveBundles  map[string]bool // used by a session not stopped, or recently
	liveGroups   map[string]bool
	liveWorkdirs map[string]bool
	servers      map[string]domain.Server
	livePIDs     map[int]bool
}

func (e *Env) collectUsage(ctx context.Context, opts Options) (*usage, error) {
	sessions, err := e.Store.Sessions(ctx)
	if err != nil {
		return nil, err
	}
	servers, err := e.Store.Servers(ctx)
	if err != nil {
		return nil, err
	}
	now := e.now()
	u := &usage{sessions: map[string]domain.Session{}, liveBundles: map[string]bool{}, liveGroups: map[string]bool{},
		liveWorkdirs: map[string]bool{}, servers: map[string]domain.Server{}, livePIDs: map[int]bool{}}
	for _, s := range sessions {
		u.sessions[s.ID] = s
		hold := s.State != domain.RunStopped || now.Sub(endedAt(s)) < opts.SessionAge
		if !hold {
			continue
		}
		if s.BundleHash != "" {
			u.liveBundles[s.BundleHash] = true
		}
		if s.GroupKey != "" {
			u.liveGroups[s.GroupKey] = true
		}
		if s.LaunchPath != "" {
			u.liveWorkdirs[filepath.Clean(s.LaunchPath)] = true
		}
	}
	for _, srv := range servers {
		u.servers[srv.GroupKey] = srv
		if srv.Status != domain.ServerStopped && srv.BundleHash != "" {
			u.liveBundles[srv.BundleHash] = true
		}
		if srv.Status == domain.ServerStarting || srv.Status == domain.ServerReady {
			u.liveGroups[srv.GroupKey] = true
			if srv.PID > 0 {
				u.livePIDs[srv.PID] = true
			}
		}
	}
	return u, nil
}

// Plan lists what a clean-up would remove (nothing is changed).
func Plan(ctx context.Context, env *Env, opts Options) (Report, error) {
	u, err := env.collectUsage(ctx, opts)
	if err != nil {
		return Report{}, err
	}
	var r Report
	add := func(items []Item, err error) {
		r.Items = append(r.Items, items...)
		if err != nil {
			r.Errors = append(r.Errors, err.Error())
		}
	}
	if opts.wants(KindBundles) || opts.wants(KindTemp) {
		b, t, err := env.planBundles(u, opts)
		if opts.wants(KindBundles) {
			add(b, err)
		}
		if opts.wants(KindTemp) {
			add(t, nil)
		}
	}
	if opts.wants(KindServers) {
		add(env.planServers(u))
	}
	if opts.wants(KindSessions) {
		add(env.planSessions(u, opts))
	}
	if opts.wants(KindWorktrees) && opts.AutoWorktrees {
		add(env.planWorktrees(ctx, u, opts), nil)
	}
	if opts.wants(KindOrphans) && env.Processes != nil {
		add(env.planOrphans(ctx, u))
	}
	if opts.wants(KindHistory) {
		h, err := env.Store.CountHistory(ctx, env.cutoffs(opts))
		r.History = h
		if err != nil {
			r.Errors = append(r.Errors, err.Error())
		}
	}
	return r, nil
}

func (e *Env) cutoffs(opts Options) Cutoffs {
	now := e.now()
	return Cutoffs{History: now.Add(-opts.HistoryAge), Usage: now.Add(-opts.UsageAge)}
}

func (e *Env) planBundles(u *usage, opts Options) (bundles, temp []Item, err error) {
	entries, err := os.ReadDir(e.BundlesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	now := e.now()
	for _, ent := range entries {
		path := filepath.Join(e.BundlesDir, ent.Name())
		info, err := ent.Info()
		if err != nil || !ent.IsDir() {
			continue
		}
		name := ent.Name()
		if strings.HasPrefix(name, ".build-") || strings.HasPrefix(name, ".fetch-") {
			if now.Sub(info.ModTime()) > tempAge {
				temp = append(temp, Item{Kind: KindTemp, ID: name, Path: path, Bytes: dirSize(path), Reason: "temp"})
			}
			continue
		}
		if strings.HasPrefix(name, ".") || u.liveBundles[name] || now.Sub(info.ModTime()) < opts.BundleGrace {
			continue
		}
		bundles = append(bundles, Item{Kind: KindBundles, ID: name, Path: path, Bytes: dirSize(path), Reason: "unused"})
	}
	return bundles, temp, nil
}

func (e *Env) planServers(u *usage) ([]Item, error) {
	entries, err := os.ReadDir(e.ServersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Item
	for _, ent := range entries {
		gk := ent.Name()
		if !ent.IsDir() || u.liveGroups[gk] {
			continue
		}
		// A group being prepared (no row yet): its starter holds the lock,
		// and a young folder is left for the next pass anyway.
		if info, err := ent.Info(); err != nil || e.now().Sub(info.ModTime()) < tempAge {
			continue
		}
		if srv, ok := u.servers[gk]; ok && srv.PID > 0 && filelock.ProcessAlive(srv.PID) && srv.Status != domain.ServerStopped && srv.Status != domain.ServerSleeping {
			continue
		}
		path := filepath.Join(e.ServersDir, gk)
		out = append(out, Item{Kind: KindServers, ID: gk, Path: path, Bytes: dirSize(path), Reason: "stopped"})
	}
	return out, nil
}

func (e *Env) planSessions(u *usage, opts Options) ([]Item, error) {
	entries, err := os.ReadDir(e.SessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	now := e.now()
	var out []Item
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		id := ent.Name()
		path := filepath.Join(e.SessionsDir, id)
		var ended time.Time
		reason := "stopped"
		if s, ok := u.sessions[id]; ok {
			if s.State != domain.RunStopped {
				continue
			}
			ended = endedAt(s)
		} else {
			info, err := ent.Info()
			if err != nil {
				continue
			}
			ended, reason = info.ModTime(), "orphan"
		}
		age := now.Sub(ended)
		if age < opts.SessionAge {
			continue
		}
		keep := age < opts.HistoryAge && dirExists(filepath.Join(path, "results"))
		bytes := dirSize(path)
		if keep {
			bytes -= dirSize(filepath.Join(path, "results"))
			if bytes <= 0 && onlyResults(path) {
				continue // nothing but the results left
			}
		}
		out = append(out, Item{Kind: KindSessions, ID: id, Path: path, Bytes: bytes, Reason: reason, KeepResults: keep})
	}
	return out, nil
}

func onlyResults(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Name() != "results" {
			return false
		}
	}
	return true
}

func (e *Env) planWorktrees(ctx context.Context, u *usage, opts Options) []Item {
	type wt struct {
		repo, branch string
		stopped      bool
	}
	byPath := map[string]*wt{}
	now := e.now()
	for _, s := range u.sessions {
		if !s.WorktreeAuto || s.LaunchPath == "" || s.WorktreeBranch == "" {
			continue
		}
		p := filepath.Clean(s.LaunchPath)
		if _, ok := byPath[p]; !ok {
			repo := ""
			if e.ProjectPath != nil {
				repo = e.ProjectPath(ctx, s.ProjectID)
			}
			byPath[p] = &wt{repo: repo, branch: s.WorktreeBranch, stopped: true}
		}
		if s.State != domain.RunStopped || now.Sub(endedAt(s)) < opts.SessionAge {
			byPath[p].stopped = false
		}
	}
	paths := make([]string, 0, len(byPath))
	for p := range byPath {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	g := e.git()
	var out []Item
	for _, p := range paths {
		w := byPath[p]
		// Any other session in the folder (not created by oh) holds it too.
		if !w.stopped || u.liveWorkdirs[p] || w.repo == "" || !dirExists(p) || filepath.Clean(w.repo) == p {
			continue
		}
		if ok, _ := g.Removable(ctx, w.repo, p, w.branch); !ok {
			continue
		}
		out = append(out, Item{Kind: KindWorktrees, ID: w.branch, Path: p, Repo: w.repo, Bytes: dirSize(p), Reason: "clean"})
	}
	return out
}

func (e *Env) planOrphans(ctx context.Context, u *usage) ([]Item, error) {
	procs, err := e.Processes.ServerProcesses(ctx)
	if err != nil {
		return nil, err
	}
	tmp := e.TmpDir
	if tmp == "" {
		tmp = os.TempDir()
	}
	var out []Item
	for _, o := range ClassifyOrphans(procs, e.HubDir, tmp, u.livePIDs, pathExists, RecentlyWritten) {
		where := o.LogPath
		if where == "" {
			where = o.WorkDir
		}
		out = append(out, Item{Kind: KindOrphans, ID: fmt.Sprint(o.PID), PID: o.PID, Path: where, Reason: o.Reason})
	}
	return out, nil
}

// Lock takes the global lock of the clean-up (ErrBusy when held).
func Lock(runDir string) (func(), error) {
	unlock, err := filelock.TryLock(filepath.Join(runDir, "gc.lock"))
	if errors.Is(err, filelock.ErrLocked) {
		return nil, ErrBusy
	}
	return unlock, err
}

// Run plans under the global lock and removes what is planned. When only
// is not nil, only the items it lists (a plan the user confirmed) are
// removed, if they are still planned.
func Run(ctx context.Context, env *Env, opts Options, only *Report) (Report, error) {
	unlock, err := Lock(env.RunDir)
	if err != nil {
		return Report{}, err
	}
	defer unlock()
	plan, err := Plan(ctx, env, opts)
	if err != nil {
		return Report{}, err
	}
	if only != nil {
		plan = intersect(plan, *only)
	}
	done := Report{Applied: true, Errors: plan.Errors}
	for _, it := range plan.Items {
		if err := env.apply(ctx, it); err != nil {
			done.Errors = append(done.Errors, fmt.Sprintf("%s %s: %v", it.Kind, it.ID, err))
			continue
		}
		done.Items = append(done.Items, it)
	}
	if plan.History.Total() > 0 {
		h, err := env.Store.DeleteHistory(ctx, env.cutoffs(opts))
		done.History = h
		if err != nil {
			done.Errors = append(done.Errors, err.Error())
		}
	}
	return done, nil
}

func intersect(plan, only Report) Report {
	keep := map[string]bool{}
	for _, it := range only.Items {
		keep[it.key()] = true
	}
	out := Report{Errors: plan.Errors}
	for _, it := range plan.Items {
		if keep[it.key()] {
			out.Items = append(out.Items, it)
		}
	}
	if only.History.Total() > 0 {
		out.History = plan.History
	}
	return out
}

func (e *Env) apply(ctx context.Context, it Item) error {
	switch it.Kind {
	case KindBundles, KindTemp:
		return removeAll(it.Path)
	case KindSessions:
		if !it.KeepResults {
			return removeAll(it.Path)
		}
		entries, err := os.ReadDir(it.Path)
		if err != nil {
			return err
		}
		for _, ent := range entries {
			if ent.Name() != "results" {
				if err := removeAll(filepath.Join(it.Path, ent.Name())); err != nil {
					return err
				}
			}
		}
		return nil
	case KindServers:
		return e.removeServer(ctx, it)
	case KindWorktrees:
		return e.git().Remove(ctx, it.Repo, it.Path, it.ID, e.baseBranch(it.Repo))
	case KindOrphans:
		return e.Processes.StopServerProcess(ctx, it.PID)
	}
	return fmt.Errorf("unknown kind %q", it.Kind)
}

func (e *Env) baseBranch(repo string) string {
	if e.BaseBranch != nil {
		return e.BaseBranch(repo)
	}
	return ""
}

// removeServer removes a server group with its lock held (a client that
// would start it waits, then finds it gone and starts afresh).
func (e *Env) removeServer(ctx context.Context, it Item) error {
	lockPath := filepath.Join(it.Path, "lock")
	if e.GroupLockPath != nil {
		lockPath = e.GroupLockPath(it.ID)
	}
	unlock, err := filelock.TryLock(lockPath)
	if err != nil {
		return fmt.Errorf("group busy: %w", err)
	}
	released := false
	release := func() {
		if !released {
			unlock()
			released = true
		}
	}
	defer release()
	entries, err := os.ReadDir(it.Path)
	if err != nil {
		return err
	}
	for _, ent := range entries {
		p := filepath.Join(it.Path, ent.Name())
		if p == lockPath {
			continue
		}
		if err := removeAll(p); err != nil {
			return err
		}
	}
	if err := e.Store.DeleteServer(ctx, it.ID); err != nil {
		return err
	}
	release()
	return removeAll(it.Path)
}

// removeAll removes a path, read-only folders included (bundles are made
// read-only once built).
func removeAll(path string) error {
	err := os.RemoveAll(path)
	if err == nil {
		return nil
	}
	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			_ = os.Chmod(p, 0o700)
		}
		return nil
	})
	return os.RemoveAll(path)
}

func dirSize(path string) int64 {
	var n int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // unreadable entries are not counted
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func pathExists(p string) bool { _, err := os.Stat(p); return err == nil }
