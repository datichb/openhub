package workflow

import (
	"fmt"
	"path"
	"strings"
)

// Validation and patch of `writes:` and `verify.spec` (R4, schema_writes.go).

// writes checks the `writes:` paths of an agent.
func (v *validator) writes(base string, a AgentRef) {
	if len(a.Writes) == 0 || a.Role == RoleDisabled {
		return
	}
	if v.s.Risk == RiskRead {
		v.err("writes_read_only", base+".writes")
	}
	seen := map[string]bool{}
	for i, p := range a.Writes {
		at := fmt.Sprintf("%s.writes[%d]", base, i)
		if why := WritesPathProblem(p); why != "" {
			v.err("writes_path_invalid", at, p, why)
			continue
		}
		if seen[p] {
			v.err("duplicate_entry", at, p)
		}
		seen[p] = true
	}
}

// specCheck checks the `verify.spec` of a checkpoint.
func (v *validator) specCheck(base string, c *SpecCheck) {
	if c == nil {
		return
	}
	if len(c.Paths) == 0 {
		v.err("field_required", base+".paths", "paths")
	}
	for i, p := range c.Paths {
		clean := path.Clean(p)
		if strings.TrimSpace(p) == "" || path.IsAbs(p) || clean == ".." || strings.HasPrefix(clean, "../") {
			v.err("verify_spec_path_invalid", fmt.Sprintf("%s.paths[%d]", base, i), p)
		}
	}
}

// writesPatch narrows the `writes:` of an inherited agent (old): every path
// of the layer must be covered by the parent's.
func (p *patcher) writes(at string, old *AgentRef, v []string) {
	var extra []string
	for _, w := range v {
		if !WritesCovered(w, old.Writes) {
			extra = append(extra, w)
		}
	}
	if len(extra) > 0 {
		parent := strings.Join(old.Writes, ",")
		if parent == "" {
			parent = "[]"
		}
		p.loosening(at, strings.Join(extra, ","), parent)
		return
	}
	old.Writes = v
}
