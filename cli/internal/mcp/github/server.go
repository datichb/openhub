// Package github implements the GitHub MCP server.
// Provides read access to issues, PRs, and Actions via the GitHub REST API.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/httplog"
	"github.com/datichb/openhub/cli/internal/mcp/protocol"
)

const (
	githubAPIBase    = "https://api.github.com"
	maxResponseSize  = 2 * 1024 * 1024 // 2 MB
)

var httpClient = httplog.Wrap(&http.Client{Timeout: 30 * time.Second}, "mcp.github")

// Serve starts the GitHub MCP server.
func Serve() error {
	server := protocol.NewServer("github-mcp", "1.0.0")

	server.RegisterTool(protocol.Tool{
		Name:        "github_get_repo",
		Description: "Get a GitHub repository by owner/repo",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"repo": map[string]interface{}{"type": "string", "description": "owner/repo (e.g. octocat/Hello-World)"},
			},
			"required": []string{"repo"},
		},
	}, handleGetRepo)

	server.RegisterTool(protocol.Tool{
		Name:        "github_list_issues",
		Description: "List issues for a repository",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"repo":     map[string]interface{}{"type": "string", "description": "owner/repo"},
				"state":    map[string]interface{}{"type": "string", "description": "open, closed, or all (default: open)"},
				"labels":   map[string]interface{}{"type": "string", "description": "Comma-separated list of labels to filter by"},
				"assignee": map[string]interface{}{"type": "string", "description": "Filter by assignee username"},
			},
			"required": []string{"repo"},
		},
	}, handleListIssues)

	server.RegisterTool(protocol.Tool{
		Name:        "github_get_issue",
		Description: "Get a specific issue by number",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"repo":         map[string]interface{}{"type": "string", "description": "owner/repo"},
				"issue_number": map[string]interface{}{"type": "integer", "description": "Issue number"},
			},
			"required": []string{"repo", "issue_number"},
		},
	}, handleGetIssue)

	server.RegisterTool(protocol.Tool{
		Name:        "github_list_prs",
		Description: "List pull requests for a repository",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"repo":  map[string]interface{}{"type": "string", "description": "owner/repo"},
				"state": map[string]interface{}{"type": "string", "description": "open, closed, or all (default: open)"},
				"base":  map[string]interface{}{"type": "string", "description": "Filter by base branch"},
			},
			"required": []string{"repo"},
		},
	}, handleListPRs)

	server.RegisterTool(protocol.Tool{
		Name:        "github_get_pr",
		Description: "Get a specific pull request with diff and review status",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"repo":      map[string]interface{}{"type": "string", "description": "owner/repo"},
				"pr_number": map[string]interface{}{"type": "integer", "description": "PR number"},
			},
			"required": []string{"repo", "pr_number"},
		},
	}, handleGetPR)

	server.RegisterTool(protocol.Tool{
		Name:        "github_list_workflows",
		Description: "List GitHub Actions workflows for a repository",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"repo": map[string]interface{}{"type": "string", "description": "owner/repo"},
			},
			"required": []string{"repo"},
		},
	}, handleListWorkflows)

	server.RegisterTool(protocol.Tool{
		Name:        "github_get_workflow_run",
		Description: "Get the latest run of a specific workflow",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"repo":        map[string]interface{}{"type": "string", "description": "owner/repo"},
				"workflow_id": map[string]interface{}{"type": "string", "description": "Workflow filename or ID"},
			},
			"required": []string{"repo", "workflow_id"},
		},
	}, handleGetWorkflowRun)

	// Write tools (opt-in)
	if os.Getenv("GITHUB_WRITE_ENABLED") == "true" {
		server.RegisterTool(protocol.Tool{
			Name:        "github_create_issue",
			Description: "Create a new issue",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"repo":   map[string]interface{}{"type": "string", "description": "owner/repo"},
					"title":  map[string]interface{}{"type": "string", "description": "Issue title"},
					"body":   map[string]interface{}{"type": "string", "description": "Issue body (markdown)"},
					"labels": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Labels to apply"},
				},
				"required": []string{"repo", "title"},
			},
		}, handleCreateIssue)
	}

	return server.Serve()
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

func handleGetRepo(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		Repo string `json:"repo"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	data, err := githubAPI(fmt.Sprintf("/repos/%s", args.Repo), nil)
	if err != nil {
		return nil, err
	}
	return textResult(data), nil
}

func handleListIssues(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		Repo     string `json:"repo"`
		State    string `json:"state"`
		Labels   string `json:"labels"`
		Assignee string `json:"assignee"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	q := url.Values{}
	if args.State != "" {
		q.Set("state", args.State)
	} else {
		q.Set("state", "open")
	}
	if args.Labels != "" {
		q.Set("labels", args.Labels)
	}
	if args.Assignee != "" {
		q.Set("assignee", args.Assignee)
	}
	q.Set("per_page", "50")
	data, err := githubAPI(fmt.Sprintf("/repos/%s/issues", args.Repo), q)
	if err != nil {
		return nil, err
	}
	return textResult(data), nil
}

func handleGetIssue(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		Repo        string `json:"repo"`
		IssueNumber int    `json:"issue_number"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	data, err := githubAPI(fmt.Sprintf("/repos/%s/issues/%d", args.Repo, args.IssueNumber), nil)
	if err != nil {
		return nil, err
	}
	return textResult(data), nil
}

func handleListPRs(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		Repo  string `json:"repo"`
		State string `json:"state"`
		Base  string `json:"base"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	q := url.Values{}
	if args.State != "" {
		q.Set("state", args.State)
	} else {
		q.Set("state", "open")
	}
	if args.Base != "" {
		q.Set("base", args.Base)
	}
	q.Set("per_page", "30")
	data, err := githubAPI(fmt.Sprintf("/repos/%s/pulls", args.Repo), q)
	if err != nil {
		return nil, err
	}
	return textResult(data), nil
}

func handleGetPR(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		Repo     string `json:"repo"`
		PRNumber int    `json:"pr_number"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	data, err := githubAPI(fmt.Sprintf("/repos/%s/pulls/%d", args.Repo, args.PRNumber), nil)
	if err != nil {
		return nil, err
	}
	// Also fetch review status
	reviews, _ := githubAPI(fmt.Sprintf("/repos/%s/pulls/%d/reviews", args.Repo, args.PRNumber), nil)
	checks, _ := githubAPI(fmt.Sprintf("/repos/%s/commits/%d/check-runs", args.Repo, args.PRNumber), nil)

	combined := fmt.Sprintf("{\"pr\":%s,\"reviews\":%s,\"checks\":%s}",
		string(data),
		ifEmpty(reviews, "[]"),
		ifEmpty(checks, "{}"),
	)
	return textResult([]byte(combined)), nil
}

func handleListWorkflows(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		Repo string `json:"repo"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	data, err := githubAPI(fmt.Sprintf("/repos/%s/actions/workflows", args.Repo), nil)
	if err != nil {
		return nil, err
	}
	return textResult(data), nil
}

func handleGetWorkflowRun(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		Repo       string `json:"repo"`
		WorkflowID string `json:"workflow_id"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	q := url.Values{"per_page": {"1"}}
	data, err := githubAPI(fmt.Sprintf("/repos/%s/actions/workflows/%s/runs", args.Repo, args.WorkflowID), q)
	if err != nil {
		return nil, err
	}
	return textResult(data), nil
}

func handleCreateIssue(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		Repo   string   `json:"repo"`
		Title  string   `json:"title"`
		Body   string   `json:"body"`
		Labels []string `json:"labels"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"title": args.Title,
	}
	if args.Body != "" {
		payload["body"] = args.Body
	}
	if len(args.Labels) > 0 {
		payload["labels"] = args.Labels
	}
	data, err := githubAPIPost(fmt.Sprintf("/repos/%s/issues", args.Repo), payload)
	if err != nil {
		return nil, err
	}
	return textResult(data), nil
}

// ─── HTTP helpers ──────────────────────────────────────────────────────────────

func githubToken() string {
	for _, env := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	return ""
}

func githubAPI(path string, query url.Values) ([]byte, error) {
	reqURL := githubAPIBase + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "oh-cli-github-mcp")
	if token := githubToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("not found: %s", path)
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("authentication required — set GITHUB_TOKEN or GH_TOKEN")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GitHub API error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return body, nil
}

func githubAPIPost(path string, payload interface{}) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling payload: %w", err)
	}

	req, err := http.NewRequest("POST", githubAPIBase+path, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "oh-cli-github-mcp")
	if token := githubToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API POST failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GitHub API error %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}

func textResult(data []byte) *protocol.ToolResult {
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}
}

func ifEmpty(data []byte, fallback string) string {
	if len(data) == 0 {
		return fallback
	}
	return string(data)
}
