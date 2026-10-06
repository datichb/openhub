package views

import (
	"bytes"

	"github.com/datichb/openhub/cli/internal/workflow"
)

// Editing state of the workflow editor (P2-T14): the YAML text and the
// draft's own prompt template, undo/redo snapshots, saved state. Pure (no
// UI): edits go through workflow.DocEdit so that comments and the presence
// of each field are kept.

type editorSnapshot struct {
	yaml   []byte
	prompt []byte // own prompt template (nil: none / unchanged)
}

func (s editorSnapshot) equal(o editorSnapshot) bool {
	return bytes.Equal(s.yaml, o.yaml) && bytes.Equal(s.prompt, o.prompt)
}

type editorModel struct {
	cur, saved editorSnapshot
	// unsaved: the text is not a saved draft yet (new workflow).
	unsaved bool
	// promptChanged: the prompt template was edited (sent on save).
	promptChanged bool
	undo, redo    []editorSnapshot
}

const editorUndoMax = 100

func newEditorModel(yaml, prompt []byte, unsaved bool) *editorModel {
	s := editorSnapshot{yaml: append([]byte(nil), yaml...), prompt: prompt}
	return &editorModel{cur: s, saved: s, unsaved: unsaved}
}

// dirty reports unsaved changes.
func (m *editorModel) dirty() bool { return m.unsaved || !m.cur.equal(m.saved) }

func (m *editorModel) push() {
	m.undo = append(m.undo, m.cur)
	if len(m.undo) > editorUndoMax {
		m.undo = m.undo[1:]
	}
	m.redo = nil
}

// edit applies fn to the document; nothing changes when fn fails or leaves
// the text as it was.
func (m *editorModel) edit(fn func(*workflow.DocEdit) error) error {
	e, err := workflow.ParseDocEdit(m.cur.yaml)
	if err != nil {
		return err
	}
	if err := fn(e); err != nil {
		return err
	}
	out, err := e.Bytes()
	if err != nil {
		return err
	}
	m.setYAML(out)
	return nil
}

// setYAML replaces the text (raw edit).
func (m *editorModel) setYAML(b []byte) {
	if bytes.Equal(b, m.cur.yaml) {
		return
	}
	m.push()
	m.cur.yaml = b
}

// setPrompt replaces the draft's own prompt template.
func (m *editorModel) setPrompt(b []byte) {
	if bytes.Equal(b, m.cur.prompt) {
		return
	}
	m.push()
	m.cur.prompt = b
	m.promptChanged = true
}

func (m *editorModel) undoOnce() bool {
	if len(m.undo) == 0 {
		return false
	}
	m.redo = append(m.redo, m.cur)
	m.cur = m.undo[len(m.undo)-1]
	m.undo = m.undo[:len(m.undo)-1]
	return true
}

func (m *editorModel) redoOnce() bool {
	if len(m.redo) == 0 {
		return false
	}
	m.undo = append(m.undo, m.cur)
	m.cur = m.redo[len(m.redo)-1]
	m.redo = m.redo[:len(m.redo)-1]
	return true
}

func (m *editorModel) markSaved() {
	m.saved, m.unsaved = m.cur, false
}

// savePrompt is the prompt to send on save (nil keeps the current one).
func (m *editorModel) savePrompt() []byte {
	if !m.promptChanged {
		return nil
	}
	if m.cur.prompt == nil {
		return []byte{}
	}
	return m.cur.prompt
}

// document parses the current text (nil when it does not parse).
func (m *editorModel) document() *workflow.Document {
	doc, _ := workflow.Parse(m.cur.yaml, workflow.Source{Layer: workflow.LayerTeam})
	return doc
}
