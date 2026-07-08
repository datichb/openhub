package gitlab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/datichb/openhub/cli/internal/mcp/protocol"
)

// registerWriteTools adds write-capable tools to the MCP server.
// These are only registered when GITLAB_WRITE_ENABLED=true.
func registerWriteTools(server *protocol.Server) {
	server.RegisterTool(protocol.Tool{
		Name:        "gitlab_create_mr",
		Description: "Create a merge request. First checks if one already exists for the source branch.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project_id":    map[string]interface{}{"type": "string", "description": "Project ID or URL-encoded path"},
				"source_branch": map[string]interface{}{"type": "string", "description": "Source branch name"},
				"target_branch": map[string]interface{}{"type": "string", "description": "Target branch (default: main)"},
				"title":         map[string]interface{}{"type": "string", "description": "MR title"},
				"description":   map[string]interface{}{"type": "string", "description": "MR description (markdown)"},
			},
			"required": []string{"project_id", "source_branch", "title"},
		},
	}, handleCreateMR)

	server.RegisterTool(protocol.Tool{
		Name:        "gitlab_add_mr_note",
		Description: "Add a comment/note to a merge request",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project_id": map[string]interface{}{"type": "string", "description": "Project ID"},
				"mr_iid":     map[string]interface{}{"type": "integer", "description": "MR internal ID (iid)"},
				"body":       map[string]interface{}{"type": "string", "description": "Comment body (markdown)"},
			},
			"required": []string{"project_id", "mr_iid", "body"},
		},
	}, handleAddMRNote)

	server.RegisterTool(protocol.Tool{
		Name:        "gitlab_update_issue",
		Description: "Update an issue (labels, assignees, state)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project_id":  map[string]interface{}{"type": "string", "description": "Project ID"},
				"issue_iid":   map[string]interface{}{"type": "integer", "description": "Issue internal ID (iid)"},
				"state_event": map[string]interface{}{"type": "string", "description": "State transition: reopen or close"},
				"add_labels":  map[string]interface{}{"type": "string", "description": "Comma-separated labels to add"},
				"assignee_ids": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "integer"},
					"description": "User IDs to assign",
				},
			},
			"required": []string{"project_id", "issue_iid"},
		},
	}, handleUpdateIssue)

	server.RegisterTool(protocol.Tool{
		Name:        "gitlab_assign_reviewer",
		Description: "Assign reviewer(s) to a merge request",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project_id": map[string]interface{}{"type": "string", "description": "Project ID"},
				"mr_iid":     map[string]interface{}{"type": "integer", "description": "MR internal ID"},
				"reviewer_ids": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "integer"},
					"description": "User IDs to assign as reviewers",
				},
			},
			"required": []string{"project_id", "mr_iid", "reviewer_ids"},
		},
	}, handleAssignReviewer)

	server.RegisterTool(protocol.Tool{
		Name:        "gitlab_add_label",
		Description: "Add labels to an issue",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project_id": map[string]interface{}{"type": "string", "description": "Project ID"},
				"issue_iid":  map[string]interface{}{"type": "integer", "description": "Issue internal ID"},
				"labels":     map[string]interface{}{"type": "string", "description": "Comma-separated labels to add"},
			},
			"required": []string{"project_id", "issue_iid", "labels"},
		},
	}, handleAddLabel)
}

func handleCreateMR(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		ProjectID    string `json:"project_id"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		Title        string `json:"title"`
		Description  string `json:"description"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	if args.TargetBranch == "" {
		args.TargetBranch = "main"
	}

	// Check if MR already exists for this source branch
	checkPath := fmt.Sprintf("/api/v4/projects/%s/merge_requests?source_branch=%s&state=opened",
		url.PathEscape(args.ProjectID), url.QueryEscape(args.SourceBranch))
	existing, err := gitlabAPI(checkPath)
	if err == nil {
		var mrs []json.RawMessage
		if json.Unmarshal(existing, &mrs) == nil && len(mrs) > 0 {
			// MR already exists — return it
			return &protocol.ToolResult{
				Content: []protocol.ContentBlock{{
					Type: "text",
					Text: fmt.Sprintf("MR already exists for branch %s:\n%s", args.SourceBranch, string(mrs[0])),
				}},
			}, nil
		}
	}

	// Create new MR
	payload := map[string]interface{}{
		"source_branch": args.SourceBranch,
		"target_branch": args.TargetBranch,
		"title":         args.Title,
	}
	if args.Description != "" {
		payload["description"] = args.Description
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests", url.PathEscape(args.ProjectID))
	data, err := gitlabAPIWrite("POST", path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func handleAddMRNote(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		ProjectID string `json:"project_id"`
		MrIID     int    `json:"mr_iid"`
		Body      string `json:"body"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	payload, _ := json.Marshal(map[string]string{"body": args.Body})
	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/notes",
		url.PathEscape(args.ProjectID), args.MrIID)

	data, err := gitlabAPIWrite("POST", path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func handleUpdateIssue(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		ProjectID   string `json:"project_id"`
		IssueIID    int    `json:"issue_iid"`
		StateEvent  string `json:"state_event"`
		AddLabels   string `json:"add_labels"`
		AssigneeIDs []int  `json:"assignee_ids"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	payload := make(map[string]interface{})
	if args.StateEvent != "" {
		payload["state_event"] = args.StateEvent
	}
	if args.AddLabels != "" {
		payload["add_labels"] = args.AddLabels
	}
	if len(args.AssigneeIDs) > 0 {
		payload["assignee_ids"] = args.AssigneeIDs
	}

	body, _ := json.Marshal(payload)
	path := fmt.Sprintf("/api/v4/projects/%s/issues/%d",
		url.PathEscape(args.ProjectID), args.IssueIID)

	data, err := gitlabAPIWrite("PUT", path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func handleAssignReviewer(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		ProjectID   string `json:"project_id"`
		MrIID       int    `json:"mr_iid"`
		ReviewerIDs []int  `json:"reviewer_ids"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	payload, _ := json.Marshal(map[string]interface{}{"reviewer_ids": args.ReviewerIDs})
	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d",
		url.PathEscape(args.ProjectID), args.MrIID)

	data, err := gitlabAPIWrite("PUT", path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func handleAddLabel(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		ProjectID string `json:"project_id"`
		IssueIID  int    `json:"issue_iid"`
		Labels    string `json:"labels"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	payload, _ := json.Marshal(map[string]string{"add_labels": args.Labels})
	path := fmt.Sprintf("/api/v4/projects/%s/issues/%d",
		url.PathEscape(args.ProjectID), args.IssueIID)

	data, err := gitlabAPIWrite("PUT", path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}
