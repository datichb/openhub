package adapters

import (
	"context"
	"time"
)

// Transcriber is implemented by adapters able to read the history of a
// session in neutral terms (`oh session trace`, evaluations): what was
// asked, written and run, by the session and by its subagent sessions.
type Transcriber interface {
	// Transcript returns the entries of a session and of its subagent
	// sessions (children, recursively), oldest first.
	Transcript(ctx context.Context, h ServerHandle, sessionID string) ([]TranscriptEntry, error)
}

// Transcript entry kinds.
const (
	EntryPrompt = "prompt" // a message given to the agent (user, synthetic, system)
	EntryText   = "text"   // the agent wrote a message
	EntryTool   = "tool"   // a tool call (subagent calls included)
	EntryUsage  = "usage"  // the cost and tokens of one model turn
	EntryEnd    = "end"    // the agent loop ended (Outcome)
)

// TranscriptEntry is one step of a session history.
type TranscriptEntry struct {
	Time time.Time
	// SessionID is the tool session the entry belongs to; ParentID is its
	// parent for a subagent session ("" for the session itself).
	SessionID string
	ParentID  string
	Kind      string
	Agent     string // agent of the turn ("" when the tool does not tell)
	Model     string // provider/model of the turn
	Role      string // EntryPrompt: user | synthetic | system | skill
	Text      string
	// Call is set on EntryTool. For a subagent call, Child is the
	// subagent session.
	Call   *ToolCall
	Output string
	Child  string
	// Usage (EntryUsage).
	Cost      float64
	TokensIn  int64
	TokensOut int64
	// Outcome (EntryEnd): succeeded | failed | interrupted.
	Outcome string
}
