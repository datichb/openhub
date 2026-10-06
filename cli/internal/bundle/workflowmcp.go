package bundle

import (
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Workflow runtime of a bundle built from an oh/v1 workflow (P3-T01): the
// `workflow` MCP server agents call at each checkpoint, and the checkpoints,
// gates and circuit breaker oh enforces while the session runs.

// applyWorkflowRuntime adds the workflow MCP server and runtime to spec.
func applyWorkflowRuntime(spec *sessionspec.BundleSpec, s *workflow.Spec) {
	if s == nil {
		return
	}
	spec.Workflow = WorkflowRuntime(s)
	for _, m := range spec.MCP {
		if m.Name == sessionspec.WorkflowMCPServer {
			return
		}
	}
	spec.MCP = append(append([]sessionspec.MCPServerDef(nil), spec.MCP...), sessionspec.WorkflowMCPDef())
}

// WorkflowRuntime extracts from a resolved workflow what a running session
// needs (tool-agnostic).
func WorkflowRuntime(s *workflow.Spec) *sessionspec.WorkflowRuntime {
	rt := &sessionspec.WorkflowRuntime{ID: s.ID}
	for _, id := range s.Checkpoints.Keys() {
		cp, _ := s.Checkpoints.Get(id)
		if cp.Disabled {
			continue
		}
		def := sessionspec.CheckpointDef{ID: id, Label: localized(cp.Label), Condition: cp.Condition, Mandatory: cp.IsMandatory(), Behaviors: map[string]string{}}
		for _, mode := range s.AllowedModes() {
			def.Behaviors[mode] = string(cp.Behavior(mode))
		}
		rt.Checkpoints = append(rt.Checkpoints, def)
	}
	for _, id := range s.Agents.Keys() {
		a, _ := s.Agents.Get(id)
		if a.Role != workflow.RoleDisabled && a.After != "" {
			rt.Gates = append(rt.Gates, sessionspec.AgentGate{Agent: id, After: a.After})
		}
	}
	if s.CircuitBreaker != nil && s.CircuitBreaker.MaxConsecutiveSubagents != nil {
		rt.MaxConsecutiveSubagents = *s.CircuitBreaker.MaxConsecutiveSubagents
	}
	for _, o := range s.Outputs {
		rt.Outputs = append(rt.Outputs, sessionspec.OutputDef{ID: o.ID, Type: string(o.Type), Label: localized(o.Label)})
	}
	return rt
}

func localized(t workflow.LocalizedText) map[string]string {
	if t.IsZero() {
		return nil
	}
	out := map[string]string{}
	if t.Default != "" {
		out[""] = t.Default
	}
	for l, v := range t.ByLang {
		out[l] = v
	}
	return out
}
