package views

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/worktree"
)

// fakeWorktrees records the git calls of the worktree view.
type fakeWorktrees struct {
	mu      sync.Mutex
	entries []worktree.Entry
	removed []string
	pruned  int
	kept    []string
}

func (f *fakeWorktrees) List(string) ([]worktree.Entry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]worktree.Entry(nil), f.entries...), nil
}
func (f *fakeWorktrees) Create(string, string) error { return nil }
func (f *fakeWorktrees) Remove(_, path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, path)
	return nil
}
func (f *fakeWorktrees) Prune(string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pruned++
	return nil
}
func (f *fakeWorktrees) CleanupMerged(_ string, keep func(string) bool) (WorktreeCleanup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var r WorktreeCleanup
	for _, e := range f.entries[1:] {
		if keep(e.Path) {
			f.kept = append(f.kept, e.Path)
			r.InUse = append(r.InUse, e.Branch)
			continue
		}
		f.removed = append(f.removed, e.Path)
		r.Removed = append(r.Removed, e.Branch)
	}
	return r, nil
}

func (f *fakeWorktrees) state() (removed []string, pruned int, kept []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.removed...), f.pruned, append([]string(nil), f.kept...)
}

// confirmShell records the confirmations asked by a view.
type confirmShell struct {
	recordingShell
	cmu     sync.Mutex
	titles  []string
	pending func()
}

func (s *confirmShell) Confirm(title, _ string, _ bool, onYes func()) {
	s.cmu.Lock()
	defer s.cmu.Unlock()
	s.titles = append(s.titles, title)
	s.pending = onYes
}

func (s *confirmShell) take() (titles []string, yes func()) {
	s.cmu.Lock()
	defer s.cmu.Unlock()
	return append([]string(nil), s.titles...), s.pending
}

// Worktrees: d, p and x ask first; nothing happens before the
// confirmation; x keeps a worktree used by a session (N19).
func TestWorktreeViewConfirmsDestructiveKeys(t *testing.T) {
	git := &fakeWorktrees{entries: []worktree.Entry{
		{Path: "/p", Branch: "main"},
		{Path: "/p-a", Branch: "feat/a"},
		{Path: "/p-b", Branch: "feat/b"},
	}}
	v := NewWorktreeView(nil, WorktreeViewConfig{Git: git,
		InUse: func(context.Context, string) map[string]bool { return map[string]bool{"/p-b": true} }})
	sh := &confirmShell{}
	sh.project = &ActiveProject{ID: "p", Path: "/p"}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	require.Eventually(t, func() bool {
		n := 0
		onLoop(app, func() { n = len(v.items) })
		return n == 2
	}, 2*time.Second, 10*time.Millisecond)

	press := func(r rune) {
		onLoop(app, func() { assert.Nil(t, v.HandleKey(tcell.NewEventKey(tcell.KeyRune, r, 0))) })
	}
	onLoop(app, func() { v.list.SetCurrentItem(1) }) // feat/a (after the main header)
	press('d')
	titles, yes := sh.take()
	require.Len(t, titles, 1)
	removed, _, _ := git.state()
	assert.Empty(t, removed, "nothing removed before the confirmation")
	onLoop(app, yes)
	require.Eventually(t, func() bool { r, _, _ := git.state(); return len(r) == 1 }, 2*time.Second, 10*time.Millisecond)
	removed, _, _ = git.state()
	assert.Equal(t, []string{"/p-a"}, removed)

	press('p')
	_, yes = sh.take()
	_, pruned, _ := git.state()
	assert.Zero(t, pruned)
	onLoop(app, yes)
	require.Eventually(t, func() bool { _, p, _ := git.state(); return p == 1 }, 2*time.Second, 10*time.Millisecond)

	press('x')
	titles, yes = sh.take()
	assert.Len(t, titles, 3)
	onLoop(app, yes)
	require.Eventually(t, func() bool { _, _, k := git.state(); return len(k) == 1 }, 2*time.Second, 10*time.Millisecond)
	removed, _, kept := git.state()
	assert.Equal(t, []string{"/p-b"}, kept, "used by a session: kept")
	assert.NotContains(t, removed[1:], "/p-b")
}
