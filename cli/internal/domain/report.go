package domain

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Report is a report an agent filed with workflow_report (L4): oh keeps its
// body under ~/.oh/sessions/<session>/reports/ and these metadata in
// session_reports (v44). The author is the agent oh saw calling the tool,
// never a value given by the agent.
type Report struct {
	SessionID string `json:"session_id"` // the oh session (root of the delegations)
	Seq       int    `json:"seq"`        // 1, 2… in the session: the id is rp-<seq>
	Agent     string `json:"agent"`
	// ToolSession is the tool session that filed it (a subagent session).
	ToolSession string    `json:"tool_session,omitempty"`
	Title       string    `json:"title"`
	Bytes       int       `json:"bytes"`
	SHA256      string    `json:"sha256"`
	Sections    []string  `json:"sections,omitempty"` // Markdown headings, in order
	CreatedAt   time.Time `json:"created_at"`
}

// ReportID returns the id of the report number seq ("rp-3").
func ReportID(seq int) string { return "rp-" + strconv.Itoa(seq) }

// ID is the id of the report ("rp-3").
func (r Report) ID() string { return ReportID(r.Seq) }

// ParseReportID reads "rp-3" (or "3").
func ParseReportID(id string) (int, bool) {
	id = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(id)), "rp-")
	n, err := strconv.Atoi(id)
	return n, err == nil && n > 0
}

// ErrReportLimit is returned when a session holds too many reports, or too
// many bytes of reports, to take another one.
var ErrReportLimit = errors.New("report limit reached")

// ReportStore persists the metadata of the reports.
type ReportStore interface {
	// AddReport records r with the next number of its session (r.Seq is
	// set), unless the session would then hold more than maxCount reports or
	// maxBytes bytes (ErrReportLimit).
	AddReport(ctx context.Context, r *Report, maxCount int, maxBytes int64) error
	// ImportReport records r with its own number; an existing report of the
	// same number is kept (false).
	ImportReport(ctx context.Context, r *Report) (bool, error)
	// DeleteReport removes a report (a body that could not be written).
	DeleteReport(ctx context.Context, sessionID string, seq int) error
	// ListReports returns the reports of a session, oldest first.
	ListReports(ctx context.Context, sessionID string) ([]Report, error)
	// GetReport returns a report (ErrNotFound).
	GetReport(ctx context.Context, sessionID string, seq int) (*Report, error)
}
