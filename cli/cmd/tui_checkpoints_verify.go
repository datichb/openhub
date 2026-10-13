package cmd

import (
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// cardVerify renders the R3 part of a checkpoint card (CP-R5): the
// verification run by oh, the evidence checked, the fix rounds.
func cardVerify(r checkpoint.CardR3) views.CheckpointVerifyView {
	v := views.CheckpointVerifyView{Unresolved: r.Unresolved}
	if r.Verify != nil {
		v.Status, v.Summary, v.Excerpt = r.Verify.Status, checkpoint.VerifySummary(*r.Verify), r.Verify.Excerpt
	}
	for _, e := range r.Evidence {
		v.Evidence = append(v.Evidence, views.CheckpointEvidenceView{Cmd: e.Cmd, Exit: e.Exit, Observed: e.Observed, Contradicted: e.Contradicted})
	}
	return v
}
