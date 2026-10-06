package bundle

import (
	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// WorkflowModels converts the `models:` block of a workflow into the workflow
// level of the model cascade (nil when the workflow sets no model).
func WorkflowModels(m *workflow.Models) *bricks.ModelOverrides {
	if m == nil || (m.Default == "" && len(m.Agents) == 0) {
		return nil
	}
	out := &bricks.ModelOverrides{Default: m.Default}
	if len(m.Agents) > 0 {
		out.Agents = make(map[string]string, len(m.Agents))
		for id, model := range m.Agents {
			out.Agents[id] = model
		}
	}
	return out
}

// ResolveModel resolves the model of an agent with the full cascade (O9):
//
//	workflow·agent > workflow > project·agent > project·family > project >
//	hub·agent > hub·family > hub > team·agent > team·family > team > frontmatter
//
// The workflow level has no family overrides. The result is normalized for
// the hub provider; "" when no level defines a model.
func (req Request) ResolveModel(agentID, family, frontmatter string) string {
	if wf := req.WorkflowModels; wf != nil {
		if m := wf.Agents[agentID]; m != "" {
			return normalize(m, req.Provider)
		}
		if wf.Default != "" {
			return normalize(wf.Default, req.Provider)
		}
	}
	return bricks.ResolveAgentModel(agentID, family, req.ProjectOverrides, req.HubOverrides, req.TeamOverrides, frontmatter, req.Provider)
}

// modelRef turns a resolved model string into a reference with a provider.
func (req Request) modelRef(m string) *sessionspec.ModelRef {
	ref := sessionspec.ParseModelRef(m)
	if ref.Provider == "" {
		ref.Provider = bricks.OpencodeProviderID(req.Provider)
	}
	return &ref
}

func normalize(model, provider string) string {
	if provider == "" {
		return model
	}
	return bricks.NormalizeModelForProvider(model, provider)
}
