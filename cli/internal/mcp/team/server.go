// Package team implements the Team MCP server.
// It exposes team-state data (members, claims, wiki, events) to AI agents
// via the MCP protocol over stdio.
package team

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/mcp/protocol"
	"github.com/datichb/openhub/cli/internal/notify"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Serve starts the Team MCP server on stdio.
func Serve() error {
	server := protocol.NewServer("team-mcp", "1.0.0")

	server.RegisterTool(protocol.Tool{
		Name:        "team_members",
		Description: "List all team members with their roles and usernames",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}, handleTeamMembers)

	server.RegisterTool(protocol.Tool{
		Name:        "team_claims",
		Description: "List active ticket claims (who is working on what)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "Filter by project ID (optional, empty = all projects)",
				},
			},
		},
	}, handleTeamClaims)

	server.RegisterTool(protocol.Tool{
		Name:        "team_wiki_list",
		Description: "List available pages in the team wiki (cross-project knowledge base)",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}, handleTeamWikiList)

	server.RegisterTool(protocol.Tool{
		Name:        "team_wiki_read",
		Description: "Read a page from the team wiki",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"page": map[string]interface{}{
					"type":        "string",
					"description": "Page name (without .md extension)",
				},
			},
			"required": []string{"page"},
		},
	}, handleTeamWikiRead)

	server.RegisterTool(protocol.Tool{
		Name:        "team_wiki_write",
		Description: "Propose a new entry to the team wiki (creates a pending proposal for human review)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"page": map[string]interface{}{
					"type":        "string",
					"description": "Target page name (without .md extension)",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Markdown content to propose (max 200 lines)",
				},
				"confidence": map[string]interface{}{
					"type":        "string",
					"description": "Confidence level: CONFIRMED, INFERRED, or UNCERTAIN",
					"enum":        []string{"CONFIRMED", "INFERRED", "UNCERTAIN"},
				},
				"project": map[string]interface{}{
					"type":        "string",
					"description": "Originating project name",
				},
			},
			"required": []string{"page", "content", "confidence", "project"},
		},
	}, handleTeamWikiWrite)

	server.RegisterTool(protocol.Tool{
		Name:        "team_events",
		Description: "List recent team activity events",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "Filter by project (optional)",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of events (default: 20)",
				},
			},
		},
	}, handleTeamEvents)

	server.RegisterTool(protocol.Tool{
		Name:        "team_notify",
		Description: "Send a notification to the team Mattermost channel",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Message text to send",
				},
			},
			"required": []string{"message"},
		},
	}, handleTeamNotify)

	return server.Serve()
}

// getRepo returns an initialized team-state repo from the hub config.
func getRepo() (*teamstate.Repo, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	if !cfg.Team.Enabled {
		return nil, fmt.Errorf("team features not enabled in hub.toml")
	}

	statePath := cfg.Team.StatePath
	if statePath == "" {
		statePath = config.DefaultTeamStatePath()
	}

	repo := teamstate.NewRepo(cfg.Team.StateRepo, statePath)
	if !repo.IsCloned() {
		return nil, fmt.Errorf("team-state repo not cloned at %s", statePath)
	}

	// Pull latest (best-effort)
	_ = repo.Pull(context.Background())

	return repo, nil
}

func handleTeamMembers(params json.RawMessage) (*protocol.ToolResult, error) {
	repo, err := getRepo()
	if err != nil {
		return nil, err
	}

	members, err := repo.ListMembers()
	if err != nil {
		return nil, fmt.Errorf("listing members: %w", err)
	}

	data, err := json.MarshalIndent(members, "", "  ")
	if err != nil {
		return nil, err
	}

	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func handleTeamClaims(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		Project string `json:"project"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	repo, err := getRepo()
	if err != nil {
		return nil, err
	}

	claims, err := repo.ListClaims(args.Project)
	if err != nil {
		return nil, fmt.Errorf("listing claims: %w", err)
	}

	data, err := json.MarshalIndent(claims, "", "  ")
	if err != nil {
		return nil, err
	}

	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func handleTeamWikiList(params json.RawMessage) (*protocol.ToolResult, error) {
	repo, err := getRepo()
	if err != nil {
		return nil, err
	}

	pages, err := repo.WikiListPages()
	if err != nil {
		return nil, fmt.Errorf("listing wiki pages: %w", err)
	}

	data, err := json.MarshalIndent(pages, "", "  ")
	if err != nil {
		return nil, err
	}

	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func handleTeamWikiRead(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		Page string `json:"page"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	repo, err := getRepo()
	if err != nil {
		return nil, err
	}

	content, err := repo.WikiReadPage(args.Page)
	if err != nil {
		return nil, fmt.Errorf("reading wiki page: %w", err)
	}

	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: content}},
	}, nil
}

func handleTeamWikiWrite(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		Page       string `json:"page"`
		Content    string `json:"content"`
		Confidence string `json:"confidence"`
		Project    string `json:"project"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	repo, err := getRepo()
	if err != nil {
		return nil, err
	}

	proposal := teamstate.WikiProposal{
		Page:       args.Page,
		Content:    args.Content,
		Confidence: args.Confidence,
		Author:     "documentarian", // Only documentarian should call this
		Project:    args.Project,
		CreatedAt:  time.Now().UTC(),
	}

	ctx := context.Background()
	if err := repo.WikiCreateProposal(ctx, proposal); err != nil {
		return nil, fmt.Errorf("creating proposal: %w", err)
	}

	// Emit event + notification
	event := teamstate.Event{
		Timestamp: time.Now().UTC(),
		Actor:     "documentarian",
		Type:      teamstate.EventWikiProposal,
		Project:   args.Project,
		Data:      map[string]interface{}{"page": args.Page},
	}
	_ = repo.AppendEvent(ctx, event)

	// Best-effort notification
	if teamCfg, err := repo.LoadConfig(); err == nil {
		d := notify.NewDispatcher(teamCfg)
		_ = d.Dispatch(ctx, event)
	}

	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Proposal created for page %q. Awaiting human review via `oh team wiki review`.", args.Page),
		}},
	}, nil
}

func handleTeamEvents(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		Project string `json:"project"`
		Limit   int    `json:"limit"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	if args.Limit <= 0 {
		args.Limit = 20
	}

	repo, err := getRepo()
	if err != nil {
		return nil, err
	}

	events, err := repo.ListEventsLimited(args.Project, args.Limit)
	if err != nil {
		return nil, fmt.Errorf("listing events: %w", err)
	}

	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return nil, err
	}

	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func handleTeamNotify(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	repo, err := getRepo()
	if err != nil {
		return nil, err
	}

	teamCfg, err := repo.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("loading team config: %w", err)
	}

	d := notify.NewDispatcher(teamCfg)
	ctx := context.Background()
	if err := d.Dispatch(ctx, teamstate.Event{
		Type:    "custom.notification",
		Project: "team",
		Data:    map[string]interface{}{"message": args.Message},
	}); err != nil {
		return nil, fmt.Errorf("sending notification: %w", err)
	}

	// For custom notifications, we send directly
	if teamCfg.Notification.Enabled {
		mm := notify.NewMattermost(
			teamCfg.Notification.MattermostWebhook,
			teamCfg.Notification.Channel,
			teamCfg.Notification.BotName,
		)
		if err := mm.Send(ctx, args.Message); err != nil {
			return nil, fmt.Errorf("sending mattermost notification: %w", err)
		}
	}

	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: "Notification sent."}},
	}, nil
}
