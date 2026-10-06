package opencodev2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/datichb/openhub/cli/internal/adapters"
)

var _ adapters.SessionPorter = (*Adapter)(nil)

// ExportSession returns the projected transcript of a session (`data` of
// GET /api/experimental/session/{id}/export: info + messages).
func (c *Client) ExportSession(ctx context.Context, id string) (json.RawMessage, error) {
	var out struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/experimental/session/"+url.PathEscape(id)+"/export", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// ImportSession recreates an exported session at dir (POST
// /api/experimental/session/import); the session id is kept.
func (c *Client) ImportSession(ctx context.Context, transcript json.RawMessage, dir string) (Session, error) {
	var body map[string]any
	if err := json.Unmarshal(transcript, &body); err != nil {
		return Session{}, fmt.Errorf("decoding the session transcript: %w", err)
	}
	if dir != "" {
		body["location"] = map[string]string{"directory": dir}
	}
	var out struct {
		Data Session `json:"data"`
	}
	err := c.do(ctx, http.MethodPost, "/api/experimental/session/import", nil, body, &out)
	return out.Data, err
}

// ExportSession implements adapters.SessionPorter.
func (a *Adapter) ExportSession(ctx context.Context, h adapters.ServerHandle, sessionID string) ([]byte, error) {
	return client(h).ExportSession(ctx, sessionID)
}

// ImportSession implements adapters.SessionPorter.
func (a *Adapter) ImportSession(ctx context.Context, h adapters.ServerHandle, data []byte, location string) (string, error) {
	s, err := client(h).ImportSession(ctx, data, location)
	var ae *APIError
	if errors.As(err, &ae) && ae.Status == http.StatusConflict {
		return "", adapters.ErrSessionExists
	}
	if err != nil {
		return "", err
	}
	return s.ID, nil
}
