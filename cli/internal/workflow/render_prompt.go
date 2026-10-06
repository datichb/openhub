package workflow

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"unicode/utf8"
)

// reBeadsID is the accepted shape of a Beads ticket id (bd-12, proj-a1b.3).
var reBeadsID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// DefaultInputMaxLength caps a free-text input (string, text, path, branch)
// that declares no max_length (O11).
const DefaultInputMaxLength = 20000

// Data tags delimiting an input in a rendered prompt (O11). The agents are
// told that the content of these tags is data, never instructions.
const (
	dataOpenTag  = "<oh:data"
	dataCloseTag = "</oh:data>"
)

// PromptContext is the session context exposed under .oh in prompt
// templates (see OhVariables).
type PromptContext struct {
	Project  string
	Location string
	// Branch is the dedicated branch the session works on (its worktree, or
	// a branch other than the base one); empty on the base branch.
	Branch   string
	Mode     string
	Runtime  string
	Lang     string
	Workflow string
}

func (c PromptContext) values() map[string]any {
	return map[string]any{
		"project":  c.Project,
		"location": c.Location,
		"branch":   c.Branch,
		"mode":     c.Mode,
		"runtime":  c.Runtime,
		"lang":     c.Lang,
		"workflow": c.Workflow,
	}
}

// PromptFuncs are the functions available in prompt templates:
//
//	{{ data "request" .request }}  the value inside data tags (O11)
//	{{ join .tickets ", " }}       a list joined with a separator
//
// data must wrap every free-text input (string, text): a value cannot
// close its tag, and is truncated to the input max_length.
func promptFuncs(s *Spec) template.FuncMap {
	return template.FuncMap{
		"data": func(name string, v any) (string, error) {
			if _, ok := s.Inputs.Get(name); !ok {
				return "", fmt.Errorf("data %q: unknown input", name)
			}
			return DelimitData(name, textOf(v)), nil
		},
		"join": func(v any, sep string) string {
			return strings.Join(listOf(v), sep)
		},
	}
}

// DelimitData wraps value in data tags named name. Tag-like sequences of
// the value are neutralised so that it cannot close the block.
func DelimitData(name, value string) string {
	value = neutralise(value)
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s name=%q>\n", dataOpenTag, name)
	sb.WriteString(strings.TrimRight(value, "\n"))
	sb.WriteString("\n" + dataCloseTag)
	return sb.String()
}

func neutralise(v string) string {
	lower := strings.ToLower(v)
	if !strings.Contains(lower, "<oh:data") && !strings.Contains(lower, "</oh:data") {
		return v
	}
	var sb strings.Builder
	for i := 0; i < len(v); {
		rest := lower[i:]
		if strings.HasPrefix(rest, "<oh:data") || strings.HasPrefix(rest, "</oh:data") {
			sb.WriteString("&lt;")
			i++
			continue
		}
		sb.WriteByte(v[i])
		i++
	}
	return sb.String()
}

// ReadPromptSource returns the template of the resolved workflow: the
// inline text, or the file read through ps relative to the document that
// set it.
func (r *Resolved) ReadPromptSource(ps PromptSource) (string, error) {
	p := r.Spec.Prompt
	switch {
	case p == nil || (p.Text == "" && p.Template == ""):
		return "", nil
	case p.Text != "":
		return p.Text, nil
	}
	if ps == nil {
		return "", fmt.Errorf("prompt template %q: no prompt source", p.Template)
	}
	origin, _ := r.Origins.Of("prompt.template")
	data, err := ps.ReadPrompt(origin, path.Clean(p.Template))
	if err != nil {
		return "", fmt.Errorf("reading prompt template %q: %w", p.Template, err)
	}
	return string(data), nil
}

// PromptValues returns the template data: every declared input (session
// value, else default, else the zero value of its type) at the top level,
// and the session context under .oh. Values are typed (bool, int, string,
// []string for beads-ids) and free-text values truncated (O11). Defaults
// may reference other inputs ({{ .ticket }}).
func (r *Resolved) PromptValues(ctx PromptContext) (map[string]any, error) {
	s := r.Spec
	vals := map[string]any{ReservedInputID: ctx.values()}
	var templated []string
	for _, k := range s.Inputs.Keys() {
		in, _ := s.Inputs.Get(k)
		v, given := r.Inputs[k]
		if !given {
			v = in.Default
			if str, ok := v.(string); ok && strings.Contains(str, "{{") {
				templated = append(templated, k)
				continue
			}
		}
		tv, err := typedInput(k, in, v)
		if err != nil {
			return nil, err
		}
		vals[k] = tv
	}
	for _, k := range templated {
		in, _ := s.Inputs.Get(k)
		str, err := execTemplate("inputs."+k+".default", in.Default.(string), s, vals)
		if err != nil {
			return nil, err
		}
		if vals[k], err = typedInput(k, in, str); err != nil {
			return nil, err
		}
	}
	return vals, nil
}

// RenderPrompt renders the prompt of the resolved workflow (see
// PromptValues and the functions documented on promptFuncs). An empty
// template gives an empty prompt.
func RenderPrompt(r *Resolved, ps PromptSource, ctx PromptContext) (string, error) {
	src, err := r.ReadPromptSource(ps)
	if err != nil || src == "" {
		return "", err
	}
	vals, err := r.PromptValues(ctx)
	if err != nil {
		return "", err
	}
	out, err := execTemplate("prompt", src, r.Spec, vals)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out) + "\n", nil
}

func execTemplate(name, src string, s *Spec, vals map[string]any) (string, error) {
	t, err := template.New(name).Funcs(promptFuncs(s)).Option("missingkey=error").Parse(src)
	if err != nil {
		return "", fmt.Errorf("parsing %s: %w", name, err)
	}
	var sb strings.Builder
	if err := t.Execute(&sb, vals); err != nil {
		return "", fmt.Errorf("rendering %s: %w", name, err)
	}
	return sb.String(), nil
}

// typedInput converts v (nil, a command-line string or a YAML value) to the
// type of the input.
func typedInput(name string, in Input, v any) (any, error) {
	bad := func() error { return fmt.Errorf("input %q: invalid %s value %v", name, in.Type, v) }
	switch in.Type {
	case InputBool:
		switch x := v.(type) {
		case nil:
			return false, nil
		case bool:
			return x, nil
		case string:
			if x == "" {
				return false, nil
			}
			b, err := strconv.ParseBool(x)
			if err != nil {
				return nil, bad()
			}
			return b, nil
		}
		return nil, bad()
	case InputInt:
		switch x := v.(type) {
		case nil:
			return 0, nil
		case int:
			return x, nil
		case int64:
			return int(x), nil
		case string:
			if x == "" {
				return 0, nil
			}
			n, err := strconv.Atoi(x)
			if err != nil {
				return nil, bad()
			}
			return n, nil
		}
		return nil, bad()
	case InputBeadsIDs, InputBeadsID:
		ids := listOf(v)
		for _, id := range ids {
			if !reBeadsID.MatchString(id) {
				return nil, fmt.Errorf("input %q: invalid Beads id %q", name, id)
			}
		}
		if in.Type == InputBeadsID {
			return strings.Join(ids, ", "), nil
		}
		return ids, nil
	}
	str := textOf(v)
	if (in.Type == InputPath || in.Type == InputBranch) && strings.ContainsAny(str, "\r\n") {
		return nil, fmt.Errorf("input %q: a %s value is a single line", name, in.Type)
	}
	limit := in.MaxLength
	if limit <= 0 {
		limit = DefaultInputMaxLength
	}
	return truncate(str, limit), nil
}

func textOf(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []string, []any:
		return strings.Join(listOf(x), ", ")
	}
	return fmt.Sprint(v)
}

// listOf accepts a list or a comma-separated string ("a, b").
func listOf(v any) []string {
	var raw []string
	switch x := v.(type) {
	case nil:
	case string:
		raw = strings.Split(x, ",")
	case []string:
		raw = x
	case []any:
		for _, e := range x {
			raw = append(raw, fmt.Sprint(e))
		}
	default:
		raw = []string{fmt.Sprint(x)}
	}
	out := []string{}
	for _, e := range raw {
		if e = strings.TrimSpace(e); e != "" {
			out = append(out, e)
		}
	}
	return out
}

// truncate cuts s to limit runes and says so (O11).
func truncate(s string, limit int) string {
	n := utf8.RuneCountInString(s)
	if n <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit]) + fmt.Sprintf("\n[… tronqué : %d caractères sur %d]", limit, n)
}
