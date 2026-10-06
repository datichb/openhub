package workflow

import (
	"reflect"
	"strings"
)

// EnforceAll locks every field of a workflow (`enforce: ["*"]`).
const EnforceAll = "*"

// identityFields may always be written by a more specific document, even
// under `enforce: ["*"]`: they name the document and its parent.
var identityFields = map[string]bool{
	"apiVersion": true, "kind": true, "id": true, "version": true, "extends": true, "enforce": true,
}

// EnforceableFields lists the top-level fields `enforce` accepts, in
// declaration order.
func EnforceableFields() []string {
	t := reflect.TypeOf(Spec{})
	var out []string
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ",")
		if name != "" && !identityFields[name] {
			out = append(out, name)
		}
	}
	return out
}

// IsEnforceable reports whether field is a valid `enforce` entry.
func IsEnforceable(field string) bool {
	if field == EnforceAll {
		return true
	}
	for _, f := range EnforceableFields() {
		if f == field {
			return true
		}
	}
	return false
}

// topField returns the top-level field of a document path
// ("checkpoints.cp-1.mode" → "checkpoints", "outputs[1].type" → "outputs").
func topField(path string) string {
	if i := strings.IndexAny(path, ".["); i >= 0 {
		return path[:i]
	}
	return path
}

// enforce reports the fields the patch writes although a less specific
// document locked them, and blocks them; then it adds the patch's own locks.
func (p *patcher) enforce() {
	locked := map[string]bool{}
	for _, f := range p.dst.Enforce {
		locked[f] = true
	}
	if len(locked) > 0 {
		for _, path := range p.doc.Paths() {
			top := topField(path)
			if identityFields[top] || p.blocked[top] || (!locked[EnforceAll] && !locked[top]) {
				continue
			}
			if p.blocked == nil {
				p.blocked = map[string]bool{}
			}
			p.blocked[top] = true
			p.report(errDiag("enforced_field", top, top, p.parent))
		}
	}
	for _, f := range p.doc.Spec.Enforce {
		if !locked[f] {
			locked[f] = true
			p.dst.Enforce = append(p.dst.Enforce, f)
		}
	}
}
