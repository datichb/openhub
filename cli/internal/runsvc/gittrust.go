package runsvc

import (
	"context"
	"errors"
	"log/slog"

	"github.com/datichb/openhub/cli/internal/gitutil"
)

// trustRepository records the git configuration and hooks of the session
// location when the session starts (v5 critical N5): oh refuses its own git
// calls that matter (checkpoints) when an agent changes them later. A
// change since the previous session of the location is reported, then
// trusted (the user starts a new session).
func trustRepository(ctx context.Context, location string) {
	if location == "" {
		return
	}
	if err := gitutil.Check(ctx, location); errors.Is(err, gitutil.ErrConfigChanged) {
		slog.Warn("runsvc: the git configuration of the location changed since the previous session", "location", location)
	}
	if err := gitutil.Trust(ctx, location); err != nil {
		slog.Debug("runsvc: git configuration not recorded", "location", location, "error", err)
	}
}
