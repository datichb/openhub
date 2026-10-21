package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/workflow/hubcat"
)

// The shipped workflows are loaded, and their bundles built, once per test
// run: the tests that only read a bundle share it. A test that writes in a
// bundle, or builds it with something else than the shipped hub, builds its
// own (Build with t.TempDir()). The files of the shared bundles and projects
// are fingerprinted when they are built and checked again after the last
// test (checkShared): a test that modified one fails the run.

// shared holds what the tests of the package share (see TestMain).
var shared struct {
	root string // bundles/ and projects/, removed after the tests

	mu     sync.Mutex
	prints map[string]string // path → sha256 of the file when it was shared

	catOnce sync.Once
	hub     string
	cat     *workflow.MemCatalog
	env     workflow.Env
	catDiag workflow.Diagnostics
	catErr  error

	builds  sync.Map // sharedKey → *sharedEntry
	project sync.Map // name → *sharedProject
}

// startShared creates the folder of the shared bundles and projects.
func startShared() error {
	dir, err := os.MkdirTemp("", "bundle-shared-")
	if err != nil {
		return err
	}
	shared.root = dir
	shared.prints = map[string]string{}
	return nil
}

// finishShared checks that the shared files did not change, removes them and
// returns the exit code of the tests.
func finishShared(code int) int {
	if shared.root == "" {
		return code
	}
	defer func() { _ = os.RemoveAll(shared.root) }()
	if problems := checkShared(); len(problems) > 0 {
		fmt.Fprintln(os.Stderr, "bundle tests: a test modified the bundles or projects shared by the tests of the package;")
		fmt.Fprintln(os.Stderr, "a test that writes in a bundle must build its own (Build with t.TempDir()):")
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "  -", p)
		}
		return 1
	}
	return code
}

// fingerprintDir records the sha256 of every file under dir.
func fingerprintDir(dir string) error {
	found := map[string]string{}
	if err := walkFiles(dir, found); err != nil {
		return err
	}
	shared.mu.Lock()
	defer shared.mu.Unlock()
	for p, sum := range found {
		shared.prints[p] = sum
	}
	return nil
}

func walkFiles(dir string, into map[string]string) error {
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		into[p] = hex.EncodeToString(sum[:])
		return nil
	})
}

// checkShared lists the shared files that changed, vanished or appeared.
func checkShared() []string {
	now := map[string]string{}
	if err := walkFiles(shared.root, now); err != nil {
		return []string{"reading " + shared.root + ": " + err.Error()}
	}
	shared.mu.Lock()
	defer shared.mu.Unlock()
	var out []string
	for p, sum := range shared.prints {
		switch got, ok := now[p]; {
		case !ok:
			out = append(out, "removed: "+p)
		case got != sum:
			out = append(out, "modified: "+p)
		}
	}
	for p := range now {
		if _, ok := shared.prints[p]; !ok {
			out = append(out, "added: "+p)
		}
	}
	sort.Strings(out)
	return out
}

// lockedSkills makes a SkillCatalog usable by tests that run in parallel (its
// loader keeps a cache).
type lockedSkills struct {
	mu *sync.Mutex
	c  *SkillCatalog
}

func (l lockedSkills) HasSkill(ref string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.c.HasSkill(ref)
}

func (l lockedSkills) Closure(roots []string) ([]string, []workflow.SkillIssue) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.c.Closure(roots)
}

// shippedWorkflows returns the hub, the catalogue of the shipped workflows
// and the validation environment. The hub and the environment are loaded
// once and shared (read-only); the catalogue is a copy, so that a test may
// add team or project workflows to it.
func shippedWorkflows(t *testing.T) (string, *workflow.MemCatalog, workflow.Env) {
	t.Helper()
	hub := repoHub(t)
	shared.catOnce.Do(func() {
		cat, diags := hubcat.LoadWorkflows(hub)
		if len(diags) > 0 || len(cat.Refs()) == 0 {
			shared.catDiag = diags
			shared.catErr = fmt.Errorf("no shipped workflow in %s/workflows", hub)
			return
		}
		hc, err := hubcat.New(hub)
		if err != nil {
			shared.catErr = err
			return
		}
		shared.hub, shared.cat, shared.env = hub, cat, hc.Env()
		shared.env.Skills = lockedSkills{mu: &sync.Mutex{}, c: NewSkillCatalog(hub)}
	})
	require.Empty(t, shared.catDiag, "loading %s/workflows", hub)
	require.NoError(t, shared.catErr)
	cat := workflow.NewMemCatalog()
	for _, ref := range shared.cat.Refs() {
		doc, _ := shared.cat.Lookup(ref)
		cat.Put(doc)
	}
	return shared.hub, cat, shared.env
}

type sharedProject struct {
	once sync.Once
	dir  string
	err  error
}

// sharedProjectDir creates a project once, under the shared folder.
func sharedProjectDir(t *testing.T, name string, create func(dir string) error) string {
	t.Helper()
	v, _ := shared.project.LoadOrStore(name, &sharedProject{})
	p := v.(*sharedProject)
	p.once.Do(func() {
		p.dir = filepath.Join(shared.root, "projects", name)
		if p.err = os.MkdirAll(p.dir, 0o755); p.err == nil {
			p.err = create(p.dir)
		}
		if p.err == nil {
			p.err = fingerprintDir(p.dir)
		}
	})
	require.NoError(t, p.err, "project %s", name)
	return p.dir
}

// shippedOpts is how a shipped workflow is launched and built.
type shippedOpts struct {
	Project  string // project path ("" = none), see capProject, instructProject
	Entry    string // entry agent of a free session (SessionOptions.EntryAgent)
	Method   workflow.Method
	Fanout   bool
	Provider string
}

type sharedEntry struct {
	once  sync.Once
	r     *workflow.Resolved
	b     *Bundle
	diags workflow.Diagnostics
	err   error

	reachOnce sync.Once
	reach     map[string][]sessionspec.SkillDef
}

type sharedKey struct {
	ref  workflow.Ref
	opts shippedOpts
}

// entryOf checks and builds a shipped workflow once (as the launch:
// selectable entries expanded) and fingerprints the bundle. It does not
// touch a testing.T: warmShipped runs it in goroutines.
func entryOf(hub string, cat workflow.Catalog, env workflow.Env, ref workflow.Ref, o shippedOpts) *sharedEntry {
	v, _ := shared.builds.LoadOrStore(sharedKey{ref, o}, &sharedEntry{})
	e := v.(*sharedEntry)
	e.once.Do(func() {
		var opts *workflow.SessionOptions
		if o.Entry != "" {
			opts = &workflow.SessionOptions{EntryAgent: o.Entry}
		}
		e.r, e.diags = workflow.Check(cat, ref, opts, env)
		if e.diags.HasErrors() {
			return
		}
		e.b, e.err = Build(Request{HubDir: hub, OutDir: filepath.Join(shared.root, "bundles"), ProjectPath: o.Project,
			Spec: e.r.Spec, Method: o.Method, Fanout: o.Fanout, Provider: o.Provider})
		if e.err == nil {
			e.err = fingerprintDir(e.b.Dir)
		}
	})
	return e
}

func sharedEntryOf(t *testing.T, ref workflow.Ref, o shippedOpts) *sharedEntry {
	t.Helper()
	hub, cat, env := shippedWorkflows(t)
	e := entryOf(hub, cat, env, ref, o)
	require.False(t, e.diags.HasErrors(), "%s: %v", ref.ID, e.diags)
	require.NoError(t, e.err, ref.ID)
	return e
}

// warmShipped builds several shipped workflows at once (a few at a time);
// the tests then read them with shippedBuildWith, and report any failure.
func warmShipped(t *testing.T, items map[workflow.Ref][]shippedOpts) {
	t.Helper()
	hub, cat, env := shippedWorkflows(t)
	sem := make(chan struct{}, min(runtime.GOMAXPROCS(0), 4))
	var wg sync.WaitGroup
	for ref, list := range items {
		for _, o := range list {
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				entryOf(hub, cat, env, ref, o)
			}()
		}
	}
	wg.Wait()
}

// warmRefs is warmShipped for every shipped workflow, for each project.
func warmRefs(t *testing.T, projects ...string) {
	t.Helper()
	_, cat, _ := shippedWorkflows(t)
	items := map[workflow.Ref][]shippedOpts{}
	for _, ref := range cat.Refs() {
		for _, p := range projects {
			items[ref] = append(items[ref], shippedOpts{Project: p})
		}
	}
	warmShipped(t, items)
}

// shippedBuildWith is the shipped workflow ref, checked then built once for
// the options; the result is shared and must not be modified.
func shippedBuildWith(t *testing.T, ref workflow.Ref, o shippedOpts) (*workflow.Resolved, *Bundle) {
	t.Helper()
	e := sharedEntryOf(t, ref, o)
	return e.r, e.b
}

// shippedBuild is shippedBuildWith for a project (see capProject).
func shippedBuild(t *testing.T, project string, ref workflow.Ref) (*workflow.Resolved, *Bundle) {
	t.Helper()
	return shippedBuildWith(t, ref, shippedOpts{Project: project})
}

// shippedReach is reachableSkills of a shipped workflow built by
// shippedBuild, computed once.
func shippedReach(t *testing.T, project string, ref workflow.Ref) map[string][]sessionspec.SkillDef {
	t.Helper()
	e := sharedEntryOf(t, ref, shippedOpts{Project: project})
	hub, _, _ := shippedWorkflows(t)
	e.reachOnce.Do(func() { e.reach = reachableSkills(t, hub, e.r.Spec, e.b, project) })
	require.NotNil(t, e.reach, ref.ID)
	return e.reach
}

type agentKey struct{ entry, provider string }

type agentEntry struct {
	once sync.Once
	b    *Bundle
	err  error
}

// agentBuild is the bundle of a free session on entry (agentSpec), built
// once for the tests that only read it; it must not be modified.
func agentBuild(t *testing.T, entry, provider string) *Bundle {
	t.Helper()
	hub := repoHub(t)
	spec := agentSpec(t, hub, entry)
	v, _ := shared.builds.LoadOrStore(agentKey{entry, provider}, &agentEntry{})
	e := v.(*agentEntry)
	e.once.Do(func() {
		e.b, e.err = Build(Request{HubDir: hub, OutDir: filepath.Join(shared.root, "bundles"), Spec: spec, Provider: provider})
		if e.err == nil {
			e.err = fingerprintDir(e.b.Dir)
		}
	})
	require.NoError(t, e.err, entry)
	return e.b
}
