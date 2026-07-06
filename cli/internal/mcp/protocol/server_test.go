package protocol

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleValidRequest(t *testing.T) {
	s := NewServer("test-server", "1.0.0")
	s.RegisterTool(Tool{
		Name:        "echo",
		Description: "Echoes back the input",
		InputSchema: map[string]interface{}{"type": "object"},
	}, func(params json.RawMessage) (*ToolResult, error) {
		return &ToolResult{
			Content: []ContentBlock{{Type: "text", Text: "hello"}},
		}, nil
	})

	req := &Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"echo","arguments":{}}`),
	}

	resp := s.handleRequest(req)
	require.NotNil(t, resp)
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, float64(1), resp.ID)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(*ToolResult)
	require.True(t, ok)
	require.Len(t, result.Content, 1)
	assert.Equal(t, "text", result.Content[0].Type)
	assert.Equal(t, "hello", result.Content[0].Text)
	assert.False(t, result.IsError)
}

func TestHandleUnknownMethod(t *testing.T) {
	s := NewServer("test-server", "1.0.0")

	req := &Request{
		JSONRPC: "2.0",
		ID:      float64(2),
		Method:  "unknown/method",
	}

	resp := s.handleRequest(req)
	require.NotNil(t, resp)
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, float64(2), resp.ID)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "Method not found")
}

func TestHandleInvalidJSON(t *testing.T) {
	s := NewServer("test-server", "1.0.0")

	// Simulate what happens in Serve() when JSON is invalid:
	// The Serve loop calls json.Unmarshal and on failure calls writeError.
	// Since we're testing handleRequest directly, we test a request with
	// invalid params for tools/call which exercises the same path.
	req := &Request{
		JSONRPC: "2.0",
		ID:      float64(3),
		Method:  "tools/call",
		Params:  json.RawMessage(`not valid json`),
	}

	resp := s.handleRequest(req)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32602, resp.Error.Code)
	assert.Equal(t, "Invalid params", resp.Error.Message)
}

func TestHandleMissingParams(t *testing.T) {
	s := NewServer("test-server", "1.0.0")
	s.RegisterTool(Tool{
		Name:        "greet",
		Description: "Greets a user",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"name": map[string]interface{}{"type": "string"}},
			"required":   []string{"name"},
		},
	}, func(params json.RawMessage) (*ToolResult, error) {
		return &ToolResult{
			Content: []ContentBlock{{Type: "text", Text: "ok"}},
		}, nil
	})

	// Send tools/call with nil Params (no params at all)
	req := &Request{
		JSONRPC: "2.0",
		ID:      float64(4),
		Method:  "tools/call",
		Params:  nil,
	}

	resp := s.handleRequest(req)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32602, resp.Error.Code)
	assert.Equal(t, "Invalid params", resp.Error.Message)
}

func TestToolRegistration(t *testing.T) {
	s := NewServer("test-server", "1.0.0")
	s.RegisterTool(Tool{
		Name:        "my_tool",
		Description: "A test tool",
		InputSchema: map[string]interface{}{"type": "object"},
	}, func(params json.RawMessage) (*ToolResult, error) {
		return nil, nil
	})

	req := &Request{
		JSONRPC: "2.0",
		ID:      float64(5),
		Method:  "tools/list",
	}

	resp := s.handleRequest(req)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)

	resultMap, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)

	tools, ok := resultMap["tools"].([]Tool)
	require.True(t, ok)
	require.Len(t, tools, 1)
	assert.Equal(t, "my_tool", tools[0].Name)
	assert.Equal(t, "A test tool", tools[0].Description)
}

func TestInitializeHandshake(t *testing.T) {
	s := NewServer("my-mcp-server", "2.0.0")

	req := &Request{
		JSONRPC: "2.0",
		ID:      float64(6),
		Method:  "initialize",
	}

	resp := s.handleRequest(req)
	require.NotNil(t, resp)
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, float64(6), resp.ID)
	assert.Nil(t, resp.Error)

	resultMap, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "2024-11-05", resultMap["protocolVersion"])

	serverInfo, ok := resultMap["serverInfo"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "my-mcp-server", serverInfo["name"])
	assert.Equal(t, "2.0.0", serverInfo["version"])

	capabilities, ok := resultMap["capabilities"].(map[string]interface{})
	require.True(t, ok)
	_, hasTools := capabilities["tools"]
	assert.True(t, hasTools)
}

func TestNotificationInitializedReturnsNil(t *testing.T) {
	s := NewServer("test-server", "1.0.0")

	req := &Request{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}

	resp := s.handleRequest(req)
	assert.Nil(t, resp, "notifications should not produce a response")
}

func TestToolCallHandlerError(t *testing.T) {
	s := NewServer("test-server", "1.0.0")
	s.RegisterTool(Tool{
		Name:        "failing",
		Description: "Always fails",
		InputSchema: map[string]interface{}{"type": "object"},
	}, func(params json.RawMessage) (*ToolResult, error) {
		return nil, assert.AnError
	})

	req := &Request{
		JSONRPC: "2.0",
		ID:      float64(7),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"failing","arguments":{}}`),
	}

	resp := s.handleRequest(req)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error, "handler errors are returned as ToolResult with IsError=true")

	result, ok := resp.Result.(*ToolResult)
	require.True(t, ok)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "assert.AnError")
}

func TestToolCallToolNotFound(t *testing.T) {
	s := NewServer("test-server", "1.0.0")

	req := &Request{
		JSONRPC: "2.0",
		ID:      float64(8),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"nonexistent","arguments":{}}`),
	}

	resp := s.handleRequest(req)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "Tool not found: nonexistent")
}

func TestWriteResponseNil(t *testing.T) {
	s := NewServer("test-server", "1.0.0")
	// writeResponse with nil should not panic
	var buf []byte
	writer := &mockWriter{buf: &buf}
	s.writeResponse(writer, nil) // should be no-op
	assert.Empty(t, *writer.buf)
}

// mockWriter is a minimal io.Writer for testing writeResponse nil handling.
type mockWriter struct {
	buf *[]byte
}

func (w *mockWriter) Write(p []byte) (n int, err error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}
