package session

import (
	"context"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
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
