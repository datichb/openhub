// Package gitlab implements the GitLab MCP server.
package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

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
	data, err := gitlabAPI(fmt.Sprintf("/api/v4/projects/%s", url.PathEscape(args.ProjectID)))
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
	path := fmt.Sprintf("/api/v4/projects/%s/issues", url.PathEscape(args.ProjectID))
	if args.State != "" {
		params := url.Values{"state": {args.State}}
		path += "?" + params.Encode()
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
	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests", url.PathEscape(args.ProjectID))
	if args.State != "" {
		params := url.Values{"state": {args.State}}
		path += "?" + params.Encode()
	}
	data, err := gitlabAPI(path)
	if err != nil {
		return nil, err
	}
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

const maxResponseSize = 50 << 20 // 50 MB

func gitlabAPI(path string) ([]byte, error) {
	token := os.Getenv("GITLAB_TOKEN")
	baseURL := os.Getenv("GITLAB_URL")
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}
	if token == "" {
		return nil, fmt.Errorf("GITLAB_TOKEN environment variable not set")
	}

	// Validate GITLAB_URL: must be https and not point to private networks
	if err := validateGitLabURL(baseURL); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", baseURL+path, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitLab API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitLab API error %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// validateGitLabURL checks that the URL is safe to send credentials to.
func validateGitLabURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("GITLAB_URL is not a valid URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("GITLAB_URL must use https:// (got %s://)", parsed.Scheme)
	}
	host := parsed.Hostname()
	if isPrivateHost(host) {
		return fmt.Errorf("GITLAB_URL must not point to a private/internal address")
	}
	return nil
}

// isPrivateHost checks if a hostname or IP belongs to a private/reserved range.
func isPrivateHost(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		// Not an IP literal — resolve to check
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			return false // cannot resolve — let the HTTP client fail later
		}
		ip = ips[0]
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}
