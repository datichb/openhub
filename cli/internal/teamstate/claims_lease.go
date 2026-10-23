package teamstate

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	toml "github.com/pelletier/go-toml/v2"
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

// TouchClaims renews the lease of the given claims in a single commit (one
// transaction, so one pull and one push): lease_until becomes now + LeaseHours.
//
// Only the claims of the session or of the member are renewed (member: the
// ClaimedBy; session: the Session of the claim); the others, the unknown
// tickets and the claims that carry no lease (pool, planned, finished) are
// ignored without error. A claim whose lease still has more than half of
// LeaseHours left is left alone, and when none needs renewing nothing is
// pulled, committed or pushed. A claim of the member without session takes
// the session; last_activity is not touched (a renewal is not activity).
//
// It returns the claims it renewed.
func (r *Repo) TouchClaims(ctx context.Context, refs []ClaimRef, member, session string, now time.Time) ([]ClaimRef, error) {
	r.mu.RLock()
	due, _ := r.leasesToRenew(refs, member, session, now)
	r.mu.RUnlock()
	if len(due) == 0 {
		return nil, nil
	}

	var touched []ClaimRef
	err := r.withWriteLock(ctx, func(ctx context.Context) error {
		touched = nil // the transaction may run again
		due, cfg := r.leasesToRenew(refs, member, session, now)
		if len(due) == 0 {
			return nil
		}
		rels := make([]string, 0, len(due))
		for _, c := range due {
			c.LeaseUntil = leaseEnd(now, cfg)
			if c.Session == "" {
				c.Session = session
			}
			data, err := toml.Marshal(c)
			if err != nil {
				return fmt.Errorf("marshaling claim: %w", err)
			}
			if err := os.WriteFile(r.claimFilePath(c.Project, c.TicketID), data, 0o644); err != nil {
				return fmt.Errorf("writing claim: %w", err)
			}
			rels = append(rels, r.claimRelPath(c.Project, c.TicketID))
			touched = append(touched, ClaimRef{Project: c.Project, TicketID: c.TicketID})
		}
		who := member
		if who == "" {
			who = session
		}
		slog.Debug("teamstate.claim.touch", "member", who, "claims", len(due))
		return r.commitAndPush(ctx, fmt.Sprintf("lease: %s renews %d claim(s)", who, len(due)), rels...)
	})
	if err != nil {
		return nil, err
	}
	return touched, nil
}

// leasesToRenew reads the claims of refs from disk and returns those whose
// lease TouchClaims renews, with the config it used. No lock is taken: the
// caller holds the read lock or the write lock.
func (r *Repo) leasesToRenew(refs []ClaimRef, member, session string, now time.Time) ([]*Claim, *TeamConfig) {
	cfg, err := r.loadConfig()
	if err != nil {
		cfg = &TeamConfig{}
	}
	half := cfg.Claim.LeaseDuration() / 2
	seen := make(map[ClaimRef]bool, len(refs))
	var due []*Claim
	for _, ref := range refs {
		if seen[ref] {
			continue
		}
		seen[ref] = true
		if _, err := SafeName(ref.Project); err != nil {
			continue
		}
		if _, err := SafeName(ref.TicketID); err != nil {
			continue
		}
		c, err := r.getClaim(ref.Project, ref.TicketID)
		if err != nil {
			continue // unknown ticket (or unreadable claim)
		}
		ours := (member != "" && c.ClaimedBy == member) || (session != "" && c.Session == session)
		if !ours || !leaseApplies(c, cfg) {
			continue
		}
		if end := c.LeaseUntil.Time(); !end.IsZero() && end.Sub(now) > half {
			continue // still long
		}
		due = append(due, c)
	}
	return due, cfg
}
