package opencodev2

import (
	"context"

	"github.com/datichb/openhub/cli/internal/adapters"
)

// Headless runs (adapters.TurnWaiter).

var _ adapters.TurnWaiter = (*Adapter)(nil)

// WaitIdle blocks until the agent loop of the session is idle.
func (a *Adapter) WaitIdle(ctx context.Context, h adapters.ServerHandle, sessionID string) error {
	return client(h).Wait(ctx, sessionID)
}

// AssistantText returns the text the assistant wrote in the session.
func (a *Adapter) AssistantText(ctx context.Context, h adapters.ServerHandle, sessionID string) (string, error) {
	return client(h).AssistantText(ctx, sessionID)
}
