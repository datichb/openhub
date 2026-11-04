package teamstate

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
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

// IsZero reports whether the Instant has no usable date.
func (i Instant) IsZero() bool { return i.Time().IsZero() }

// UnmarshalText reads the value of a claim file. It also takes a TOML
// datetime written without quotes (last_activity = 2026-07-08T14:30:00Z, as a
// hand edit or another tool writes it), which go-toml would otherwise refuse
// to put in a string type. The text is kept as it is: Time judges it.
func (i *Instant) UnmarshalText(text []byte) error {
	*i = Instant(strings.TrimSpace(string(text)))
	return nil
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

// ClaimSessionOf is the session ctx carries (WithClaimSession), "" when it
// carries none: a caller that creates a claim itself records it.
func ClaimSessionOf(ctx context.Context) string { return claimSession(ctx) }

// ClaimRef designates a claim: a ticket ID is only unique within its project.
type ClaimRef struct {
	Project  string
	TicketID string
}

// InReview reports whether the claim waits for a review: status review, or
// the review or ready-for-review label on the ticket. Such a claim waits for
// a person, not for a session: it stays reserved until the merge or until
// its owner releases it, it never expires.
func (c *Claim) InReview() bool {
	return c.Status == ClaimStatusReview || hasReviewLabel(c.Labels)
}

// InReviewOn is InReview on a board whose columns have their own review
// column: a claim in a column of role review (BoardConfig.ReviewStatuses),
// whatever its ID, waits for a review too. Every caller that knows the board
// asks this one; InReview alone only knows the status review and the labels.
func (c *Claim) InReviewOn(board BoardConfig) bool {
	return c.InReview() || slices.Contains(board.ReviewStatuses(), c.Status)
}

// boardOf is the board of cfg (the default one without a config).
func boardOf(cfg *TeamConfig) BoardConfig {
	if cfg == nil {
		return BoardConfig{}
	}
	return cfg.Board
}

// inReview is InReviewOn with the board of the repo (the default one when
// the config cannot be read).
func (r *Repo) inReview(c *Claim) bool {
	if c.InReview() {
		return true
	}
	cfg, err := r.loadConfig()
	if err != nil {
		return false
	}
	return c.InReviewOn(cfg.Board)
}

// ReviewStale reports whether the claim has been in review (InReviewOn) for
// longer than the stale_days of cfg (nil cfg: the default), counted from its
// last activity or, without one, from claimed_at. It does not make the claim
// expired: the doctor only tells the owner it has been waiting for long.
func (c *Claim) ReviewStale(now time.Time, cfg *TeamConfig) bool {
	if !c.InReviewOn(boardOf(cfg)) {
		return false
	}
	staleDays := 0
	if cfg != nil {
		staleDays = cfg.Takeover.StaleDays
	}
	return claimStale(c, staleDays, now)
}

// IsExpired reports whether the claim is expired at now: its lease is past
// when it has one, otherwise it is stale (IsStale, with the stale_days of
// cfg; nil cfg: the default). It does not look at the owner: a pool claim
// (no owner) is judged like the others. A claim in review (InReview) is
// never expired.
func (c *Claim) IsExpired(now time.Time, cfg *TeamConfig) bool {
	if c.InReviewOn(boardOf(cfg)) {
		return false
	}
	if end := c.LeaseUntil.Time(); !end.IsZero() {
		return now.After(end)
	}
	staleDays := 0
	if cfg != nil {
		staleDays = cfg.Takeover.StaleDays
	}
	return claimStale(c, staleDays, now)
}

// LastActive is the last activity on the claim, or claimed_at when it never
// had one.
func (c *Claim) LastActive() time.Time {
	if t := c.LastActivity.Time(); !t.IsZero() {
		return t
	}
	return c.ClaimedAt
}

// claimStale is IsStale at a given instant.
func claimStale(claim *Claim, staleDays int, now time.Time) bool {
	if staleDays <= 0 {
		staleDays = 3
	}
	return now.Sub(claim.LastActive()) > time.Duration(staleDays)*24*time.Hour
}

// HasLease reports whether the claim carries a lease that can run out under
// cfg (nil: the defaults): it has an owner and work started on it. A pool
// claim, a planned or initial-column claim, a finished one and one in review
// (InReview) have none, so they never expire.
func (c *Claim) HasLease(cfg *TeamConfig) bool {
	if cfg == nil {
		cfg = &TeamConfig{}
	}
	return !c.InReviewOn(cfg.Board) && leaseApplies(c, cfg)
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

// leaseEndFor is the end of a lease that starts at now on a claim held by the
// session: LeaseHours, or, with no session on the ticket (a claim taken by
// hand: nobody renews it), the longer LeaseHoursManual.
func leaseEndFor(now time.Time, cfg *TeamConfig, session string) Instant {
	if session == "" {
		return InstantOf(now.Add(cfg.Claim.ManualLeaseDuration()))
	}
	return leaseEnd(now, cfg)
}

// renewLeaseAfterReview gives a claim that just left review (wasInReview, and
// no longer InReview) a lease that runs from now when the one it holds is
// missing or past: a claim in review does not expire, so its lease was not
// renewed and may have run out long ago, and the claim would be taken over
// the moment the review ends. A lease still valid is kept.
func renewLeaseAfterReview(c *Claim, cfg *TeamConfig, wasInReview bool, now time.Time) {
	if !wasInReview || c.InReviewOn(cfg.Board) || !leaseApplies(c, cfg) {
		return
	}
	if end := c.LeaseUntil.Time(); !end.IsZero() && end.After(now) {
		return
	}
	c.LeaseUntil = leaseEndFor(now, cfg, c.Session)
}

// leaveReview is renewLeaseAfterReview with the config of the repo (the
// defaults when it cannot be read).
func (r *Repo) leaveReview(c *Claim, wasInReview bool, now time.Time) {
	if !wasInReview {
		return
	}
	cfg, err := r.loadConfig()
	if err != nil {
		cfg = &TeamConfig{}
	}
	renewLeaseAfterReview(c, cfg, true, now)
}

// stampLease sets the lease of a claim that is created or taken over:
// now + LeaseHours and the session, or now + LeaseHoursManual when the caller
// knows no session (a claim taken by hand), or no lease (the session is left
// as it is) when none applies. A config that cannot be read gives the
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
	c.LeaseUntil = leaseEndFor(now, cfg, session)
	c.Session = session
}

// RenewClaim renews the lease of a claim of the member by hand (oh team claim
// on one's own claim), expired or not, in one commit ("lease: member renews
// project/ticket"). The new lease lasts LeaseHoursManual when no session
// works on the claim, LeaseHours otherwise; last_activity is not touched. It
// returns the new end of the lease, or ErrClaimNotFound, ErrClaimNotOwned
// (another member's claim) or ErrNoLease (a claim that carries none: pool,
// planned, finished).
func (r *Repo) RenewClaim(ctx context.Context, project, ticketID, member string, now time.Time) (time.Time, error) {
	if _, err := SafeName(project); err != nil {
		return time.Time{}, fmt.Errorf("invalid project name: %w", err)
	}
	if _, err := SafeName(ticketID); err != nil {
		return time.Time{}, fmt.Errorf("invalid ticket ID: %w", err)
	}
	var end time.Time
	err := r.withWriteLock(ctx, func(ctx context.Context) error {
		c, err := r.getClaim(project, ticketID)
		if err != nil {
			return err
		}
		if member == "" || c.ClaimedBy != member {
			return ErrClaimNotOwned
		}
		cfg, err := r.loadConfig()
		if err != nil {
			cfg = &TeamConfig{}
		}
		if !leaseApplies(c, cfg) {
			return ErrNoLease
		}
		c.LeaseUntil = leaseEndFor(now, cfg, c.Session)
		end = c.LeaseUntil.Time()
		data, err := marshalClaim(c)
		if err != nil {
			return fmt.Errorf("marshaling claim: %w", err)
		}
		if err := os.WriteFile(r.claimFilePath(project, ticketID), data, 0o644); err != nil {
			return fmt.Errorf("writing claim: %w", err)
		}
		slog.Debug("teamstate.claim.renew", "project", project, "ticket", ticketID, "member", member)
		return r.commitAndPush(ctx, fmt.Sprintf("lease: %s renews %s/%s", member, project, ticketID), r.claimRelPath(project, ticketID))
	})
	if err != nil {
		return time.Time{}, err
	}
	return end, nil
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
			data, err := marshalClaim(c)
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
