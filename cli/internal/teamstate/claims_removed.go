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
//
// A commit that does other things as well (the tracker sync, which also
// cleans up the done claims and releases the expired ones) names the claims it
// removed on purpose with a ReleasedTrailer line of its body.
var removalPrefixes = []string{"release: ", "cleanup: ", "claims-queue: ", "claims: "}

// releasedTrailerKey starts the lines of the body of a commit that name a claim
// the commit removed on purpose.
const releasedTrailerKey = "Released-claim: "

// ReleasedTrailer is the line of a commit message that says the commit removed
// the claim of the ticket on purpose (see ClaimRemovedOnPurpose).
func ReleasedTrailer(project, ticketID string) string {
	return releasedTrailerKey + project + "/" + ticketID
}

// WithReleasedTrailers adds to a commit message the lines that name the claims
// the commit removed on purpose.
func WithReleasedTrailers(msg string, released []Claim) string {
	if len(released) == 0 {
		return msg
	}
	var b strings.Builder
	b.WriteString(msg)
	b.WriteString("\n")
	for _, c := range released {
		b.WriteString("\n" + ReleasedTrailer(c.Project, c.TicketID))
	}
	return b.String()
}

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
	out, err := r.git(ctx, r.path, "log", "-1", "--no-renames", "--diff-filter=D", "--format=%B", "--", r.claimRelPath(project, ticketID))
	if err != nil {
		return false
	}
	subject, body, _ := strings.Cut(strings.TrimSpace(out), "\n")
	for _, p := range removalPrefixes {
		if strings.HasPrefix(subject, p) {
			return true
		}
	}
	trailer := ReleasedTrailer(project, ticketID)
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == trailer {
			return true
		}
	}
	return false
}
