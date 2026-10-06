package workflow

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/datichb/openhub/cli/internal/limits"
)

// ApplyPatch returns parent with patch applied (`extends`). parent is not
// modified. Security fields may only be hardened: a loosening is reported as
// an error and the parent value is kept.
//
// Patch rules (see schema.go): a field written in the patch replaces the
// parent value, lists are replaced as a whole, inputs / agents / checkpoints
// and preconditions (and a few nested maps) are merged by key; `role:
// disabled` removes an agent, `disabled: true` removes a checkpoint unless it
// is mandatory, or a precondition.
func ApplyPatch(parent *Spec, patch *Document, parentRef string) (*Spec, Diagnostics) {
	out := parent.Clone()
	p := &patcher{dst: out, doc: patch, parent: parentRef}
	p.apply()
	return out, p.diags
}

type patcher struct {
	dst    *Spec
	doc    *Document
	parent string
	diags  Diagnostics
	// removed lists the map entries the patch removed ("agents.x").
	removed []string
	// blocked holds the top-level fields locked by a parent (`enforce`).
	blocked map[string]bool
}

func (p *patcher) has(path string) bool { return !p.blocked[topField(path)] && p.doc.Has(path) }

func (p *patcher) report(d Diagnostic) {
	d.Source = p.doc.Source.String()
	d.Pos = p.doc.Pos(d.Path)
	p.diags = append(p.diags, d)
}

func (p *patcher) loosening(path string, value, parentValue any) {
	p.report(errDiag("loosening", path, fmt.Sprint(value), fmt.Sprint(parentValue), p.parent))
}

func (p *patcher) apply() {
	s, d := p.doc.Spec, p.dst
	d.ID = s.ID
	d.Version = s.Version
	d.Extends = ""
	p.enforce()

	if p.has("category") {
		d.Category = s.Category
	}
	if p.has("label") {
		d.Label = s.Label
	}
	if p.has("description") {
		d.Description = s.Description
	}
	if p.has("risk") {
		if d.Risk.Valid() && s.Risk.Rank() > d.Risk.Rank() {
			p.loosening("risk", s.Risk, d.Risk)
		} else {
			d.Risk = s.Risk
		}
	}
	if p.has("isolation") {
		if d.Isolation == IsolationStrict && s.Isolation != IsolationStrict {
			p.loosening("isolation", s.Isolation, d.Isolation)
		} else {
			d.Isolation = s.Isolation
		}
	}
	if p.has("code_mode") {
		d.CodeMode = s.CodeMode
	}
	if p.has("entry.agent") || (p.has("entry") && s.Entry == nil) {
		d.Entry = s.Entry
	}
	p.inputs()
	if p.has("prompt") {
		d.Prompt = s.Prompt
	}
	p.agents()
	p.checkpoints()
	p.modes()
	if p.has("circuit_breaker.max_consecutive_subagents") {
		if d.CircuitBreaker == nil {
			d.CircuitBreaker = &CircuitBreaker{}
		}
		d.CircuitBreaker.MaxConsecutiveSubagents = s.CircuitBreaker.MaxConsecutiveSubagents
	}
	p.models()
	if p.has("skills.extra") || p.has("skills.deny") {
		if d.Skills == nil {
			d.Skills = &SkillSelection{}
		}
		if p.has("skills.extra") {
			d.Skills.Extra = s.Skills.Extra
		}
		if p.has("skills.deny") {
			d.Skills.Deny = s.Skills.Deny
		}
	}
	if p.has("plugins") {
		d.Plugins = s.Plugins
	}
	if p.has("mcp") {
		d.MCP = s.MCP
	}
	p.beads()
	p.runtime()
	if p.has("outputs") {
		d.Outputs = s.Outputs
	}
	p.limits()
	p.preconditions()
}

// preconditions are merged by id; `disabled: true` removes one.
func (p *patcher) preconditions() {
	for _, k := range p.doc.Spec.Preconditions.Keys() {
		v, _ := p.doc.Spec.Preconditions.Get(k)
		base := "preconditions." + k
		old, exists := p.dst.Preconditions.Get(k)
		if v.Disabled {
			if !exists {
				p.report(warnDiag("patch_unknown_precondition", base, k, p.parent))
			} else {
				p.dst.Preconditions.Delete(k)
				p.removed = append(p.removed, base)
			}
			continue
		}
		if !exists {
			p.dst.Preconditions.Set(k, v)
			continue
		}
		h := func(f string) bool { return p.has(base + "." + f) }
		if h("label") {
			old.Label = v.Label
		}
		if h("check") {
			old.Check = v.Check
		}
		if h("on_fail") {
			old.OnFail = v.OnFail
		}
		if h("suggest") {
			old.Suggest = v.Suggest
		}
		p.dst.Preconditions.Set(k, old)
	}
}

func (p *patcher) inputs() {
	if p.blocked["inputs"] {
		return
	}
	for _, k := range p.doc.Spec.Inputs.Keys() {
		v, _ := p.doc.Spec.Inputs.Get(k)
		base := "inputs." + k
		old, ok := p.dst.Inputs.Get(k)
		if !ok {
			p.dst.Inputs.Set(k, v)
			continue
		}
		h := func(f string) bool { return p.has(base + "." + f) }
		if h("type") {
			old.Type = v.Type
		}
		if h("required") {
			old.Required = v.Required
		}
		if h("default") {
			old.Default = v.Default
		}
		if h("label") {
			old.Label = v.Label
		}
		if h("help") {
			old.Help = v.Help
		}
		if h("values") {
			old.Values = v.Values
		}
		if h("max_length") {
			old.MaxLength = v.MaxLength
		}
		if h("picker") {
			switch {
			case v.Picker == nil || old.Picker == nil:
				old.Picker = v.Picker
			default:
				if h("picker.filter") {
					old.Picker.Filter = v.Picker.Filter
				}
				if h("picker.epic") {
					old.Picker.Epic = v.Picker.Epic
				}
				if h("picker.multi") {
					old.Picker.Multi = v.Picker.Multi
				}
			}
		}
		p.dst.Inputs.Set(k, old)
	}
}

func (p *patcher) agents() {
	if p.blocked["agents"] {
		return
	}
	for _, k := range p.doc.Spec.Agents.Keys() {
		v, _ := p.doc.Spec.Agents.Get(k)
		base := "agents." + k
		old, exists := p.dst.Agents.Get(k)
		if v.Role == RoleDisabled {
			if k == p.dst.EntryAgent() {
				p.report(errDiag("entry_disabled", base+".role", k))
				continue
			}
			p.dst.Agents.Delete(k)
			p.removed = append(p.removed, base)
			continue
		}
		if !exists {
			p.dst.Agents.Set(k, v)
			continue
		}
		h := func(f string) bool { return p.has(base + "." + f) }
		if h("role") {
			old.Role = v.Role
		}
		if h("mode") {
			old.Mode = v.Mode
		}
		if h("after") {
			old.After = v.After
		}
		if h("calls") {
			old.Calls = v.Calls
		}
		p.dst.Agents.Set(k, old)
	}
}

func (p *patcher) checkpoints() {
	if p.blocked["checkpoints"] {
		return
	}
	for _, k := range p.doc.Spec.Checkpoints.Keys() {
		v, _ := p.doc.Spec.Checkpoints.Get(k)
		base := "checkpoints." + k
		old, exists := p.dst.Checkpoints.Get(k)
		if v.Disabled {
			switch {
			case !exists:
				p.report(warnDiag("patch_unknown_checkpoint", base, k, p.parent))
			case old.IsMandatory():
				p.report(errDiag("mandatory_checkpoint_removed", base+".disabled", k))
			default:
				p.dst.Checkpoints.Delete(k)
				p.removed = append(p.removed, base)
			}
			continue
		}
		if !exists {
			p.dst.Checkpoints.Set(k, v)
			continue
		}
		h := func(f string) bool { return p.has(base + "." + f) }
		if h("label") {
			old.Label = v.Label
		}
		if h("description") {
			old.Description = v.Description
		}
		if h("condition") {
			old.Condition = v.Condition
		}
		if h("remote") {
			if v.RemotePolicy().Rank() < old.RemotePolicy().Rank() {
				p.loosening(base+".remote", v.RemotePolicy(), old.RemotePolicy())
			} else {
				old.Remote = v.Remote
			}
		}
		for _, mode := range sortedKeys(v.Mode) {
			if !h("mode." + mode) {
				continue
			}
			b := v.Mode[mode]
			if old.IsMandatory() && b.Strictness() < old.Behavior(mode).Strictness() {
				p.loosening(base+".mode."+mode, b, old.Behavior(mode))
				continue
			}
			if old.Mode == nil {
				old.Mode = map[string]CheckpointBehavior{}
			}
			old.Mode[mode] = b
		}
		if h("mandatory") {
			if old.IsMandatory() && !v.IsMandatory() {
				p.loosening(base+".mandatory", false, true)
			} else {
				old.Mandatory = v.Mandatory
			}
		}
		p.dst.Checkpoints.Set(k, old)
	}
}

func (p *patcher) modes() {
	s := p.doc.Spec
	if !p.has("modes.default") && !p.has("modes.allowed") {
		return
	}
	if p.dst.Modes == nil {
		p.dst.Modes = &Modes{}
	}
	if p.has("modes.default") {
		p.dst.Modes.Default = s.Modes.Default
	}
	if p.has("modes.allowed") {
		p.dst.Modes.Allowed = s.Modes.Allowed
	}
}

func (p *patcher) models() {
	s := p.doc.Spec
	if s.Models == nil || p.blocked["models"] {
		return
	}
	if p.dst.Models == nil {
		p.dst.Models = &Models{}
	}
	if p.has("models.default") {
		p.dst.Models.Default = s.Models.Default
	}
	for _, agent := range sortedKeys(s.Models.Agents) {
		if p.dst.Models.Agents == nil {
			p.dst.Models.Agents = map[string]string{}
		}
		p.dst.Models.Agents[agent] = s.Models.Agents[agent]
	}
}

func (p *patcher) beads() {
	if !p.has("beads") {
		return
	}
	s := p.doc.Spec
	parent := p.dst.Beads
	if parent == nil { // unrestricted parent: any list hardens it
		if s.Beads != nil {
			p.dst.Beads = &BeadsAccess{Allow: s.Beads.Allow}
		}
		return
	}
	if s.Beads == nil {
		p.loosening("beads", "null", strings.Join(parent.Allow, ","))
		return
	}
	if extra := missingFrom(s.Beads.Allow, parent.Allow); len(extra) > 0 {
		p.loosening("beads.allow", strings.Join(extra, ","), strings.Join(parent.Allow, ","))
		return
	}
	p.dst.Beads = &BeadsAccess{Allow: s.Beads.Allow}
}

func (p *patcher) runtime() {
	s := p.doc.Spec
	if !p.has("runtime.default") && !p.has("runtime.allowed") {
		return
	}
	parentAllowed := p.dst.AllowedRuntimes()
	if p.dst.Runtime == nil {
		p.dst.Runtime = &RuntimeSpec{}
	}
	if p.has("runtime.allowed") {
		if extra := missingFrom(s.Runtime.Allowed, parentAllowed); len(extra) > 0 {
			p.loosening("runtime.allowed", joinRuntimes(extra), joinRuntimes(parentAllowed))
		} else {
			p.dst.Runtime.Allowed = s.Runtime.Allowed
		}
	} else if len(p.dst.Runtime.Allowed) == 0 {
		// Keep the parent's effective list: a new default must not widen it.
		p.dst.Runtime.Allowed = parentAllowed
	}
	if p.has("runtime.default") {
		p.dst.Runtime.Default = s.Runtime.Default
	}
}

func (p *patcher) limits() {
	p.limitModels()
	if !p.has("limits.budget_usd") {
		return
	}
	newV := p.doc.Spec.Limits.BudgetUSD
	if p.dst.Limits != nil && p.dst.Limits.BudgetUSD != nil {
		old := *p.dst.Limits.BudgetUSD
		if newV == nil || *newV > old {
			shown := "null"
			if newV != nil {
				shown = fmt.Sprint(*newV)
			}
			p.loosening("limits.budget_usd", shown, old)
			return
		}
	}
	if p.dst.Limits == nil {
		p.dst.Limits = &Limits{}
	}
	p.dst.Limits.BudgetUSD = newV
}

// limitModels narrows the model allow-list: every pattern of the child must
// be covered by a pattern of the parent list (when the parent has one).
func (p *patcher) limitModels() {
	if !p.has("limits.models") {
		return
	}
	newV := p.doc.Spec.Limits.Models
	if p.dst.Limits != nil && len(p.dst.Limits.Models) > 0 {
		parent := p.dst.Limits.Models
		if len(newV) == 0 {
			p.loosening("limits.models", "[]", strings.Join(parent, ", "))
			return
		}
		if covered := limits.CoveredModels(newV, parent); len(covered) != len(newV) {
			p.loosening("limits.models", strings.Join(newV, ", "), strings.Join(parent, ", "))
			return
		}
	}
	if p.dst.Limits == nil {
		p.dst.Limits = &Limits{}
	}
	p.dst.Limits.Models = slices.Clone(newV)
}

// ---------------------------------------------------------------------------
// Origins
// ---------------------------------------------------------------------------

// mergedPaths are the paths whose children are merged by a patch; any other
// written path replaces the parent value as a whole.
var mergedPaths = regexp.MustCompile(`^(?:entry|inputs|agents|checkpoints|modes|circuit_breaker|models|models\.agents|skills|beads|runtime|limits|inputs\.[^.]+|inputs\.[^.]+\.picker|agents\.[^.]+|checkpoints\.[^.]+|checkpoints\.[^.]+\.mode)$`)

// headerPaths are not tracked in origins.
var headerPaths = map[string]bool{"apiVersion": true, "kind": true, "extends": true}

// record updates origins with every path written by doc.
func recordOrigins(origins Origins, doc *Document, origin Origin, removed []string) {
	for _, r := range removed {
		origins.deleteUnder(r)
		delete(origins, r)
	}
	for _, path := range doc.Paths() {
		if headerPaths[path] || underAny(path, removed) {
			continue
		}
		if !mergedPaths.MatchString(path) {
			origins.deleteUnder(path)
		}
		o := origin
		o.Pos = doc.Pos(path)
		origins[path] = o
	}
}

func underAny(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if path == p || strings.HasPrefix(path, p+".") || strings.HasPrefix(path, p+"[") {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// missingFrom returns the items of s that are not in allowed.
func missingFrom[T comparable](s, allowed []T) []T {
	set := make(map[T]bool, len(allowed))
	for _, a := range allowed {
		set[a] = true
	}
	var out []T
	for _, v := range s {
		if !set[v] {
			out = append(out, v)
		}
	}
	return out
}

func joinRuntimes(rs []Runtime) string {
	s := make([]string, len(rs))
	for i, r := range rs {
		s[i] = string(r)
	}
	return strings.Join(s, ",")
}
