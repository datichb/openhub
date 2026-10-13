package bundle

import (
	"github.com/datichb/openhub/cli/internal/services/wiki"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Wiki MCP server of the project (modernisation LD-J): in a bundle built
// from a workflow, for a project that has docs/wiki/. Every agent reads
// the wiki and proposes enrichments (wiki_index, wiki_read, wiki_propose,
// wiki_pending, wiki_check); only the documentarian applies proposals
// (wiki_apply, asked to the user), which the daemon checks again on the
// agent it sees calling.

func wikiAction(tool string) string {
	return sessionspec.MCPToolAction(sessionspec.WikiMCPServer, tool)
}

// BaseWikiRules are the bundle rules of the wiki MCP server: wiki_apply is
// refused (the documentarian gets its own rule).
func BaseWikiRules() []sessionspec.PermissionRule {
	var out []sessionspec.PermissionRule
	for _, t := range sessionspec.WikiTools {
		effect := sessionspec.EffectAllow
		if t == sessionspec.WikiToolApply {
			effect = sessionspec.EffectDeny
		}
		out = append(out, sessionspec.PermissionRule{Action: wikiAction(t), Resource: "*", Effect: effect})
	}
	return out
}

// WikiApplyRules are the rules of agent on wiki_apply: asked for the
// documentarian, refused to any other agent.
func WikiApplyRules(agent string) []sessionspec.PermissionRule {
	effect := sessionspec.EffectDeny
	if agent == sessionspec.WikiApplier {
		effect = sessionspec.EffectAsk
	}
	return []sessionspec.PermissionRule{{Action: wikiAction(sessionspec.WikiToolApply), Resource: "*", Effect: effect}}
}

// wantsWikiMCP reports whether a bundle gets the wiki MCP server.
func wantsWikiMCP(s *workflow.Spec, projectPath string) bool {
	return s != nil && projectPath != "" && wiki.Exists(projectPath)
}

// applyWikiRuntime adds the wiki MCP server and its rules to spec.
func applyWikiRuntime(spec *sessionspec.BundleSpec) {
	spec.Permissions = append(append([]sessionspec.PermissionRule(nil), spec.Permissions...), BaseWikiRules()...)
	for _, m := range spec.MCP {
		if m.Name == sessionspec.WikiMCPServer {
			return
		}
	}
	spec.MCP = append(append([]sessionspec.MCPServerDef(nil), spec.MCP...), sessionspec.WikiMCPDef())
}
