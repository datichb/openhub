// Package adapters defines the contract between oh and an agentic coding tool
// (opencode V2, opencode V1, future tools). oh services depend only on these
// interfaces; tool-specific names (native agents, config keys, API routes)
// live exclusively in the adapter implementations.
package adapters

import (
	"context"
	"errors"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// ToolInfo describes the detected tool binary.
type ToolInfo struct {
	Name    string // adapter name, e.g. "opencode-v2"
	Binary  string // absolute path
	Version string
}

// Capabilities advertises what an adapter can guarantee or do.
type Capabilities struct {
	Isolation         sessionspec.IsolationLevel
	Events            bool // real-time event stream
	HeadlessDecisions bool // permissions/questions can be answered through the API
	MultiLocation     bool // one server can host sessions in several directories
	PluginHooks       bool // tool plugin hooks (prompt injection, evaluate…)
	Attach            bool // an interactive client can attach to a running session
}

// RenderedConfig is the tool-specific output of Render.
type RenderedConfig struct {
	Env   map[string]string // non-secret environment for the server process
	Files map[string][]byte // files to write under the bundle "rendered/<adapter>" dir
}

// ServerGroup is the input needed to start one tool server.
type ServerGroup struct {
	Key      sessionspec.GroupKey
	Bundle   sessionspec.BundleSpec
	Provider sessionspec.ProviderSpec
	DataDir  string // isolated tool data directory
	WorkDir  string // process working directory
	Env      map[string]string

	// Runtime and Prepared, when set, run the server in another environment
	// (container): paths are translated with Prepared.Paths; the server URL
	// stays on the machine loopback.
	Runtime  ohruntime.Runtime
	Prepared *ohruntime.Prepared
}

// ServerHandle identifies a running tool server.
type ServerHandle struct {
	Key      sessionspec.GroupKey
	URL      string
	Password string
	PID      int
	Started  time.Time
}

// VisibilityReport is the result of Attest: what the model can actually see.
type VisibilityReport struct {
	Agents     []string
	Skills     []string
	MCP        []string
	Unexpected []string // anything outside the bundle (non-empty = isolation broken)
	Warnings   []string // non-blocking findings (e.g. user plugins loaded globally)
	Level      sessionspec.IsolationLevel
}

// OK reports whether the closed world invariant holds.
func (r VisibilityReport) OK() bool { return len(r.Unexpected) == 0 }

// EventKind is the tool-agnostic meaning of an event.
type EventKind string

const (
	EventConnected       EventKind = "connected"        // stream (re)connected: resynchronize
	EventExecStarted     EventKind = "exec_started"     // the agent loop started working
	EventExecEnded       EventKind = "exec_ended"       // the agent loop stopped (turn finished, failed or interrupted)
	EventDecisionAsked   EventKind = "decision_asked"   // a permission or question waits for an answer
	EventDecisionReplied EventKind = "decision_replied" // a pending decision was answered
	EventUsage           EventKind = "usage"            // cost/token usage changed
	EventSessionCreated  EventKind = "session_created"
	EventActivity        EventKind = "activity" // any other session activity (text, tools…)
	EventOther           EventKind = "other"
)

// ToolEvent is a normalized event from the tool event stream.
type ToolEvent struct {
	ID        string
	Kind      EventKind
	Outcome   string // for EventExecEnded: succeeded | failed | interrupted | …
	Type      string // raw tool event type, e.g. "permission.asked"
	SessionID string
	ParentID  string // EventSessionCreated: parent of a child (subagent) session
	Location  string
	Time      time.Time
	Data      map[string]any
	Feed      *domain.FeedItem // live feed entry, when the event is shown to users
}

// DecisionKind classifies a pending human decision.
type DecisionKind string

const (
	DecisionPermission DecisionKind = "permission"
	DecisionQuestion   DecisionKind = "question"
)

// PendingDecision is a request waiting for a human (or policy) answer.
type PendingDecision struct {
	ID        string
	SessionID string
	Kind      DecisionKind
	Action    string   // permission action
	Resources []string // permission resources
	Title     string   // question/form title
	Message   string   // optional explanation from the tool
	Fields    []FormField
}

// ErrRequestGone is returned by Reply when the request no longer waits for
// an answer (answered elsewhere, cancelled, or unknown): first answer wins.
var ErrRequestGone = errors.New("the request was already answered or no longer exists")

// ErrInvalidAnswer is returned by Reply when the tool rejects the answer.
var ErrInvalidAnswer = errors.New("the tool rejected the answer")

// FormField is a typed question field.
type FormField struct {
	Key         string
	Title       string
	Description string
	Type        string
	Options     []FormOption
	Custom      bool
	Required    bool
}

// FormOption is one choice of a FormField.
type FormOption struct {
	Value       string
	Label       string
	Description string
}

// DecisionReply answers a PendingDecision.
type DecisionReply struct {
	SessionID string
	ID        string
	Kind      DecisionKind
	Decision  string         // permission: once | always | reject
	Message   string         // optional note forwarded to the agent
	Answer    map[string]any // question answers by field key
}

// Control operation kinds.
const (
	ControlPrompt      = "prompt"       // user prompt (Text)
	ControlSynthetic   = "synthetic"    // synthetic message (Text), S6
	ControlInterrupt   = "interrupt"    // stop the running agent loop
	ControlSwitchModel = "switch_model" // Model, for the next turns
	ControlCompact     = "compact"      // compact the session history
)

// Delivery tells when a prompt or synthetic message is taken into account.
type Delivery string

const (
	DeliveryDefault Delivery = ""      // tool default
	DeliverySteer   Delivery = "steer" // at the next step boundary of the running loop
	DeliveryQueue   Delivery = "queue" // after the running loop
)

// ControlOp is a session control operation.
type ControlOp struct {
	Kind     string
	Text     string
	Model    *sessionspec.ModelRef
	Region   string // provider region (model id resolution)
	Delivery Delivery
}

// ChildLister is implemented by adapters whose sessions can delegate to
// child (subagent) sessions: it returns child session id → parent id.
type ChildLister interface {
	Children(ctx context.Context, h ServerHandle) (map[string]string, error)
}

// Forker is implemented by adapters that can fork a session (S9): a new
// session with a copy of the history. It returns the new tool session ID.
type Forker interface {
	Fork(ctx context.Context, h ServerHandle, sessionID string) (string, error)
}

// FileChange is one changed file of a session.
type FileChange struct {
	File      string
	Status    string
	Additions int
	Deletions int
	Patch     string
}

// SessionResult summarizes a session.
type SessionResult struct {
	SessionID        string
	Title            string
	Agent            string
	Cost             float64
	TokensIn         int64
	TokensOut        int64
	TokensReasoning  int64
	TokensCacheRead  int64
	TokensCacheWrite int64
	Model            string // provider/model of the session ("" if unknown)
	Branch           string // current VCS branch of the session location ("" if unknown)
	Changes          []FileChange
}

// ToolAdapter is implemented once per tool/version.
type ToolAdapter interface {
	Name() string
	Detect(ctx context.Context) (ToolInfo, error)
	Capabilities() Capabilities

	Render(b sessionspec.BundleSpec, p sessionspec.ProviderSpec) (RenderedConfig, error)
	StartServer(ctx context.Context, g ServerGroup) (ServerHandle, error)
	StopServer(ctx context.Context, h ServerHandle) error
	Attest(ctx context.Context, h ServerHandle, b sessionspec.BundleSpec, location string) (VisibilityReport, error)

	CreateSession(ctx context.Context, h ServerHandle, s sessionspec.SessionSpec) error
	SendPrompt(ctx context.Context, h ServerHandle, sessionID, text string) error
	AttachCommand(h ServerHandle, sessionID string) (argv []string, env []string)

	Events(ctx context.Context, h ServerHandle) (<-chan ToolEvent, error)
	ActiveSessions(ctx context.Context, h ServerHandle) ([]string, error)
	Pending(ctx context.Context, h ServerHandle, sessionID string) ([]PendingDecision, error)
	Reply(ctx context.Context, h ServerHandle, d DecisionReply) error
	Control(ctx context.Context, h ServerHandle, sessionID string, op ControlOp) error
	Results(ctx context.Context, h ServerHandle, sessionID string) (SessionResult, error)
}

// TurnWaiter is implemented by adapters able to wait for the end of a
// session turn and to read what the assistant wrote (headless runs).
type TurnWaiter interface {
	WaitIdle(ctx context.Context, h ServerHandle, sessionID string) error
	AssistantText(ctx context.Context, h ServerHandle, sessionID string) (string, error)
}

// SessionEnvSetter is implemented by adapters that can (re)apply the
// session environment (S7) of an existing session. Tools may keep it in
// memory only (opencode V2): it must be applied again after a server restart.
type SessionEnvSetter interface {
	SetSessionEnv(ctx context.Context, h ServerHandle, sessionID string, env map[string]string) error
}
