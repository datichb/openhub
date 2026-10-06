package workflow

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Migration of the legacy overrides (P2-T11, v38): a WorkflowOverride of
// the former hard-coded workflow becomes an oh/v1 patch of the shipped
// `feature` workflow, which kept the same checkpoint and agent ids.

// LegacyWorkflowID is the shipped workflow the legacy overrides apply to.
const LegacyWorkflowID = "feature"

// LegacyPatch is the translation of a legacy override.
type LegacyPatch struct {
	// YAML is the oh/v1 patch document.
	YAML []byte
	// Untranslated lists what has no oh/v1 equivalent (kept in the archive
	// of the raw override, and as comments at the top of YAML).
	Untranslated []string
}

// TranslateLegacyOverride turns ov into an oh/v1 document `id` extending
// parent. enforce locks the whole workflow (former team `enforced`).
// known lists the checkpoint ids of the parent (overrides of other
// checkpoints are not translated).
func TranslateLegacyOverride(ov *WorkflowOverride, id, parent string, enforce bool, known []string) LegacyPatch {
	var out LegacyPatch
	var body strings.Builder
	isKnown := map[string]bool{}
	for _, k := range known {
		isKnown[k] = true
	}
	skip := func(format string, args ...any) {
		out.Untranslated = append(out.Untranslated, fmt.Sprintf(format, args...))
	}

	if ov != nil && len(ov.CheckpointOverrides) > 0 {
		var lines []string
		for i, c := range ov.CheckpointOverrides {
			switch {
			case c.ID == "":
				skip("checkpoint_overrides[%d]: id missing", i)
				continue
			case c.Action != ActionAdd && !isKnown[c.ID]:
				skip("checkpoint %s (%s): no such checkpoint in %s", c.ID, c.Action, parent)
				continue
			case c.Action == ActionRemove:
				lines = append(lines, fmt.Sprintf("  %s: { disabled: true }", c.ID))
				continue
			}
			var f []string
			if c.Label != nil {
				f = append(f, "label: "+yamlScalar(*c.Label))
			}
			if c.Description != nil {
				f = append(f, "description: "+yamlScalar(*c.Description))
			}
			if len(c.Behavior) > 0 {
				modes := make([]string, 0, len(c.Behavior))
				for m := range c.Behavior {
					modes = append(modes, m)
				}
				sort.Strings(modes)
				var mb []string
				for _, m := range modes {
					mb = append(mb, yamlScalar(m)+": "+string(c.Behavior[m]))
				}
				f = append(f, "mode: { "+strings.Join(mb, ", ")+" }")
			}
			if c.Condition != nil {
				f = append(f, "condition: "+yamlScalar(*c.Condition))
			}
			if c.Agents != nil {
				skip("checkpoint %s: agents %v (no checkpoint agents in oh/v1)", c.ID, *c.Agents)
			}
			if c.Action == ActionAdd && c.InsertAfter != nil {
				skip("checkpoint %s: insert_after %q (new checkpoints are appended)", c.ID, *c.InsertAfter)
			}
			if len(f) == 0 {
				if c.Action != ActionAdd {
					continue
				}
				f = append(f, "label: "+yamlScalar(c.ID))
			}
			lines = append(lines, fmt.Sprintf("  %s: { %s }", c.ID, strings.Join(f, ", ")))
		}
		if len(lines) > 0 {
			body.WriteString("checkpoints:\n" + strings.Join(lines, "\n") + "\n")
		}
	}

	if ov != nil && len(ov.AgentOverrides) > 0 {
		var lines []string
		for _, a := range ov.AgentOverrides {
			if a.AgentID == "" {
				continue
			}
			var f []string
			switch {
			case a.Disabled != nil && *a.Disabled:
				f = append(f, "role: disabled")
			case a.Role != nil:
				f = append(f, "role: "+string(*a.Role))
			}
			if a.Mode != nil {
				f = append(f, "mode: "+string(*a.Mode))
			}
			if a.Position != nil && a.Position.AfterCheckpoint != "" {
				f = append(f, "after: "+yamlScalar(a.Position.AfterCheckpoint))
				if a.Position.Branch != "" {
					skip("agent %s: branch %q", a.AgentID, a.Position.Branch)
				}
			}
			if a.TaskPermissions != nil {
				skip("agent %s: task_permissions %+v (delegations come from the agent permissions or `calls:`)", a.AgentID, *a.TaskPermissions)
			}
			if a.Disabled != nil && !*a.Disabled && a.Role == nil {
				f = append(f, "role: workflow")
			}
			if len(f) > 0 {
				lines = append(lines, fmt.Sprintf("  %s: { %s }", a.AgentID, strings.Join(f, ", ")))
			}
		}
		if len(lines) > 0 {
			body.WriteString("agents:\n" + strings.Join(lines, "\n") + "\n")
		}
	}

	if ov != nil && ov.ModeOverrides != nil {
		var f []string
		if ov.ModeOverrides.Default != nil {
			f = append(f, "default: "+yamlScalar(*ov.ModeOverrides.Default))
		}
		if ov.ModeOverrides.Available != nil {
			vals := make([]string, len(*ov.ModeOverrides.Available))
			for i, m := range *ov.ModeOverrides.Available {
				vals[i] = yamlScalar(m)
			}
			f = append(f, "allowed: ["+strings.Join(vals, ", ")+"]")
		}
		if len(f) > 0 {
			body.WriteString("modes: { " + strings.Join(f, ", ") + " }\n")
		}
	}
	if ov != nil && ov.CircuitBreakerOverride != nil {
		body.WriteString("circuit_breaker: { max_consecutive_subagents: " + strconv.Itoa(*ov.CircuitBreakerOverride) + " }\n")
	}

	var doc strings.Builder
	doc.WriteString("# Migrated from the former workflow configuration (oh v5, migration v38).\n")
	for _, u := range out.Untranslated {
		doc.WriteString("# Not translated: " + strings.ReplaceAll(u, "\n", " ") + "\n")
	}
	doc.WriteString("apiVersion: oh/v1\nkind: Workflow\n")
	doc.WriteString("id: " + id + "\n")
	doc.WriteString("extends: " + parent + "\n")
	if enforce {
		doc.WriteString("enforce: [\"*\"]\n")
	}
	doc.WriteString(body.String())
	out.YAML = []byte(doc.String())
	return out
}

// yamlScalar renders s as a double-quoted YAML scalar (JSON strings are
// valid YAML, safe inside flow mappings).
func yamlScalar(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return strconv.Quote(s)
	}
	return string(b)
}
