package teamstate

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"time"
)

// ReleaseExpiredClaims removes the claims of the given projects (all when
// none is given) that are expired at now (Claim.IsExpired), in one commit
// ("release: N expired claim(s)"); none is made when nothing is expired. It
// returns the claims it released.
//
// Only the claims that carry a lease are concerned (HasLease: an owner and
// work started; a pool claim, a planned claim, a finished claim never
// expire), and a claim that waits for a review is kept (status review, or the
// review or ready-for-review label): it waits for a person, not for a session.
// The claim is judged on the state of the clone, under the write lock.
//
// The synchronization of the tracker calls it when [claim]
// release_expired_on_sync is set; it joins the transaction of the run.
func (r *Repo) ReleaseExpiredClaims(ctx context.Context, now time.Time, projects ...string) ([]Claim, error) {
	var released []Claim
	err := r.withWriteLock(ctx, func(ctx context.Context) error {
		released = nil // the transaction may run again
		cfg, err := r.loadConfig()
		if err != nil {
			return err
		}
		claims, err := r.listAllClaims()
		if err != nil {
			return err
		}
		var removed []string
		for i := range claims {
			c := &claims[i]
			if len(projects) > 0 && !slices.Contains(projects, c.Project) {
				continue
			}
			if !c.HasLease(cfg) || !c.IsExpired(now, cfg) || c.Status == ClaimStatusReview || hasReviewLabel(c.Labels) {
				continue
			}
			path := r.claimFilePath(c.Project, c.TicketID)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				continue // best-effort, like the retention cleanup
			}
			removed = append(removed, r.claimRelPath(c.Project, c.TicketID))
			released = append(released, *c)
			slog.Info("teamstate.claim.release_expired", "project", c.Project, "ticket", c.TicketID, "member", c.ClaimedBy)
		}
		if len(released) == 0 {
			return nil
		}
		return r.commitAndPush(ctx, fmt.Sprintf("release: %d expired claim(s)", len(released)), removed...)
	})
	if err != nil {
		return nil, err
	}
	return released, nil
}
