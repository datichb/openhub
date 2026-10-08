package views

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Fields of the workflow editor sections and their edits.

// edField is an editable field of a section.
type edField struct {
	label string
	help  string
	// path is the YAML path written by the field (nil: action).
	path []string
	// lock is the top-level field whose `enforce` lock applies.
	lock   string
	header bool
	// value renders the resolved value.
	value func(sp *workflow.Spec) string
	// boolean: true/false shown on/off (as `oh workflow show`).
	boolean bool
	// edit asks for a new value (Enter).
	edit func()
	// remove overrides the default removal (x).
	remove func()
	// action replaces the edit (an « add » or « $EDITOR » line).
	action func()
}

// unsetValue is the choice that removes a field from the document.
const unsetValue = "\x00unset"

func (v *WorkflowEditorView) unsetOption() SelectOption {
	return SelectOption{Label: i18n.T("tui.editor.unset"), Value: unsetValue}
}

// setOrUnset writes val at path, or removes the field for unsetValue / "".
func (v *WorkflowEditorView) setOrUnset(val any, path ...string) {
	v.edit(func(e *workflow.DocEdit) error {
		if s, ok := val.(string); ok && (s == unsetValue || s == "") {
			e.Unset(path...)
			return nil
		}
		return e.Set(val, path...)
	})
}

// current returns the value written in the document at path ("" if none).
func (v *WorkflowEditorView) current(path ...string) string {
	e, err := workflow.ParseDocEdit(v.m.cur.yaml)
	if err != nil {
		return ""
	}
	if s, ok := e.Scalar(path...); ok {
		return s
	}
	if l, ok := e.Strings(path...); ok {
		return strings.Join(l, ", ")
	}
	return ""
}

// textField edits a scalar string.
func (v *WorkflowEditorView) textField(label, help, lock string, value func(*workflow.Spec) string, path ...string) *edField {
	f := &edField{label: label, help: help, path: path, lock: lock, value: value}
	f.edit = func() {
		cur := v.current(path...)
		if cur == "" && value != nil {
			cur = value(v.spec())
		}
		v.shell.ShowInputModal(label, cur, func(s string) { v.setOrUnset(strings.TrimSpace(s), path...) })
	}
	return f
}

// selectField edits a scalar among options.
func (v *WorkflowEditorView) selectField(label, help, lock string, opts func() []string, value func(*workflow.Spec) string, path ...string) *edField {
	f := &edField{label: label, help: help, path: path, lock: lock, value: value}
	f.edit = func() {
		choices := []SelectOption{v.unsetOption()}
		for _, o := range opts() {
			choices = append(choices, SelectOption{Label: o, Value: o})
		}
		cur := v.current(path...)
		if cur == "" {
			cur = unsetValue
		}
		v.shell.ShowSelectModal(label, choices, cur, func(s string) { v.setOrUnset(s, path...) })
	}
	return f
}

// boolField edits a boolean (on / off / inherited).
func (v *WorkflowEditorView) boolField(label, help, lock string, value func(*workflow.Spec) string, path ...string) *edField {
	f := &edField{label: label, help: help, path: path, lock: lock, value: value, boolean: true}
	f.edit = func() {
		choices := []SelectOption{v.unsetOption(), {Label: "on", Value: "true"}, {Label: "off", Value: "false"}}
		cur := v.current(path...)
		if cur == "" {
			cur = unsetValue
		}
		v.shell.ShowSelectModal(label, choices, cur, func(s string) {
			if s == unsetValue {
				v.setOrUnset(s, path...)
				return
			}
			v.setOrUnset(s == "true", path...)
		})
	}
	return f
}

// listField edits a list of strings, among known options when given
// (multi-select), else typed comma-separated.
func (v *WorkflowEditorView) listField(label, help, lock string, opts func() []string, value func(*workflow.Spec) []string, path ...string) *edField {
	f := &edField{label: label, help: help, path: path, lock: lock, value: func(sp *workflow.Spec) string {
		if value == nil {
			return ""
		}
		return strings.Join(value(sp), ", ")
	}}
	f.edit = func() {
		cur := splitList(v.current(path...))
		if len(cur) == 0 && value != nil {
			cur = value(v.spec())
		}
		apply := func(list []string) {
			v.edit(func(e *workflow.DocEdit) error {
				if list == nil {
					e.Unset(path...)
					return nil
				}
				return e.Set(list, path...)
			})
		}
		if opts == nil {
			v.shell.ShowInputModal(label+" "+i18n.T("tui.editor.list_hint"), strings.Join(cur, ", "), func(s string) {
				l := splitList(s)
				if strings.TrimSpace(s) == "" {
					l = nil
				}
				apply(l)
			})
			return
		}
		var choices []SelectOption
		known := opts()
		for _, c := range cur {
			if !slices.Contains(known, c) {
				known = append(known, c)
			}
		}
		for _, o := range known {
			choices = append(choices, SelectOption{Label: o, Value: o})
		}
		v.shell.ShowMultiSelectModal(label, choices, cur, func(sel []string) {
			if sel == nil {
				sel = []string{}
			}
			apply(sel)
		})
	}
	return f
}

// numberField edits a number (int or float).
func (v *WorkflowEditorView) numberField(label, help, lock string, float bool, value func(*workflow.Spec) string, path ...string) *edField {
	f := &edField{label: label, help: help, path: path, lock: lock, value: value}
	f.edit = func() {
		v.shell.ShowInputModal(label, v.current(path...), func(s string) {
			s = strings.TrimSpace(s)
			if s == "" {
				v.setOrUnset("", path...)
				return
			}
			if float {
				n, err := strconv.ParseFloat(s, 64)
				if err != nil {
					v.toast(i18n.Tf("tui.editor.bad_number", s), false)
					return
				}
				v.setOrUnset(n, path...)
				return
			}
			n, err := strconv.Atoi(s)
			if err != nil {
				v.toast(i18n.Tf("tui.editor.bad_number", s), false)
				return
			}
			v.setOrUnset(n, path...)
		})
	}
	return f
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func lit(vals ...string) func() []string { return func() []string { return vals } }

func (v *WorkflowEditorView) modeChoices() []string {
	modes := []string{workflow.ModeManual, workflow.ModeSemiAuto, workflow.ModeAuto}
	if sp := v.spec(); sp != nil {
		for _, m := range sp.AllowedModes() {
			if !slices.Contains(modes, m) {
				modes = append(modes, m)
			}
		}
	}
	return modes
}

func (v *WorkflowEditorView) agentChoices() []string {
	out := append([]string(nil), v.cfg.Agents...)
	if sp := v.spec(); sp != nil {
		for _, id := range sp.Agents.Keys() {
			if !slices.Contains(out, id) {
				out = append(out, id)
			}
		}
	}
	slices.Sort(out)
	return out
}

func specStr(fn func(*workflow.Spec) string) func(*workflow.Spec) string {
	return func(sp *workflow.Spec) string {
		if sp == nil {
			return ""
		}
		return fn(sp)
	}
}

func specList(fn func(*workflow.Spec) []string) func(*workflow.Spec) []string {
	return func(sp *workflow.Spec) []string {
		if sp == nil {
			return nil
		}
		return fn(sp)
	}
}

func runtimesOf(rs []workflow.Runtime) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = string(r)
	}
	return out
}

// ── General ────────────────────────────────────────────────────────────────

func (v *WorkflowEditorView) generalFields() []*edField {
	t := i18n.T
	lang := v.cfg.Lang
	return []*edField{
		{label: t("tui.editor.group.identity"), header: true},
		{label: t("tui.editor.field.id"), help: t("tui.editor.help.id"), value: specStr(func(sp *workflow.Spec) string { return sp.ID }),
			edit: func() { v.toast(t("tui.editor.help.id"), false) }},
		v.textField(t("tui.editor.field.extends"), t("tui.editor.help.extends"), "", specStr(func(sp *workflow.Spec) string { return v.current("extends") }), "extends"),
		v.textField(t("tui.editor.field.label"), "", "label", specStr(func(sp *workflow.Spec) string { return sp.Label.Text(lang) }), "label"),
		v.textField(t("tui.editor.field.description"), "", "description", specStr(func(sp *workflow.Spec) string { return sp.Description.Text(lang) }), "description"),
		v.selectField(t("tui.editor.field.category"), "", "category", lit("develop", "frame", "quality", "knowledge", "other"),
			specStr(func(sp *workflow.Spec) string { return string(sp.Category) }), "category"),
		{label: t("tui.editor.group.security"), header: true},
		v.selectField(t("tui.editor.field.risk"), t("tui.editor.help.risk"), "risk", lit("read", "plan", "write", "publish"),
			specStr(func(sp *workflow.Spec) string { return string(sp.Risk) }), "risk"),
		v.selectField(t("tui.editor.field.isolation"), t("tui.editor.help.isolation"), "isolation", lit("standard", "strict"),
			specStr(func(sp *workflow.Spec) string { return string(sp.Isolation) }), "isolation"),
		v.boolField(t("tui.editor.field.code_mode"), t("tui.editor.help.code_mode"), "code_mode",
			specStr(func(sp *workflow.Spec) string { return boolText(sp.CodeMode) }), "code_mode"),
		v.listField(t("tui.editor.field.enforce"), t("tui.editor.help.enforce"), "enforce",
			func() []string { return append([]string{workflow.EnforceAll}, workflow.EnforceableFields()...) },
			specList(func(sp *workflow.Spec) []string { return sp.Enforce }), "enforce"),
		{label: t("tui.editor.group.run"), header: true},
		v.selectField(t("tui.editor.field.entry"), t("tui.editor.help.entry"), "entry", v.agentChoices,
			specStr(func(sp *workflow.Spec) string { return sp.EntryAgent() }), "entry", "agent"),
		v.selectField(t("tui.editor.field.mode_default"), "", "modes", v.modeChoices,
			specStr(func(sp *workflow.Spec) string { return sp.DefaultMode() }), "modes", "default"),
		v.listField(t("tui.editor.field.modes_allowed"), "", "modes", v.modeChoices,
			specList(func(sp *workflow.Spec) []string { return sp.AllowedModes() }), "modes", "allowed"),
		v.selectField(t("tui.editor.field.runtime_default"), "", "runtime", lit("local", "container", "remote"),
			specStr(func(sp *workflow.Spec) string {
				if sp.Runtime != nil {
					return string(sp.Runtime.Default)
				}
				return ""
			}), "runtime", "default"),
		v.listField(t("tui.editor.field.runtime_allowed"), t("tui.editor.help.runtime_allowed"), "runtime", lit("local", "container", "remote"),
			specList(func(sp *workflow.Spec) []string { return runtimesOf(sp.AllowedRuntimes()) }), "runtime", "allowed"),
		v.numberField(t("tui.editor.field.circuit_breaker"), t("tui.editor.help.circuit_breaker"), "circuit_breaker", false,
			specStr(func(sp *workflow.Spec) string {
				if sp.CircuitBreaker != nil && sp.CircuitBreaker.MaxConsecutiveSubagents != nil {
					return strconv.Itoa(*sp.CircuitBreaker.MaxConsecutiveSubagents)
				}
				return ""
			}), "circuit_breaker", "max_consecutive_subagents"),
		v.textField(t("tui.editor.field.model"), t("tui.editor.help.model"), "models", specStr(func(sp *workflow.Spec) string {
			if sp.Models != nil {
				return sp.Models.Default
			}
			return ""
		}), "models", "default"),
		v.numberField(t("tui.editor.field.budget"), t("tui.editor.help.budget"), "limits", true, specStr(func(sp *workflow.Spec) string {
			if sp.Limits != nil && sp.Limits.BudgetUSD != nil {
				return strconv.FormatFloat(*sp.Limits.BudgetUSD, 'f', -1, 64)
			}
			return ""
		}), "limits", "budget_usd"),
	}
}

// ── Resources ──────────────────────────────────────────────────────────────

var beadsCommands = []string{"show", "list", "ready", "search", "children", "create", "update", "close", "reopen", "comments", "label", "dep"}

func (v *WorkflowEditorView) resourceFields() []*edField {
	t := i18n.T
	return []*edField{
		{label: t("tui.editor.group.skills"), header: true},
		v.listField(t("tui.editor.field.skills_extra"), t("tui.editor.help.skills_extra"), "skills", nil,
			specList(skillExtraOf), "skills", "extra"),
		v.listField(t("tui.editor.field.skills_deny"), t("tui.editor.help.skills_deny"), "skills", nil,
			specList(func(sp *workflow.Spec) []string {
				if sp.Skills != nil {
					return sp.Skills.Deny
				}
				return nil
			}), "skills", "deny"),
		{label: t("tui.editor.group.tools"), header: true},
		v.listField(t("tui.editor.field.mcp"), t("tui.editor.help.mcp"), "mcp", lit("gitlab", "figma", "jira", "team", "gslides"),
			specList(func(sp *workflow.Spec) []string { return sp.MCP }), "mcp"),
		v.listField(t("tui.editor.field.beads"), t("tui.editor.help.beads"), "beads", func() []string { return beadsCommands },
			specList(func(sp *workflow.Spec) []string {
				if sp.Beads != nil {
					return sp.Beads.Allow
				}
				return nil
			}), "beads", "allow"),
		v.listField(t("tui.editor.field.plugins"), t("tui.editor.help.plugins"), "plugins", nil,
			specList(func(sp *workflow.Spec) []string {
				var out []string
				for _, p := range sp.Plugins {
					out = append(out, p.ID)
				}
				return out
			}), "plugins"),
		{label: t("tui.editor.field.outputs"), help: t("tui.editor.help.outputs"), path: []string{"outputs"}, lock: "outputs",
			value: specStr(func(sp *workflow.Spec) string {
				var out []string
				for _, o := range sp.Outputs {
					out = append(out, o.ID+" ("+string(o.Type)+")")
				}
				return strings.Join(out, ", ")
			}),
			edit: func() { v.editRaw(0) }},
	}
}

func skillExtraOf(sp *workflow.Spec) []string {
	if sp.Skills != nil {
		return sp.Skills.Extra
	}
	return nil
}

// ── Inputs & prompt ────────────────────────────────────────────────────────

func (v *WorkflowEditorView) inputFields() []*edField {
	t := i18n.T
	out := []*edField{{label: t("tui.editor.group.inputs"), header: true}}
	if sp := v.spec(); sp != nil {
		for _, id := range sp.Inputs.Keys() {
			in, _ := sp.Inputs.Get(id)
			out = append(out, v.inputField(id, in))
		}
	}
	out = append(out,
		&edField{label: "+ " + t("tui.editor.inputs.add"), help: t("tui.editor.inputs.add_help"), action: v.addInput},
		&edField{label: t("tui.editor.group.prompt"), header: true},
		v.textField(t("tui.editor.field.prompt_template"), t("tui.editor.help.prompt_template"), "prompt",
			specStr(func(sp *workflow.Spec) string {
				if sp.Prompt != nil {
					if sp.Prompt.Template != "" {
						return sp.Prompt.Template
					}
					if sp.Prompt.Text != "" {
						return t("tui.editor.inputs.inline")
					}
				}
				return ""
			}), "prompt", "template"),
		&edField{label: "✎ " + t("tui.editor.inputs.edit_prompt"), help: t("tui.editor.inputs.edit_prompt_help"), action: v.editPrompt},
	)
	return out
}

func (v *WorkflowEditorView) inputField(id string, in workflow.Input) *edField {
	path := []string{"inputs", id}
	return &edField{label: id, path: path, lock: "inputs", help: in.Help.Text(v.cfg.Lang),
		value: func(*workflow.Spec) string {
			s := string(in.Type)
			if in.Required {
				s += " *"
			}
			if in.Default != nil {
				s += "  = " + fmt.Sprint(in.Default)
			}
			if lbl := in.Label.Text(v.cfg.Lang); lbl != "" {
				s += "  « " + lbl + " »"
			}
			return s
		},
		edit: func() { v.editInput(id, in) },
		remove: func() {
			doc := v.m.document()
			if doc == nil || !doc.Has("inputs."+id) {
				v.toast(i18n.T("tui.editor.inputs.inherited"), false)
				return
			}
			v.edit(func(e *workflow.DocEdit) error { e.Unset("inputs", id); return nil })
		},
	}
}

var inputTypes = []string{"string", "text", "bool", "int", "enum", "path", "branch", "beads-id", "beads-ids"}

// editInput edits an input with a form; only the fields that change are
// written (a patch keeps the inherited ones).
func (v *WorkflowEditorView) editInput(id string, in workflow.Input) {
	t := i18n.T
	var types []SelectOption
	for _, ty := range inputTypes {
		types = append(types, SelectOption{Label: ty, Value: ty})
	}
	def := ""
	if in.Default != nil {
		def = fmt.Sprint(in.Default)
	}
	initial := map[string]string{"type": string(in.Type), "required": strconv.FormatBool(in.Required), "default": def,
		"label": in.Label.Text(v.cfg.Lang), "values": strings.Join(in.Values, ", ")}
	v.shell.ShowInlineForm(InlineFormConfig{
		Title: i18n.Tf("tui.editor.inputs.edit_title", id),
		Fields: []FormField{
			{Key: "type", Label: t("tui.editor.field.input_type"), Type: FieldSelect, Options: types, Default: initial["type"]},
			{Key: "required", Label: t("tui.editor.field.required"), Type: FieldSelect, Default: initial["required"],
				Options: []SelectOption{{Label: "false", Value: "false"}, {Label: "true", Value: "true"}}},
			{Key: "default", Label: t("tui.editor.field.default"), Type: FieldText, Default: def, Hint: t("tui.editor.help.default")},
			{Key: "label", Label: t("tui.editor.field.label"), Type: FieldText, Default: initial["label"]},
			{Key: "values", Label: t("tui.editor.field.values"), Type: FieldText, Default: initial["values"], Hint: t("tui.editor.help.values")},
		},
		OnSubmit: func(vals map[string]string, _ map[string][]string) {
			v.edit(func(e *workflow.DocEdit) error {
				for _, k := range []string{"type", "required", "default", "label", "values"} {
					if vals[k] == initial[k] {
						continue
					}
					p := []string{"inputs", id, k}
					var err error
					switch k {
					case "required":
						err = e.Set(vals[k] == "true", p...)
					case "values":
						if l := splitList(vals[k]); len(l) > 0 {
							err = e.Set(l, p...)
						} else {
							e.Unset(p...)
						}
					default:
						if vals[k] == "" {
							e.Unset(p...)
						} else {
							err = e.Set(vals[k], p...)
						}
					}
					if err != nil {
						return err
					}
				}
				if !e.Has("inputs", id, "type") && !v.inherited("inputs."+id) {
					return e.Set(vals["type"], "inputs", id, "type")
				}
				return nil
			})
		},
	})
}

// inherited reports whether path comes from a parent document.
func (v *WorkflowEditorView) inherited(path string) bool {
	return v.originLabel(path) != "" && v.originLabel(path) != i18n.T("tui.editor.origin.default")
}

func (v *WorkflowEditorView) addInput() {
	v.shell.ShowInputModal(i18n.T("tui.editor.inputs.add_id"), "", func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		v.edit(func(e *workflow.DocEdit) error {
			if e.Has("inputs", id) {
				return fmt.Errorf("%s", i18n.Tf("tui.editor.exists", id))
			}
			return e.Set("string", "inputs", id, "type")
		})
	})
}

// ── Graph ──────────────────────────────────────────────────────────────────

var checkpointBehaviors = []string{"pause", "auto", "conditional", "skip"}

// editGraphElement edits the selected agent or checkpoint (Enter).
func (v *WorkflowEditorView) editGraphElement(e widgets.GraphElement) {
	sp := v.spec()
	if sp == nil || v.shell == nil {
		return
	}
	switch e.Type {
	case widgets.ElementCheckpoint:
		if v.check.locked("checkpoints") {
			v.toast(i18n.Tf("tui.editor.locked_detail", "checkpoints"), false)
			return
		}
		v.editCheckpoint(sp, e.ID)
	case widgets.ElementAgent, widgets.ElementIndependentAgent:
		if v.check.locked("agents") {
			v.toast(i18n.Tf("tui.editor.locked_detail", "agents"), false)
			return
		}
		v.editAgent(sp, e.ID)
	case widgets.ElementStart, widgets.ElementEdge:
		v.addAgent()
	}
}

func opts(vals ...string) []SelectOption {
	out := make([]SelectOption, len(vals))
	for i, s := range vals {
		label := s
		if s == "" {
			label = "—"
		}
		out[i] = SelectOption{Label: label, Value: s}
	}
	return out
}

func (v *WorkflowEditorView) editAgent(sp *workflow.Spec, id string) {
	t := i18n.T
	a, _ := sp.Agents.Get(id)
	after := []string{""}
	after = append(after, sp.Checkpoints.Keys()...)
	for _, k := range sp.Agents.Keys() {
		if k != id {
			after = append(after, k)
		}
	}
	initial := map[string]string{"role": string(a.Role), "mode": string(a.Mode), "after": a.After, "calls": strings.Join(a.Calls, ", ")}
	if initial["role"] == "" {
		initial["role"] = string(workflow.RoleWorkflow)
	}
	v.shell.ShowInlineForm(InlineFormConfig{
		Title: i18n.Tf("tui.editor.graph.agent_title", id),
		Fields: []FormField{
			{Key: "role", Label: t("tui.editor.field.role"), Type: FieldSelect, Default: initial["role"], Options: opts("workflow", "independent", "disabled")},
			{Key: "mode", Label: t("tui.editor.field.agent_mode"), Type: FieldSelect, Default: initial["mode"], Options: opts("", "primary", "subagent")},
			{Key: "after", Label: t("tui.editor.field.after"), Type: FieldSelect, Default: initial["after"], Options: opts(after...)},
			{Key: "calls", Label: t("tui.editor.field.calls"), Type: FieldText, Default: initial["calls"], Hint: t("tui.editor.help.calls")},
		},
		OnSubmit: func(vals map[string]string, _ map[string][]string) {
			v.edit(func(e *workflow.DocEdit) error {
				for _, k := range []string{"role", "mode", "after", "calls"} {
					if vals[k] == initial[k] {
						continue
					}
					p := []string{"agents", id, k}
					switch {
					case vals[k] == "":
						e.Unset(p...)
					case k == "calls":
						if err := e.Set(splitList(vals[k]), p...); err != nil {
							return err
						}
					default:
						if err := e.Set(vals[k], p...); err != nil {
							return err
						}
					}
				}
				if !e.Has("agents", id, "role") && !v.inherited("agents."+id) {
					return e.Set(initial["role"], "agents", id, "role")
				}
				return nil
			})
		},
	})
}

func (v *WorkflowEditorView) editCheckpoint(sp *workflow.Spec, id string) {
	t := i18n.T
	cp, _ := sp.Checkpoints.Get(id)
	initial := map[string]string{"label": cp.Label.Text(v.cfg.Lang), "condition": cp.Condition, "remote": string(cp.Remote), "mandatory": boolText(cp.Mandatory)}
	fields := []FormField{{Key: "label", Label: t("tui.editor.field.label"), Type: FieldText, Default: initial["label"]}}
	modes := sp.AllowedModes()
	for _, m := range modes {
		b := string(cp.Mode[m])
		initial["mode:"+m] = b
		fields = append(fields, FormField{Key: "mode:" + m, Label: i18n.Tf("tui.editor.field.behavior", m), Type: FieldSelect,
			Default: b, Options: opts(append([]string{""}, checkpointBehaviors...)...)})
	}
	fields = append(fields,
		FormField{Key: "condition", Label: t("tui.editor.field.condition"), Type: FieldText, Default: cp.Condition, Hint: t("tui.editor.help.condition")},
		FormField{Key: "remote", Label: t("tui.editor.field.remote"), Type: FieldSelect, Default: initial["remote"], Options: opts("", "auto", "defer", "forbid")},
		FormField{Key: "mandatory", Label: t("tui.editor.field.mandatory"), Type: FieldSelect, Default: initial["mandatory"], Options: opts("", "true", "false")},
	)
	v.shell.ShowInlineForm(InlineFormConfig{
		Title:  i18n.Tf("tui.editor.graph.checkpoint_title", id),
		Fields: fields,
		OnSubmit: func(vals map[string]string, _ map[string][]string) {
			v.edit(func(e *workflow.DocEdit) error {
				for k, old := range initial {
					nv := vals[k]
					if nv == old {
						continue
					}
					p := []string{"checkpoints", id, k}
					if m, ok := strings.CutPrefix(k, "mode:"); ok {
						p = []string{"checkpoints", id, "mode", m}
					}
					var err error
					switch {
					case nv == "":
						e.Unset(p...)
					case k == "mandatory":
						err = e.Set(nv == "true", p...)
					default:
						err = e.Set(nv, p...)
					}
					if err != nil {
						return err
					}
				}
				return nil
			})
		},
	})
}

// addAgent adds an agent of the catalogue to the workflow (a).
func (v *WorkflowEditorView) addAgent() {
	if v.check.locked("agents") {
		v.toast(i18n.Tf("tui.editor.locked_detail", "agents"), false)
		return
	}
	var choices []SelectOption
	sp := v.spec()
	for _, id := range v.cfg.Agents {
		if sp != nil {
			if _, ok := sp.Agents.Get(id); ok {
				continue
			}
		}
		choices = append(choices, SelectOption{Label: id, Value: id})
	}
	if len(choices) == 0 {
		v.shell.ShowInputModal(i18n.T("tui.editor.graph.add_agent"), "", func(id string) { v.addAgentID(strings.TrimSpace(id)) })
		return
	}
	v.shell.ShowSelectModal(i18n.T("tui.editor.graph.add_agent"), choices, "", func(id string) { v.addAgentID(id) })
}

func (v *WorkflowEditorView) addAgentID(id string) {
	if id == "" {
		return
	}
	v.edit(func(e *workflow.DocEdit) error {
		if e.Has("agents", id) {
			return fmt.Errorf("%s", i18n.Tf("tui.editor.exists", id))
		}
		return e.Set(string(workflow.RoleWorkflow), "agents", id, "role")
	})
}

// addCheckpoint adds a checkpoint (appended: the resolver adds new
// checkpoints after the inherited ones).
func (v *WorkflowEditorView) addCheckpoint() {
	if v.check.locked("checkpoints") {
		v.toast(i18n.Tf("tui.editor.locked_detail", "checkpoints"), false)
		return
	}
	v.shell.ShowInputModal(i18n.T("tui.editor.graph.add_checkpoint"), "cp-", func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || id == "cp-" {
			return
		}
		v.edit(func(e *workflow.DocEdit) error {
			if e.Has("checkpoints", id) {
				return fmt.Errorf("%s", i18n.Tf("tui.editor.exists", id))
			}
			return e.Set(id, "checkpoints", id, "label")
		})
	})
}

// removeGraphElement removes the selected agent or checkpoint: written
// only here, it is deleted; inherited, it is disabled (role: disabled /
// disabled: true).
func (v *WorkflowEditorView) removeGraphElement() {
	if v.graph == nil {
		return
	}
	sel := v.graph.SelectedElement()
	if sel == nil {
		return
	}
	var key string
	switch sel.Type {
	case widgets.ElementCheckpoint:
		key = "checkpoints"
	case widgets.ElementAgent, widgets.ElementIndependentAgent:
		key = "agents"
	default:
		return
	}
	if v.check.locked(key) {
		v.toast(i18n.Tf("tui.editor.locked_detail", key), false)
		return
	}
	id, inherited := sel.ID, v.inherited(key+"."+sel.ID)
	v.shell.ShowScrollableModal(i18n.Tf("tui.editor.graph.remove_title", id), i18n.T("tui.editor.graph.remove_body_"+map[bool]string{true: "inherited", false: "own"}[inherited]), []ModalAction{
		{Label: i18n.T("tui.editor.graph.remove"), Callback: func() {
			v.edit(func(e *workflow.DocEdit) error {
				if !inherited {
					e.Unset(key, id)
					return nil
				}
				if key == "agents" {
					return e.Set(string(workflow.RoleDisabled), key, id, "role")
				}
				return e.Set(true, key, id, "disabled")
			})
		}},
		{Label: i18n.T("tui.catalog.edit.cancel"), Callback: func() {}, Separator: true},
	})
}
