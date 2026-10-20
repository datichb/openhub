package daemon

import (
	"context"
	"log/slog"
	"time"
)

// Revocations write to the database: an error there means a token revoked in
// memory could be restored by the next daemon (audit v5, S-29). They are
// logged and tried once more; the orphan purge at startup is the last net.

const (
	// revokeRetryWait is the pause before a failed revocation is tried again.
	revokeRetryWait = 200 * time.Millisecond
	// revokeRetryMax bounds the second try, which does not depend on the
	// context of the caller (cancelled by the very shutdown that revokes).
	revokeRetryMax = 5 * time.Second
)

// revokeStored runs a revocation of the database. On error it logs, waits a
// moment and tries once more; it reports whether the token is revoked.
func revokeStored(ctx context.Context, what string, attrs []any, revoke func(ctx context.Context) error) bool {
	err := revoke(ctx)
	if err == nil {
		return true
	}
	slog.Warn("ohd: revocation failed, trying again", append([]any{"what", what, "error", err}, attrs...)...)
	time.Sleep(revokeRetryWait)
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), revokeRetryMax)
	defer cancel()
	if err := revoke(rctx); err != nil {
		slog.Error("ohd: revocation failed, the token stays valid in the database until the next purge", append([]any{"what", what, "error", err}, attrs...)...)
		return false
	}
	slog.Info("ohd: revocation succeeded on the second try", append([]any{"what", what}, attrs...)...)
	return true
}

// revokeGrant revokes one persisted grant (by token hash).
func (d *Daemon) revokeGrant(ctx context.Context, tokenHash, owner string) bool {
	if d.opts.Grants == nil {
		return true
	}
	return revokeStored(ctx, "grant", []any{"owner", owner}, func(c context.Context) error {
		return d.opts.Grants.Revoke(c, tokenHash, time.Now())
	})
}

// revokeOwnerGrants revokes every persisted grant of an owner.
func (d *Daemon) revokeOwnerGrants(ctx context.Context, owner string) bool {
	if d.opts.Grants == nil {
		return true
	}
	return revokeStored(ctx, "owner grants", []any{"owner", owner}, func(c context.Context) error {
		return d.opts.Grants.RevokeOwner(c, owner, time.Now())
	})
}

// orphanRevoker is implemented by grant stores able to revoke, in one
// statement, the active grants no live server group holds: the grants that
// cannot be listed any more (unreadable credential source or model list)
// are reached too.
type orphanRevoker interface {
	// RevokeOrphans revokes the active grants created before the cutoff
	// whose owner has no server row in the starting or ready status.
	RevokeOrphans(ctx context.Context, createdBefore, at time.Time) (int, error)
}

// purgeOrphanGrants revokes at startup the persisted grants whose owner is
// gone or ended (S-29): after a crash, or a revocation that could not be
// written, nothing else would revoke them. Recent grants are kept (a client
// may be starting the server of the group, orphanGrace).
func (d *Daemon) purgeOrphanGrants(ctx context.Context) {
	r, ok := d.opts.Grants.(orphanRevoker)
	if !ok || d.opts.Servers == nil {
		return
	}
	var n int
	revokeStored(ctx, "orphan purge", nil, func(c context.Context) error {
		var err error
		n, err = r.RevokeOrphans(c, time.Now().Add(-orphanGrace), time.Now())
		return err
	})
	if n > 0 {
		slog.Info("ohd: grants without a live server group revoked at startup", "count", n)
	}
}
