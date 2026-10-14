package daemon

import (
	"context"
	"errors"
	"log/slog"

	"github.com/datichb/openhub/cli/internal/gateway"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
)

// guardCreate refuses to create tickets before the checkpoint that unlocks
// `create` (N9: cadrage and feature create tickets only once the split is
// approved). Kept apart from guardTickets (closing guard, Beads gateway
// track).
func (d *Daemon) guardCreate(ctx context.Context, g gateway.Grant) error {
	err := d.opts.Checkpoints.GuardCreate(ctx, g.SessionID)
	var lock *checkpoint.LockError
	if err != nil && !errors.As(err, &lock) {
		slog.Debug("ohd: ticket creation guard", "session", g.SessionID, "error", err)
		return nil // no workflow state: the allow-list still applies
	}
	return err
}

// guardUpdate refuses to change tickets before the checkpoint that unlocks
// `update` (r9b: triage shows its changes before any write). closing: the
// change closes tickets (see checkpoint.GuardUpdate).
func (d *Daemon) guardUpdate(ctx context.Context, g gateway.Grant, closing bool) error {
	err := d.opts.Checkpoints.GuardUpdate(ctx, g.SessionID, closing)
	var lock *checkpoint.LockError
	if err != nil && !errors.As(err, &lock) {
		slog.Debug("ohd: ticket change guard", "session", g.SessionID, "error", err)
		return nil // no workflow state: the allow-list still applies
	}
	return err
}
