package api

import (
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/termlaunch"
)

// CapabilityHeader carries the issuing capability on privileged requests.
const CapabilityHeader = "X-Oh-Capability"

// ClientKind distinguishes heartbeating clients.
type ClientKind string

const (
	ClientAttach   ClientKind = "attach"   // interactive client attached to a session (group)
	ClientPresence ClientKind = "presence" // an oh UI is open (TUI)
)

// HeartbeatRequest is the body of POST /v1/clients/heartbeat.
type HeartbeatRequest struct {
	ClientID  string     `json:"client_id"`
	Kind      ClientKind `json:"kind"`
	Group     string     `json:"group,omitempty"`
	SessionID string     `json:"session_id,omitempty"`
	TTL       int        `json:"ttl_seconds,omitempty"` // default 90
	// Where locates an attached client (its window is brought back to the
	// front instead of opening another one, A43).
	Where *termlaunch.Location `json:"where,omitempty"`
}

// QuitPolicy is applied to a server group when oh quits.
type QuitPolicy string

const (
	PolicySleepWhenIdle QuitPolicy = "sleep_when_idle" // finish the current step, then sleep
	PolicyBackground    QuitPolicy = "background"      // keep running (normal idle timer)
	PolicyStopNow       QuitPolicy = "stop_now"        // stop the server now
)

// PolicyRequest is the body of POST /v1/groups/{group}/policy.
type PolicyRequest struct {
	Policy QuitPolicy `json:"policy"`
}

// StreamEvent is one line of GET /v1/stream (exactly one field is set; an
// empty object is a keep-alive).
type StreamEvent struct {
	Feed   *domain.FeedItem      `json:"feed,omitempty"`
	Change *domain.SessionChange `json:"change,omitempty"`
}
