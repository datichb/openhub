package sessionspec

import "path/filepath"

// OhMCPName reports whether m runs an oh MCP server (`oh mcp serve <name>`)
// and returns its name. Outside the machine (container), these servers are
// served by the oh daemon over HTTP (P4-T08): their tokens stay in the
// machine secret store.
func OhMCPName(m MCPServerDef) (string, bool) {
	if m.Type == "remote" || len(m.Command) < 4 || m.Command[1] != "mcp" || m.Command[2] != "serve" || m.Command[3] == "" {
		return "", false
	}
	bin := m.Command[0]
	if bin == OhBinVar || bin == OhExecutable() || filepath.Base(bin) == "oh" {
		return m.Command[3], true
	}
	return "", false
}
