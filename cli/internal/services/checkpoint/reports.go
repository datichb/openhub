package checkpoint

import (
	"context"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
)

// Reports of workflow_report (modernisation L4) in the workflow state: each
// stored report is a timeline entry (report, rp-<n>, by its author); the
// checkpoint card attaches the reports filed since the last checkpoint
// passed, and those the agent cites in workflow_checkpoint (Call.Reports).

// Reported records a stored report in the timeline of a session.
func (s *Service) Reported(ctx context.Context, root, id, agent string) error {
	_, err := s.update(ctx, root, func(st *domain.CheckpointState) error {
		s.record(st, domain.CheckpointReport, id, agent, "")
		return nil
	})
	return err
}

// citedReports reads the reports cited in a workflow_checkpoint input.
func citedReports(v any) []string {
	var out []string
	add := func(id string) {
		if id = strings.TrimSpace(id); id != "" && len(out) < 20 {
			out = append(out, id)
		}
	}
	switch l := v.(type) {
	case []any:
		for _, e := range l {
			if s, ok := e.(string); ok {
				add(s)
			}
		}
	case []string:
		for _, e := range l {
			add(e)
		}
	case string:
		for _, e := range strings.Split(l, ",") {
			add(e)
		}
	}
	return out
}
