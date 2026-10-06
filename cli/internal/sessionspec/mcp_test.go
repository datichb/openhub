package sessionspec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOhMCPName(t *testing.T) {
	for _, tc := range []struct {
		def  MCPServerDef
		name string
		ok   bool
	}{
		{WorkflowMCPDef(), "workflow", true},
		{MCPServerDef{Name: "gitlab", Type: "local", Command: []string{"/opt/homebrew/bin/oh", "mcp", "serve", "gitlab", "--token-key", "k"}}, "gitlab", true},
		{MCPServerDef{Name: "gitlab", Type: "local", Command: []string{OhExecutable(), "mcp", "serve", "gitlab"}}, "gitlab", true},
		{MCPServerDef{Name: "fs", Type: "local", Command: []string{"npx", "mcp", "serve", "fs"}}, "", false},
		{MCPServerDef{Name: "x", Type: "remote", URL: "http://x"}, "", false},
		{MCPServerDef{Name: "x", Type: "local", Command: []string{"oh", "mcp"}}, "", false},
	} {
		name, ok := OhMCPName(tc.def)
		assert.Equal(t, tc.ok, ok, tc.def.Command)
		assert.Equal(t, tc.name, name)
	}
}
