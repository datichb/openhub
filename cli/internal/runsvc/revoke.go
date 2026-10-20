package runsvc

import (
	"context"
	"log/slog"
	"time"
)

const (
	// revokeRetryWait is the pause before a failed revocation is tried again.
	revokeRetryWait = 200 * time.Millisecond
	// revokeRetryMax bounds the second try, which does not depend on the
	// context of the caller (often cancelled by the Ctrl+C that revokes).
	revokeRetryMax = 5 * time.Second
)

// revokeOwner asks the daemon to revoke the proxy grants of a server group.
// A refusal is logged and the call is made once more (audit v5, S-29): a
// grant left valid would outlive its server. If it still fails, the daemon
// purges the grants without a live group at its next start.
func revokeOwner(ctx context.Context, dc DaemonClient, group string) {
	err := dc.RevokeOwner(ctx, group)
	if err == nil {
		return
	}
	slog.Warn("runsvc: proxy grants not revoked, trying again", "group", group, "error", err)
	time.Sleep(revokeRetryWait)
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), revokeRetryMax)
	defer cancel()
	if err := dc.RevokeOwner(rctx, group); err != nil {
		slog.Error("runsvc: proxy grants still not revoked, the daemon purges them at its next start", "group", group, "error", err)
	}
}
