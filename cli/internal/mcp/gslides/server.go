// Package gslides implements the Google Slides MCP server.
package gslides

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/datichb/openhub/cli/internal/mcp/protocol"
)

// Serve starts the Google Slides MCP server.
func Serve() error {
	server := protocol.NewServer("gslides-mcp", "2.0.0")

	server.RegisterTool(protocol.Tool{
		Name:        "gslides_get_presentation",
		Description: "Get a Google Slides presentation",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"presentation_id": map[string]interface{}{"type": "string", "description": "The presentation ID"},
			},
			"required": []string{"presentation_id"},
		},
	}, handleGetPresentation)

	server.RegisterTool(protocol.Tool{
		Name:        "gslides_get_slide",
		Description: "Get a specific slide from a presentation",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"presentation_id": map[string]interface{}{"type": "string", "description": "The presentation ID"},
				"slide_id":        map[string]interface{}{"type": "string", "description": "The slide object ID"},
			},
			"required": []string{"presentation_id", "slide_id"},
		},
	}, handleGetSlide)

	return server.Serve()
}

func handleGetPresentation(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		PresentationID string `json:"presentation_id"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	data, err := slidesAPI(fmt.Sprintf("/v1/presentations/%s", args.PresentationID))
	if err != nil {
		return nil, err
	}
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func handleGetSlide(params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		PresentationID string `json:"presentation_id"`
		SlideID        string `json:"slide_id"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	data, err := slidesAPI(fmt.Sprintf("/v1/presentations/%s/pages/%s", args.PresentationID, args.SlideID))
	if err != nil {
		return nil, err
	}
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func slidesAPI(path string) ([]byte, error) {
	token := os.Getenv("GOOGLE_ACCESS_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GOOGLE_ACCESS_TOKEN environment variable not set")
	}

	req, err := http.NewRequest("GET", "https://slides.googleapis.com"+path, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Google Slides API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Google Slides API error %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}
