package sessionspec

// Wiki MCP server of the project (modernisation LD-J): `oh mcp serve wiki`,
// served by the oh daemon like the workflow server, injected in the bundles
// built from a workflow for a project that has a wiki (docs/wiki/). Tools
// reach the model as wiki_<tool>.

// WikiMCPServer is the name of the wiki MCP server.
const WikiMCPServer = "wiki"

// Tools of the wiki MCP server.
const (
	WikiToolIndex   = "index"
	WikiToolRead    = "read"
	WikiToolPropose = "propose"
	WikiToolPending = "pending"
	WikiToolApply   = "apply"
	WikiToolCheck   = "check"
)

// WikiTools are the tools of the wiki MCP server.
var WikiTools = []string{WikiToolIndex, WikiToolRead, WikiToolPropose, WikiToolPending, WikiToolApply, WikiToolCheck}

// WikiApplier is the only agent that may apply proposals (wiki_apply).
const WikiApplier = "documentarian"

// WikiMCPDef is the declaration of the wiki MCP server.
func WikiMCPDef() MCPServerDef {
	return MCPServerDef{Name: WikiMCPServer, Type: "local", Command: []string{OhBinVar, "mcp", "serve", WikiMCPServer}}
}

// OhDaemonMCPServer reports whether an MCP server is one of oh served by
// its daemon (workflow, wiki): it gets the group token and only talks to
// the daemon socket.
func OhDaemonMCPServer(name string) bool { return name == WorkflowMCPServer || name == WikiMCPServer }
