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
	// front instead of opening another one).
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

// StreamEvent is one line of GET /v1/stream. Exactly one of Feed, Change or
// Resync is set, except for a keep-alive, which has none of them. Seq is
// additive: an entry carries its sequence number (strictly increasing within
// one daemon run), a keep-alive or the end of the initial replay carries the
// current one. A client that ignores Seq and Resync keeps working: it only
// loses the resume.
type StreamEvent struct {
	Feed   *domain.FeedItem      `json:"feed,omitempty"`
	Change *domain.SessionChange `json:"change,omitempty"`
	// Seq is the sequence number of the entry (0 when the daemon predates it).
	Seq uint64 `json:"seq,omitempty"`
	// Resync tells the client that it may have missed entries (a resume past
	// the replay window, a slow reader, a daemon restart): it reloads what it
	// shows. The feed backlog of the session follows, with the original Seq.
	Resync bool `json:"resync,omitempty"`
}

// StreamSinceParam is the query parameter of GET /v1/stream that resumes after
// the given sequence number.
const StreamSinceParam = "since"
