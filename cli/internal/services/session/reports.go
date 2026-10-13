package session

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// Reports of workflow_report on a checkpoint card (modernisation L4): the
// reports filed since the last checkpoint passed before the decision, and
// those the agent cited when it reached the checkpoint.

var errNoReports = errors.New("session: no report store")

// cardReports returns the reports attached to a checkpoint decision.
func (s *Service) cardReports(ctx context.Context, d domain.Decision) []domain.Report {
	if s.Reports == nil {
		return nil
	}
	all, err := s.Reports.List(ctx, d.SessionID)
	if err != nil || len(all) == 0 {
		return nil
	}
	since := s.lastPass(ctx, d.SessionID, d.CreatedAt)
	cited := citedIDs(d.Payload.Data[domain.DataReports])
	var out []domain.Report
	for _, r := range all {
		if r.CreatedAt.After(since) || slices.Contains(cited, r.ID()) {
			out = append(out, r)
		}
	}
	return out
}

// lastPass returns when the last checkpoint was passed before t (zero when
// none was).
func (s *Service) lastPass(ctx context.Context, sessionID string, t time.Time) time.Time {
	if s.Checkpoints == nil {
		return time.Time{}
	}
	st, err := s.Checkpoints.State(ctx, sessionID)
	if err != nil {
		return time.Time{}
	}
	var last time.Time
	for _, e := range st.Timeline {
		if e.Kind == domain.CheckpointPassed && (t.IsZero() || !e.At.After(t)) && e.At.After(last) {
			last = e.At
		}
	}
	return last
}

func citedIDs(v any) []string {
	var out []string
	switch l := v.(type) {
	case []string:
		out = l
	case []any:
		for _, e := range l {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
	}
	for i, id := range out {
		if seq, ok := domain.ParseReportID(id); ok {
			out[i] = domain.ReportID(seq)
		}
	}
	return out
}

// Report returns a report of a session and its body.
func (s *Service) Report(ctx context.Context, sessionID, id string) (domain.Report, string, error) {
	if s.Reports == nil {
		return domain.Report{}, "", errNoReports
	}
	return s.Reports.Get(ctx, sessionID, id)
}

// SessionReports lists the reports of a session, oldest first.
func (s *Service) SessionReports(ctx context.Context, sessionID string) ([]domain.Report, error) {
	if s.Reports == nil {
		return nil, errNoReports
	}
	return s.Reports.List(ctx, sessionID)
}
