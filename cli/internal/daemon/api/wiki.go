package api

import (
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/wiki"
)

// WikiProposeRequest is the body of POST /v1/wiki/propose.
type WikiProposeRequest struct {
	ToolSession string `json:"tool_session"`
	Page        string `json:"page"`
	Section     string `json:"section,omitempty"`
	Content     string `json:"content"`
	Tag         string `json:"tag"`
	Source      string `json:"source,omitempty"`
	// Replaces is the line of the page the content replaces (p5-fixes).
	Replaces string `json:"replaces,omitempty"`
}

// WikiApplyRequest is the body of POST /v1/wiki/apply.
type WikiApplyRequest struct {
	ToolSession string   `json:"tool_session"`
	IDs         []string `json:"ids"`
}

// WikiProposal is a stored proposal.
type WikiProposal struct {
	ID string `json:"id"`
	domain.WikiProposal
	// PreviouslyRefused is set on the answer of a proposal that repeats one
	// the user refused (same page, same text): the agent is warned, the
	// proposal is queued all the same.
	PreviouslyRefused *WikiRefusal `json:"previously_refused,omitempty"`
}

// WikiRefusal is a refused proposal that a new one repeats.
type WikiRefusal struct {
	ID     string     `json:"id"`
	Reason string     `json:"reason,omitempty"`
	At     *time.Time `json:"at,omitempty"`
}

// WikiPageInfo is a page of the index.
type WikiPageInfo struct {
	Path       string `json:"path"`
	Title      string `json:"title,omitempty"`
	Updated    string `json:"updated,omitempty"`
	Confidence string `json:"confidence,omitempty"`
}

// WikiIndex answers GET /v1/wiki/index.
type WikiIndex struct {
	Title    string         `json:"title,omitempty"`
	Sections []string       `json:"sections,omitempty"`
	Pages    []WikiPageInfo `json:"pages"`
	GodNodes []wiki.GodNode `json:"god_nodes,omitempty"`
	Pending  int            `json:"pending"`
	NoIndex  bool           `json:"no_index,omitempty"`
	Skipped  []string       `json:"skipped,omitempty"`
	Session  []WikiProposal `json:"session_proposals,omitempty"`
}

// WikiPage answers GET /v1/wiki/page.
type WikiPage struct {
	Page    string `json:"page"`
	Section string `json:"section,omitempty"`
	Text    string `json:"text"`
}
