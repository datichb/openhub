package daemon

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/datichb/openhub/cli/internal/gateway/fakebd"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/mcp/protocol"
	"github.com/datichb/openhub/cli/internal/testenv"
)

// TestMain runs the test binary as a stdio MCP server (MCP gateway tests).
// With OH_DAEMON_TEST_FAKEBD=1 it is the fake bd of the sessions.
func TestMain(m *testing.M) {
	if os.Getenv("OH_DAEMON_TEST_FAKEBD") == "1" {
		os.Exit(fakebd.Main(os.Args[1:]))
	}
	if os.Getenv("OH_DAEMON_TEST_MCP") == "1" {
		s := protocol.NewServer("test", "1")
		s.RegisterTool(protocol.Tool{Name: "whoami", InputSchema: map[string]any{"type": "object"}},
			func(ctx context.Context, _ json.RawMessage) (*protocol.ToolResult, error) {
				wd, _ := os.Getwd()
				return &protocol.ToolResult{Content: []protocol.ContentBlock{{Type: "text", Text: os.Getenv("OH_TEST_SERVICE_TOKEN") + "@" + wd}}}, nil
			})
		_ = s.Serve()
		os.Exit(0)
	}
	// The helper processes above must keep the variables their parent gave them.
	finish := testenv.Isolate()
	os.Exit(finish(gittest.Main(m)))
}
