// Package linear implements the Linear MCP server.
// Provides read/write access to Linear issues via the GraphQL API.
package linear

import (
	"context"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/mcp/protocol"
)

const (
	linearAPIURL    = "https://api.linear.app/graphql"
	maxResponseSize = 2 * 1024 * 1024 // 2 MB
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Serve starts the Linear MCP server.
func Serve() error {
	server := protocol.NewServer("linear-mcp", "1.0.0")

	server.RegisterTool(protocol.Tool{
		Name:        "linear_list_issues",
		Description: "List Linear issues with optional filters",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"team_key":  map[string]interface{}{"type": "string", "description": "Team identifier key (e.g. ENG)"},
				"state":     map[string]interface{}{"type": "string", "description": "Filter by state name (e.g. In Progress)"},
				"assignee":  map[string]interface{}{"type": "string", "description": "Filter by assignee name or email"},
				"first":     map[string]interface{}{"type": "integer", "description": "Number of results (default 50)"},
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

func handleListIssues(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
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

	filter := ""
	if args.TeamKey != "" {
		filter += fmt.Sprintf(`, filter: { team: { key: { eq: "%s" } } }`, args.TeamKey)
	}

	query := fmt.Sprintf(`{
		issues(first: %d%s) {
			nodes {
				id identifier title state { name } assignee { name email }
				priority createdAt updatedAt
				description
			}
		}
	}`, args.First, filter)

	data, err := linearQuery(query)
	if err != nil {
		return nil, err
	}
	return textResult(data), nil
}

func handleGetIssue(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		IssueID string `json:"issue_id"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`{
		issue(id: "%s") {
			id identifier title description state { name }
			assignee { name email } priority
			createdAt updatedAt
			comments { nodes { body createdAt user { name } } }
		}
	}`, args.IssueID)

	data, err := linearQuery(query)
	if err != nil {
		return nil, err
	}
	return textResult(data), nil
}

func handleCreateIssue(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		TeamID      string `json:"team_id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    int    `json:"priority"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	mutation := fmt.Sprintf(`mutation {
		issueCreate(input: {
			teamId: "%s"
			title: "%s"
			description: "%s"
			priority: %d
		}) {
			success
			issue { id identifier title }
		}
	}`, args.TeamID, escapeGraphQL(args.Title), escapeGraphQL(args.Description), args.Priority)

	data, err := linearQuery(mutation)
	if err != nil {
		return nil, err
	}
	return textResult(data), nil
}

func handleUpdateIssue(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		IssueID    string `json:"issue_id"`
		StateID    string `json:"state_id"`
		AssigneeID string `json:"assignee_id"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	input := ""
	if args.StateID != "" {
		input += fmt.Sprintf(`stateId: "%s"`, args.StateID)
	}
	if args.AssigneeID != "" {
		if input != "" {
			input += " "
		}
		input += fmt.Sprintf(`assigneeId: "%s"`, args.AssigneeID)
	}

	mutation := fmt.Sprintf(`mutation {
		issueUpdate(id: "%s", input: { %s }) {
			success
			issue { id identifier title state { name } }
		}
	}`, args.IssueID, input)

	data, err := linearQuery(mutation)
	if err != nil {
		return nil, err
	}
	return textResult(data), nil
}

func linearQuery(query string) ([]byte, error) {
	token := os.Getenv("LINEAR_API_KEY")
	if token == "" {
		return nil, fmt.Errorf("LINEAR_API_KEY environment variable not set")
	}

	payload := map[string]string{"query": query}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", linearAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)
	req.Header.Set("User-Agent", "oh-cli-linear-mcp")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Linear API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Linear API error %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}

func textResult(data []byte) *protocol.ToolResult {
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}
}

func escapeGraphQL(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}
