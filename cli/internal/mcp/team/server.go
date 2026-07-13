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

	server.RegisterTool(protocol.Tool{
		Name:        "team_policies",
		Description: "Get active team policies (merged global + project overrides). Returns all rules the agent must respect.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "Project name to get merged policies for (optional, empty = global only)",
				},
			},
		},
	}, handleTeamPolicies)

	server.RegisterTool(protocol.Tool{
		Name:        "team_takeover_brief",
		Description: "Read the takeover brief for a ticket (context from previous owner). Returns enriched version if available, otherwise template.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "Project name",
				},
				"ticket_id": map[string]interface{}{
					"type":        "string",
					"description": "Ticket ID to get brief for",
				},
			},
			"required": []string{"project", "ticket_id"},
		},
	}, handleTeamTakeoverBrief)

	server.RegisterTool(protocol.Tool{
		Name:        "team_patterns_list",
		Description: "List available decomposition patterns from the team patterns library. Use tags to filter relevant patterns.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"tags": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Tags to filter by (patterns matching >= 2 tags are returned)",
				},
			},
		},
	}, handleTeamPatternsList)

	server.RegisterTool(protocol.Tool{
		Name:        "team_patterns_read",
		Description: "Read the full content of a decomposition pattern (Markdown with structure, dependencies, variants).",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Pattern name (without .md extension)",
				},
			},
			"required": []string{"name"},
		},
	}, handleTeamPatternsRead)

	server.RegisterTool(protocol.Tool{
		Name:        "team_patterns_propose",
		Description: "Propose a new pattern to the patterns library (created as validated=false, awaiting human validation).",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Pattern name (slug format, e.g. crud-api)",
				},
				"tags": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Tags for the pattern",
				},
				"complexity": map[string]interface{}{
					"type":        "string",
					"description": "Complexity level: low, medium, high",
					"enum":        []string{"low", "medium", "high"},
				},
				"project": map[string]interface{}{
					"type":        "string",
					"description": "Originating project",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Full Markdown content of the pattern",
				},
			},
			"required": []string{"name", "tags", "complexity", "content"},
		},
	}, handleTeamPatternsPropose)

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

func handleTeamPolicies(params json.RawMessage) (*protocol.ToolResult, error) {
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

	policies, err := repo.LoadPolicies(args.Project)
	if err != nil {
		return nil, fmt.Errorf("loading policies: %w", err)
	}

	if policies == nil {
		return &protocol.ToolResult{
			Content: []protocol.ContentBlock{{
				Type: "text",
				Text: "No team policies configured. Create policies.toml in the team-state repo.",
			}},
		}, nil
	}

	data, err := json.MarshalIndent(policies, "", "  ")
	if err != nil {
		return nil, err
	}

	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func handleTeamTakeoverBrief(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		Project  string `json:"project"`
		TicketID string `json:"ticket_id"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	repo, err := getRepo()
	if err != nil {
		return nil, err
	}

	content, err := repo.ReadBrief(args.Project, args.TicketID)
	if err != nil {
		if err == teamstate.ErrBriefNotFound {
			return &protocol.ToolResult{
				Content: []protocol.ContentBlock{{
					Type: "text",
					Text: fmt.Sprintf("No takeover brief found for %s/%s.", args.Project, args.TicketID),
				}},
			}, nil
		}
		return nil, fmt.Errorf("reading brief: %w", err)
	}

	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: content}},
	}, nil
}

func handleTeamPatternsList(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	repo, err := getRepo()
	if err != nil {
		return nil, err
	}

	minTags := 2
	if len(args.Tags) < 2 {
		minTags = len(args.Tags)
	}

	patterns, err := repo.ListPatterns(args.Tags, minTags)
	if err != nil {
		return nil, fmt.Errorf("listing patterns: %w", err)
	}

	if len(patterns) == 0 {
		return &protocol.ToolResult{
			Content: []protocol.ContentBlock{{
				Type: "text",
				Text: "No patterns found in the library. Use `oh patterns add` to create one.",
			}},
		}, nil
	}

	data, err := json.MarshalIndent(patterns, "", "  ")
	if err != nil {
		return nil, err
	}

	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func handleTeamPatternsRead(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	repo, err := getRepo()
	if err != nil {
		return nil, err
	}

	content, err := repo.ReadPattern(args.Name)
	if err != nil {
		return nil, fmt.Errorf("reading pattern: %w", err)
	}

	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: content}},
	}, nil
}

func handleTeamPatternsPropose(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		Name       string   `json:"name"`
		Tags       []string `json:"tags"`
		Complexity string   `json:"complexity"`
		Project    string   `json:"project"`
		Content    string   `json:"content"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}

	repo, err := getRepo()
	if err != nil {
		return nil, err
	}

	p := teamstate.Pattern{
		Name:       args.Name,
		Tags:       args.Tags,
		Complexity: args.Complexity,
		Source:     "planner",
		Project:    args.Project,
		Validated:  false, // proposals always start unvalidated
	}

	if err := repo.CreatePattern(context.Background(), p, args.Content); err != nil {
		return nil, fmt.Errorf("creating pattern: %w", err)
	}

	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Pattern %q proposed. Awaiting human validation via `oh patterns validate %s`.", args.Name, args.Name),
		}},
	}, nil
}
