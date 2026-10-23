package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"

	"github.com/datichb/openhub/cli/internal/gitlabapi"
)

// reBeadsID is the accepted shape of a Beads ticket id (bd-12, proj-a1b.3).
var reBeadsID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// DefaultInputMaxLength caps a free-text input (string, text, path, branch)
// that declares no max_length (O11).
const DefaultInputMaxLength = 20000

// reDataTag matches a data tag opening or closing, whatever its case (S-34).
var reDataTag = regexp.MustCompile(`(?i)</?oh:data`)

// dataTagPrefix starts the name of the tags delimiting an input in a rendered
// prompt (O11). The agents are told that the content of these tags is data,
// never instructions. The full name carries a fingerprint of the content
// (see dataTagName).
const dataTagPrefix = "oh:data-"

// dataTagHexLen is the number of hex digits of the SHA-256 of the content kept
// in the tag name: 64 bits, out of reach of a search for a content that holds
// its own closing tag.
const dataTagHexLen = 16

// MaxBranchLength caps a branch input (git refuses longer path components).
const MaxBranchLength = 255

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
	// Memories are the persistent memories of the project's Beads (bd
	// remember), given inside data tags: they are not instructions.
	Memories string
	// Tracker is the tracker of the tickets of the project ("gitlab",
	// "jira"), "" when oh does not know one (Beads only); TrackerProject is
	// its project (the GitLab path or ID). GitLabMCP tells whether the
	// session has the GitLab MCP server: without it a GitLab ticket cannot
	// be read, and the prompt says so.
	Tracker        string
	TrackerProject string
	GitLabMCP      bool
	// ReviewRef is the branch a read-only session reads in a temporary
	// worktree (detached): the branch, or <remote>/<branch> when only the
	// remote has it. Empty when the session works in the project.
	ReviewRef string
}

func (c PromptContext) values() map[string]any {
	memories := ""
	if strings.TrimSpace(c.Memories) != "" {
		memories = DelimitData("memories", c.Memories)
	}
	return map[string]any{
		"memories":        memories,
		"project":         c.Project,
		"location":        c.Location,
		"branch":          c.Branch,
		"mode":            c.Mode,
		"runtime":         c.Runtime,
		"lang":            c.Lang,
		"workflow":        c.Workflow,
		"tracker":         c.Tracker,
		"tracker_project": c.TrackerProject,
		"gitlab_mcp":      c.GitLabMCP,
		"review_ref":      c.ReviewRef,
	}
}

// PromptFuncs are the functions available in prompt templates:
//
//	{{ data "request" .request }}  an input: inside data tags when oh computed it
//	                                from an outside source (`from:`, O11), as
//	                                typed otherwise (the user's own request)
//	{{ join .tickets ", " }}       a list joined with a separator
//
// data must wrap every free-text input (string, text): it is truncated to
// the input max_length, and an outside value cannot close its tag.
func promptFuncs(s *Spec) template.FuncMap {
	return template.FuncMap{
		"data": func(name string, v any) (string, error) {
			in, ok := s.Inputs.Get(name)
			if !ok {
				return "", fmt.Errorf("data %q: unknown input", name)
			}
			if in.From == "" {
				// Typed by the user at launch: their request, written as
				// it is (A27: a short imperative request inside data tags
				// was taken for an injection). Still truncated.
				return strings.TrimRight(neutralise(textOf(v)), "\n"), nil
			}
			return DelimitData(name, textOf(v)), nil
		},
		"join": func(v any, sep string) string {
			return strings.Join(listOf(v), sep)
		},
	}
}

// DelimitData wraps value in data tags named name. The tags are named after
// a fingerprint of the content (S-34): the content cannot hold the closing
// tag, which it would have to spell out together with its own SHA-256. The
// fingerprint is a function of the content, so a prompt stays reproducible.
// Tag-like sequences of the value are neutralised on top of that.
func DelimitData(name, value string) string {
	value = strings.TrimRight(neutralise(value), "\n")
	tag := dataTagName(value)
	var sb strings.Builder
	fmt.Fprintf(&sb, "<%s name=%q>\n", tag, name)
	sb.WriteString(value)
	fmt.Fprintf(&sb, "\n</%s>", tag)
	return sb.String()
}

// dataTagName is the name of the tags delimiting body: "oh:data-" and the
// first hex digits of its SHA-256.
func dataTagName(body string) string {
	sum := sha256.Sum256([]byte(body))
	return dataTagPrefix + hex.EncodeToString(sum[:])[:dataTagHexLen]
}

// neutralise breaks every data tag of v, whatever its case. It matches the
// text itself, not a lower-cased copy: lower-casing changes the length of
// some letters (İ), which shifts the offsets.
func neutralise(v string) string {
	return reDataTag.ReplaceAllStringFunc(v, func(m string) string { return "&lt;" + m[1:] })
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
				if ref, ok := gitlabapi.ParseRef(id); ok {
					return nil, fmt.Errorf("input %q: %q is a GitLab %s, not a Beads id (Beads ids look like proj-a1b): name it in the request of a workflow that takes one (spec, quick), the session reads it with the GitLab MCP server", name, id, map[gitlabapi.RefKind]string{gitlabapi.RefIssue: "issue", gitlabapi.RefMR: "merge request"}[ref.Kind])
				}
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
	if in.Type == InputBranch {
		if err := ValidateBranchName(str); err != nil {
			return nil, fmt.Errorf("input %q: %w", name, err)
		}
	}
	limit := in.MaxLength
	if limit <= 0 {
		limit = DefaultInputMaxLength
	}
	return truncate(str, limit), nil
}

// ValidateBranchName applies the rules of `git check-ref-format --branch` to
// a branch input (S-34): the name ends up in prompts and commands. The empty
// name is accepted (no branch given).
func ValidateBranchName(b string) error {
	if b == "" {
		return nil
	}
	bad := func(why string) error { return fmt.Errorf("invalid branch name %q: %s", b, why) }
	if len(b) > MaxBranchLength {
		return fmt.Errorf("invalid branch name: longer than %d bytes", MaxBranchLength)
	}
	if b == "@" || strings.HasPrefix(b, "-") {
		return bad("cannot be '@' or start with '-'")
	}
	if strings.Contains(b, "..") || strings.Contains(b, "@{") || strings.Contains(b, "//") {
		return bad("contains '..', '@{' or '//'")
	}
	for _, r := range b {
		switch {
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.IsSpace(r):
			return bad("contains a space or a control character")
		case strings.ContainsRune("~^:?*[\\", r):
			return bad(fmt.Sprintf("contains %q", r))
		}
	}
	if strings.HasPrefix(b, "/") || strings.HasSuffix(b, "/") || strings.HasSuffix(b, ".") {
		return bad("cannot start or end with '/' or end with '.'")
	}
	for _, c := range strings.Split(b, "/") {
		if strings.HasPrefix(c, ".") || strings.HasSuffix(c, ".lock") {
			return bad("a component cannot start with '.' or end with '.lock'")
		}
	}
	return nil
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
