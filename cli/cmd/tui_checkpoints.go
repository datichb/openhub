package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Checkpoint card and timeline of the Sessions view (P3-T17).

// timelineSteps is the number of steps shown in the session detail.
const timelineSteps = 12

var _ views.CheckpointBackend = (*tuiSessions)(nil)

// sessionTimeline renders the checkpoint timeline of a session ("" without a workflow).
func sessionTimeline(ctx context.Context, svc *sessionsvc.Service, sessionID string) string {
	steps, err := svc.Timeline(ctx, sessionID)
	if err != nil {
		return ""
	}
	return checkpoint.TimelineText(steps, timelineSteps)
}

func (t *tuiSessions) CheckpointCard(ctx context.Context, decisionID string) (views.CheckpointCardView, error) {
	svc, err := t.service(ctx)
	if err != nil {
		return views.CheckpointCardView{}, err
	}
	c, err := svc.CheckpointCard(ctx, decisionID)
	if err != nil {
		return views.CheckpointCardView{}, err
	}
	out := views.CheckpointCardView{Checkpoint: c.Checkpoint, Label: c.Label, Summary: c.Summary,
		Additions: c.Additions, Deletions: c.Deletions, Patch: c.Patch, Timeline: checkpoint.TimelineText(c.Timeline, timelineSteps)}
	for _, f := range c.Files {
		out.Files = append(out.Files, views.CheckpointFile{File: f.File, Status: f.Status, Additions: f.Additions, Deletions: f.Deletions})
	}
	for _, m := range c.Messages {
		out.Messages = append(out.Messages, sessionsvc.FeedLine(m))
	}
	return out, nil
}
