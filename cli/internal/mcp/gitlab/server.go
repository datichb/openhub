// Package gitlab implements the GitLab MCP server.
package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/datichb/openhub/cli/internal/mcp/protocol"
)

// Serve starts the GitLab MCP server.
func Serve() error {
	server := protocol.NewServer("gitlab-mcp", "2.0.0")

	server.RegisterTool(protocol.Tool{
		Name:        "gitlab_get_project",
		Description: "Get a GitLab project by ID or path",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project_id": map[string]interface{}{"type": "string", "description": "Project ID or URL-encoded path"},
			},
			"required": []string{"project_id"},
		},
	}, handleGetProject)

	server.RegisterTool(protocol.Tool{
		Name:        "gitlab_list_issues",
		Description: "List issues for a project",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project_id": map[string]interface{}{"type": "string", "description": "Project ID"},
				"state":      map[string]interface{}{"type": "string", "description": "Filter by state (opened, closed, all)"},
			},
			"required": []string{"project_id"},
		},
	}, handleListIssues)

	server.RegisterTool(protocol.Tool{
		Name:        "gitlab_list_mrs",
		Description: "List merge requests for a project",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project_id": map[string]interface{}{"type": "string", "description": "Project ID"},
				"state":      map[string]interface{}{"type": "string", "description": "Filter by state (opened, merged, closed, all)"},
			},
			"required": []string{"project_id"},
		},
	}, handleListMRs)

	return server.Serve()
}

func handleGetProject(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	data, err := gitlabAPI(fmt.Sprintf("/api/v4/projects/%s", args.ProjectID))
	if err != nil {
		return nil, err
	}
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func handleListIssues(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		ProjectID string `json:"project_id"`
		State     string `json:"state"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/api/v4/projects/%s/issues", args.ProjectID)
	if args.State != "" {
		path += "?state=" + args.State
	}
	data, err := gitlabAPI(path)
	if err != nil {
		return nil, err
	}
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func handleListMRs(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		ProjectID string `json:"project_id"`
		State     string `json:"state"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests", args.ProjectID)
	if args.State != "" {
		path += "?state=" + args.State
	}
	data, err := gitlabAPI(path)
	if err != nil {
		return nil, err
	}
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func gitlabAPI(path string) ([]byte, error) {
	token := os.Getenv("GITLAB_TOKEN")
	baseURL := os.Getenv("GITLAB_URL")
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}
	if token == "" {
		return nil, fmt.Errorf("GITLAB_TOKEN environment variable not set")
	}

	req, err := http.NewRequest("GET", baseURL+path, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitLab API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitLab API error %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}
