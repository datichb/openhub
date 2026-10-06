package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/mcp/protocol"
)

// TestMain runs the test binary as a stdio MCP server when asked to.
func TestMain(m *testing.M) {
	if os.Getenv("OH_GATEWAY_TEST_MCP") == "1" {
		s := protocol.NewServer("test", "1")
		s.RegisterTool(protocol.Tool{Name: "echo", InputSchema: map[string]any{"type": "object"}},
			func(ctx context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
				var in struct {
					Text  string `json:"text"`
					Sleep int    `json:"sleep"`
					Exit  bool   `json:"exit"`
				}
				_ = json.Unmarshal(params, &in)
				if in.Exit {
					os.Exit(3)
				}
				time.Sleep(time.Duration(in.Sleep) * time.Millisecond)
				meta, _ := json.Marshal(protocol.Meta(ctx))
				return &protocol.ToolResult{Content: []protocol.ContentBlock{{Type: "text",
					Text: in.Text + "|" + os.Getenv("OH_TEST_TOKEN") + "|" + string(meta)}}}, nil
			})
		_ = s.Serve()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type mcpFixture struct {
	m        *MCP
	resolved []string
	mu       sync.Mutex
}

func newMCPFixture(t *testing.T) *mcpFixture {
	t.Helper()
	exe, err := os.Executable()
	require.NoError(t, err)
	f := &mcpFixture{}
	f.m = &MCP{Resolve: func(_ context.Context, group, name string) (MCPCommand, error) {
		f.mu.Lock()
		f.resolved = append(f.resolved, group+"/"+name)
		f.mu.Unlock()
		if name != "echo" {
			return MCPCommand{}, errors.New("unknown server")
		}
		return MCPCommand{Argv: []string{exe}, Env: map[string]string{"OH_GATEWAY_TEST_MCP": "1", "OH_TEST_TOKEN": "machine-secret"}}, nil
	}}
	t.Cleanup(f.m.Close)
	return f
}

func (f *mcpFixture) post(t *testing.T, group, name, body string) (int, map[string]json.RawMessage) {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	f.m.ServeHTTP(w, r, group, name)
	var out map[string]json.RawMessage
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &out)
	}
	return w.Code, out
}

func toolText(t *testing.T, resp map[string]json.RawMessage) string {
	t.Helper()
	var res protocol.ToolResult
	require.NoError(t, json.Unmarshal(resp["result"], &res), string(resp["error"]))
	require.NotEmpty(t, res.Content)
	return res.Content[0].Text
}

func TestMCPRelaysJSONRPCOverHTTP(t *testing.T) {
	f := newMCPFixture(t)
	code, resp := f.post(t, "g1", "echo", `{"jsonrpc":"2.0","id":"init-1","method":"initialize","params":{}}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, `"init-1"`, string(resp["id"]), "the client id is given back")

	code, _ = f.post(t, "g1", "echo", `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	assert.Equal(t, http.StatusAccepted, code)

	_, resp = f.post(t, "g1", "echo", `{"jsonrpc":"2.0","id":7,"method":"tools/list"}`)
	assert.Contains(t, string(resp["result"]), `"echo"`)

	_, resp = f.post(t, "g1", "echo", `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"echo","arguments":{"text":"hi"},"_meta":{"ai.opencode/sessionID":"ses_x"}}}`)
	assert.Equal(t, `8`, string(resp["id"]))
	assert.Equal(t, `hi|machine-secret|{"ai.opencode/sessionID":"ses_x"}`, toolText(t, resp), "secrets read on the machine, _meta relayed")
	assert.Equal(t, []string{"g1/echo"}, f.resolved, "one process per group and server")
	assert.Equal(t, []string{"echo"}, f.m.Running("g1"))

	// GET (server stream) is not offered; DELETE (end of session) is accepted.
	w := httptest.NewRecorder()
	f.m.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil), "g1", "echo")
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	w = httptest.NewRecorder()
	f.m.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/", nil), "g1", "echo")
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestMCPConcurrentClientsSameIDs(t *testing.T) {
	f := newMCPFixture(t)
	var wg sync.WaitGroup
	got := make([]string, 6)
	for i := range got {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Every client numbers its requests from 1: ids collide.
			body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"text":"c%d","sleep":%d}}}`, i, (5-i)*10)
			_, resp := f.post(t, "g1", "echo", body)
			got[i] = strings.SplitN(toolText(t, resp), "|", 2)[0]
		}(i)
	}
	wg.Wait()
	for i, g := range got {
		assert.Equal(t, "c"+string(rune('0'+i)), g)
	}
}

func TestMCPServerExitAndRestart(t *testing.T) {
	f := newMCPFixture(t)
	_, resp := f.post(t, "g1", "echo", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"exit":true}}}`)
	assert.Contains(t, string(resp["error"]), "exited")
	require.Eventually(t, func() bool { return len(f.m.Running("g1")) == 0 }, 3*time.Second, 20*time.Millisecond)
	_, resp = f.post(t, "g1", "echo", `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo","arguments":{"text":"again"}}}`)
	assert.True(t, strings.HasPrefix(toolText(t, resp), "again|"))
	assert.Len(t, f.resolved, 2, "restarted on demand")

	f.m.StopGroup("g1")
	assert.Empty(t, f.m.Running("g1"))
}

func TestMCPCheckTimeoutAndUnknown(t *testing.T) {
	f := newMCPFixture(t)
	f.m.Check = func(_ context.Context, group string, msg map[string]json.RawMessage) error {
		if bytes.Contains(msg["params"], []byte("ses_other")) {
			return ErrMCPDenied
		}
		return nil
	}
	code, _ := f.post(t, "g1", "echo", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","_meta":{"sessionID":"ses_other"}}}`)
	assert.Equal(t, http.StatusForbidden, code)

	f.m.Timeout = 100 * time.Millisecond
	_, resp := f.post(t, "g1", "echo", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"sleep":900}}}`)
	assert.Contains(t, string(resp["error"]), "deadline")

	_, resp = f.post(t, "g1", "nope", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	assert.Contains(t, string(resp["error"]), "unavailable")

	code, resp = f.post(t, "g1", "echo", `not json`)
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(resp["error"]), "Parse error")
}
