// Package figma implements the Figma MCP server.
package figma

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/datichb/openhub/cli/internal/httplog"
	"github.com/datichb/openhub/cli/internal/mcp/protocol"
)

// Serve starts the Figma MCP server.
func Serve() error {
	server := protocol.NewServer("figma-mcp", "2.0.0")
	server.AcceptLegacyNames("figma_")

	server.RegisterTool(protocol.Tool{
		Name:        "get_file",
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
		Name:        "get_node",
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
		Name:        "get_styles",
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

// fileKeyRe is the shape of a Figma file key. Dots and slashes are refused:
// url.PathEscape leaves ".." as it is, which would climb one segment of the
// API path (/v1/files/../styles).
var fileKeyRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// filePath is the API path of a file, "/v1/files/<key>" followed by sub.
func filePath(key string, sub ...string) (string, error) {
	if !fileKeyRe.MatchString(key) {
		return "", fmt.Errorf("invalid file key %q", key)
	}
	p := "/v1/files/" + url.PathEscape(key)
	for _, s := range sub {
		p += "/" + s
	}
	return p, nil
}

func handleGetFile(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		FileKey string `json:"file_key"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	path, err := filePath(args.FileKey)
	if err != nil {
		return nil, err
	}
	data, err := figmaAPI(path)
	if err != nil {
		return nil, err
	}
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func handleGetNode(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		FileKey string `json:"file_key"`
		NodeID  string `json:"node_id"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	path, err := filePath(args.FileKey, "nodes")
	if err != nil {
		return nil, err
	}
	data, err := figmaAPI(path + "?ids=" + url.QueryEscape(args.NodeID))
	if err != nil {
		return nil, err
	}
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func handleGetStyles(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		FileKey string `json:"file_key"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	path, err := filePath(args.FileKey, "styles")
	if err != nil {
		return nil, err
	}
	data, err := figmaAPI(path)
	if err != nil {
		return nil, err
	}
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

var httpClient = httplog.Wrap(&http.Client{Timeout: 30 * time.Second}, "mcp.figma")

const maxResponseSize = 50 << 20 // 50 MB

// apiBase is the address of the Figma API. It is not configurable: a URL
// recorded for Figma in hub.toml, in a project or in the team config is
// ignored.
const apiBase = "https://api.figma.com"

func figmaAPI(path string) ([]byte, error) {
	token := os.Getenv("FIGMA_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("FIGMA_TOKEN environment variable not set")
	}

	req, err := http.NewRequest("GET", apiBase+path, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Figma-Token", token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("figma API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("figma API error %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}
