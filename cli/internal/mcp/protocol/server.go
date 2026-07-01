// Package protocol implements the MCP (Model Context Protocol) JSON-RPC stdio transport.
// This provides the server-side infrastructure for hosting MCP tools within the oh binary.
package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Request represents a JSON-RPC request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC response.
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Tool represents an MCP tool definition.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// ToolResult represents the result of a tool call.
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content block in a tool result.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Handler is a function that handles a tool call.
type Handler func(params json.RawMessage) (*ToolResult, error)

// Server is an MCP server that communicates over stdio.
type Server struct {
	name     string
	version  string
	tools    map[string]Tool
	handlers map[string]Handler
}

// NewServer creates a new MCP server.
func NewServer(name, version string) *Server {
	return &Server{
		name:     name,
		version:  version,
		tools:    make(map[string]Tool),
		handlers: make(map[string]Handler),
	}
}

// RegisterTool adds a tool to the server.
func (s *Server) RegisterTool(tool Tool, handler Handler) {
	s.tools[tool.Name] = tool
	s.handlers[tool.Name] = handler
}

// Serve starts the server, reading from stdin and writing to stdout.
func (s *Server) Serve() error {
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("reading stdin: %w", err)
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeError(writer, nil, -32700, "Parse error")
			continue
		}

		resp := s.handleRequest(&req)
		s.writeResponse(writer, resp)
	}
}

func (s *Server) handleRequest(req *Request) *Response {
	switch req.Method {
	case "initialize":
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]interface{}{
					"name":    s.name,
					"version": s.version,
				},
			},
		}

	case "tools/list":
		toolList := make([]Tool, 0, len(s.tools))
		for _, t := range s.tools {
			toolList = append(toolList, t)
		}
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"tools": toolList,
			},
		}

	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.errorResponse(req.ID, -32602, "Invalid params")
		}

		handler, ok := s.handlers[params.Name]
		if !ok {
			return s.errorResponse(req.ID, -32601, fmt.Sprintf("Tool not found: %s", params.Name))
		}

		result, err := handler(params.Arguments)
		if err != nil {
			return &Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: err.Error()}},
					IsError: true,
				},
			}
		}

		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}

	case "notifications/initialized":
		// No response needed for notifications
		return nil

	default:
		return s.errorResponse(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func (s *Server) errorResponse(id interface{}, code int, message string) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: message},
	}
}

func (s *Server) writeResponse(w io.Writer, resp *Response) {
	if resp == nil {
		return
	}
	data, _ := json.Marshal(resp)
	fmt.Fprintf(w, "%s\n", data)
}

func (s *Server) writeError(w io.Writer, id interface{}, code int, message string) {
	resp := s.errorResponse(id, code, message)
	s.writeResponse(w, resp)
}
