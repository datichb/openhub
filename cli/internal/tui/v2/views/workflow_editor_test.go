package views

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
	"github.com/datichb/openhub/cli/internal/workflow"
)

const editorDoc = `apiVersion: oh/v1
kind: Workflow
id: ticket-hotfix
# Hotfix of the hub ticket.
extends: hub:ticket
description: Hotfix
`

const editorParent = `apiVersion: oh/v1
kind: Workflow
id: ticket
risk: write
entry: { agent: orchestrator-dev }
agents:
  orchestrator-dev: { role: workflow }
  developer: { role: workflow, after: cp-1 }
  reviewer: { role: workflow, after: developer }
  documentarian: { role: independent }
checkpoints:
  cp-1: { label: Démarrer, mode: { manuel: pause, semi-auto: auto } }
  cp-2: { label: Commit, mandatory: true, remote: defer }
modes: { default: semi-auto, allowed: [manuel, semi-auto] }
`

// editorCheckOf resolves the edited text over editorParent (test check).
func editorCheckOf(t *testing.T, yaml []byte) *EditorCheck {
	t.Helper()
	cat := workflow.NewMemCatalog()
	parent, diags := workflow.Parse([]byte(editorParent), workflow.Source{Layer: workflow.LayerHub})
	require.False(t, diags.HasErrors(), "%v", diags)
	cat.Put(parent)
	chk := &EditorCheck{}
	doc, diags := workflow.Parse(yaml, workflow.Source{Layer: workflow.LayerTeam})
	for _, d := range diags {
		chk.Diagnostics = append(chk.Diagnostics, EditorDiag{Error: d.Severity == workflow.SeverityError, Path: d.Path, Line: d.Pos.Line, Message: d.Message})
	}
	if doc == nil || diags.HasErrors() {
		return chk
	}
	cat.Put(doc)
	r, ds := workflow.ResolveSpec(cat, doc.Ref(), nil)
	for _, d := range ds {
		chk.Diagnostics = append(chk.Diagnostics, EditorDiag{Error: d.Severity == workflow.SeverityError, Path: d.Path, Message: d.Message})
	}
	if r != nil {
		chk.Spec, chk.Origins = r.Spec, r.Origins
	}
	return chk
}

type editorShell struct {
	formShell
	selectFn  func(string)
	selectOpt []SelectOption
	multiFn   func([]string)
}

func (s *editorShell) ShowSelectModal(_ string, o []SelectOption, _ string, fn func(string)) {
	s.selectOpt, s.selectFn = o, fn
}
func (s *editorShell) ShowMultiSelectModal(_ string, _ []SelectOption, _ []string, fn func([]string)) {
	s.multiFn = fn
}

type editorEnv struct {
	v     *WorkflowEditorView
	sh    *editorShell
	app   *tview.Application
	mu    sync.Mutex
	saved [][]byte
}

func newEditorEnv(t *testing.T, cfg WorkflowEditorConfig) *editorEnv {
	t.Helper()
	env := &editorEnv{sh: &editorShell{}}
	cfg.Check = func(_ context.Context, yaml, _ []byte) (*EditorCheck, error) { return editorCheckOf(t, yaml), nil }
	cfg.Save = func(_ context.Context, yaml, _ []byte) (*EditorSaved, error) {
		env.mu.Lock()
		env.saved = append(env.saved, yaml)
		env.mu.Unlock()
		return &EditorSaved{Ref: cfg.Ref, Pushed: true}, nil
	}
	if cfg.Ref == "" {
		cfg.Ref, cfg.Layer = "team:ticket-hotfix", workflow.LayerTeam
	}
	if cfg.YAML == nil {
		cfg.YAML = []byte(editorDoc)
	}
	cfg.Agents = append(cfg.Agents, "debugger", "developer")
	env.v = NewWorkflowEditorView(cfg)
	env.v.SetShell(env.sh)
	content := tview.NewFlex()
	env.app = runApp(t, content)
	onLoop(env.app, func() { env.v.Mount(content, env.app) })
	env.waitCheck(t)
	return env
}

// waitCheck waits for the check of the current text.
func (e *editorEnv) waitCheck(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		var ok bool
		onLoop(e.app, func() { ok = e.v.check != nil && !e.v.checking })
		return ok
	}, 3*time.Second, 20*time.Millisecond)
}

func (e *editorEnv) key(k tcell.Key, r rune) {
	onLoop(e.app, func() { e.v.HandleKey(tcell.NewEventKey(k, r, tcell.ModNone)) })
}

func (e *editorEnv) text() string {
	var s string
	onLoop(e.app, func() { s = string(e.v.m.cur.yaml) })
	return s
}

// selectField selects the field with label in the current list.
func (e *editorEnv) selectField(t *testing.T, label string) {
	t.Helper()
	onLoop(e.app, func() {
		for i, it := range e.v.list.GetItems() {
			if f, ok := it.Reference.(*edField); ok && f.label == label {
				e.v.list.SelectIndex(i)
				return
			}
		}
		t.Fatalf("no field %q", label)
	})
}

func TestEditorModelUndoRedo(t *testing.T) {
	m := newEditorModel([]byte(editorDoc), nil, false)
	assert.False(t, m.dirty())
	require.NoError(t, m.edit(func(e *workflow.DocEdit) error { return e.Set("read", "risk") }))
	require.NoError(t, m.edit(func(e *workflow.DocEdit) error { return e.Set("strict", "isolation") }))
	assert.True(t, m.dirty())
	assert.Contains(t, string(m.cur.yaml), "# Hotfix of the hub ticket.", "comments kept")
	assert.True(t, m.undoOnce())
	assert.NotContains(t, string(m.cur.yaml), "isolation")
	assert.True(t, m.redoOnce())
	assert.Contains(t, string(m.cur.yaml), "isolation: strict")
	assert.False(t, m.redoOnce())
	m.undoOnce()
	m.undoOnce()
	assert.False(t, m.dirty(), "back to the saved text")
	assert.False(t, m.undoOnce())
	assert.Nil(t, m.savePrompt(), "prompt unchanged: kept")
	m.setPrompt([]byte("Hotfix {{ .ticket }}"))
	assert.Equal(t, "Hotfix {{ .ticket }}", string(m.savePrompt()))
	m.markSaved()
	assert.False(t, m.dirty())
	assert.True(t, newEditorModel(nil, nil, true).dirty(), "a new workflow is unsaved")
}

func TestWorkflowEditorGeneralUndoSave(t *testing.T) {
	env := newEditorEnv(t, WorkflowEditorConfig{})
	// Inherited value and origin are shown.
	var items string
	onLoop(env.app, func() {
		for _, it := range env.v.list.GetItems() {
			items += it.MainText + " " + it.SecondaryText + "\n"
		}
	})
	assert.Contains(t, items, "write")
	assert.Contains(t, items, "hub:ticket")

	env.selectField(t, "Risk")
	onLoop(env.app, func() { env.v.activate(env.v.selectedRef()) })
	require.NotNil(t, env.sh.selectFn)
	onLoop(env.app, func() { env.sh.selectFn("read") })
	assert.Contains(t, env.text(), "risk: read")
	assert.Contains(t, env.text(), "# Hotfix of the hub ticket.")

	env.key(tcell.KeyRune, 'u')
	assert.NotContains(t, env.text(), "risk:")
	env.key(tcell.KeyRune, 'U')
	assert.Contains(t, env.text(), "risk: read")

	// Esc with changes: guard (save / discard / keep editing).
	env.waitCheck(t)
	env.key(tcell.KeyEscape, 0)
	require.Len(t, env.sh.modal, 3)
	assert.Zero(t, env.sh.popped)
	env.key(tcell.KeyRune, 'w')
	require.Eventually(t, func() bool { env.mu.Lock(); defer env.mu.Unlock(); return len(env.saved) == 1 }, 2*time.Second, 20*time.Millisecond)
	assert.Contains(t, string(env.saved[0]), "risk: read")
	var dirty bool
	require.Eventually(t, func() bool { onLoop(env.app, func() { dirty = env.v.Dirty() }); return !dirty }, 2*time.Second, 20*time.Millisecond,
		"clean once the save is applied")
	env.key(tcell.KeyEscape, 0)
	assert.Equal(t, 1, env.sh.popped, "no guard once saved")

	// x goes back to the inherited value.
	env.selectField(t, "Risk")
	env.key(tcell.KeyRune, 'x')
	assert.NotContains(t, env.text(), "risk:")
}

// A39: Code Mode is offered as on/off, and the written value is shown even
// when the resolution refuses it (loosening: the parent value is kept).
func TestWorkflowEditorCodeModeShowsWrittenValue(t *testing.T) {
	env := newEditorEnv(t, WorkflowEditorConfig{})
	env.selectField(t, "Code Mode")
	onLoop(env.app, func() { env.v.activate(env.v.selectedRef()) })
	require.NotNil(t, env.sh.selectFn)
	var labels []string
	for _, o := range env.sh.selectOpt[1:] {
		labels = append(labels, o.Label+"="+o.Value)
	}
	assert.Equal(t, []string{"on=true", "off=false"}, labels)
	onLoop(env.app, func() { env.sh.selectFn("true") })
	assert.Contains(t, env.text(), "code_mode: true")
	env.waitCheck(t)
	var line string
	var loosening bool
	onLoop(env.app, func() {
		for _, it := range env.v.list.GetItems() {
			if f, ok := it.Reference.(*edField); ok && f.label == "Code Mode" {
				line = it.MainText
			}
		}
		for _, d := range env.v.check.Diagnostics {
			loosening = loosening || (d.Error && strings.Contains(d.Path, "code_mode"))
		}
	})
	assert.True(t, loosening, "the loosening is reported")
	assert.Contains(t, line, "✎")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(line), " on"), line)
}

func TestWorkflowEditorInvalidAndJump(t *testing.T) {
	env := newEditorEnv(t, WorkflowEditorConfig{YAML: []byte(editorDoc + "risk: read\n"), Unsaved: true})
	// The test check only resolves: findings are forced.
	onLoop(env.app, func() {
		env.v.SetCheck(&EditorCheck{Spec: env.v.check.Spec, Origins: env.v.check.Origins,
			Diagnostics: []EditorDiag{{Error: true, Path: "agents.developer.after", Message: "bad"}, {Error: true, Path: "risk", Message: "x"}}})
	})
	env.key(tcell.KeyRune, 'w')
	env.mu.Lock()
	assert.Empty(t, env.saved, "invalid: not saved")
	env.mu.Unlock()
	var section int
	onLoop(env.app, func() { section = env.v.section })
	assert.Equal(t, edPreview, section)

	// Enter on the first finding jumps to the graph, on the agent.
	onLoop(env.app, func() {
		env.v.list.SelectIndex(0)
		_, it, _ := env.v.list.CurrentItem()
		env.v.jumpTo(it.Reference.(EditorDiag))
	})
	var sel string
	onLoop(env.app, func() {
		section = env.v.section
		if e := env.v.graph.SelectedElement(); e != nil {
			sel = e.ID
		}
	})
	assert.Equal(t, edGraph, section)
	assert.Equal(t, "developer", sel)
	// Esc with an invalid draft: no « save » choice.
	env.key(tcell.KeyEscape, 0)
	assert.Len(t, env.sh.modal, 2)
}

func TestWorkflowEditorGraph(t *testing.T) {
	env := newEditorEnv(t, WorkflowEditorConfig{})
	env.key(tcell.KeyTab, 0)
	var ids []string
	onLoop(env.app, func() {
		require.NotNil(t, env.v.graph)
		for _, n := range widgets.ComputeModelLayout(env.v.graphModel()).Nodes {
			ids = append(ids, n.ID)
		}
	})
	assert.Equal(t, []string{"▶", "cp-1", "cp-2", "orchestrator-dev", "developer", "reviewer", "documentarian"}, ids,
		"the resolved workflow (inherited elements included, B10): reviewer follows developer under cp-1")

	// a: add an agent of the catalogue.
	env.key(tcell.KeyRune, 'a')
	require.NotNil(t, env.sh.selectFn)
	assert.Equal(t, "debugger", env.sh.selectOpt[0].Value, "agents already in the workflow are not offered")
	onLoop(env.app, func() { env.sh.selectFn("debugger") })
	assert.Contains(t, env.text(), "debugger:\n    role: workflow")

	// x on an inherited checkpoint: disabled in the draft.
	env.waitCheck(t)
	onLoop(env.app, func() { env.v.graph.Select("cp-1") })
	env.key(tcell.KeyRune, 'x')
	require.NotEmpty(t, env.sh.modal)
	onLoop(env.app, func() { env.sh.modal[0].Callback() })
	assert.Contains(t, env.text(), "cp-1:\n    disabled: true")

	// m cycles the shown mode.
	env.waitCheck(t)
	var before, after string
	onLoop(env.app, func() { before = env.v.graphMode() })
	env.key(tcell.KeyRune, 'm')
	onLoop(env.app, func() { after = env.v.graphMode() })
	assert.NotEqual(t, before, after)
}

func TestWorkflowEditorInputsAndRaw(t *testing.T) {
	var external []string
	env := newEditorEnv(t, WorkflowEditorConfig{External: func(name string, content []byte, line int) ([]byte, error) {
		external = append(external, name)
		if strings.HasSuffix(name, ".tmpl") {
			return []byte("Hotfix {{ .ticket }}"), nil
		}
		return []byte(strings.Replace(string(content), "description: Hotfix", "description: Raw", 1)), nil
	}})
	env.key(tcell.KeyTab, 0)
	env.key(tcell.KeyTab, 0)
	env.key(tcell.KeyRune, 'a')
	require.NotNil(t, env.sh.input)
	onLoop(env.app, func() { env.sh.input("severity") })
	assert.Contains(t, env.text(), "severity:\n    type: string")

	env.key(tcell.KeyRune, 'P')
	assert.Contains(t, env.text(), "template: prompts/ticket-hotfix.md.tmpl", "a template reference is added")
	var prompt []byte
	onLoop(env.app, func() { prompt = env.v.m.savePrompt() })
	assert.Equal(t, "Hotfix {{ .ticket }}", string(prompt))

	env.key(tcell.KeyRune, 'y')
	assert.Contains(t, env.text(), "description: Raw")
	assert.Equal(t, []string{"prompt.md.tmpl", "ticket-hotfix.yaml"}, external)
}
