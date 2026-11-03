package views

import (
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/worktree"
)

// The letter keys of the projects and worktrees views, pinned before they went
// through the keys package.

// inputModalShell records the input modal asked by a view.
type inputModalShell struct {
	recordingShell
	title, current string
	confirm        func(string)
}

func (s *inputModalShell) ShowInputModal(title, current string, onConfirm func(string)) {
	s.title, s.current, s.confirm = title, current, onConfirm
}

func TestProjectsViewLetterKeys(t *testing.T) {
	cfg := ProjectsViewConfig{Projects: []ProjectItem{
		{ID: "p1", Name: "first", Path: "/tmp/first"},
		{ID: "p2", Name: "second", Path: "/tmp/second"},
	}}
	v := NewProjectsView(cfg)
	sh := &inputModalShell{}
	v.SetShell(sh)
	v.list = tview.NewList()
	v.detail = tview.NewTextView()
	for _, p := range cfg.Projects {
		v.list.AddItem(p.Name, p.Path, 0, nil)
	}
	v.list.SetCurrentItem(1)

	var entered *ActiveProject
	v.SetOnEnterProject(func(p *ActiveProject) { entered = p })
	var beads []string
	v.SetOnInitBeads(func(id, name, path string) { beads = []string{id, name, path} })
	var moved []string
	v.SetOnMove(func(id, path string) { moved = []string{id, path} })

	assert.Nil(t, v.HandleKey(runeKey('p')), "p is consumed")
	require.NotNil(t, entered)
	assert.Equal(t, "p2", entered.ID, "p enters the project mode on the project under the cursor")

	assert.Nil(t, v.HandleKey(runeKey('b')), "b is consumed")
	assert.Equal(t, []string{"p2", "second", "/tmp/second"}, beads, "b initialises beads for the project under the cursor")

	assert.Nil(t, v.HandleKey(runeKey('m')), "m is consumed")
	require.NotNil(t, sh.confirm, "m asks the new path")
	assert.Equal(t, "/tmp/second", sh.current, "starting from the current path")
	sh.confirm("/tmp/elsewhere")
	assert.Equal(t, []string{"p2", "/tmp/elsewhere"}, moved)

	other := runeKey('z')
	assert.Same(t, other, v.HandleKey(other), "another letter goes on")
}

func TestWorktreeViewOpenKey(t *testing.T) {
	git := &fakeWorktrees{entries: []worktree.Entry{
		{Path: "/p", Branch: "main"},
		{Path: "/p-a", Branch: "feat/a"},
	}}
	opened := make(chan string, 1)
	v := NewWorktreeView(nil, WorktreeViewConfig{Git: git, OpenSession: func(path string) { opened <- path }})
	sh := &confirmShell{}
	sh.project = &ActiveProject{ID: "p", Path: "/p"}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	require.Eventually(t, func() bool {
		n := 0
		onLoop(app, func() { n = len(v.items) })
		return n == 1
	}, 2*time.Second, 10*time.Millisecond)

	onLoop(app, func() {
		v.list.SetCurrentItem(1) // feat/a, after the header of the main worktree
		assert.Nil(t, v.HandleKey(runeKey('o')), "o is consumed")
		other := runeKey('z')
		assert.Same(t, other, v.HandleKey(other), "another letter goes on")
	})
	select {
	case path := <-opened:
		assert.Equal(t, "/p-a", path, "o opens a session in the worktree under the cursor")
	case <-time.After(2 * time.Second):
		t.Fatal("o did not open the worktree")
	}
}
