// Package gslides implements the Google Slides MCP server.
package gslides

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/datichb/openhub/cli/internal/httplog"
	"github.com/datichb/openhub/cli/internal/mcp/protocol"
)

// Serve starts the Google Slides MCP server.
func Serve() error {
	server := protocol.NewServer("gslides-mcp", "2.0.0")
	server.AcceptLegacyNames("gslides_")

	server.RegisterTool(protocol.Tool{
		Name:        "get_presentation",
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
		Name:        "get_slide",
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

// Identifiers are refused unless they have the shape Google gives them:
// url.PathEscape leaves ".." as it is, which would climb one segment of the
// API path (/v1/presentations/../x). An object id (a slide) may also hold ":".
var (
	presentationIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
	slideIDRe        = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_:-]{0,127}$`)
)

// presentationPath is the API path of a presentation, followed by sub.
func presentationPath(id string, sub ...string) (string, error) {
	if !presentationIDRe.MatchString(id) {
		return "", fmt.Errorf("invalid presentation id %q", id)
	}
	p := "/v1/presentations/" + url.PathEscape(id)
	for _, s := range sub {
		p += "/" + s
	}
	return p, nil
}

func handleGetPresentation(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		PresentationID string `json:"presentation_id"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	path, err := presentationPath(args.PresentationID)
	if err != nil {
		return nil, err
	}
	data, err := slidesAPI(path)
	if err != nil {
		return nil, err
	}
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

func handleGetSlide(_ context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var args struct {
		PresentationID string `json:"presentation_id"`
		SlideID        string `json:"slide_id"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	if !slideIDRe.MatchString(args.SlideID) {
		return nil, fmt.Errorf("invalid slide id %q", args.SlideID)
	}
	path, err := presentationPath(args.PresentationID, "pages", url.PathEscape(args.SlideID))
	if err != nil {
		return nil, err
	}
	data, err := slidesAPI(path)
	if err != nil {
		return nil, err
	}
	return &protocol.ToolResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(data)}},
	}, nil
}

var httpClient = httplog.Wrap(&http.Client{Timeout: 30 * time.Second}, "mcp.gslides")

const maxResponseSize = 50 << 20 // 50 MB

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

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google Slides API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("google Slides API error %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}
