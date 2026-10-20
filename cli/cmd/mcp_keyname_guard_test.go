package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The MCP token key names are built by internal/config only: no writer
// rebuilds the legacy "<service>-token" name by hand.
func TestMCPWritersDoNotBuildLegacyKeyNames(t *testing.T) {
	for _, file := range []string{"mcp.go", "misc.go", "../internal/tui/v2/views/mcp_view.go"} {
		src, err := os.ReadFile(file)
		require.NoError(t, err, file)
		for i, line := range strings.Split(string(src), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			assert.NotContains(t, line, `-token"`, "%s:%d builds a legacy key name", file, i+1)
			assert.NotContains(t, line, `-token-"`, "%s:%d builds a legacy key name", file, i+1)
		}
	}
}
