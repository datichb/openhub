package teamstate

import (
	"context"
	"strings"
)

// removalPrefixes are the commit subjects of the operations of oh that remove
// a claim file on purpose: ReleaseClaim ("release:"), the retention of the
// done claims ("cleanup:"), the claims queue, which applies the releases of
// the sessions that end ("claims-queue:"), and the re-homing of the claims
// of a project that changed ID ("claims:").
var removalPrefixes = []string{"release: ", "cleanup: ", "claims-queue: ", "claims: "}

// ClaimRemovedOnPurpose reports whether the claim file of the ticket was last
// removed by one of the operations that remove a claim on purpose (release of
// the owner, end of a session, retention of the done claims, re-homing) as
// opposed to a claim lost by accident (a conflict, a deletion by hand, a
// corrupted clone). It reads the history of the clone, so it holds for a
// claim removed by another member or another machine. false when the history
// does not say (no removal commit, shallow clone, git failing): the caller
// then treats the claim as lost.
func (r *Repo) ClaimRemovedOnPurpose(ctx context.Context, project, ticketID string) bool {
	if _, err := SafeName(project); err != nil {
		return false
	}
	if _, err := SafeName(ticketID); err != nil {
		return false
	}
	out, err := r.git(ctx, r.path, "log", "-1", "--no-renames", "--diff-filter=D", "--format=%s", "--", r.claimRelPath(project, ticketID))
	if err != nil {
		return false
	}
	subject := strings.TrimSpace(out)
	for _, p := range removalPrefixes {
		if strings.HasPrefix(subject, p) {
			return true
		}
	}
	return false
}
