package views

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Notes under the inputs of the launch form (modernisation tui-launch): the
// type error that blocks the launch (« Entier attendu »), the warning of a
// path or branch not found, what oh computes for an input with `from:`
// (« ⚙ calculé par oh : session.output(spec.spec) → ses_41c0 »), the notes
// of the wiring layer, the choices of an enum and the `help` of the input.

// inputCheckDelay waits for the typing to pause before checking a path or
// a branch.
const inputCheckDelay = 300 * time.Millisecond

// inputNotes is the state of the notes of the inputs step.
type inputNotes struct {
	notes map[string]*formNote
	// previews are the values computed by oh (FromPreview), loaded once.
	previews     map[string]string
	previewed    bool
	previewLoads loadSlot
	// checkLoads number the reads of CheckInput, per input.
	checkLoads map[string]*loadSlot
	// checks are the results of CheckInput, per input; gen the last check
	// asked per input.
	checks map[string]*inputCheck
	mu     sync.Mutex
	gen    map[string]int
}

// nextGen starts a check of input k; current tells whether it is still
// the last one asked.
func (n *inputNotes) nextGen(k string) (current func() bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.gen[k]++
	g := n.gen[k]
	return func() bool {
		n.mu.Lock()
		defer n.mu.Unlock()
		return n.gen[k] == g
	}
}

// checkSlot is the slot of the reads of input k (event loop only).
func (n *inputNotes) checkSlot(k string) *loadSlot {
	if n.checkLoads == nil {
		n.checkLoads = map[string]*loadSlot{}
	}
	if n.checkLoads[k] == nil {
		n.checkLoads[k] = &loadSlot{}
	}
	return n.checkLoads[k]
}

type inputCheck struct{ value, warning string }

// reset forgets the notes of a previous rendering (the previews and the
// checks stay).
func (n *inputNotes) reset(*tview.Form) {
	n.notes = map[string]*formNote{}
	if n.previews == nil {
		n.previews = map[string]string{}
	}
	if n.checks == nil {
		n.checks = map[string]*inputCheck{}
	}
	n.mu.Lock()
	if n.gen == nil {
		n.gen = map[string]int{}
	}
	n.mu.Unlock()
}

// add puts the note of input k ("" = not an input) after its field.
func (n *inputNotes) add(f *tview.Form, k, text string) {
	note := newFormNote(f, text)
	f.AddFormItem(note)
	if k != "" {
		n.notes[k] = note
	}
}

// given reports whether input k has a value in the form.
func (m *launchModel) given(k string) bool {
	if k == m.ticketInput {
		return len(m.tickets) > 0
	}
	return strings.TrimSpace(m.values[k]) != ""
}

// plainLabel is the label of input k, without the required mark.
func (m *launchModel) plainLabel(k string) string {
	in, _ := m.spec.Inputs.Get(k)
	if l := in.Label.Text(m.lang); l != "" {
		return l
	}
	return k
}

// noteText is the note under input k (tview tags).
func (v *LaunchFormView) noteText(k string) string {
	m := v.m
	in, _ := m.spec.Inputs.Get(k)
	var lines []string
	if e := m.typeError(k); e != "" {
		lines = append(lines, theme.Paint(theme.ErrorHex, theme.SymFailure+" "+tview.Escape(e)))
	} else if c := v.inputs.checks[k]; c != nil && c.warning != "" && c.value == strings.TrimSpace(m.values[k]) {
		lines = append(lines, theme.Paint(theme.WarningHex, theme.SymWarning+" "+tview.Escape(c.warning)))
	}
	if in.From != "" && !m.given(k) {
		what, ok := v.inputs.previews[k]
		if !ok {
			what = in.From
			if v.cfg.FromPreview != nil {
				what += " …"
			}
		}
		lines = append(lines, theme.Paint(theme.InfoHex, "⚙ "+tview.Escape(i18n.Tf("tui.launch.input.from", what))))
	}
	if note := v.cfg.InputNotes[k]; note != "" {
		lines = append(lines, theme.Paint(theme.WarningHex, tview.Escape(note)))
	}
	help := in.Help.Text(m.lang)
	if in.Type == workflow.InputEnum && len(in.Values) > 1 {
		choices := strings.Join(in.Values, " · ")
		if help == "" {
			help = choices
		} else {
			help = choices + " — " + help
		}
	}
	if help != "" {
		lines = append(lines, muted(tview.Escape(help)))
	}
	return strings.Join(lines, "\n")
}

// refreshNote rewrites the note of input k.
func (v *LaunchFormView) refreshNote(k string) {
	if n := v.inputs.notes[k]; n != nil {
		n.SetText(v.noteText(k))
	}
}

// inputChanged follows a value typed in input k: its note, the check of a
// path or a branch, and the notes of the inputs computed by oh (shown only
// while they have no value).
func (v *LaunchFormView) inputChanged(k string) {
	v.refreshNote(k)
	v.checkInput(k)
}

// loadFromPreviews asks, once, what oh will compute for the inputs with
// `from:`.
func (v *LaunchFormView) loadFromPreviews() {
	if v.cfg.FromPreview == nil || v.inputs.previewed || v.app == nil {
		return
	}
	var ks []string
	for _, k := range v.m.spec.Inputs.Keys() {
		if in, _ := v.m.spec.Inputs.Get(k); in.From != "" {
			ks = append(ks, k)
		}
	}
	if len(ks) == 0 {
		return
	}
	v.inputs.previewed = true
	values := make(map[string]string, len(v.m.values))
	for k, val := range v.m.values {
		values[k] = val
	}
	load(v.app, v.ctx, &v.inputs.previewLoads, func(ctx context.Context) (map[string]string, error) {
		out := map[string]string{}
		for _, k := range ks {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			out[k] = v.cfg.FromPreview(ctx, k, values)
		}
		return out, nil
	}, func(out map[string]string, err error) {
		if err != nil {
			return
		}
		for k, text := range out {
			if text != "" {
				v.inputs.previews[k] = text
			}
			v.refreshNote(k)
		}
	})
}

// checkInput checks the path or branch of input k off the event loop, once
// the typing pauses.
func (v *LaunchFormView) checkInput(k string) {
	in, _ := v.m.spec.Inputs.Get(k)
	if v.cfg.CheckInput == nil || v.app == nil || (in.Type != workflow.InputPath && in.Type != workflow.InputBranch) {
		return
	}
	value := strings.TrimSpace(v.m.values[k])
	current, app, ctx := v.inputs.nextGen(k), v.app, v.ctx
	if value == "" {
		v.inputs.checkSlot(k).Invalidate()
		v.inputs.checks[k] = &inputCheck{}
		v.refreshNote(k)
		return
	}
	slot := v.inputs.checkSlot(k)
	time.AfterFunc(inputCheckDelay, func() {
		if ctx.Err() != nil || !current() {
			return
		}
		app.QueueUpdate(func() { // the typing paused: read the input, on the loop
			if !current() {
				return
			}
			load(app, ctx, slot, func(ctx context.Context) (string, error) {
				return v.cfg.CheckInput(ctx, in, value), nil
			}, func(w string, _ error) {
				if !current() {
					return
				}
				v.inputs.checks[k] = &inputCheck{value: value, warning: w}
				v.refreshNote(k)
			})
		})
	})
}
