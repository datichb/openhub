package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/sessionresults"
)

// Checkpoint and circuit breaker decisions are answered by the
// CheckpointService (P3-T17): it updates the workflow state and acts on the
// session through this service.

// UseCheckpoints registers the CheckpointService as the resolver of the
// checkpoint and circuit breaker decisions. refresh re-applies the session
// rules (the oh daemon owns them).
func (s *Service) UseCheckpoints(cp *checkpoint.Service, refresh func(ctx context.Context, sessionID string) error) {
	if s.Resolvers == nil {
		s.Resolvers = map[domain.DecisionKind]Resolver{}
	}
	s.Checkpoints = cp
	tool := checkpointTool{s: s, refresh: refresh}
	res := func(ctx context.Context, d domain.Decision, r Reply) error {
		return cp.Resolve(ctx, tool, d, r.Decision, r.Message)
	}
	s.Resolvers[domain.DecisionCheckpoint] = res
	s.Resolvers[domain.DecisionCircuit] = res
}

type checkpointTool struct {
	s       *Service
	refresh func(ctx context.Context, sessionID string) error
}

func (t checkpointTool) ReplyPermission(ctx context.Context, d domain.Decision, decision string) error {
	return t.s.replyTool(ctx, &d, adapters.DecisionReply{SessionID: d.ToolSessionID(), ID: d.ToolRef, Kind: adapters.DecisionPermission, Decision: decision})
}

func (t checkpointTool) Steer(ctx context.Context, sessionID, text string) error {
	return t.s.Send(ctx, sessionID, text, SendOptions{Delivery: adapters.DeliverySteer})
}

func (t checkpointTool) Refresh(ctx context.Context, sessionID string) error {
	if t.refresh == nil {
		return nil
	}
	return t.refresh(ctx, sessionID)
}

// CheckpointCard is the decision card of a checkpoint (10 §7.2): what the
// agent says, the changes so far, its last messages and the timeline.
type CheckpointCard struct {
	Decision   domain.Decision
	Checkpoint string
	Label      string
	Summary    string
	Files      []sessionresults.FileStat
	Additions  int
	Deletions  int
	Patch      string
	// Messages are the last texts of the session (oldest first).
	Messages []domain.FeedItem
	Timeline []checkpoint.Step
}

// cardMessages is the number of messages shown on a checkpoint card.
const cardMessages = 3

// CheckpointCard gathers the card of a checkpoint decision. Diff and
// messages are best effort (server asleep, daemon stopped).
func (s *Service) CheckpointCard(ctx context.Context, decisionID string) (*CheckpointCard, error) {
	if s.Decisions == nil {
		return nil, errors.New("session: no decision store")
	}
	d, err := s.Decisions.Get(ctx, decisionID)
	if err != nil {
		return nil, fmt.Errorf("decision %s: %w", decisionID, err)
	}
	if d.Kind != domain.DecisionCheckpoint {
		return nil, fmt.Errorf("decision %s is not a checkpoint", decisionID)
	}
	c := &CheckpointCard{Decision: *d, Label: d.Payload.Title, Summary: d.Payload.Message}
	c.Checkpoint, _ = d.Payload.Data[checkpoint.DataCheckpoint].(string)
	if res, err := s.Results(ctx, d.SessionID); err == nil {
		c.Files, c.Additions, c.Deletions, c.Patch = res.Files, res.Additions, res.Deletions, res.Patch
	}
	c.Messages = s.RecentTexts(ctx, d.SessionID, cardMessages)
	c.Timeline, _ = s.Timeline(ctx, d.SessionID)
	return c, nil
}

// Timeline returns the checkpoint timeline of a session (nil when it does
// not run a workflow).
func (s *Service) Timeline(ctx context.Context, sessionID string) ([]checkpoint.Step, error) {
	if s.Checkpoints == nil {
		return nil, nil
	}
	wf, mode, err := s.Checkpoints.Workflow(ctx, sessionID)
	if err != nil || wf == nil {
		return nil, err
	}
	st, err := s.Checkpoints.State(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return checkpoint.Timeline(wf, mode, i18n.Locale(), st), nil
}

// recentQuiet ends the read of the feed backlog (sent at once by the daemon).
const recentQuiet = 250 * time.Millisecond

// RecentTexts returns the last n texts of a session feed (from the daemon
// backlog; nil without the daemon).
func (s *Service) RecentTexts(ctx context.Context, sessionID string, n int) []domain.FeedItem {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	feed, err := s.Follow(ctx, sessionID)
	if err != nil {
		return nil
	}
	var out []domain.FeedItem
	for {
		select {
		case it, ok := <-feed:
			if !ok {
				return out
			}
			if it.Kind == domain.FeedText {
				out = append(out, it)
				if len(out) > n {
					out = out[1:]
				}
			}
		case <-time.After(recentQuiet):
			return out
		case <-ctx.Done():
			return out
		}
	}
}
