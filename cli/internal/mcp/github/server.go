// Package github implements the GitHub MCP server.
// Provides read access to issues, PRs, and Actions via the GitHub REST API.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/httplog"
	"github.com/datichb/openhub/cli/internal/mcp/protocol"
)

// EnvWriteEnabled enables the write tools of the server when set to "true".
const EnvWriteEnabled = "GITHUB_WRITE_ENABLED"

const (
	githubAPIBase   = "https://api.github.com"
	maxResponseSize = 2 * 1024 * 1024 // 2 MB
)

var httpClient = httplog.Wrap(&http.Client{Timeout: 30 * time.Second}, "mcp.github")

// apiBase is the server the token is sent to (a variable for the tests).
var apiBase = githubAPIBase

// Formats of the identifiers the agent gives (S-32): anything else is
// refused before a request is built. Owners may hold "_" (managed users).
var (
	ownerRe    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,38}$`)
	repoRe     = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
	workflowRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]{0,254}$`)
	shaRe      = regexp.MustCompile(`^[0-9a-fA-F]{40,64}$`)
)

// repoPath returns /repos/<owner>/<repo>[/<segment>…]: repo must be
// "owner/repo" and every segment is escaped, so that none can reach another
// resource (`../`, `?`, `#`, `%2F`). Segments taken from the agent are
// checked by the caller (positive, workflowRe).
func repoPath(repo string, segments ...string) (string, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || !ownerRe.MatchString(owner) || !repoRe.MatchString(name) || name == "." || name == ".." {
		return "", fmt.Errorf("invalid repo %q: expected owner/repo", repo)
	}
	p := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
	for _, s := range segments {
		p += "/" + url.PathEscape(s)
	}
	return p, nil
}

// positive checks an issue or pull request number.
func positive(what string, n int) (string, error) {
	if n < 1 {
		return "", fmt.Errorf("invalid %s %d: expected a positive number", what, n)
	}
	return strconv.Itoa(n), nil
}

// Serve starts the GitHub MCP server.
func Serve() error {
	if githubToken() == "" {
		slog.Warn("github-mcp: no GITHUB_TOKEN or GH_TOKEN set — requests will use unauthenticated rate limit (60/hr)")
	}
	server := protocol.NewServer("github-mcp", "1.0.0")
	server.AcceptLegacyNames("github_")

	server.RegisterTool(protocol.Tool{
		Name:        "get_repo",
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
		Name:        "list_issues",
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
		Name:        "get_issue",
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
		Name:        "list_prs",
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
		Name:        "get_pr",
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
		Name:        "list_workflows",
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
		Name:        "get_workflow_run",
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
	if os.Getenv(EnvWriteEnabled) == "true" {
		server.RegisterTool(protocol.Tool{
			Name:        "create_issue",
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
	path, err := repoPath(args.Repo)
	if err != nil {
		return nil, err
	}
	data, err := githubAPI(path, nil)
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
	path, err := repoPath(args.Repo, "issues")
	if err != nil {
		return nil, err
	}
	data, err := githubAPI(path, q)
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
	number, err := positive("issue number", args.IssueNumber)
	if err != nil {
		return nil, err
	}
	path, err := repoPath(args.Repo, "issues", number)
	if err != nil {
		return nil, err
	}
	data, err := githubAPI(path, nil)
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
	path, err := repoPath(args.Repo, "pulls")
	if err != nil {
		return nil, err
	}
	data, err := githubAPI(path, q)
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
	number, err := positive("pull request number", args.PRNumber)
	if err != nil {
		return nil, err
	}
	path, err := repoPath(args.Repo, "pulls", number)
	if err != nil {
		return nil, err
	}
	data, err := githubAPI(path, nil)
	if err != nil {
		return nil, err
	}

	// Extract head SHA for the check-runs endpoint (requires a commit ref, not a PR number).
	var checks []byte
	var pr struct {
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.Unmarshal(data, &pr); err == nil && shaRe.MatchString(pr.Head.SHA) {
		if checksPath, err := repoPath(args.Repo, "commits", pr.Head.SHA, "check-runs"); err == nil {
			checks, _ = githubAPI(checksPath, nil)
		}
	}

	// Also fetch review status
	reviews, _ := githubAPI(path+"/reviews", nil)

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
	path, err := repoPath(args.Repo, "actions", "workflows")
	if err != nil {
		return nil, err
	}
	data, err := githubAPI(path, nil)
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
	if !workflowRe.MatchString(args.WorkflowID) {
		return nil, fmt.Errorf("invalid workflow_id %q: expected a workflow file name or id", args.WorkflowID)
	}
	path, err := repoPath(args.Repo, "actions", "workflows", args.WorkflowID, "runs")
	if err != nil {
		return nil, err
	}
	q := url.Values{"per_page": {"1"}}
	data, err := githubAPI(path, q)
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

	path, err := repoPath(args.Repo, "issues")
	if err != nil {
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
	data, err := githubAPIPost(path, payload)
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
	reqURL := apiBase + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}

	req, err := http.NewRequest("GET", reqURL, http.NoBody)
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

	req, err := http.NewRequest("POST", apiBase+path, strings.NewReader(string(body)))
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
