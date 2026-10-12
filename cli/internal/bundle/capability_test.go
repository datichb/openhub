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
	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/workflow/hubcat"
)

// Capability lint (N11): the instructions shipped to an agent (its prompt,
// inlined skills and the skills it may load) only ask for what its
// permissions allow. For each shipped workflow and each agent:
//   - every shell command written in the content (code spans, shell blocks)
//     whose first word is a known tool is evaluated against the agent rules,
//     guards included: a refused command is an error, unless it is marked
//     `<!-- capability: user -->` on its line (a command for the user) or
//     listed in testdata/capability_allow.yaml;
//   - a command refused only while an operation is locked (`unlocks:`) is
//     reported, not refused;
//   - every task(subagent_type: "x") names an agent the agent may call;
//   - every bd command the agent may run is granted by the workflow's
//     Beads list (beads.allow), as the gateway checks it;
//   - every checkpoint that unlocks an operation has a member able to run it;
//   - an agent gated `after: cp-x` is not asked in the prompt before cp-x.
// The report is pinned in testdata/capabilities.golden (-update).

var (
	capCodeSpan   = regexp.MustCompile("`([^`\n]+)`")
	capPlacehold  = regexp.MustCompile(`<[^<>\n]+>|\[[A-Z][^\[\]\n]*\]`)
	capSplit      = regexp.MustCompile(`&&|\|\||;|\|`)
	capTaskRef    = regexp.MustCompile(`task\(subagent_type:\s*"([a-z0-9-]+)"`)
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
	// References are reference skills (command sheets): what they list is
	// not asked of the agent.
	References []string `yaml:"references"`
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

// capTaskRefs lists the delegations written in a text, except on the lines
// marked `<!-- capability: optional -->` (an agent used when the workflow
// has it).
func capTaskRefs(text string) []string {
	var out []string
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
			continue
		}
		if strings.HasPrefix(t, capUserMarker) && strings.HasSuffix(t, "-->") {
			skipNext = true
			continue
		}
		if skipFence || strings.Contains(line, capUserMarker) {
			continue
		}
		for _, m := range capTaskRef.FindAllStringSubmatch(line, -1) {
			if !slices.Contains(out, m[1]) {
				out = append(out, m[1])
			}
		}
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

func TestCapabilities(t *testing.T) {
	hub, cat, env := shippedWorkflows(t)
	hc, err := hubcat.New(hub)
	require.NoError(t, err)
	files, err := bricks.FindAgentFiles(hub)
	require.NoError(t, err)
	var allow capAllow
	if data, err := os.ReadFile(filepath.Join("testdata", "capability_allow.yaml")); err == nil {
		require.NoError(t, yaml.Unmarshal(data, &allow))
	}
	allowed := map[string]bool{}
	for _, c := range append(allow.Commands, allow.Tasks...) {
		allowed[c] = true
	}
	used := map[string]bool{}

	var report, errs []string
	for _, ref := range cat.Refs() {
		r, diags := workflow.Check(cat, ref, nil, env)
		require.False(t, diags.HasErrors(), "%v", diags)
		b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec})
		require.NoError(t, err)
		wf := b.Spec.Workflow
		locks := SessionRules(wf, r.Spec.DefaultMode(), nil)
		for _, a := range b.Spec.Agents {
			text := capAgentText(t, hub, b, a, files, allow.References)
			key := ref.ID + "/" + a.ID
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
			for _, m := range capTaskRefs(text) {
				if calls[m] {
					continue
				}
				id := key + ": " + m
				report = append(report, "task   "+id)
				if allowed[id] {
					used[id] = true
				} else {
					errs = append(errs, "delegation outside the graph: "+id)
				}
			}
		}
		// Every unlocked operation has a member able to run it.
		probe := map[string]string{sessionspec.UnlockCommit: "git commit -m x", sessionspec.UnlockClose: "bd close x", sessionspec.UnlockCreate: "bd create x"}
		for _, c := range wf.Checkpoints {
			for _, op := range c.Unlocks {
				cmd, ok := probe[op]
				if !ok {
					continue
				}
				able := false
				for _, a := range b.Spec.Agents {
					able = able || capEffect(a.Permissions, cmd) != sessionspec.EffectDeny
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
	sort.Strings(errs)
	for _, e := range errs {
		t.Log(e)
	}
	assert.Empty(t, errs)
	sort.Strings(report)
	assertGoldenFile(t, filepath.Join("testdata", "capabilities.golden"), strings.Join(report, "\n")+"\n")
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

func assertGoldenFile(t *testing.T, name, content string) {
	t.Helper()
	if *updateGolden {
		require.NoError(t, os.WriteFile(name, []byte(content), 0o644))
		return
	}
	want, err := os.ReadFile(name)
	require.NoError(t, err, "missing %s (go test ./internal/bundle -run TestCapabilities -update)", name)
	assert.Equal(t, string(want), content, name)
}

func TestCapCommands(t *testing.T) {
	got := capCommands("Lance `bd show <ID>` puis `git add . && git commit -m \"x\"`.\n```bash\n# comment\nnpm test\nnpx tsc --noEmit # types\n```\n`npm audit` <!-- capability: user -->\nTexte `foo bar`.")
	assert.Equal(t, []string{"bd show x", "git add .", "git commit -m \"x\"", "npm test", "npx tsc --noEmit"}, got)
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
