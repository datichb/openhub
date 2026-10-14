package bundle

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Variants by role and by session (modernisation L3). A skill (frontmatter
// `when:`) or a block of text is kept only for the agents whose session
// satisfies its condition:
//
//	when: role=subagent                      (skill frontmatter)
//	<!-- oh:when role=entry -->              (block, on its own line)
//	…
//	<!-- /oh:when -->
//
// A condition is a list of terms separated by spaces, all of which must
// hold, or several such lists separated by `|`, one of which must hold. A
// term is `key=v1,v2` (one of the values), `key!=v1,v2` (none of
// them), or, for a yes/no key, `key` and `!key`. Keys:
//
//	role         entry (talks to the user: the entry agent, an agent no one
//	             delegates to) or subagent (launched with `task`); an entry
//	             agent others may launch has both roles
//	agent        the agent id (producer side of a shared format)
//	calls        an agent it may launch (consumer side of a shared format)
//	mcp          an MCP server of the session (workflow, gitlab, team…)
//	plugin       a plugin of the workflow
//	method       the method of the session (R7: standard, tdd)
//	websearch    the web search tools are on and the agent may use them
//	beads_write  the workflow lets agents write to Beads and the agent may run bd
//	ticket       the workflow takes a Beads ticket as input
//	wiki         the project has a wiki (docs/wiki/)
//	shell        the agent may run shell commands
//	headless     nobody follows the session (workflow `headless: true`)
//	writes       the workflow opens documentation paths to the agent (R4,
//	             `agents.<id>.writes`)
//
// Inlined skills and the agent body are filtered for the agent; an
// on-demand skill for the union of the agents whose catalogue holds it (a
// block is kept when one of them satisfies it). Skill mentions are read
// after the filtering: a skill named in a block left out is not offered.

// whenEnv is the session of an agent, as the conditions see it.
type whenEnv struct {
	agent  string
	roles  map[string]bool
	calls  map[string]bool
	mcp    map[string]bool
	plugin map[string]bool
	method string
	flags  map[string]bool // yes/no keys
	// ownEdits: the agent's own rules let it edit files beyond its
	// `writes:` paths (the onboarder: ONBOARDING.md), not a condition key.
	ownEdits bool
	// refusedShell: the usual helpers of a command line its own rules
	// refuse while it has a shell (restrictedShellProbes), not a
	// condition key.
	refusedShell []string
}

// Role values of the `role` key.
const (
	roleEntry    = "entry"
	roleSubagent = "subagent"
)

var (
	whenSetKeys  = []string{"role", "agent", "calls", "mcp", "plugin", "method"}
	whenFlagKeys = []string{"websearch", "beads_write", "ticket", "wiki", "shell", "headless", "writes"}
	whenRoles    = []string{roleEntry, roleSubagent}
)

type whenTerm struct {
	key    string
	neg    bool
	values []string
}

// whenAll holds when every term holds.
type whenAll []whenTerm

// whenExpr is a parsed condition: one of its groups holds; the empty one
// always holds.
type whenExpr []whenAll

var whenValueRe = regexp.MustCompile(`^[a-z0-9][a-z0-9@/._-]*$`)

// parseWhen parses a condition.
func parseWhen(s string) (whenExpr, error) {
	var out whenExpr
	if strings.TrimSpace(s) == "" {
		return out, nil
	}
	for _, group := range strings.Split(s, "|") {
		g, err := parseWhenAll(group)
		if err != nil {
			return nil, err
		}
		if len(g) == 0 {
			return nil, fmt.Errorf("%q: empty alternative around |", s)
		}
		out = append(out, g)
	}
	return out, nil
}

func parseWhenAll(s string) (whenAll, error) {
	var out whenAll
	for _, f := range strings.Fields(s) {
		var t whenTerm
		key, values, hasValue := strings.Cut(f, "=")
		if hasValue && strings.HasSuffix(key, "!") {
			key, t.neg = strings.TrimSuffix(key, "!"), true
		}
		if !hasValue && strings.HasPrefix(key, "!") {
			key, t.neg = strings.TrimPrefix(key, "!"), true
		}
		t.key = key
		switch {
		case slices.Contains(whenFlagKeys, key):
			if hasValue {
				return nil, fmt.Errorf("%q: %s is a yes/no key (write %s or !%s)", f, key, key, key)
			}
		case slices.Contains(whenSetKeys, key):
			if !hasValue || values == "" {
				return nil, fmt.Errorf("%q: %s needs a value (%s=…)", f, key, key)
			}
			t.values = strings.Split(values, ",")
			for _, v := range t.values {
				if !whenValueRe.MatchString(v) {
					return nil, fmt.Errorf("%q: invalid value %q", f, v)
				}
				if key == "role" && !slices.Contains(whenRoles, v) {
					return nil, fmt.Errorf("%q: unknown role %q (%s)", f, v, strings.Join(whenRoles, ", "))
				}
				if key == "method" && v != string(workflow.MethodStandard) && !workflow.Method(v).Valid() {
					names := []string{string(workflow.MethodStandard)}
					for _, m := range workflow.Methods {
						names = append(names, string(m))
					}
					return nil, fmt.Errorf("%q: unknown method %q (%s)", f, v, strings.Join(names, ", "))
				}
			}
		default:
			return nil, fmt.Errorf("%q: unknown key %q (%s)", f, key, strings.Join(append(append([]string(nil), whenSetKeys...), whenFlagKeys...), ", "))
		}
		out = append(out, t)
	}
	return out, nil
}

func (e whenExpr) holds(env whenEnv) bool {
	if len(e) == 0 {
		return true
	}
	for _, g := range e {
		if g.holds(env) {
			return true
		}
	}
	return false
}

func (e whenAll) holds(env whenEnv) bool {
	for _, t := range e {
		var got bool
		switch t.key {
		case "role":
			got = anyIn(t.values, env.roles)
		case "agent":
			got = slices.Contains(t.values, env.agent)
		case "calls":
			got = anyIn(t.values, env.calls)
		case "mcp":
			got = anyIn(t.values, env.mcp)
		case "plugin":
			got = anyIn(t.values, env.plugin)
		case "method":
			got = slices.Contains(t.values, env.method)
		default:
			got = env.flags[t.key]
		}
		if got == t.neg {
			return false
		}
	}
	return true
}

func anyIn(values []string, set map[string]bool) bool {
	for _, v := range values {
		if set[v] {
			return true
		}
	}
	return false
}

// holdsAny reports whether one of envs satisfies e (no env: e is kept).
func (e whenExpr) holdsAny(envs []whenEnv) bool {
	if len(envs) == 0 {
		return true
	}
	for _, env := range envs {
		if e.holds(env) {
			return true
		}
	}
	return false
}

// whenMarkerRe matches a block marker line: its condition, or a closing.
var whenMarkerRe = regexp.MustCompile(`^\s*<!--\s*(/?)oh:when\b(.*?)-->\s*$`)

// whenBlock is a marker of a text: line number (1-based), opening condition.
type whenBlock struct {
	line  int
	close bool
	expr  string
}

var errWhenUnbalanced = errors.New("unbalanced oh:when markers")

// scanWhen returns the markers of text, checking their syntax and their
// balance.
func scanWhen(text string) ([]whenBlock, error) {
	var out []whenBlock
	depth := 0
	for i, ln := range strings.Split(text, "\n") {
		if !strings.Contains(ln, "oh:when") {
			continue
		}
		m := whenMarkerRe.FindStringSubmatch(ln)
		if m == nil {
			return nil, fmt.Errorf("line %d: an oh:when marker stands alone on its line (<!-- oh:when … --> or <!-- /oh:when -->)", i+1)
		}
		b := whenBlock{line: i + 1, close: m[1] == "/", expr: strings.TrimSpace(m[2])}
		if b.close {
			if b.expr != "" {
				return nil, fmt.Errorf("line %d: a closing marker has no condition", i+1)
			}
			if depth == 0 {
				return nil, fmt.Errorf("line %d: %w (closing without opening)", i+1, errWhenUnbalanced)
			}
			depth--
		} else {
			if b.expr == "" {
				return nil, fmt.Errorf("line %d: oh:when without condition", i+1)
			}
			if _, err := parseWhen(b.expr); err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			depth++
		}
		out = append(out, b)
	}
	if depth != 0 {
		return nil, fmt.Errorf("%w (%d block(s) left open)", errWhenUnbalanced, depth)
	}
	return out, nil
}

// filterWhen keeps the blocks of text whose condition holds for one of envs
// and removes the markers. A malformed text is returned unchanged, with an
// error.
func filterWhen(text string, envs []whenEnv) (string, error) {
	if !strings.Contains(text, "oh:when") {
		return text, nil
	}
	if _, err := scanWhen(text); err != nil {
		return text, err
	}
	var b strings.Builder
	var stack []bool // kept, per open block
	kept := true
	trim := false // a block or a marker was removed: no double blank line
	lastBlank := true
	for _, ln := range strings.SplitAfter(text, "\n") {
		if m := whenMarkerRe.FindStringSubmatch(strings.TrimRight(ln, "\n")); m != nil {
			if m[1] == "/" {
				stack = stack[:len(stack)-1]
			} else {
				e, _ := parseWhen(m[2])
				stack = append(stack, e.holdsAny(envs))
			}
			kept = !slices.Contains(stack, false)
			trim = true
			continue
		}
		if !kept {
			continue
		}
		blank := strings.TrimSpace(ln) == ""
		if blank && trim && lastBlank {
			continue
		}
		if !blank {
			trim = false
		}
		lastBlank = blank
		b.WriteString(ln)
	}
	return b.String(), nil
}

// filterFor filters text for envs, keeping it whole (warning) when its
// markers are malformed: a session never fails on a team skill.
func filterFor(source, text string, envs ...whenEnv) string {
	out, err := filterWhen(text, envs)
	if err != nil {
		slog.Warn("bundle: oh:when markers ignored", "source", source, "error", err)
	}
	return out
}

// skillWhen reports whether the frontmatter condition of a skill holds for
// env (an invalid condition holds: CheckSkills reports it).
func skillWhen(d *skillDoc, env whenEnv) bool {
	if strings.TrimSpace(d.Front.When) == "" {
		return true
	}
	e, err := parseWhen(d.Front.When)
	return err != nil || e.holds(env)
}

// sessionEnv is the part of whenEnv shared by every agent of a bundle.
type sessionEnv struct {
	mcp, plugin            map[string]bool
	websearch, beadsWrite  bool
	ticket, wiki, headless bool
	entry, method          string
	graph, callers         map[string][]string
	// writes: the documentation paths of each agent given some (R4, `writes:`).
	writes map[string][]string
}

// newSessionEnv reads the session of a bundle being built (spec: its MCP
// servers, plugins, permissions and entry agent) from the workflow w, the
// project and the delegation graph of the bundle.
func newSessionEnv(w *workflow.Spec, projectPath string, spec sessionspec.BundleSpec, graph map[string][]string) sessionEnv {
	s := sessionEnv{mcp: map[string]bool{}, plugin: map[string]bool{}, entry: spec.EntryAgent, method: string(workflow.MethodStandard), graph: graph, callers: callersOf(graph)}
	if spec.Workflow != nil && spec.Workflow.Method != "" {
		s.method = spec.Workflow.Method
	}
	for _, m := range spec.MCP {
		s.mcp[m.Name] = true
	}
	for _, p := range spec.Plugins {
		s.plugin[p.ID] = true
	}
	s.websearch = ToolEffect(spec.Permissions, sessionspec.ActionWebSrch, "*") == sessionspec.EffectAllow
	if w != nil {
		s.headless = w.Headless
		for _, id := range w.Agents.Keys() {
			if paths := w.WritesOf(id); len(paths) > 0 {
				if s.writes == nil {
					s.writes = map[string][]string{}
				}
				s.writes[id] = paths
			}
		}
		for _, k := range w.Inputs.Keys() {
			if in, _ := w.Inputs.Get(k); in.Type == workflow.InputBeadsID || in.Type == workflow.InputBeadsIDs {
				s.ticket = true
			}
		}
		if w.Beads != nil {
			for _, e := range w.Beads.Allow {
				s.beadsWrite = s.beadsWrite || workflow.BeadsWrites(e)
			}
		}
	}
	if projectPath != "" {
		if st, err := os.Stat(filepath.Join(projectPath, "docs", "wiki")); err == nil && st.IsDir() {
			s.wiki = true
		}
	}
	return s
}

// bundleWhenEnvs returns the session of each agent of a built bundle.
func bundleWhenEnvs(w *workflow.Spec, projectPath string, spec sessionspec.BundleSpec) map[string]whenEnv {
	s := newSessionEnv(w, projectPath, spec, spec.SubagentGraph)
	out := map[string]whenEnv{}
	for _, a := range spec.Agents {
		out[a.ID] = s.agentEnv(a.ID, a.Permissions)
	}
	return out
}

// editsBeyond reports whether perms allow the edit of a file other than
// the `writes:` paths (a rule of the agent's own, not one of WritesRules).
func editsBeyond(perms []sessionspec.PermissionRule, paths []string) bool {
	for _, r := range perms {
		if r.Effect != sessionspec.EffectAllow || !toolMatch(sessionspec.ActionEdit, r.Action) {
			continue
		}
		if !slices.ContainsFunc(paths, func(p string) bool { return WritesPattern(p) == r.Resource }) {
			return true
		}
	}
	return false
}

// agentEnv is the session of agent id, whose permissions are perms.
func (s sessionEnv) agentEnv(id string, perms []sessionspec.PermissionRule) whenEnv {
	env := whenEnv{agent: id, roles: map[string]bool{}, calls: map[string]bool{}, mcp: s.mcp, plugin: s.plugin, method: s.method, flags: map[string]bool{}}
	if id == s.entry || len(s.callers[id]) == 0 {
		env.roles[roleEntry] = true
	}
	if len(s.callers[id]) > 0 {
		env.roles[roleSubagent] = true
	}
	for _, t := range s.graph[id] {
		env.calls[t] = true
	}
	shell := false
	for _, r := range perms {
		if toolMatch(sessionspec.ActionShell, r.Action) && r.Effect != sessionspec.EffectDeny {
			shell = true
		}
	}
	env.flags["shell"] = shell
	if shell {
		env.refusedShell = refusedShellHelpers(perms)
	}
	env.flags["websearch"] = s.websearch && ToolEffect(perms, sessionspec.ActionWebSrch, "*") != sessionspec.EffectDeny
	env.flags["beads_write"] = s.beadsWrite && ShellEffect(perms, "bd update x") != sessionspec.EffectDeny
	env.flags["ticket"] = s.ticket
	env.flags["wiki"] = s.wiki
	env.flags["headless"] = s.headless
	env.flags["writes"] = len(s.writes[id]) > 0
	if len(s.writes[id]) > 0 {
		env.ownEdits = editsBeyond(perms, s.writes[id])
	}
	return env
}
