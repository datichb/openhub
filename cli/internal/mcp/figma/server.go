// Package figma implements the Figma MCP server.
package figma

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/datichb/openhub/cli/internal/mcp/protocol"
)

// Serve starts the Figma MCP server.
func Serve() error {
	server := protocol.NewServer("figma-mcp", "2.0.0")

	server.RegisterTool(protocol.Tool{
		Name:        "figma_get_file",
		Description: "Get a Figma file by key",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_key": map[string]interface{}{"type": "string", "description": "The Figma file key"},
			},
			"required": []string{"file_key"},
		},
	}, handleGetFile)

	server.RegisterTool(protocol.Tool{
		Name:        "figma_get_node",
		Description: "Get a specific node from a Figma file",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_key": map[string]interface{}{"type": "string", "description": "The Figma file key"},
				"node_id":  map[string]interface{}{"type": "string", "description": "The node ID"},
			},
			"required": []string{"file_key", "node_id"},
		},
	}, handleGetNode)

	server.RegisterTool(protocol.Tool{
		Name:        "figma_get_styles",
		Description: "Get styles from a Figma file",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_key": map[string]interface{}{"type": "string", "description": "The Figma file key"},
			},
			"required": []string{"file_key"},
		},
	}, handleGetStyles)

	return server.Serve()
}

func handleGetFile(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		FileKey string `json:"file_key"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	data, err := figmaAPI(fmt.Sprintf("/v1/files/%s", args.FileKey))
	if err != nil {
		return nil, err
	}
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func handleGetNode(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		FileKey string `json:"file_key"`
		NodeID  string `json:"node_id"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	data, err := figmaAPI(fmt.Sprintf("/v1/files/%s/nodes?ids=%s", args.FileKey, args.NodeID))
	if err != nil {
		return nil, err
	}
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func handleGetStyles(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		FileKey string `json:"file_key"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	data, err := figmaAPI(fmt.Sprintf("/v1/files/%s/styles", args.FileKey))
	if err != nil {
		return nil, err
	}
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func figmaAPI(path string) ([]byte, error) {
	token := os.Getenv("FIGMA_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("FIGMA_TOKEN environment variable not set")
	}

	req, err := http.NewRequest("GET", "https://api.figma.com"+path, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Figma-Token", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Figma API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Figma API error %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}
