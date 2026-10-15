package workflow

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"text/template/parse"

	"github.com/datichb/openhub/cli/internal/gateway/bdargs"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Env is what validation may check a workflow against. Every member is
// optional: the rules that need a missing one are skipped.
type Env struct {
	Agents  AgentCatalog
	Skills  SkillCatalog
	Prompts PromptSource
	// Workflows tells whether a workflow id exists (target of a
	// precondition suggestion). Check fills it from the catalog.
	Workflows WorkflowCatalog
	// Isolation is the closed-world level of the current adapter (empty:
	// unknown, the isolation rule is skipped).
	Isolation sessionspec.IsolationLevel
}

// SkillCatalog gives access to the skills of the brick catalogue.
type SkillCatalog interface {
	HasSkill(ref string) bool
	// Closure returns every skill needed by roots (dependencies included)
	// and the problems found on the way.
	Closure(roots []string) ([]string, []SkillIssue)
}

// SkillIssue is a problem found while computing a skill closure.
type SkillIssue struct {
	Skill string
	// Kind is "missing" (unknown dependency), "duplicate" (two skills with
	// the same identifier) or "cycle".
	Kind   string
	Detail string
}

// WorkflowCatalog tells whether a workflow id exists in any layer.
type WorkflowCatalog interface {
	HasWorkflow(id string) bool
}

// PromptSource reads prompt templates. origin is the document that set
// `prompt.template`; path is relative to that layer's workflows directory.
type PromptSource interface {
	ReadPrompt(origin Origin, path string) ([]byte, error)
}

// Prompt template variables (contract with the workflows and the launcher):
// inputs are top-level fields ({{ .ticket }}); the session context is under
// .oh ({{ .oh.project }}). An input cannot be named "oh".
const ReservedInputID = "oh"

// OhVariables are the fields available under .oh in prompt templates.
var OhVariables = []string{"project", "location", "branch", "mode", "runtime", "lang", "workflow", "memories"}

// BeadsWrites reports whether a beads.allow entry may write to Beads: every
// entry that does not name a read command of bd (bdargs). A workflow whose
// entries all name read commands is read-only, write forms included
// (`comments add`, `ready --claim` are refused by the gateway).
func BeadsWrites(entry string) bool { return !bdargs.EntryIsRead(entry) }

// ExecPlugins are the plugins whose tools run commands (context-mode:
// ctx_execute…): refused in a read-only workflow, where they would bypass
// `bash: deny` (N24, S-19).
var ExecPlugins = []string{"context-mode"}

// PluginPackage strips the version or tag of a plugin spec
// ("@scope/pkg@^1" → "@scope/pkg", "pkg@latest" → "pkg").
func PluginPackage(spec string) string {
	start := 0
	if strings.HasPrefix(spec, "@") {
		start = 1
	}
	if i := strings.Index(spec[start:], "@"); i >= 0 {
		return spec[:start+i]
	}
	return spec
}

// BeadsPlanForbidden are the write subcommands refused even with risk: plan.
var BeadsPlanForbidden = []string{"delete"}

var (
	reKebabID = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	reInputID = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// Check resolves ref against cat and validates the result. The resolved
// workflow is nil when the extends chain cannot be built.
func Check(cat Catalog, ref Ref, opts *SessionOptions, env Env) (*Resolved, Diagnostics) {
	r, diags := ResolveSpec(cat, ref, opts)
	if r == nil {
		return nil, diags
	}
	switch {
	case opts != nil && opts.EntryAgent != "":
		diags = append(diags, r.SelectEntry(opts.EntryAgent, env.Agents)...)
	case r.Spec.Entry != nil && r.Spec.Entry.Selectable && env.Agents != nil:
		// default entry of a selectable workflow: same closure
		diags = append(diags, r.SelectEntry(r.Spec.EntryAgent(), env.Agents)...)
	}
	if wc, ok := cat.(WorkflowCatalog); ok && env.Workflows == nil {
		env.Workflows = wc
	}
	diags = append(diags, r.Validate(env)...)
	if opts != nil {
		// R7: the session keeps the checkpoints of its method only
		// (validated above with the others).
		r.Spec = r.Spec.ForMethod(r.Method)
		// R6: a fan-out session keeps its fan-out checkpoints, the others
		// the checkpoints of before.
		r.Spec = r.Spec.ForFanout(r.Fanout)
	}
	return r, diags
}

// Validate checks the resolved workflow (03 §3.2): schema values, agents and
// graph, checkpoints, prompt variables, skills and security rules. Each
// finding points to the document that set the offending value.
func (r *Resolved) Validate(env Env) Diagnostics {
	v := &validator{r: r, s: r.Spec, env: env}
	v.header()
	v.entry()
	v.inputs()
	v.prompt()
	v.agents()
	v.checkpoints()
	v.checkpointsVerify()
	v.methods()
	v.order()
	v.checkpointsBy()
	v.checkpointsApprover() // CP-R9
	v.preconditions()
	v.modes()
	v.resources()
	v.security()
	v.publish()
	v.composition()
	v.fanout() // R6
	return v.diags
}

type validator struct {
	r     *Resolved
	s     *Spec
	env   Env
	diags Diagnostics
}

func (v *validator) add(d Diagnostic) {
	v.r.Locate(&d)
	v.diags = append(v.diags, d)
}

func (v *validator) err(code, at string, args ...any)  { v.add(errDiag(code, at, args...)) }
func (v *validator) warn(code, at string, args ...any) { v.add(warnDiag(code, at, args...)) }

func (v *validator) enum(at, value string, valid bool, allowed ...string) {
	if !valid {
		v.err("enum_invalid", at, value, strings.Join(allowed, ", "))
	}
}

func (v *validator) header() {
	s := v.s
	if !reKebabID.MatchString(s.ID) {
		v.err("id_invalid", "id", s.ID)
	}
	if s.Category != "" {
		v.enum("category", string(s.Category), s.Category.Valid(), "develop", "frame", "quality", "knowledge", "other")
	}
	for i, f := range s.Enforce {
		if !IsEnforceable(f) {
			v.err("enforce_unknown_field", fmt.Sprintf("enforce[%d]", i), f, strings.Join(EnforceableFields(), ", "))
		}
	}
	if s.Risk == "" {
		v.err("field_required", "risk", "risk")
	} else {
		v.enum("risk", string(s.Risk), s.Risk.Valid(), "read", "plan", "write", "publish")
	}
	if s.Isolation != "" {
		v.enum("isolation", string(s.Isolation), s.Isolation.Valid(), "strict", "standard")
	}
	if s.CircuitBreaker != nil && s.CircuitBreaker.MaxConsecutiveSubagents != nil && *s.CircuitBreaker.MaxConsecutiveSubagents < 0 {
		v.err("negative_value", "circuit_breaker.max_consecutive_subagents", *s.CircuitBreaker.MaxConsecutiveSubagents)
	}
	if s.Limits != nil && s.Limits.BudgetUSD != nil && *s.Limits.BudgetUSD < 0 {
		v.err("negative_value", "limits.budget_usd", *s.Limits.BudgetUSD)
	}
	if s.Limits != nil && s.Limits.MaxFixRounds != nil && *s.Limits.MaxFixRounds < 0 {
		v.err("negative_value", "limits.max_fix_rounds", *s.Limits.MaxFixRounds)
	}
	if s.Limits != nil {
		for i, m := range s.Limits.Models {
			if strings.TrimSpace(m) == "" {
				v.err("field_required", fmt.Sprintf("limits.models[%d]", i), fmt.Sprintf("limits.models[%d]", i))
			}
		}
	}
}

// entryPath is where the entry agent is declared.
func (v *validator) entryPath() string {
	if v.s.Entry != nil && v.s.Entry.Agent != "" {
		return "entry.agent"
	}
	return "entry"
}

func (v *validator) entry() {
	entry := v.s.EntryAgent()
	ref, listed := v.s.Agents.Get(entry)
	if listed && ref.Role != RoleWorkflow && ref.Role.Valid() {
		v.err("entry_role", "agents."+entry+".role", entry, string(ref.Role))
	}
	mode := ref.Mode
	if v.env.Agents != nil {
		info, ok := v.env.Agents.Agent(entry)
		if !ok {
			if !listed { // a listed agent is reported by agents()
				v.add(unknownAgentDiag("agent_unknown", v.entryPath(), entry))
			}
			return
		}
		if mode == "" {
			mode = info.Mode
		}
	}
	if mode == ModeSubagent {
		at := v.entryPath()
		if listed && ref.Mode != "" {
			at = "agents." + entry + ".mode"
		}
		v.err("entry_not_primary", at, entry)
	}
}

func (v *validator) inputs() {
	names := map[string]bool{}
	for _, k := range v.s.Inputs.Keys() {
		names[k] = true
	}
	for _, k := range v.s.Inputs.Keys() {
		in, _ := v.s.Inputs.Get(k)
		base := "inputs." + k
		switch {
		case k == ReservedInputID:
			v.err("input_id_reserved", base, k)
		case !reInputID.MatchString(k):
			v.err("input_id_invalid", base, k)
		}
		if in.Type == "" {
			v.err("field_required", base+".type", "type")
		} else {
			v.enum(base+".type", string(in.Type), in.Type.Valid(),
				"string", "text", "bool", "int", "enum", "path", "branch", "beads-id", "beads-ids")
		}
		if in.Type == InputEnum && len(in.Values) == 0 {
			v.err("enum_values_missing", base+".values", k)
		}
		if in.Type != InputEnum && len(in.Values) > 0 {
			v.warn("field_ignored", base+".values", "values", string(in.Type))
		}
		if in.Picker != nil && in.Type != InputBeadsID && in.Type != InputBeadsIDs {
			v.warn("field_ignored", base+".picker", "picker", string(in.Type))
		}
		if in.MaxLength < 0 {
			v.err("negative_value", base+".max_length", in.MaxLength)
		}
		if in.Scope {
			v.scopeInput(base+".scope", k, in)
		}
		if IsSessionSource(in.From) {
			v.sessionFrom(base+".from", k, in)
		} else if in.From != "" {
			src, arg, ok := ParseFrom(in.From)
			switch {
			case !ok:
				v.err("input_from_invalid", base+".from", in.From)
			case !containsStr(InputSources, src):
				v.err("input_from_unknown_source", base+".from", src, strings.Join(InputSources, ", "))
			case arg == k || !names[arg]:
				v.err("input_from_unknown_input", base+".from", arg)
			}
		}
		if in.Default == nil || !in.Type.Valid() {
			continue
		}
		if str, ok := in.Default.(string); ok && strings.Contains(str, "{{") {
			// A default may refer to other inputs, not to itself.
			others := map[string]bool{}
			for n := range names {
				if n != k {
					others[n] = true
				}
			}
			v.template(str, base+".default", others)
			continue
		}
		if want, ok := checkInputValue(in, in.Default); !ok {
			v.err("input_default_invalid", base+".default", fmt.Sprint(in.Default), want)
		}
	}
}

func (v *validator) inputNames() map[string]bool {
	out := map[string]bool{}
	for _, k := range v.s.Inputs.Keys() {
		out[k] = true
	}
	return out
}

func (v *validator) prompt() {
	p := v.s.Prompt
	if p == nil {
		// Allowed, but the session starts without any instruction (A10);
		// reported on the workflows of the teams and projects.
		if v.r.Ref.Layer != LayerHub {
			v.warn("prompt_none", "prompt")
		}
		return
	}
	switch {
	case p.Template != "" && p.Text != "":
		v.err("prompt_both", "prompt")
		return
	case p.Template == "" && p.Text == "":
		v.err("prompt_empty", "prompt")
		return
	case p.Text != "":
		v.template(p.Text, "prompt.text", v.inputNames())
		return
	}
	clean := path.Clean(p.Template)
	if path.IsAbs(p.Template) || clean == ".." || strings.HasPrefix(clean, "../") {
		v.err("prompt_template_path", "prompt.template", p.Template)
		return
	}
	if v.env.Prompts == nil {
		return
	}
	origin, _ := v.r.Origins.Of("prompt.template")
	data, err := v.env.Prompts.ReadPrompt(origin, clean)
	if err != nil {
		v.err("prompt_template_missing", "prompt.template", p.Template, err)
		return
	}
	v.template(string(data), "prompt.template", v.inputNames())
}

// template checks that a Go text/template only uses known variables.
func (v *validator) template(src, at string, inputs map[string]bool) {
	unknown, err := TemplateVariables(src, inputs)
	if err != nil {
		v.err("prompt_parse", at, err)
		return
	}
	for _, name := range unknown {
		v.err("prompt_unknown_variable", at, name, strings.Join(sortedKeys(inputs), ", "))
	}
}

// TemplateVariables parses a prompt template and returns the variables it
// uses that are neither inputs nor .oh fields, in order of appearance.
// Template functions are not checked (the renderer defines them).
func TemplateVariables(src string, inputs map[string]bool) ([]string, error) {
	t := parse.New("prompt")
	t.Mode = parse.SkipFuncCheck
	trees := map[string]*parse.Tree{}
	if _, err := t.Parse(src, "", "", trees); err != nil {
		return nil, err
	}
	var unknown []string
	check := func(idents []string) {
		if len(idents) == 0 {
			return
		}
		name := idents[0]
		if name == ReservedInputID {
			if len(idents) > 1 && !containsStr(OhVariables, idents[1]) {
				name = ReservedInputID + "." + idents[1]
			} else {
				return
			}
		} else if inputs[name] {
			return
		}
		if !containsStr(unknown, name) {
			unknown = append(unknown, name)
		}
	}
	var walk func(n parse.Node, rooted bool)
	walk = func(n parse.Node, rooted bool) {
		switch x := n.(type) {
		case nil:
		case *parse.ListNode:
			if x == nil {
				return
			}
			for _, c := range x.Nodes {
				walk(c, rooted)
			}
		case *parse.ActionNode:
			walk(x.Pipe, rooted)
		case *parse.PipeNode:
			if x == nil {
				return
			}
			for _, c := range x.Cmds {
				walk(c, rooted)
			}
		case *parse.CommandNode:
			for _, a := range x.Args {
				walk(a, rooted)
			}
		case *parse.FieldNode:
			if rooted {
				check(x.Ident)
			}
		case *parse.VariableNode:
			if len(x.Ident) > 1 && x.Ident[0] == "$" {
				check(x.Ident[1:])
			}
		case *parse.ChainNode:
			walk(x.Node, rooted)
		case *parse.IfNode:
			walk(x.Pipe, rooted)
			walk(x.List, rooted)
			walk(x.ElseList, rooted)
		case *parse.RangeNode:
			walk(x.Pipe, rooted)
			walk(x.List, false) // dot is the element
			walk(x.ElseList, rooted)
		case *parse.WithNode:
			walk(x.Pipe, rooted)
			walk(x.List, false) // dot is the value
			walk(x.ElseList, rooted)
		case *parse.TemplateNode:
			walk(x.Pipe, rooted)
		}
	}
	for _, name := range sortedKeys(trees) {
		walk(trees[name].Root, true)
	}
	return unknown, nil
}

func (v *validator) agents() {
	s := v.s
	members := s.Members()
	checkpoints := map[string]bool{}
	for _, k := range s.Checkpoints.Keys() {
		checkpoints[k] = true
	}
	afterGraph := map[string][]string{}
	for _, k := range s.Agents.Keys() {
		a, _ := s.Agents.Get(k)
		base := "agents." + k
		if !reKebabID.MatchString(k) {
			v.err("agent_id_invalid", base, k)
		}
		if a.Role == "" {
			v.err("field_required", base+".role", "role")
		} else {
			v.enum(base+".role", string(a.Role), a.Role.Valid(), "workflow", "independent", "disabled")
		}
		if a.Mode != "" {
			v.enum(base+".mode", string(a.Mode), a.Mode.Valid(), "primary", "subagent")
		}
		if v.env.Agents != nil {
			if _, ok := v.env.Agents.Agent(k); !ok {
				v.add(unknownAgentDiag("agent_unknown", base, k))
			}
		}
		if a.After != "" {
			_, isAgent := s.Agents.Get(a.After)
			switch {
			case a.After == k || (!isAgent && !checkpoints[a.After]):
				v.err("after_unknown", base+".after", a.After)
			case isAgent:
				afterGraph[k] = append(afterGraph[k], a.After)
			}
		}
		for i, c := range a.Calls {
			if !containsStr(members, c) {
				v.err("calls_unknown", fmt.Sprintf("%s.calls[%d]", base, i), c)
			}
		}
		v.scratch(base, k, a, members)
		v.writes(base, a)
	}
	if cycle := findCycle(s.Agents.Keys(), afterGraph); cycle != nil {
		v.err("after_cycle", "agents."+cycle[0]+".after", strings.Join(cycle, " → "))
	}

	graph := DelegationGraph(s, v.env.Agents)
	if cycle := findCycle(members, graph); cycle != nil {
		v.err("graph_cycle", v.callsPath(cycle[0]), strings.Join(cycle, " → "))
	}
	if !graphKnown(s, v.env.Agents) {
		return
	}
	reach := Reachable(s.EntryAgent(), graph)
	for _, k := range s.Agents.Keys() {
		if a, _ := s.Agents.Get(k); a.Role == RoleWorkflow && !reach[k] {
			v.err("agent_unreachable", "agents."+k, k, s.EntryAgent())
		}
	}
}

// scratch checks the scratch folder of an agent (SCR): its readers are
// other agents of the workflow, each named once.
func (v *validator) scratch(base, owner string, a AgentRef, members []string) {
	if a.Scratch == nil || a.Role == RoleDisabled {
		return
	}
	seen := map[string]bool{}
	for i, r := range a.Scratch.Readers {
		p := fmt.Sprintf("%s.scratch.readers[%d]", base, i)
		switch {
		case r == owner:
			v.err("scratch_reader_owner", p, r)
		case !containsStr(members, r):
			v.err("scratch_reader_unknown", p, r)
		case seen[r]:
			v.err("duplicate_entry", p, r)
		}
		seen[r] = true
	}
}

func (v *validator) callsPath(agent string) string {
	if ref, ok := v.s.Agents.Get(agent); ok && ref.Calls != nil {
		return "agents." + agent + ".calls"
	}
	if _, ok := v.s.Agents.Get(agent); ok {
		return "agents." + agent
	}
	return v.entryPath()
}

func (v *validator) checkpoints() {
	s := v.s
	allowed := s.AllowedModes()
	unlockedBy := map[UnlockOp]string{}
	for _, k := range s.Checkpoints.Keys() {
		cp, _ := s.Checkpoints.Get(k)
		base := "checkpoints." + k
		if !reKebabID.MatchString(k) {
			v.err("checkpoint_id_invalid", base, k)
		}
		if _, clash := s.Agents.Get(k); clash || k == s.EntryAgent() {
			v.err("checkpoint_agent_clash", base, k)
		}
		for _, mode := range sortedKeys(cp.Mode) {
			b := cp.Mode[mode]
			// Behaviors of built-in modes that are not allowed are kept for
			// patches that allow them again; other modes are likely typos.
			if !containsStr(allowed, mode) && !containsStr(DefaultModes, mode) {
				v.err("checkpoint_mode_unknown", base+".mode."+mode, mode, strings.Join(allowed, ", "))
			}
			v.enum(base+".mode."+mode, string(b), b.Valid(), "pause", "auto", "skip", "conditional")
			if b == BehaviorConditional && strings.TrimSpace(cp.Condition) == "" && cp.When.IsZero() {
				v.err("checkpoint_condition_missing", base+".mode."+mode, k, mode)
			}
		}
		for _, mode := range allowed {
			if _, ok := cp.Mode[mode]; !ok {
				v.warn("checkpoint_mode_missing", base+".mode", k, mode)
			}
		}
		if cp.Remote != "" {
			v.enum(base+".remote", string(cp.Remote), cp.Remote.Valid(), "auto", "defer", "forbid")
		}
		if cp.Wiki != "" {
			v.enum(base+".wiki", string(cp.Wiki), cp.Wiki.Valid(), string(WikiSync))
			if cp.Wiki.Valid() && !containsOp(cp.Unlocks, UnlockCommit) && !cp.Disabled {
				v.err("checkpoint_wiki_commit", base+".wiki", k)
			}
		}
		if len(cp.Unlocks) > 0 {
			for _, mode := range sortedKeys(cp.Mode) {
				if cp.Mode[mode] == BehaviorSkip {
					// Treated as pause: skipping would drop the lock (N24).
					v.warn("unlock_skip", base+".mode."+mode, k, mode)
				}
			}
		}
		for i, op := range cp.Unlocks {
			at := fmt.Sprintf("%s.unlocks[%d]", base, i)
			if !op.Valid() {
				// An operation of a newer oh: warned, not refused, so that a
				// team on mixed versions keeps working (DC9). It locks nothing
				// here.
				v.warn("unlock_unknown", at, string(op), UnlockOpNames())
				continue
			}
			if op == UnlockPush && s.Risk != RiskPublish {
				// No agent may push outside a publishing workflow (DC7): the
				// lock opens nothing.
				v.warn("unlock_push_without_publish", at, k)
			}
			prev, dup := unlockedBy[op]
			switch {
			case dup && op == UnlockCommit && cp.UnlockAgain:
				// p5-fixes: a second commit checkpoint (schema_unlock.go).
			case dup:
				v.err("unlock_duplicate", at, string(op), prev, k)
			case op == UnlockCommit && cp.UnlockAgain:
				v.err("unlock_again_first", base+".unlock_again", k)
				unlockedBy[op] = k
			default:
				unlockedBy[op] = k
			}
		}
		if cp.UnlockAgain && !containsOp(cp.Unlocks, UnlockCommit) {
			v.err("unlock_again_without_commit", base+".unlock_again", k)
		}
	}
	v.tasksChecks()
}

// tasksChecks checks the breakdowns read by oh (`tasks:`, sweep-fanout):
// one checkpoint at most, a bounded number of tasks, domains named once.
func (v *validator) tasksChecks() {
	first := ""
	for _, k := range v.s.Checkpoints.Keys() {
		cp, _ := v.s.Checkpoints.Get(k)
		if cp.Tasks == nil || cp.Disabled {
			continue
		}
		base := "checkpoints." + k + ".tasks"
		if first != "" {
			v.err("tasks_duplicate", base, k, first)
		} else {
			first = k
		}
		if cp.Tasks.Max < 0 || cp.Tasks.Max > MaxTasks {
			v.err("tasks_max", base+".max", cp.Tasks.Max, MaxTasks)
		}
		v.list(base+".domains", cp.Tasks.Domains, "")
	}
}

// checkpointsBy checks `by:` of each checkpoint (L3): workflow members,
// once each. A headless workflow (nobody follows it) warns about a
// checkpoint that may wait for the user.
func (v *validator) checkpointsBy() {
	s := v.s
	members := map[string]bool{}
	for _, id := range s.Members() {
		if a, ok := s.Agents.Get(id); !ok || a.Role != RoleDisabled {
			members[id] = true
		}
	}
	for _, k := range s.Checkpoints.Keys() {
		cp, _ := s.Checkpoints.Get(k)
		seen := map[string]bool{}
		for i, id := range cp.By {
			at := fmt.Sprintf("checkpoints.%s.by[%d]", k, i)
			switch {
			case !members[id]:
				v.err("checkpoint_by_unknown", at, id, k)
			case seen[id]:
				v.warn("checkpoint_by_duplicate", at, id, k)
			}
			seen[id] = true
		}
		if s.Headless && !cp.Disabled {
			for _, mode := range s.AllowedModes() {
				if b := cp.Behavior(mode); b == BehaviorPause || b == BehaviorConditional {
					v.warn("headless_checkpoint_pause", "checkpoints."+k, k, mode)
					break
				}
			}
		}
	}
}

func (v *validator) preconditions() {
	s := v.s
	for _, k := range s.Preconditions.Keys() {
		pc, _ := s.Preconditions.Get(k)
		base := "preconditions." + k
		if !reKebabID.MatchString(k) {
			v.err("precondition_id_invalid", base, k)
		}
		if pc.Check.IsZero() {
			v.err("precondition_check_invalid", base+".check", k)
		}
		v.preconditionRepoChecks(base, pc.Check)
		for i, p := range pc.Check.PathExists {
			clean := path.Clean(p)
			if p == "" || path.IsAbs(p) || clean == ".." || strings.HasPrefix(clean, "../") {
				v.err("precondition_path_invalid", fmt.Sprintf("%s.check.path_exists[%d]", base, i), p)
			}
		}
		if pc.OnFail != "" {
			v.enum(base+".on_fail", string(pc.OnFail), pc.OnFail.Valid(), "suggest", "block")
		}
		if pc.Suggest == nil {
			continue
		}
		switch wf := pc.Suggest.Workflow; {
		case wf == "":
			v.err("field_required", base+".suggest.workflow", "workflow")
		case wf == s.ID:
			v.err("precondition_self", base+".suggest.workflow", wf)
		case v.env.Workflows != nil && !v.env.Workflows.HasWorkflow(wf):
			v.err("precondition_unknown_workflow", base+".suggest.workflow", wf)
		}
	}
}

func (v *validator) modes() {
	s := v.s
	if s.Modes == nil {
		return
	}
	seen := map[string]bool{}
	for i, m := range s.Modes.Allowed {
		p := fmt.Sprintf("modes.allowed[%d]", i)
		switch {
		case strings.TrimSpace(m) == "":
			v.err("field_required", p, "mode")
		case seen[m]:
			v.err("duplicate_entry", p, m)
		}
		seen[m] = true
	}
	if s.Modes.Default != "" && !containsStr(s.AllowedModes(), s.Modes.Default) {
		v.err("mode_default_not_allowed", "modes.default", s.Modes.Default, strings.Join(s.AllowedModes(), ", "))
	}
}

func (v *validator) resources() {
	s := v.s
	members := s.Members()
	if s.Models != nil {
		if s.Models.Default != "" && !validModelRef(s.Models.Default) {
			v.err("model_invalid", "models.default", s.Models.Default)
		}
		for _, agent := range sortedKeys(s.Models.Agents) {
			p := "models.agents." + agent
			if !containsStr(members, agent) {
				v.err("model_agent_unknown", p, agent)
			}
			if !validModelRef(s.Models.Agents[agent]) {
				v.err("model_invalid", p, s.Models.Agents[agent])
			}
		}
	}
	v.skills(members)
	pluginIDs := make([]string, len(s.Plugins))
	for i, p := range s.Plugins {
		pluginIDs[i] = p.ID
	}
	v.list("plugins", pluginIDs, "id")
	v.list("mcp", s.MCP, "")
	if s.Beads != nil {
		v.list("beads.allow", s.Beads.Allow, "")
	}
	if s.Runtime != nil {
		for i, rt := range s.Runtime.Allowed {
			v.enum(fmt.Sprintf("runtime.allowed[%d]", i), string(rt), rt.Valid(), "local", "container", "remote")
		}
		if s.Runtime.Default != "" {
			v.enum("runtime.default", string(s.Runtime.Default), s.Runtime.Default.Valid(), "local", "container", "remote")
			if !s.AllowsRuntime(s.Runtime.Default) {
				v.err("runtime_default_not_allowed", "runtime.default", string(s.Runtime.Default), joinRuntimes(s.AllowedRuntimes()))
			}
		}
	}
	ids := make([]string, len(s.Outputs))
	for i, o := range s.Outputs {
		ids[i] = o.ID
		p := fmt.Sprintf("outputs[%d]", i)
		if o.Type == "" {
			v.err("field_required", p+".type", "type")
		} else {
			v.enum(p+".type", string(o.Type), o.Type.Valid(), "branch", "merge_request", "beads-ids", "path", "report", "text")
		}
		v.outputRecord(p, o)
	}
	v.list("outputs", ids, "id")
}

// list reports empty and duplicate identifiers of a list field.
func (v *validator) list(field string, ids []string, sub string) {
	seen := map[string]bool{}
	for i, id := range ids {
		p := fmt.Sprintf("%s[%d]", field, i)
		if sub != "" {
			p += "." + sub
		}
		switch {
		case strings.TrimSpace(id) == "":
			v.err("field_required", p, field)
		case seen[id]:
			v.err("duplicate_entry", p, id)
		}
		seen[id] = true
	}
}

func (v *validator) skills(members []string) {
	s := v.s
	cat := v.env.Skills
	var extra, deny []string
	if s.Skills != nil {
		extra, deny = s.Skills.Extra, s.Skills.Deny
	}
	if cat == nil {
		return
	}
	for i, ref := range extra {
		if !cat.HasSkill(ref) {
			v.err("skill_unknown", fmt.Sprintf("skills.extra[%d]", i), ref)
		}
	}
	for i, ref := range deny {
		// A deny entry is a ref or a bare identifier (any skill of that name).
		if strings.Contains(ref, "/") && !cat.HasSkill(ref) {
			v.warn("skill_unknown_deny", fmt.Sprintf("skills.deny[%d]", i), ref)
		}
	}
	var roots []string
	if v.env.Agents != nil {
		for _, m := range members {
			if info, ok := v.env.Agents.Agent(m); ok {
				roots = append(roots, info.Skills...)
			}
		}
	}
	roots = append(roots, extra...)
	denied := func(ref string) bool { return containsStr(deny, ref) || containsStr(deny, path.Base(ref)) }
	var kept []string
	for _, r := range roots {
		if !denied(r) && !containsStr(kept, r) {
			kept = append(kept, r)
		}
	}
	closure, issues := cat.Closure(kept)
	for _, is := range issues {
		if is.Kind == "missing" && containsStr(extra, is.Skill) {
			continue // already reported on skills.extra
		}
		code := "skill_closure"
		if is.Kind == "duplicate" {
			code = "skill_duplicate"
		}
		v.err(code, "skills", is.Skill, is.Detail)
	}
	for _, ref := range closure {
		if denied(ref) {
			v.err("skill_denied_required", "skills.deny", ref)
		}
	}
}

func (v *validator) security() {
	s := v.s
	if s.Risk == RiskPlan {
		v.planRules()
	}
	if s.Risk == RiskRead {
		if v.env.Agents != nil {
			for _, m := range s.Members() {
				info, ok := v.env.Agents.Agent(m)
				if !ok {
					continue
				}
				p := "agents." + m
				if _, listed := s.Agents.Get(m); !listed {
					p = v.entryPath()
				}
				if info.Edits {
					v.err("read_agent_writes", p, m, "edit")
				}
				if info.Shell {
					v.err("read_agent_writes", p, m, "shell")
				}
			}
		}
		for i, pl := range s.Plugins {
			if containsStr(ExecPlugins, PluginPackage(pl.ID)) {
				v.err("read_plugin_exec", fmt.Sprintf("plugins[%d]", i), pl.ID)
			}
		}
		if s.Beads == nil {
			v.err("read_beads_unrestricted", "risk")
		} else {
			for i, cmd := range s.Beads.Allow {
				if BeadsWrites(cmd) {
					v.err("read_beads_write", fmt.Sprintf("beads.allow[%d]", i), cmd)
				}
			}
		}
	}
	v.parallelWriters()
	if s.AllowsRuntime(RuntimeRemote) {
		for _, k := range s.Checkpoints.Keys() {
			cp, _ := s.Checkpoints.Get(k)
			if cp.RemotePolicy() != RemoteForbid {
				continue
			}
			pauses := false
			for _, mode := range s.AllowedModes() {
				if cp.Behavior(mode) == BehaviorPause {
					pauses = true
				}
			}
			if cp.IsMandatory() || pauses {
				v.err("remote_forbidden_checkpoint", "checkpoints."+k+".remote", k)
			}
		}
	}
	if s.Isolation == IsolationStrict && v.env.Isolation != "" && v.env.Isolation != sessionspec.IsolationFull {
		v.warn("isolation_unsupported", "isolation", string(v.env.Isolation))
	}
}

// parallelWriters warns about a writing workflow whose agents (two or more
// that edit files) may all commit at will: no checkpoint unlocks commit
// (N7). A free session (entry.selectable, `libre`) has no gate by nature.
func (v *validator) parallelWriters() {
	s := v.s
	if s.Risk != RiskWrite || v.env.Agents == nil || (s.Entry != nil && s.Entry.Selectable) {
		return
	}
	for _, k := range s.Checkpoints.Keys() {
		if cp, _ := s.Checkpoints.Get(k); containsOp(cp.Unlocks, UnlockCommit) {
			return
		}
	}
	var writers []string
	for _, m := range s.Members() {
		if info, ok := v.env.Agents.Agent(m); ok && info.Edits {
			writers = append(writers, m)
		}
	}
	if len(writers) >= 2 {
		v.warn("parallel_writers_without_commit_gate", "checkpoints", strings.Join(writers, ", "))
	}
}

// planRules: risk: plan edits no file and writes Beads only through an
// explicit allow-list without delete.
func (v *validator) planRules() {
	s := v.s
	if v.env.Agents != nil {
		for _, m := range s.Members() {
			info, ok := v.env.Agents.Agent(m)
			if !ok {
				continue
			}
			p := "agents." + m
			if _, listed := s.Agents.Get(m); !listed {
				p = v.entryPath()
			}
			if info.Edits {
				v.err("plan_agent_writes", p, m, "edit")
			}
			if info.Shell {
				v.err("plan_agent_writes", p, m, "shell")
			}
		}
	}
	if s.Beads == nil {
		v.err("plan_beads_unrestricted", "risk")
		return
	}
	writes := false
	for i, cmd := range s.Beads.Allow {
		if containsStr(BeadsPlanForbidden, cmd) {
			v.err("plan_beads_delete", fmt.Sprintf("beads.allow[%d]", i), cmd)
		} else if BeadsWrites(cmd) {
			writes = true
		}
	}
	if !writes {
		v.warn("plan_without_beads_write", "risk")
	}
}

// validModelRef accepts "provider/model" with an optional "#variant".
func validModelRef(s string) bool {
	provider, model, ok := strings.Cut(s, "/")
	return ok && provider != "" && model != "" && !strings.ContainsAny(s, " \t")
}
