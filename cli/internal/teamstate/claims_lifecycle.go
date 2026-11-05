package teamstate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"
)

// Life cycle of the claims and of the members: the release and the transfer
// of a claim that says who holds it and who acts, the hand-over of all the
// claims of a member, and the departure of a member. Each one is a single
// transaction (one commit, one push) that also leaves the events of what it
// did, signed by the member who acts (WithActor).

// Reasons of a claim.released event (Data["reason"]).
const (
	// ReleaseManual: the owner, or a member who named the owner, released it.
	ReleaseManual = "manual"
	// ReleaseSessionEnd: the session that worked on it ended.
	ReleaseSessionEnd = "session_end"
	// ReleaseExpired: the lease ran out and the team releases expired claims
	// at each sync ([claim] release_expired_on_sync).
	ReleaseExpired = "expired"
	// ReleaseCleanup: a finished claim left the board (done_retention_days).
	ReleaseCleanup = "cleanup"
	// ReleaseMemberRemoved: the card of the owner was removed from the team.
	ReleaseMemberRemoved = "member_removed"
	// ReleaseLeftTeam: the owner left the team.
	ReleaseLeftTeam = "left_team"
	// ReleaseBulk: released with all the other claims of its owner.
	ReleaseBulk = "bulk"
)

// ErrClaimClosed is returned when a transfer targets a claim whose work is
// finished: nobody takes over a ticket that is done.
var ErrClaimClosed = errors.New("claim is finished: nothing to hand over")

// ErrRemoveNeedsLead is returned when a member who is not a lead removes
// another member. A member removes their own card without being a lead; a
// team with no lead is not locked (anyone may remove a card), like for the
// roles (ErrRoleNeedsLead).
var ErrRemoveNeedsLead = errors.New("only a lead can remove another member")

// ClaimNotOwnedError is ErrClaimNotOwned with the member who holds the claim,
// so that the message names them.
type ClaimNotOwnedError struct {
	Project, TicketID string
	// Owner holds the claim now ("" for a claim of the pool); Expected is
	// who the caller thought held it.
	Owner, Expected string
}

func (e *ClaimNotOwnedError) Error() string {
	return fmt.Sprintf("%s: %s/%s is held by %q, not %q", ErrClaimNotOwned, e.Project, e.TicketID, e.Owner, e.Expected)
}

func (e *ClaimNotOwnedError) Unwrap() error { return ErrClaimNotOwned }

// MemberHasClaimsError is returned when a member who still holds claims is
// removed without saying what becomes of them.
type MemberHasClaimsError struct {
	Member string
	Count  int
}

func (e *MemberHasClaimsError) Error() string {
	return fmt.Sprintf("member %s still holds %d open claim(s): hand them over or release them", e.Member, e.Count)
}

// ReleaseOpts qualifies ReleaseClaimAs.
type ReleaseOpts struct {
	// Owner, when set, is the member who must still hold the claim: another
	// owner (the claim moved while the caller was reading) refuses the
	// release (*ClaimNotOwnedError). "" does not check.
	Owner string
	// Reason is the Data["reason"] of the event (ReleaseManual by default).
	Reason string
}

// ReleaseClaimAs removes a claim like ReleaseClaim, in one commit, and leaves
// a claim.released event signed by the member of ctx (WithActor) with the
// owner it had. It returns the claim as it was. ErrClaimNotFound when there is
// none.
func (r *Repo) ReleaseClaimAs(ctx context.Context, project, ticketID string, opts ReleaseOpts) (Claim, error) {
	if _, err := SafeName(project); err != nil {
		return Claim{}, fmt.Errorf("invalid project name: %w", err)
	}
	if _, err := SafeName(ticketID); err != nil {
		return Claim{}, fmt.Errorf("invalid ticket ID: %w", err)
	}
	var released Claim
	err := r.Transact(ctx, func(ctx context.Context, tx *Tx) (TxResult, error) {
		c, err := r.getClaim(project, ticketID)
		if err != nil {
			return TxResult{}, err
		}
		if opts.Owner != "" && c.ClaimedBy != opts.Owner {
			return TxResult{}, &ClaimNotOwnedError{Project: project, TicketID: ticketID, Owner: c.ClaimedBy, Expected: opts.Owner}
		}
		if err := tx.Remove(r.claimRelPath(project, ticketID)); err != nil {
			return TxResult{}, fmt.Errorf("removing claim file: %w", err)
		}
		if err := tx.AppendEvent(releasedEvent(ActorOf(ctx), *c, firstNonEmpty(opts.Reason, ReleaseManual))); err != nil {
			return TxResult{}, err
		}
		released = *c
		slog.Info("teamstate.claim.release", "project", project, "ticket", ticketID, "owner", c.ClaimedBy, "by", ActorOf(ctx))
		return TxResult{Message: fmt.Sprintf("release: %s/%s", project, ticketID)}, nil
	})
	if err != nil {
		return Claim{}, err
	}
	return released, nil
}

// releasedEvent is the claim.released event of c.
func releasedEvent(actor string, c Claim, reason string) Event {
	data := map[string]interface{}{"reason": reason}
	if c.ClaimedBy != "" {
		data["owner"] = c.ClaimedBy
	}
	return Event{Actor: actor, Type: EventClaimReleased, Project: c.Project, Ticket: c.TicketID, Data: data}
}

// transferredEvent is the claim.transferred event of a claim handed from one
// member to another by actor.
func transferredEvent(actor string, c Claim, from, to, reason string) Event {
	data := map[string]interface{}{"to": to, "reason": reason}
	if from != "" {
		data["from"] = from
	}
	return Event{Actor: actor, Type: EventClaimTransferred, Project: c.Project, Ticket: c.TicketID, Data: data}
}

// ReleaseEventFor returns the claim.released event the engines that remove
// claims inside their own transaction (the tracker sync: expired and finished
// claims) write for c.
func ReleaseEventFor(actor string, c Claim, reason string) Event {
	return releasedEvent(actor, c, reason)
}

// memberIDs are the IDs of the cards of members.toml; known is false when
// the file does not exist (a team-state without registry: nothing is checked
// against it).
func (r *Repo) memberIDs() (ids map[string]Member, known bool, err error) {
	mf, err := r.readMembersFile()
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return mf.Members, true, nil
}

// checkHandOver refuses to hand a claim to a member who is not on the team.
func checkHandOver(members map[string]Member, known bool, to string) error {
	if to == "" {
		return errors.New("no member to hand the claim to")
	}
	if !known {
		return nil
	}
	if _, ok := members[to]; !ok {
		return fmt.Errorf("%w: %q", ErrMemberNotFound, to)
	}
	return nil
}

// finished reports whether the status of c is a terminal one of cfg.
func finished(c *Claim, cfg *TeamConfig) bool {
	if cfg == nil {
		cfg = &TeamConfig{}
	}
	return slices.Contains(cfg.Board.TerminalStatuses(), c.Status)
}

// handOver gives c to a member in the transaction: the claim is written with
// its new owner, no session (the previous owner's), a fresh activity and a
// lease from now.
func (r *Repo) handOver(ctx context.Context, tx *Tx, c *Claim, to string, now time.Time) error {
	c.ClaimedBy = to
	c.Session = ""                  // the session of the previous owner
	c.LastActivity = InstantOf(now) // a takeover is not stale on arrival
	r.stampLease(c, claimSession(ctx), now)
	data, err := marshalClaim(c)
	if err != nil {
		return fmt.Errorf("marshaling claim: %w", err)
	}
	if err := tx.WriteFile(r.claimRelPath(c.Project, c.TicketID), data); err != nil {
		return fmt.Errorf("writing claim: %w", err)
	}
	return nil
}

// ReassignOpts says whose claims move and where.
type ReassignOpts struct {
	// From is the member whose open claims move.
	From string
	// To receives them; "" with Release releases them. With neither, a
	// member who holds open claims gives *MemberHasClaimsError.
	To      string
	Release bool
	// ReleaseReason is the reason of the claim.released events (ReleaseBulk by
	// default).
	ReleaseReason string
	// Brief writes a takeover brief for the new owner of each claim handed
	// over (reason "transfer"), in the same commit.
	Brief bool
	// RemoveMember also takes the card of From out of members.toml, in the
	// same commit (the rights of removal apply: ErrRemoveNeedsLead).
	RemoveMember bool
}

// ReassignResult says what ReassignClaims did.
type ReassignResult struct {
	// Transferred went to To, Released were removed.
	Transferred, Released []ClaimRef
	// Kept are the finished claims of From, left as they are: the retention of
	// the done claims takes them off the board.
	Kept []ClaimRef
	// MemberRemoved is true when the card was removed.
	MemberRemoved bool
}

// OpenClaimsOf lists the claims of the member that are not finished (a
// finished claim stays until done_retention_days), then the finished ones.
// It reads the local clone.
func (r *Repo) OpenClaimsOf(ctx context.Context, member string) (open, done []Claim, err error) {
	claims, err := r.ListClaims(ctx, "")
	if err != nil {
		return nil, nil, err
	}
	cfg, _ := r.LoadConfig(ctx)
	for i := range claims {
		if claims[i].ClaimedBy != member {
			continue
		}
		if finished(&claims[i], cfg) {
			done = append(done, claims[i])
		} else {
			open = append(open, claims[i])
		}
	}
	return open, done, nil
}

// ReassignClaims moves the open claims of a member to another member
// (transfer) or releases them, all in one commit on top of the latest remote
// state, with one event per claim signed by the member of ctx. With
// RemoveMember the card of From goes in the same commit, with a member.removed
// event. The target must be a card of the team; a member does not hand their
// claims to themselves. Finished claims are left alone (Kept).
//
// The commit subject starts with "release: " when claims were released (the
// claims are then known to be removed on purpose, ClaimRemovedOnPurpose),
// "transfer: " when they were handed over, "members: remove" for a card alone.
func (r *Repo) ReassignClaims(ctx context.Context, opts ReassignOpts) (ReassignResult, error) {
	if opts.From == "" {
		return ReassignResult{}, errors.New("reassigning claims: no member")
	}
	if opts.To != "" && opts.Release {
		return ReassignResult{}, errors.New("reassigning claims: hand over or release, not both")
	}
	if opts.To == opts.From {
		return ReassignResult{}, errors.New("reassigning claims: the member cannot hand the claims to themselves")
	}
	actor := ActorOf(ctx)
	var res ReassignResult
	err := r.Transact(ctx, func(ctx context.Context, tx *Tx) (TxResult, error) {
		res = ReassignResult{} // the build runs again when a push is rejected
		members, known, err := r.memberIDs()
		if err != nil {
			return TxResult{}, err
		}
		if opts.RemoveMember {
			if _, ok := members[opts.From]; known && !ok {
				return TxResult{}, fmt.Errorf("%w: %q", ErrMemberNotFound, opts.From)
			}
			if err := checkMemberRemoval(actor, members, opts.From); err != nil {
				return TxResult{}, err
			}
		}
		cfg, _ := tx.Config()
		claims, err := r.listAllClaims()
		if err != nil {
			return TxResult{}, err
		}
		var mine []Claim
		for _, c := range claims {
			if c.ClaimedBy != opts.From {
				continue
			}
			ref := ClaimRef{Project: c.Project, TicketID: c.TicketID}
			if finished(&c, cfg) {
				res.Kept = append(res.Kept, ref)
				continue
			}
			mine = append(mine, c)
		}
		if len(mine) > 0 {
			switch {
			case opts.To != "":
				if err := checkHandOver(members, known, opts.To); err != nil {
					return TxResult{}, err
				}
			case !opts.Release:
				return TxResult{}, &MemberHasClaimsError{Member: opts.From, Count: len(mine)}
			}
		}
		now := time.Now().UTC()
		reason := firstNonEmpty(opts.ReleaseReason, ReleaseBulk)
		for i := range mine {
			c := &mine[i]
			ref := ClaimRef{Project: c.Project, TicketID: c.TicketID}
			if opts.To != "" {
				if opts.Brief && opts.From != "" {
					brief, berr := r.GenerateRawBrief(ctx, c.Project, c.TicketID, opts.From, opts.To, "transfer")
					if berr == nil {
						berr = r.SaveBrief(ctx, brief)
					}
					if berr != nil {
						slog.Warn("teamstate.claim.brief_failed", "project", c.Project, "ticket", c.TicketID, "error", berr)
					}
				}
				if err := r.handOver(ctx, tx, c, opts.To, now); err != nil {
					return TxResult{}, err
				}
				if err := tx.AppendEvent(transferredEvent(actor, *c, opts.From, opts.To, firstNonEmpty(opts.ReleaseReason, ReleaseBulk))); err != nil {
					return TxResult{}, err
				}
				res.Transferred = append(res.Transferred, ref)
				continue
			}
			if err := tx.Remove(r.claimRelPath(c.Project, c.TicketID)); err != nil {
				return TxResult{}, err
			}
			if err := tx.AppendEvent(releasedEvent(actor, *c, reason)); err != nil {
				return TxResult{}, err
			}
			res.Released = append(res.Released, ref)
		}
		if opts.RemoveMember {
			if err := r.removeMemberInTx(tx, opts.From, actor, opts.To, len(res.Transferred), len(res.Released)); err != nil {
				return TxResult{}, err
			}
			res.MemberRemoved = true
		}
		return TxResult{Message: reassignMessage(opts, res)}, nil
	})
	if err != nil {
		return ReassignResult{}, err
	}
	return res, nil
}

func reassignMessage(opts ReassignOpts, res ReassignResult) string {
	switch {
	case len(res.Released) > 0:
		msg := fmt.Sprintf("release: %d claim(s) of %s", len(res.Released), opts.From)
		if res.MemberRemoved {
			msg += ", member removed"
		}
		return msg
	case len(res.Transferred) > 0:
		msg := fmt.Sprintf("transfer: %d claim(s) of %s to %s", len(res.Transferred), opts.From, opts.To)
		if res.MemberRemoved {
			msg += ", member removed"
		}
		return msg
	}
	return "members: remove " + opts.From
}

// checkMemberRemoval refuses to a member who is not a lead (actor, the member
// who asks; "" for the code that has no member to blame, which is not checked)
// the removal of the card of another member. A member removes their own card.
// A team with no lead is not locked.
func checkMemberRemoval(actor string, members map[string]Member, id string) error {
	if actor == "" || actor == id {
		return nil
	}
	hasLead := false
	for mid, m := range members {
		if m.Role != RoleLead {
			continue
		}
		hasLead = true
		if mid == actor {
			return nil
		}
	}
	if hasLead {
		return ErrRemoveNeedsLead
	}
	return nil
}

// removeMemberInTx removes the card of id from members.toml in the transaction
// and leaves the member.removed event.
func (r *Repo) removeMemberInTx(tx *Tx, id, actor, to string, transferred, released int) error {
	mf, err := r.readMembersFile()
	if err != nil {
		return err
	}
	if _, ok := mf.Members[id]; !ok {
		return ErrMemberNotFound
	}
	delete(mf.Members, id)
	if err := r.writeMembersFile(mf); err != nil {
		return err
	}
	tx.Touch("members.toml")
	data := map[string]interface{}{"member": id, "self": actor == id}
	if transferred > 0 {
		data["claims_transferred"] = transferred
		data["to"] = to
	}
	if released > 0 {
		data["claims_released"] = released
	}
	return tx.AppendEvent(Event{Actor: actor, Type: EventMemberRemoved, Project: TeamEventsProject, Data: data})
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// PendingWork is what a clone holds that the remote does not have.
type PendingWork struct {
	// Unpushed is the number of local commits no remote branch has.
	Unpushed int
	// Uncommitted lists the files changed and not committed.
	Uncommitted []string
}

// None reports whether the clone has nothing the remote lacks.
func (p PendingWork) None() bool { return p.Unpushed == 0 && len(p.Uncommitted) == 0 }

// PendingWork reads the clone: deleting it loses these changes.
func (r *Repo) PendingWork(ctx context.Context) (PendingWork, error) {
	var p PendingWork
	if !r.IsCloned() {
		return p, ErrNotCloned
	}
	out, err := r.git(ctx, r.path, "status", "--porcelain")
	if err != nil {
		return p, err
	}
	for _, line := range strings.Split(out, "\n") {
		if len(line) > 3 {
			p.Uncommitted = append(p.Uncommitted, line[3:])
		}
	}
	out, err = r.git(ctx, r.path, "rev-list", "--count", "HEAD", "--not", "--remotes")
	if err != nil {
		return p, err
	}
	if _, err := fmt.Sscanf(strings.TrimSpace(out), "%d", &p.Unpushed); err != nil {
		return p, fmt.Errorf("counting unpushed commits: %w", err)
	}
	return p, nil
}

type takenEventKey struct{}

// WithClaimEvent asks CreateClaim and ClaimPoolTicket to leave a claim.taken
// event signed by actor in the same commit as the claim (one round trip to
// the remote instead of two). Without it they leave none: the callers that
// announce the claim themselves keep doing so.
func WithClaimEvent(ctx context.Context, actor string) context.Context {
	return context.WithValue(ctx, takenEventKey{}, actor)
}

// announceTaken writes the claim.taken event of a claim just written, when
// ctx asked for it (WithClaimEvent) and a transaction carries it.
func (r *Repo) announceTaken(ctx context.Context, c *Claim) error {
	actor, ok := ctx.Value(takenEventKey{}).(string)
	if !ok {
		return nil
	}
	tx := r.activeTx(ctx)
	if tx == nil {
		return nil
	}
	return tx.AppendEvent(Event{Actor: actor, Type: EventClaimTaken, Project: c.Project, Ticket: c.TicketID})
}

// HasLead reports whether any card of the team has the role lead; a team with
// none is not locked (anyone may remove a card or change a role).
func (r *Repo) HasLead(ctx context.Context) bool {
	members, err := r.ListMembers(ctx)
	if err != nil {
		return false
	}
	for _, m := range members {
		if m.Role == RoleLead {
			return true
		}
	}
	return false
}
