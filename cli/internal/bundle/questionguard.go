package bundle

import (
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Questions of a subagent (p4-fixes): the user never sees the `question`
// calls of an agent launched through `task`, so an agent that has no entry
// role in the workflow (only callers, never the entry agent) is refused the
// tool, last of its rules, whatever its frontmatter allows. Its decisions go
// to `### Questions bloquantes` of its envelope (session contract); its
// caller asks the user. An agent that is also the entry agent (the reviewer
// of `review`) keeps the tool.
//
// opencode (2.0.20, Tool.snapshot) leaves out of the tools given to the
// model a tool whose last rule for its action is a deny on "*": the agent
// does not see `question` at all, so no call is ever refused, and the
// contract tells it where its questions go.

// QuestionGuard returns the rule that refuses the `question` tool to a pure
// subagent (nil for an agent with an entry role).
func QuestionGuard(env whenEnv) []sessionspec.PermissionRule {
	if !pureSubagent(env) {
		return nil
	}
	return []sessionspec.PermissionRule{{Action: sessionspec.ActionQuestion, Resource: "*", Effect: sessionspec.EffectDeny}}
}

// pureSubagent reports an agent launched by others that is never the entry
// agent of its session.
func pureSubagent(env whenEnv) bool {
	return env.roles[roleSubagent] && !env.roles[roleEntry]
}

// Outputs of the session (p5-fixes): workflow_outputs declares an output
// of the workflow, and oh records some itself (`record:`). In a workflow
// with no output the agents may declare, the tool is refused to every
// agent by a deny on "*", the last rule of the action, so that the tool
// hides it: the debugger of `incident` called it for its postmortem.

// OutputsGuard returns the rule that refuses workflow_outputs in a workflow
// without a declarable output (nil otherwise, and outside a workflow).
func OutputsGuard(s *workflow.Spec) []sessionspec.PermissionRule {
	if s == nil {
		return nil
	}
	for _, o := range s.Outputs {
		if o.Record == "" {
			return nil
		}
	}
	return []sessionspec.PermissionRule{{Action: actionOutputs, Resource: "*", Effect: sessionspec.EffectDeny}}
}
