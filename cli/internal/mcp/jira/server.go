// Package jira implements the Jira MCP server.
// Supports Jira Cloud (API v3) via Personal Access Token.
package jira

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/mcp/protocol"
)

const maxResponseSize = 2 * 1024 * 1024 // 2 MB

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Serve starts the Jira MCP server.
func Serve() error {
	server := protocol.NewServer("jira-mcp", "1.0.0")

	server.RegisterTool(protocol.Tool{
		Name:        "jira_list_issues",
		Description: "List Jira issues using JQL",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"jql":        map[string]interface{}{"type": "string", "description": "JQL query (e.g. 'project = MYPROJ AND status = \"In Progress\"')"},
				"max_results": map[string]interface{}{"type": "integer", "description": "Max results (default 50)"},
			},
			"required": []string{"jql"},
		},
	}, handleListIssues)

	server.RegisterTool(protocol.Tool{
		Name:        "jira_get_issue",
		Description: "Get a specific Jira issue by key",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"issue_key": map[string]interface{}{"type": "string", "description": "Issue key (e.g. PROJ-123)"},
			},
			"required": []string{"issue_key"},
		},
	}, handleGetIssue)

	server.RegisterTool(protocol.Tool{
		Name:        "jira_get_project",
		Description: "Get a Jira project by key",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project_key": map[string]interface{}{"type": "string", "description": "Project key (e.g. MYPROJ)"},
			},
			"required": []string{"project_key"},
		},
	}, handleGetProject)

	if os.Getenv("JIRA_WRITE_ENABLED") == "true" {
		server.RegisterTool(protocol.Tool{
			Name:        "jira_transition_issue",
			Description: "Transition a Jira issue to a new status",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"issue_key":     map[string]interface{}{"type": "string", "description": "Issue key"},
					"transition_id": map[string]interface{}{"type": "string", "description": "Transition ID"},
				},
				"required": []string{"issue_key", "transition_id"},
			},
		}, handleTransitionIssue)
	}

	return server.Serve()
}

func handleListIssues(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		JQL        string `json:"jql"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	if args.MaxResults == 0 {
		args.MaxResults = 50
	}
	q := url.Values{
		"jql":        {args.JQL},
		"maxResults": {fmt.Sprintf("%d", args.MaxResults)},
	}
	data, err := jiraAPI("/rest/api/3/search", q)
	if err != nil {
		return nil, err
	}
	return textResult(data), nil
}

func handleGetIssue(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		IssueKey string `json:"issue_key"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	data, err := jiraAPI(fmt.Sprintf("/rest/api/3/issue/%s", args.IssueKey), nil)
	if err != nil {
		return nil, err
	}
	return textResult(data), nil
}

func handleGetProject(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		ProjectKey string `json:"project_key"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	data, err := jiraAPI(fmt.Sprintf("/rest/api/3/project/%s", args.ProjectKey), nil)
	if err != nil {
		return nil, err
	}
	return textResult(data), nil
}

func handleTransitionIssue(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		IssueKey     string `json:"issue_key"`
		TransitionID string `json:"transition_id"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	payload := map[string]interface{}{
		"transition": map[string]string{"id": args.TransitionID},
	}
	data, err := jiraAPIPost(fmt.Sprintf("/rest/api/3/issue/%s/transitions", args.IssueKey), payload)
	if err != nil {
		return nil, err
	}
	return textResult(data), nil
}

func jiraBaseURL() string {
	return strings.TrimRight(os.Getenv("JIRA_URL"), "/")
}

func jiraAPI(path string, query url.Values) ([]byte, error) {
	base := jiraBaseURL()
	if base == "" {
		return nil, fmt.Errorf("JIRA_URL environment variable not set")
	}
	reqURL := base + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "oh-cli-jira-mcp")
	if token := os.Getenv("JIRA_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if user := os.Getenv("JIRA_USER"); user != "" {
		req.SetBasicAuth(user, os.Getenv("JIRA_API_TOKEN"))
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Jira API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Jira API error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func jiraAPIPost(path string, payload interface{}) ([]byte, error) {
	base := jiraBaseURL()
	if base == "" {
		return nil, fmt.Errorf("JIRA_URL environment variable not set")
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", base+path, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token := os.Getenv("JIRA_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Jira API POST failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Jira API error %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}

func textResult(data []byte) *protocol.ToolResult {
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}
}
