package domain

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

// WikiProposal is a proposal for the wiki of a project (docs/wiki/),
// filed by an agent with wiki_propose (modernisation LD-J), kept in
// wiki_proposals (v45) until the user validates it on the card of the
// commit checkpoint (oh then applies it) or the documentarian applies it
// (wiki_apply). The author is the agent oh saw calling the tool, never a
// value given by the agent. Proposals outlive their session: those of the
// workflows without a commit stay queued (doc-sync).
type WikiProposal struct {
	ProjectID string `json:"project_id"`
	Seq       int    `json:"seq"` // 1, 2… in the project: the id is wp-<seq>
	SessionID string `json:"session_id"`
	// ToolSession is the tool session that filed it (a subagent session).
	ToolSession string `json:"tool_session,omitempty"`
	Agent       string `json:"agent"`
	// Page is relative to docs/wiki/ (technical/conventions.md).
	Page    string `json:"page"`
	Section string `json:"section,omitempty"`
	Content string `json:"content"`
	Tag     string `json:"tag"` // CONFIRMÉ | DÉDUIT | INCERTAIN
	Source  string `json:"source,omitempty"`
	// Replaces is the line of the page the proposal replaces (p5-fixes):
	// "" adds the content to its section.
	Replaces string `json:"replaces,omitempty"`
	Status   string `json:"status"`
	// Edited: the content was changed by the user on the card (Original
	// keeps the content of the agent).
	Original  string     `json:"original,omitempty"`
	DecidedBy string     `json:"decided_by,omitempty"`
	DecidedAt *time.Time `json:"decided_at,omitempty"`
	// AppliedBy is "oh" (validated on the card, applied at the commit
	// checkpoint) or the agent of wiki_apply.
	AppliedBy string     `json:"applied_by,omitempty"`
	AppliedAt *time.Time `json:"applied_at,omitempty"`
	// Error is the last failure to apply it.
	Error string `json:"error,omitempty"`
	// Reason is why the user refused it (p9-wiki-refusal): optional, "" for
	// a proposal not refused or refused without a word.
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Statuses of a wiki proposal.
const (
	WikiPending  = "pending"
	WikiAccepted = "accepted"
	WikiRefused  = "refused"
	WikiApplied  = "applied"
	// WikiStale: the line a replacement replaces is no longer in the page
	// (p5-fixes): not applied.
	WikiStale = "stale"
)

// WikiProposalID returns the id of the proposal number seq ("wp-3").
func WikiProposalID(seq int) string { return "wp-" + strconv.Itoa(seq) }

// ID is the id of the proposal ("wp-3").
func (p WikiProposal) ID() string { return WikiProposalID(p.Seq) }

// Open reports whether the proposal still waits to be applied (pending or
// accepted).
func (p WikiProposal) Open() bool { return p.Status == WikiPending || p.Status == WikiAccepted }

// ParseWikiProposalID reads "wp-3" (or "3").
func ParseWikiProposalID(id string) (int, bool) {
	id = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(id)), "wp-")
	n, err := strconv.Atoi(id)
	return n, err == nil && n > 0
}

// ErrWikiLimit is returned when a session or a project holds too many open
// proposals to take another one.
var ErrWikiLimit = errors.New("wiki proposal limit reached")

// WikiProposalFilter selects proposals (zero fields: any).
type WikiProposalFilter struct {
	ProjectID string
	SessionID string
	Statuses  []string
}

// WikiProposalStore persists the wiki proposals.
type WikiProposalStore interface {
	// AddWikiProposal records p with the next number of its project (p.Seq
	// is set), unless its session would then hold more than maxSession
	// open proposals or its project more than maxProject
	// (ErrWikiLimit).
	AddWikiProposal(ctx context.Context, p *WikiProposal, maxSession, maxProject int) error
	// ListWikiProposals returns the proposals of the filter, oldest first.
	ListWikiProposals(ctx context.Context, f WikiProposalFilter) ([]WikiProposal, error)
	// GetWikiProposal returns a proposal (ErrNotFound).
	GetWikiProposal(ctx context.Context, projectID string, seq int) (*WikiProposal, error)
	// UpdateWikiProposal writes the status, content, decision, refusal
	// reason, application and error fields of p.
	UpdateWikiProposal(ctx context.Context, p *WikiProposal) error
}
