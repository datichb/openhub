package bundle

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/gateway"
	"github.com/datichb/openhub/cli/internal/gateway/bdargs"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/testutil/golden"
	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/workflow/hubcat"
)

// Capability lint (N11): the instructions shipped to an agent (its prompt,
// inlined skills and the skills it may load) only ask for what its
// permissions allow. For each shipped workflow and each agent (and for each
// agent of the hub as the entry of a libre session, libre@<agent>):
//   - every shell command written in the content (code spans, shell blocks)
//     whose first word is a known tool is evaluated against the agent rules,
//     guards included: a refused command is an error, unless it is marked
//     `<!-- capability: user -->` on its line (a command for the user) or
//     listed in testdata/capability_allow.yaml;
//   - a command refused only while an operation is locked (`unlocks:`) is
//     reported, not refused;
//   - every task(subagent_type: "x") (on one line or several) and every
//     delegation written in prose (« délègue au `x` », « invoque `x` »,
//     x an agent of the hub) names an agent the agent may call; a skill
//     listed `conditional:` only delegates when the workflow map allows it;
//   - every bd command the agent may run is granted by the workflow's
//     Beads list (beads.allow), as the gateway checks it;
//   - every checkpoint that unlocks an operation has a member able to run it;
//   - an agent gated `after: cp-x` is not asked in the prompt before cp-x;
//   - every checkpoint id (`cp-x`) named in the content is a checkpoint of
//     the workflow, except on the lines marked `<!-- capability: optional -->`
//     (a line describing another workflow). A workflow without checkpoints
//     (libre) is not checked: its generated section says there are none;
// The report is pinned in testdata/capabilities.golden (-update).
//
// The bundles are built for capProject, a project of the common stacks: the
// commands granted from the detected stack (the verification commands of the
// reviewer, bricks.VerifyCommands) are those of a real session, and a
// command the content names but no stack grants is reported.

var (
	capCodeSpan   = regexp.MustCompile("`([^`\n]+)`")
	capPlacehold  = regexp.MustCompile(`<[^<>\n]+>|\[[A-Z][^\[\]\n]*\]`)
	capSplit      = regexp.MustCompile(`&&|\|\||;|\|`)
	capTaskRef    = regexp.MustCompile(`(?i)\btask\(\s*\{?[^()]*?\bsubagent_type:\s*"([a-z0-9-]+)"`)
	capProseTask  = regexp.MustCompile("(?i)\\b(?:d[ée]l[èée]gu|invoqu)[a-z]*\\s+(?:(?:à|au|aux|le|la|les|l'|l’|un|une)\\s*)?(?:(?:l'|l’)?(?:sous-)?agent\\s+)?`([a-z0-9-]+)`")
	capCheckpoint = regexp.MustCompile(`\bcp-[a-z0-9]+(?:-[a-z0-9]+)*\b`)
	capUserMarker = "<!-- capability:" // user (a command for the user), example, optional
	// A line that forbids, quotes an instruction for another agent or
	// documents a refusal is not a request to the agent.
	capNotAsked = regexp.MustCompile(`(?i)❌|⛔|jamais|never|ne pas|n'utilise|interdit|refus|«|»|\bpas de\b|\bsans\b`)
	// A quoted list item (`> 1. git commit …`) relays an instruction to
	// another agent.
	capRelay      = regexp.MustCompile("^\\s*>\\s*(?:[0-9]+\\.|«|`)")
	capNotSection = regexp.MustCompile(`(?i)ne fais pas|ne pas|interdit|jamais|NE |à éviter|anti-pattern`)
	capVocabulary = map[string]bool{
		"bd": true, "git": true, "npm": true, "npx": true, "yarn": true, "pnpm": true, "go": true, "cargo": true, "pytest": true,
		"python": true, "python3": true, "uv": true, "poetry": true, "tsc": true, "ruff": true, "make": true, "docker": true,
		"psql": true, "mysql": true, "sqlite3": true, "shasum": true, "sha256sum": true, "mkdir": true, "k6": true,
		"lighthouse": true, "hyperfine": true, "curl": true, "kubectl": true, "terraform": true, "helm": true,
	}
)

type capAllow struct {
	// Commands: "<workflow>/<agent>: <command>" refused on purpose (the
	// content tells the agent it may be refused, or targets the user).
	Commands []string `yaml:"commands"`
	// Tasks: "<workflow>/<agent>: <target>" delegations written in a skill
	// shared by several workflows.
	Tasks []string `yaml:"tasks"`
	// Checkpoints: "<workflow>/<agent>: <cp-id>" checkpoints named by a skill
	// shared by several workflows.
	Checkpoints []string `yaml:"checkpoints"`
	// References are reference skills (command sheets): what they list is
	// not asked of the agent.
	References []string `yaml:"references"`
	// Conditional are skills whose delegations depend on the workflow map
	// (« si `x` est dans ta colonne Peut déléguer à »): a delegation outside
	// the graph written only by one of them is reported, not refused.
	Conditional []string `yaml:"conditional"`
	// Claims: "<agent>: edit <pattern>" paths an agent claiming to change no
	// file still writes (readonly_claims_test.go).
	Claims []string `yaml:"claims"`
}

// capCommands extracts the shell commands of a text, normalized.
func capCommands(text string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(raw string) {
		for _, part := range capSplit.Split(raw, -1) {
			c := strings.TrimSpace(part)
			c = strings.TrimPrefix(c, "$ ")
			if i := strings.Index(c, " #"); i > 0 {
				c = strings.TrimSpace(c[:i])
			}
			c = capPlacehold.ReplaceAllString(c, "x")
			c = strings.Join(strings.Fields(c), " ")
			first, rest, _ := strings.Cut(c, " ")
			if (!capVocabulary[first] && !strings.HasPrefix(first, ".venv/bin/")) || rest == "" {
				continue // not a command, or a bare tool name (`bd`, `make`)
			}
			if !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
		}
	}
	notAsked := false // inside a section that lists what not to do
	var block []string
	inFence, skipNext, skipFence := false, false, false
	flush := func() {
		// A block that is a prompt for another agent ([Retours …], task(…))
		// or a template is not run by this agent.
		body := strings.Join(block, "\n")
		block = nil
		if len(strings.TrimSpace(body)) == 0 || notAsked {
			return
		}
		if first := strings.TrimSpace(strings.SplitN(strings.TrimSpace(body), "\n", 2)[0]); strings.HasPrefix(first, "[") ||
			strings.Contains(body, "task(") || strings.Contains(body, "Action requise") {
			return
		}
		for _, line := range strings.Split(body, "\n") {
			if !strings.Contains(line, capUserMarker) && !capNotAsked.MatchString(line) {
				add(line)
			}
		}
	}
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			if inFence {
				flush()
			} else if skipNext {
				skipFence = true
			}
			inFence, skipNext = !inFence, false
			if !inFence {
				skipFence = false
			}
			continue
		}
		if inFence && skipFence {
			continue
		}
		if t == strings.TrimSpace(t) && strings.HasPrefix(t, capUserMarker) && strings.HasSuffix(t, "-->") {
			skipNext = true // a marker alone on its line covers the next block
			continue
		}
		if inFence {
			block = append(block, line)
			continue
		}
		if strings.HasPrefix(t, "#") || (strings.HasPrefix(t, "**") && strings.HasSuffix(t, ":**")) || strings.HasSuffix(t, ":**") {
			notAsked = capNotSection.MatchString(t)
		}
		if notAsked || strings.Contains(line, capUserMarker) || capNotAsked.MatchString(line) || capRelay.MatchString(line) {
			continue
		}
		for _, m := range capCodeSpan.FindAllStringSubmatch(line, -1) {
			add(m[1])
		}
	}
	sort.Strings(out)
	return out
}

// capTaskRefs lists the delegations written in a text, task(…) calls on one
// line or several (`task({` then `subagent_type: "x"`), except on the lines
// marked `<!-- capability: optional -->` (an agent used when the workflow
// has it).
func capTaskRefs(text string) []string {
	var visible []string
	for _, l := range capVisibleLines(text) {
		visible = append(visible, l.text)
	}
	var out []string
	for _, m := range capTaskRef.FindAllStringSubmatch(strings.Join(visible, "\n"), -1) {
		if !slices.Contains(out, m[1]) {
			out = append(out, m[1])
		}
	}
	return out
}

// capProseTaskRefs lists the agents of known a text delegates to in prose
// (« délègue au `x` », « invoque `x` »), outside code blocks, the sections
// that list what not to do and the lines that forbid or quote (capNotAsked).
func capProseTaskRefs(text string, known map[string]string) []string {
	var out []string
	notAsked := false
	for _, l := range capVisibleLines(text) {
		if l.fenced {
			continue
		}
		if t := strings.TrimSpace(l.text); strings.HasPrefix(t, "#") || strings.HasSuffix(t, ":**") {
			notAsked = capNotSection.MatchString(t)
		}
		if notAsked || capNotAsked.MatchString(l.text) {
			continue
		}
		for _, m := range capProseTask.FindAllStringSubmatch(l.text, -1) {
			if _, ok := known[m[1]]; ok && !slices.Contains(out, m[1]) {
				out = append(out, m[1])
			}
		}
	}
	return out
}

// capCheckpointRefs lists the checkpoint ids named in a text, with the same
// marker as capTaskRefs.
func capCheckpointRefs(text string) []string {
	return capRefs(text, capCheckpoint, 0)
}

// capRefs lists the distinct matches (group) of re in text, except on the
// lines marked `<!-- capability: … -->` (or in the code block that follows a
// marker alone on its line).
func capRefs(text string, re *regexp.Regexp, group int) []string {
	var out []string
	for _, l := range capVisibleLines(text) {
		for _, m := range re.FindAllStringSubmatch(l.text, -1) {
			if !slices.Contains(out, m[group]) {
				out = append(out, m[group])
			}
		}
	}
	return out
}

type capLine struct {
	text   string
	fenced bool
}

// capVisibleLines are the lines of a text the refs are read from: a line
// marked `<!-- capability: … -->`, a code block following a marker alone on
// its line and the fence lines are blanked (a multi-line match does not
// cross them).
func capVisibleLines(text string) []capLine {
	var out []capLine
	inFence, skipNext, skipFence := false, false, false
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			if !inFence && skipNext {
				skipFence = true
			}
			inFence, skipNext = !inFence, false
			if !inFence {
				skipFence = false
			}
			out = append(out, capLine{})
			continue
		}
		if strings.HasPrefix(t, capUserMarker) && strings.HasSuffix(t, "-->") {
			skipNext = true
			out = append(out, capLine{})
			continue
		}
		if skipFence || strings.Contains(line, capUserMarker) {
			out = append(out, capLine{})
			continue
		}
		out = append(out, capLine{text: line, fenced: inFence})
	}
	return out
}

func capEffect(rules []sessionspec.PermissionRule, cmd string) sessionspec.Effect {
	var shell []sessionspec.PermissionRule
	for _, r := range rules {
		if r.Action == sessionspec.ActionShell || r.Action == sessionspec.ActionAll {
			shell = append(shell, r)
		}
	}
	eff := func(c string) sessionspec.Effect {
		if e, ok := evaluate(shell, c); ok {
			return e
		}
		return sessionspec.EffectAsk
	}
	e := eff(cmd)
	if e == sessionspec.EffectDeny {
		// A bare mention (`bd show`) stands for the command with arguments.
		if e2 := eff(cmd + " x"); e2 != sessionspec.EffectDeny {
			return e2
		}
	}
	return e
}

// capHasShell reports whether an agent may run any shell command at all: a
// coordinator without shell only writes commands for the agents it calls.
func capHasShell(rules []sessionspec.PermissionRule) bool {
	for _, r := range rules {
		if (r.Action == sessionspec.ActionShell || r.Action == sessionspec.ActionAll) && r.Effect != sessionspec.EffectDeny {
			return true
		}
	}
	return false
}

// capAgentText is what an agent reads: its prompt (inlined skills) and the
// skills it may load on demand.
func capAgentText(t *testing.T, hub string, b *Bundle, a sessionspec.AgentDef, files map[string]string, references []string) string {
	t.Helper()
	var sb strings.Builder
	body := a.Body
	// Inlined reference skills start at their heading and end at the next one.
	for _, ref := range references {
		h := inlinedSkillHeading(path.Base(ref))
		if i := strings.Index(body, h); i >= 0 {
			rest := body[i+len(h):]
			if j := strings.Index(rest, "> Skill `"); j >= 0 {
				body = body[:i] + rest[j:]
			} else {
				body = body[:i]
			}
		}
	}
	sb.WriteString(body)
	f, ok := files[a.ID]
	if !ok {
		return sb.String()
	}
	fm, err := bricks.ParseAgentFrontmatter(f)
	require.NoError(t, err)
	shipped := map[string]string{}
	for _, s := range b.Spec.Skills {
		shipped[s.ID] = s.Dir
	}
	for _, ref := range fm.NativeSkills {
		if slices.Contains(references, ref) {
			continue
		}
		if dir, ok := shipped[path.Base(ref)]; ok {
			data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
			if err == nil {
				sb.WriteString("\n")
				sb.Write(data)
			}
		}
	}
	return sb.String()
}

// capProject writes a project of the common stacks: Go, Node with
// TypeScript (web frontend: no skill filtered), Python with uv and Poetry,
// Rust, Maven, Make.
func capProject(t *testing.T) string {
	t.Helper()
	return sharedProjectDir(t, "cap", func(dir string) error {
		for name, content := range map[string]string{
			"go.mod":         "module example.com/demo\n",
			"package.json":   `{"dependencies":{"react":"19","react-dom":"19"},"devDependencies":{"typescript":"6","vitest":"4"}}`,
			"pyproject.toml": "[project]\nname = \"demo\"\ndependencies = [\"pytest\"]\n",
			"Cargo.toml":     "[package]\nname = \"demo\"\n",
			"pom.xml":        "<project/>\n",
			"Makefile":       "test:\n",
			"uv.lock":        "",
			"poetry.lock":    "",
		} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				return err
			}
		}
		return nil
	})
}

func TestCapabilities(t *testing.T) {
	t.Parallel()
	hub, cat, _ := shippedWorkflows(t)
	project := capProject(t)
	hc, err := hubcat.New(hub)
	require.NoError(t, err)
	files, err := bricks.FindAgentFiles(hub)
	require.NoError(t, err)
	var allow capAllow
	if data, err := os.ReadFile(filepath.Join("testdata", "capability_allow.yaml")); err == nil {
		require.NoError(t, yaml.Unmarshal(data, &allow))
	}
	allowed := map[string]bool{}
	for _, c := range slices.Concat(allow.Commands, allow.Tasks, allow.Checkpoints) {
		allowed[c] = true
	}
	used := map[string]bool{}
	usedCond := map[string]bool{}

	// Every agent is linted, members of no shipped workflow included: each
	// one is also the entry of a libre session (key libre@<agent>), linted
	// alone (its delegates are entries of their own libre session).
	type capRun struct {
		ref   workflow.Ref
		key   string
		entry string // only this agent is linted ("" = every member)
		opts  *workflow.SessionOptions
	}
	var runs []capRun
	libre := workflow.Ref{Layer: workflow.LayerHub, ID: "libre"}
	for _, ref := range cat.Refs() {
		runs = append(runs, capRun{ref: ref, key: ref.ID})
		if ref == libre {
			doc, ok := cat.Lookup(ref)
			require.True(t, ok)
			for _, id := range sortedKeys(files) {
				if id != doc.Spec.EntryAgent() {
					runs = append(runs, capRun{ref: ref, key: "libre@" + id, entry: id, opts: &workflow.SessionOptions{EntryAgent: id}})
				}
			}
		}
	}

	items := map[workflow.Ref][]shippedOpts{}
	for _, run := range runs {
		o := shippedOpts{Project: project}
		if run.opts != nil {
			o.Entry = run.opts.EntryAgent
		}
		items[run.ref] = append(items[run.ref], o)
	}
	warmShipped(t, items)

	var report, errs []string
	for _, run := range runs {
		ref := run.ref
		var entry string
		if run.opts != nil {
			entry = run.opts.EntryAgent
		}
		r, b := shippedBuildWith(t, ref, shippedOpts{Project: project, Entry: entry})
		wf := b.Spec.Workflow
		locks := SessionRules(wf, r.Spec.DefaultMode(), nil)
		for _, a := range b.Spec.Agents {
			if run.entry != "" && a.ID != run.entry {
				continue
			}
			text := capAgentText(t, hub, b, a, files, allow.References)
			key := run.key + "/" + a.ID
			if run.entry != "" {
				key = run.key
			}
			cmds := capCommands(text)
			if !capHasShell(a.Permissions) {
				cmds = nil
			}
			for _, cmd := range cmds {
				id := key + ": " + cmd
				switch eff := capEffect(a.Permissions, cmd); {
				case eff == sessionspec.EffectDeny:
					report = append(report, "deny   "+id)
					if allowed[id] {
						used[id] = true
					} else {
						errs = append(errs, "refused by the permissions of the agent: "+id)
					}
				case capEffect(locks, cmd) == sessionspec.EffectDeny:
					report = append(report, "locked "+id)
				}
				// C0.5: a bd command the agent may run must also be granted
				// by the workflow's Beads list (the gateway refuses it).
				if rest, ok := strings.CutPrefix(cmd, "bd "); ok && capEffect(a.Permissions, cmd) != sessionspec.EffectDeny {
					var allowList []string
					if r.Spec.Beads != nil {
						allowList = r.Spec.Beads.Allow
					}
					if c, err := bdargs.Parse(strings.Fields(rest)); err == nil && c.Name() != "" && !gateway.BeadsAllowed(c, allowList) {
						report = append(report, "beads  "+id)
						if allowed[id] {
							used[id] = true
						} else {
							errs = append(errs, "refused by the Beads list of the workflow: "+id)
						}
					}
				}
			}
			calls := map[string]bool{}
			for _, c := range b.Spec.SubagentGraph[a.ID] {
				calls[c] = true
			}
			// conditional names the conditional skill a delegation comes
			// from ("" when another text asks it too).
			without := map[string]string{}
			conditional := func(m string, refs func(string) []string) string {
				for _, c := range allow.Conditional {
					if _, ok := without[c]; !ok {
						without[c] = capAgentText(t, hub, b, a, files, append(slices.Clone(allow.References), c))
					}
					if without[c] != text && !slices.Contains(refs(without[c]), m) {
						return c
					}
				}
				return ""
			}
			prose := func(s string) []string { return capProseTaskRefs(s, files) }
			for _, m := range capTaskRefs(text) {
				if calls[m] {
					continue
				}
				id := key + ": " + m
				if c := conditional(m, capTaskRefs); c != "" {
					report = append(report, "cond   "+id+" ("+c+")")
					usedCond[c] = true
					continue
				}
				report = append(report, "task   "+id)
				if allowed[id] {
					used[id] = true
				} else {
					errs = append(errs, "delegation outside the graph: "+id)
				}
			}
			for _, m := range prose(text) {
				if calls[m] || m == a.ID || slices.Contains(capTaskRefs(text), m) {
					continue
				}
				id := key + ": " + m
				if c := conditional(m, prose); c != "" {
					report = append(report, "cond   "+id+" ("+c+")")
					usedCond[c] = true
					continue
				}
				report = append(report, "prose  "+id)
				if allowed[id] {
					used[id] = true
				} else {
					errs = append(errs, "delegation in prose outside the graph: "+id)
				}
			}
			for _, cp := range capCheckpointRefs(text) {
				if _, ok := wf.Checkpoint(cp); ok || len(wf.Checkpoints) == 0 {
					continue
				}
				id := key + ": " + cp
				report = append(report, "cp     "+id)
				if allowed[id] {
					used[id] = true
				} else {
					errs = append(errs, "checkpoint outside the workflow: "+id)
				}
			}
		}
		// Every unlocked operation has a member able to run it.
		probe := map[string]string{sessionspec.UnlockCommit: `git commit -m "x"`, sessionspec.UnlockClose: "bd close x", sessionspec.UnlockCreate: "bd create x"}
		for _, c := range wf.Checkpoints {
			for _, op := range c.Unlocks {
				cmd, ok := probe[op]
				if !ok {
					continue
				}
				able := false
				for _, a := range b.Spec.Agents {
					able = able || capEffect(a.Permissions, cmd) != sessionspec.EffectDeny
					// R4: an agent given documentation paths commits them only.
					for _, w := range r.Spec.WritesOf(a.ID) {
						only := cmd + " -- " + strings.ReplaceAll(WritesPattern(w), "*", "x.md")
						able = able || (op == sessionspec.UnlockCommit && capEffect(a.Permissions, only) != sessionspec.EffectDeny)
					}
				}
				if !able {
					errs = append(errs, fmt.Sprintf("%s: %s unlocks %s but no member may run %q", ref.ID, c.ID, op, cmd))
				}
			}
		}
		// An agent gated after a checkpoint is not asked before it.
		prompt, err := workflow.RenderPrompt(r, hc, workflow.PromptContext{Project: "demo", Location: "/src/demo", Mode: r.Spec.DefaultMode(), Runtime: "local", Lang: "fr", Workflow: ref.ID})
		require.NoError(t, err)
		errs = append(errs, gatedBeforeCheckpoint(ref.ID, wf, prompt)...)
	}
	for id := range allowed {
		if !used[id] {
			errs = append(errs, "stale entry of testdata/capability_allow.yaml: "+id)
		}
	}
	for _, c := range allow.Conditional {
		if !usedCond[c] {
			errs = append(errs, "stale conditional skill of testdata/capability_allow.yaml: "+c)
		}
	}
	sort.Strings(errs)
	for _, e := range errs {
		t.Log(e)
	}
	assert.Empty(t, errs)
	sort.Strings(report)
	golden.Assert(t, "capabilities", strings.Join(report, "\n")+"\n")
}

var capAsk = regexp.MustCompile(`(?i)invoqu|délègu|lance|task\(`)

// gatedBeforeCheckpoint finds the lines of the prompt that ask an agent
// gated after a checkpoint before any line naming that checkpoint.
func gatedBeforeCheckpoint(wfID string, wf *sessionspec.WorkflowRuntime, prompt string) []string {
	var out []string
	lines := strings.Split(prompt, "\n")
	for _, g := range wf.Gates {
		if _, isCP := wf.Checkpoint(g.After); !isCP {
			continue
		}
		cpSeen := false
		for _, l := range lines {
			if strings.Contains(l, "`"+g.After+"`") {
				cpSeen = true
			}
			if !cpSeen && strings.Contains(l, "`"+g.Agent+"`") && capAsk.MatchString(l) {
				out = append(out, fmt.Sprintf("%s: %s (after %s) asked before %s: %s", wfID, g.Agent, g.After, g.After, strings.TrimSpace(l)))
				break
			}
		}
	}
	return out
}

func TestCapCommands(t *testing.T) {
	got := capCommands("Lance `bd show <ID>` puis `git add . && git commit -m \"x\"`.\n```bash\n# comment\nnpm test\nnpx tsc --noEmit # types\n```\n`npm audit` <!-- capability: user -->\nTexte `foo bar`.")
	assert.Equal(t, []string{"bd show x", "git add .", "git commit -m \"x\"", "npm test", "npx tsc --noEmit"}, got)
}

func TestCapTaskRefs(t *testing.T) {
	text := "Lance `task(subagent_type: \"a\")` puis `Task(subagent_type: \"b\")`.\n```\ntask({\n  description: \"x\",\n  subagent_type: \"c\",\n})\n```\n<!-- capability: optional -->\n```\ntask({\n  subagent_type: \"d\"\n})\n```"
	assert.Equal(t, []string{"a", "b", "c"}, capTaskRefs(text))

	known := map[string]string{"planner": "", "designer": "", "debugger": "", "reviewer": "", "pathfinder": ""}
	prose := "Tu délègues au `planner`.\nInvoquer le `designer` en mode recon.\nNe délègue jamais au `debugger`.\n" +
		"Déléguer à l'agent `reviewer` <!-- capability: optional -->\nDélègue au `inconnu`.\n```\ndélègue au `pathfinder`\n```\n" +
		"## Ce que tu NE fais PAS\n- Invoquer le `pathfinder`"
	assert.Equal(t, []string{"planner", "designer"}, capProseTaskRefs(prose, known))
}

func TestCapCheckpointRefs(t *testing.T) {
	text := "Passe `cp-1`, puis `workflow_checkpoint({ id: \"cp-2\" })`.\nDans `quick`, `cp-commit`. <!-- capability: optional -->\n<!-- capability: optional -->\n```\ncp-feature\n```\nCP-2 : commit ? Le checkpoint `cp-<id>`."
	assert.Equal(t, []string{"cp-1", "cp-2"}, capCheckpointRefs(text))
}

// The former sweep prompt asked a developer (gated after cp-plan) to list
// the files before cp-plan (N7): the lint catches it.
func TestGatedBeforeCheckpoint(t *testing.T) {
	wf := &sessionspec.WorkflowRuntime{Checkpoints: []sessionspec.CheckpointDef{{ID: "cp-plan"}}, Gates: []sessionspec.AgentGate{{Agent: "developer", After: "cp-plan"}}}
	bad := "Découpage : fais lister les fichiers par un `developer`, lance-le.\n- `cp-plan` : présente la liste."
	assert.Len(t, gatedBeforeCheckpoint("sweep", wf, bad), 1)
	good := "1. Découpage par le `pathfinder`.\n2. `cp-plan` : présente le plan.\n3. Lance le `developer`."
	assert.Empty(t, gatedBeforeCheckpoint("sweep", wf, good))
}
