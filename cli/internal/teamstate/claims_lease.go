package teamstate

import (
	"context"
	"time"
)

// Lease of a claim: lease_until in the claim file, set when the claim is
// created or taken over and renewed by TouchClaims. A claim without
// lease_until (written before the lease existed, or by an older oh) falls
// back on the inactivity rule of IsStale.

// Instant is a point in time written in a claim file as an RFC 3339 string in
// UTC, to the second ("2026-07-08T14:30:00Z"); the empty value is omitted.
// A string and not a time.Time because go-toml drops a time.Time tagged
// omitempty, even when it is set.
type Instant string

// InstantOf is the Instant of t (empty for the zero time).
func InstantOf(t time.Time) Instant {
	if t.IsZero() {
		return ""
	}
	return Instant(t.UTC().Truncate(time.Second).Format(time.RFC3339))
}

// Time is the time of the Instant; the zero time when it is empty or not a
// date (a hand-edited claim: it is then judged like a claim without lease).
func (i Instant) Time() time.Time {
	t, err := time.Parse(time.RFC3339, string(i))
	if err != nil {
		return time.Time{}
	}
	return t
}

type claimSessionKey struct{}

// WithClaimSession tells the claim operations of ctx that do not take a
// session (TransferClaim, ClaimPoolTicket) which session works on the
// ticket: it is recorded on the claim next to the lease.
func WithClaimSession(ctx context.Context, session string) context.Context {
	return context.WithValue(ctx, claimSessionKey{}, session)
}

func claimSession(ctx context.Context) string {
	s, _ := ctx.Value(claimSessionKey{}).(string)
	return s
}

// ClaimRef designates a claim: a ticket ID is only unique within its project.
type ClaimRef struct {
	Project  string
	TicketID string
}

// IsExpired reports whether the claim is expired at now: its lease is past
// when it has one, otherwise it is stale (IsStale, with the stale_days of
// cfg; nil cfg: the default). It does not look at the owner: a pool claim
// (no owner) is judged like the others.
func (c *Claim) IsExpired(now time.Time, cfg *TeamConfig) bool {
	if end := c.LeaseUntil.Time(); !end.IsZero() {
		return now.After(end)
	}
	staleDays := 0
	if cfg != nil {
		staleDays = cfg.Takeover.StaleDays
	}
	return claimStale(c, staleDays, now)
}

// claimStale is IsStale at a given instant.
func claimStale(claim *Claim, staleDays int, now time.Time) bool {
	if staleDays <= 0 {
		staleDays = 3
	}
	lastActive := claim.LastActivity
	if lastActive.IsZero() {
		lastActive = claim.ClaimedAt
	}
	return now.Sub(lastActive) > time.Duration(staleDays)*24*time.Hour
}

// leaseApplies reports whether a claim carries a lease: it needs an owner
// and work started on it (a pool claim, a planned or initial-column claim
// and a finished one are not worked on, so nobody renews them).
func leaseApplies(c *Claim, cfg *TeamConfig) bool {
	if c.ClaimedBy == "" || c.Status == ClaimStatusPlanned || c.Status == cfg.Board.InitialStatus() {
		return false
	}
	for _, s := range cfg.Board.TerminalStatuses() {
		if c.Status == s {
			return false
		}
	}
	return true
}

// leaseEnd is the end of a lease that starts at now.
func leaseEnd(now time.Time, cfg *TeamConfig) Instant {
	return InstantOf(now.Add(cfg.Claim.LeaseDuration()))
}

// stampLease sets the lease of a claim that is created or taken over:
// now + LeaseHours and the session (empty when the caller does not know
// it), or no lease (the session is left as it is) when none applies. A config that cannot be read gives the
// default duration. The caller holds the write lock.
func (r *Repo) stampLease(c *Claim, session string, now time.Time) {
	cfg, err := r.loadConfig()
	if err != nil {
		cfg = &TeamConfig{}
	}
	c.LeaseUntil = ""
	if !leaseApplies(c, cfg) {
		return
	}
	c.LeaseUntil = leaseEnd(now, cfg)
	c.Session = session
}
