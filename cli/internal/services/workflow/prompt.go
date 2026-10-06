package workflow

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// Prompt rendering contract (with the shipped workflows, piste E):
//   - inputs are top-level fields ({{ .ticket }}), the session context is
//     under .oh (project, location, mode, runtime, lang, workflow);
//   - free text values are wrapped in data tags and truncated (O11):
//     <oh:input name="request">…</oh:input>;
//   - the rendered prompt starts with "Mode de workflow : <mode>" (the
//     generated workflow skills read the mode from the first message), added
//     when the template does not write it.

// ModeLinePrefix starts the line giving the workflow mode to the agents.
const ModeLinePrefix = "Mode de workflow : "

// Default truncation lengths (characters) when `max_length` is not set.
const (
	DefaultTextMaxLength   = 8000
	DefaultStringMaxLength = 1000
)

// PromptContext is the session context available under .oh.
type PromptContext struct {
	Project  string
	Location string
	Lang     string
}

// List is a list input value; it prints as a comma-separated list and can
// be ranged over in templates.
type List []string

func (l List) String() string { return strings.Join(l, ", ") }

// Values returns the effective inputs of the resolution: given values,
// typed (the command line gives strings), then defaults, whose templates
// may refer to other inputs.
func (r *Resolution) Values() (map[string]any, error) {
	sp := r.Spec
	out := map[string]any{}
	var templated []string
	for _, k := range sp.Inputs.Keys() {
		in, _ := sp.Inputs.Get(k)
		v, given := r.Inputs[k]
		if !given {
			if in.Default == nil {
				continue
			}
			if s, ok := in.Default.(string); ok && strings.Contains(s, "{{") {
				templated = append(templated, k)
				continue
			}
			v = in.Default
		}
		tv, err := typed(in, v)
		if err != nil {
			return nil, fmt.Errorf("input %s: %w", k, err)
		}
		out[k] = tv
	}
	// Defaults may chain (a default using another templated default): a few
	// passes resolve them in dependency order.
	for pass := 0; pass <= len(templated) && len(templated) > 0; pass++ {
		var next []string
		for _, k := range templated {
			in, _ := sp.Inputs.Get(k)
			if refersToPending(in.Default.(string), templated, k) {
				next = append(next, k)
				continue
			}
			s, err := execTemplate(in.Default.(string), withEmpty(sp, out))
			if err != nil {
				return nil, fmt.Errorf("default of input %s: %w", k, err)
			}
			tv, err := typed(in, s)
			if err != nil {
				return nil, fmt.Errorf("default of input %s: %w", k, err)
			}
			out[k] = tv
		}
		templated = next
	}
	for _, k := range templated { // cycles: rendered with what is known
		in, _ := sp.Inputs.Get(k)
		s, _ := execTemplate(in.Default.(string), withEmpty(sp, out))
		out[k] = s
	}
	return out, nil
}

// HasPrompt reports whether the workflow declares an initial prompt.
func (r *Resolution) HasPrompt() bool {
	p := r.Spec.Prompt
	return p != nil && (p.Text != "" || p.Template != "")
}

// RenderPrompt renders the initial prompt with values (see Values). It
// returns "" when the workflow has no prompt.
func (r *Resolution) RenderPrompt(pc PromptContext, values map[string]any) (string, error) {
	if !r.HasPrompt() {
		return "", nil
	}
	src := r.Spec.Prompt.Text
	if src == "" {
		if r.env.Prompts == nil {
			return "", fmt.Errorf("prompt template %s: no prompt source", r.Spec.Prompt.Template)
		}
		origin, _ := r.Origins.Of("prompt.template")
		data, err := r.env.Prompts.ReadPrompt(origin, r.Spec.Prompt.Template)
		if err != nil {
			return "", fmt.Errorf("reading prompt template: %w", err)
		}
		src = string(data)
	}
	data := map[string]any{}
	for _, k := range r.Spec.Inputs.Keys() {
		in, _ := r.Spec.Inputs.Get(k)
		data[k] = delimited(k, in, values[k])
	}
	data[wf.ReservedInputID] = map[string]any{
		"project": pc.Project, "location": pc.Location, "mode": r.Mode,
		"runtime": string(r.Runtime), "lang": pc.Lang, "workflow": r.Spec.ID,
	}
	out, err := execTemplate(src, data)
	if err != nil {
		return "", fmt.Errorf("rendering prompt: %w", err)
	}
	out = strings.TrimSpace(out)
	if !strings.Contains(out, ModeLinePrefix) {
		out = ModeLinePrefix + r.Mode + "\n\n" + out
	}
	return out, nil
}

// typed converts a command-line string to the input type.
func typed(in wf.Input, v any) (any, error) {
	s, isStr := v.(string)
	switch in.Type {
	case wf.InputBool:
		if isStr {
			return strconv.ParseBool(s)
		}
	case wf.InputInt:
		if isStr {
			return strconv.Atoi(s)
		}
	case wf.InputBeadsIDs:
		return toList(v), nil
	case wf.InputBeadsID:
		if in.Picker != nil && in.Picker.Multi {
			l := toList(v)
			if len(l) == 1 {
				return l[0], nil
			}
			return l, nil
		}
	}
	return v, nil
}

func toList(v any) List {
	switch x := v.(type) {
	case string:
		var out List
		for _, p := range strings.Split(x, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out
	case List:
		return x
	case []string:
		return List(x)
	case []any:
		out := make(List, 0, len(x))
		for _, e := range x {
			out = append(out, fmt.Sprint(e))
		}
		return out
	}
	return List{fmt.Sprint(v)}
}

var reSafeValue = regexp.MustCompile(`^[A-Za-z0-9._/@+#-]{1,200}$`)

// delimited wraps free text in data tags (O11). Identifiers (tickets,
// branches, paths) are kept raw when they only use safe characters.
func delimited(name string, in wf.Input, v any) any {
	switch in.Type {
	case wf.InputBool, wf.InputInt, wf.InputEnum:
		return v
	}
	if l, ok := v.(List); ok {
		out := make(List, len(l))
		for i, e := range l {
			out[i] = delimitString(name, in, e)
		}
		return out
	}
	if v == nil {
		return ""
	}
	return delimitString(name, in, fmt.Sprint(v))
}

func delimitString(name string, in wf.Input, s string) string {
	if s == "" {
		return ""
	}
	if in.Type != wf.InputString && in.Type != wf.InputText && reSafeValue.MatchString(s) {
		return s
	}
	limit := in.MaxLength
	if limit == 0 {
		limit = DefaultStringMaxLength
		if in.Type == wf.InputText {
			limit = DefaultTextMaxLength
		}
	}
	if r := []rune(s); len(r) > limit {
		s = string(r[:limit]) + " […]"
	}
	s = strings.ReplaceAll(s, "</oh:input", "<\\/oh:input")
	return fmt.Sprintf("<oh:input name=%q>%s</oh:input>", name, s)
}

var promptFuncs = template.FuncMap{
	"join":  func(l List, sep string) string { return strings.Join(l, sep) },
	"lower": strings.ToLower,
	"upper": strings.ToUpper,
	"trim":  strings.TrimSpace,
}

func execTemplate(src string, data map[string]any) (string, error) {
	t, err := template.New("prompt").Funcs(promptFuncs).Option("missingkey=zero").Parse(src)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := t.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

// withEmpty returns values with every declared input present ("" when
// unset): templates never print "<no value>".
func withEmpty(sp *wf.Spec, values map[string]any) map[string]any {
	out := map[string]any{}
	for _, k := range sp.Inputs.Keys() {
		out[k] = ""
	}
	for k, v := range values {
		out[k] = v
	}
	return out
}

func refersToPending(src string, pending []string, self string) bool {
	vars := map[string]bool{}
	for _, p := range pending {
		if p != self {
			vars[p] = true
		}
	}
	unknown, err := wf.TemplateVariables(src, map[string]bool{})
	if err != nil {
		return false
	}
	for _, u := range unknown {
		if vars[u] {
			return true
		}
	}
	return false
}

// WithInputs returns a copy of the resolution whose session inputs are
// replaced by inputs (one session of a multi-ticket launch). The values are
// not re-validated: use it with values of the same types.
func (r *Resolution) WithInputs(inputs map[string]any) *Resolution {
	cp := *r
	resolved := *r.Resolved
	resolved.Inputs = inputs
	cp.Resolved = &resolved
	return &cp
}

// FirstInput returns the first input of a type ("" when none).
func FirstInput(sp *wf.Spec, t wf.InputType) string {
	for _, k := range sp.Inputs.Keys() {
		if in, _ := sp.Inputs.Get(k); in.Type == t {
			return k
		}
	}
	return ""
}
