package teamstate

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"time"
)

// AdoptClaim attaches a session to the claim a member already holds: a
// session that starts on a ticket whose claim exists (taken by hand, or kept
// by a session that ended) becomes the session of the claim, so that the
// operations it queues at its end (conditioned on the session) apply to it
// and not to a claim another session works on.
//
// The lease starts again from now, last_activity is bumped, and the
// session-ended label of the previous session is removed. It returns false,
// without commit, when the claim already carries the session and nothing
// else changes; false and no error as well when the claim is not the
// member's (not adopted: it is someone else's, or in the pool). A claim that
// does not exist gives ErrClaimNotFound.
func (r *Repo) AdoptClaim(ctx context.Context, project, ticketID, member, session string) (bool, error) {
	if _, err := SafeName(project); err != nil {
		return false, fmt.Errorf("invalid project name: %w", err)
	}
	if _, err := SafeName(ticketID); err != nil {
		return false, fmt.Errorf("invalid ticket ID: %w", err)
	}
	if member == "" || session == "" {
		return false, nil
	}
	adopted := false
	err := r.withWriteLock(ctx, func(ctx context.Context) error {
		adopted = false // the transaction may run again
		c, err := r.getClaim(project, ticketID)
		if err != nil {
			return err
		}
		if c.ClaimedBy != member {
			return nil
		}
		marked := slices.Contains(c.Labels, LabelSessionEnded)
		if c.Session == session && !marked {
			return nil
		}
		now := time.Now().UTC()
		c.Labels = slices.DeleteFunc(c.Labels, func(l string) bool { return l == LabelSessionEnded })
		c.LastActivity = InstantOf(now)
		cfg, cfgErr := r.loadConfig()
		if cfgErr != nil {
			cfg = &TeamConfig{}
		}
		c.Session = session
		if leaseApplies(c, cfg) {
			c.LeaseUntil = leaseEnd(now, cfg)
		}
		data, err := marshalClaim(c)
		if err != nil {
			return fmt.Errorf("marshaling claim: %w", err)
		}
		if err := os.WriteFile(r.claimFilePath(project, ticketID), data, 0o644); err != nil {
			return fmt.Errorf("writing claim: %w", err)
		}
		slog.Info("teamstate.claim.adopt", "project", project, "ticket", ticketID, "member", member, "session", session)
		adopted = true
		return r.commitAndPush(ctx, fmt.Sprintf("session: %s/%s taken by %s", project, ticketID, session), r.claimRelPath(project, ticketID))
	})
	if err != nil {
		return false, err
	}
	return adopted, nil
}
