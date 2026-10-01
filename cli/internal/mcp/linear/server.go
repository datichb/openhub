// Package linear implements the Linear MCP server.
// Provides read/write access to Linear issues via the GraphQL API.
package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/httplog"
	"github.com/datichb/openhub/cli/internal/mcp/protocol"
)

const (
	linearAPIURL    = "https://api.linear.app/graphql"
	maxResponseSize = 2 * 1024 * 1024 // 2 MB
)

var httpClient = httplog.Wrap(&http.Client{Timeout: 30 * time.Second}, "mcp.linear")

// Serve starts the Linear MCP server.
func Serve() error {
	server := protocol.NewServer("linear-mcp", "1.0.0")

	server.RegisterTool(protocol.Tool{
		Name:        "linear_list_issues",
		Description: "List Linear issues with optional filters",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"team_key": map[string]interface{}{"type": "string", "description": "Team identifier key (e.g. ENG)"},
				"state":    map[string]interface{}{"type": "string", "description": "Filter by state name (e.g. In Progress)"},
				"assignee": map[string]interface{}{"type": "string", "description": "Filter by assignee name or email"},
				"first":    map[string]interface{}{"type": "integer", "description": "Number of results (default 50)"},
			},
		},
	}, handleListIssues)

	server.RegisterTool(protocol.Tool{
		Name:        "linear_get_issue",
		Description: "Get a Linear issue by identifier",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"issue_id": map[string]interface{}{"type": "string", "description": "Issue identifier (e.g. ENG-123)"},
			},
			"required": []string{"issue_id"},
		},
	}, handleGetIssue)

	if os.Getenv("LINEAR_WRITE_ENABLED") == "true" {
		server.RegisterTool(protocol.Tool{
			Name:        "linear_create_issue",
			Description: "Create a new Linear issue",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"team_id":     map[string]interface{}{"type": "string", "description": "Team UUID"},
					"title":       map[string]interface{}{"type": "string", "description": "Issue title"},
					"description": map[string]interface{}{"type": "string", "description": "Issue description (markdown)"},
					"priority":    map[string]interface{}{"type": "integer", "description": "Priority 0-4 (0=none, 1=urgent, 2=high, 3=medium, 4=low)"},
				},
				"required": []string{"team_id", "title"},
			},
		}, handleCreateIssue)

		server.RegisterTool(protocol.Tool{
			Name:        "linear_update_issue",
			Description: "Update a Linear issue state or assignee",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"issue_id":    map[string]interface{}{"type": "string", "description": "Issue UUID or identifier"},
					"state_id":    map[string]interface{}{"type": "string", "description": "New state UUID"},
					"assignee_id": map[string]interface{}{"type": "string", "description": "Assignee user UUID"},
				},
				"required": []string{"issue_id"},
			},
		}, handleUpdateIssue)
	}

	return server.Serve()
}

// ── Handlers ────────────────────────────────────────────────────────────────

func handleListIssues(ctx context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		TeamKey  string `json:"team_key"`
		State    string `json:"state"`
		Assignee string `json:"assignee"`
		First    int    `json:"first"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	if args.First == 0 {
		args.First = 50
	}

	query := `query($first: Int!, $filter: IssueFilter) {
		issues(first: $first, filter: $filter) {
			nodes {
				id identifier title state { name } assignee { name email }
				priority createdAt updatedAt
				description
			}
		}
	}`

	variables := map[string]interface{}{
		"first": args.First,
	}

	// Build filter incrementally — sub-filters are siblings in IssueFilter, ANDed by Linear.
	filter := map[string]interface{}{}
	if args.TeamKey != "" {
		filter["team"] = map[string]interface{}{
			"key": map[string]interface{}{"eq": args.TeamKey},
		}
	}
	if args.State != "" {
		filter["state"] = map[string]interface{}{
			"name": map[string]interface{}{"eq": args.State},
		}
	}
	if args.Assignee != "" {
		filter["assignee"] = map[string]interface{}{
			"or": []map[string]interface{}{
				{"name": map[string]interface{}{"containsIgnoreCase": args.Assignee}},
				{"email": map[string]interface{}{"containsIgnoreCase": args.Assignee}},
			},
		}
	}
	if len(filter) > 0 {
		variables["filter"] = filter
	}

	data, err := linearQuery(ctx, query, variables)
	if err != nil {
		return nil, err
	}
	return textResult(data), nil
}

func handleGetIssue(ctx context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		IssueID string `json:"issue_id"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	query := `query($id: String!) {
		issue(id: $id) {
			id identifier title description state { name }
			assignee { name email } priority
			createdAt updatedAt
			comments { nodes { body createdAt user { name } } }
		}
	}`

	data, err := linearQuery(ctx, query, map[string]interface{}{"id": args.IssueID})
	if err != nil {
		return nil, err
	}
	return textResult(data), nil
}

func handleCreateIssue(ctx context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		TeamID      string `json:"team_id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    int    `json:"priority"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	query := `mutation($input: IssueCreateInput!) {
		issueCreate(input: $input) {
			success
			issue { id identifier title }
		}
	}`

	input := map[string]interface{}{
		"teamId":      args.TeamID,
		"title":       args.Title,
		"description": args.Description,
		"priority":    args.Priority,
	}

	data, err := linearQuery(ctx, query, map[string]interface{}{"input": input})
	if err != nil {
		return nil, err
	}
	return textResult(data), nil
}

func handleUpdateIssue(ctx context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		IssueID    string `json:"issue_id"`
		StateID    string `json:"state_id"`
		AssigneeID string `json:"assignee_id"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	if args.StateID == "" && args.AssigneeID == "" {
		return &protocol.ToolResult{
			Content: []protocol.ContentBlock{{
				Type: "text",
				Text: "No fields to update. Provide at least one of: state_id, assignee_id.",
			}},
			IsError: true,
		}, nil
	}

	query := `mutation($id: String!, $input: IssueUpdateInput!) {
		issueUpdate(id: $id, input: $input) {
			success
			issue { id identifier title state { name } }
		}
	}`

	input := map[string]interface{}{}
	if args.StateID != "" {
		input["stateId"] = args.StateID
	}
	if args.AssigneeID != "" {
		input["assigneeId"] = args.AssigneeID
	}

	data, err := linearQuery(ctx, query, map[string]interface{}{
		"id":    args.IssueID,
		"input": input,
	})
	if err != nil {
		return nil, err
	}
	return textResult(data), nil
}

// ── GraphQL transport ───────────────────────────────────────────────────────

// linearQuery executes a GraphQL query against the Linear API using variables
// for parameterization. This eliminates GraphQL injection risks by separating
// the query structure from user-supplied data.
func linearQuery(ctx context.Context, query string, variables map[string]interface{}) ([]byte, error) {
	token := os.Getenv("LINEAR_API_KEY")
	if token == "" {
		return nil, fmt.Errorf("LINEAR_API_KEY environment variable not set")
	}

	payload := map[string]interface{}{"query": query}
	if len(variables) > 0 {
		payload["variables"] = variables
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", linearAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)
	req.Header.Set("User-Agent", "oh-cli-linear-mcp")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("linear API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("linear API error %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}

func textResult(data []byte) *protocol.ToolResult {
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}
}
