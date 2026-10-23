package api

import "github.com/datichb/openhub/cli/internal/termlaunch"

// AttachedClient is an interactive client attached to a session (GET
// /v1/clients?session=<id>).
type AttachedClient struct {
	ClientID  string               `json:"client_id"`
	SessionID string               `json:"session_id"`
	Where     *termlaunch.Location `json:"where,omitempty"`
}
