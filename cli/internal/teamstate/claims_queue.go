package teamstate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/datichb/openhub/cli/internal/filelock"
	"github.com/datichb/openhub/cli/internal/fsutil"
)

// Durable queue of the claim operations of a session: what a session does to
// its claims when it ends (release, status, label) and the renewals of the
// lease that could not be pushed (touch). It lives in .git/oh-claims-queue.json
// of the team-state clone: local to the machine, never committed. FlushClaimQueue
// applies the whole queue in one commit when the remote is reachable; until
// then the operations stay in the file, so a session that ends offline (or an
// oh that is killed) loses nothing.
//
// An operation carries the conditions under which it applies: the claim must
// still belong to the member (ClaimedBy) and to the session (Session) that
// queued it. A claim that was released, taken over or moved to another
// session in the meantime is none of its business: the operation is dropped
// and logged, never applied.

// Kinds of queued operation.
const (
	// ClaimOpRelease removes the claim.
	ClaimOpRelease = "release"
	// ClaimOpStatus sets the status of the claim (Status).
	ClaimOpStatus = "status"
	// ClaimOpLabel adds a label to the claim (Label).
	ClaimOpLabel = "label"
	// ClaimOpTouch renews the lease of the claim, from the time it was queued.
	ClaimOpTouch = "touch"
	// ClaimOpAdopt ties the claim to a session that started on a ticket the
	// member already holds (AdoptClaim), when the remote could not be
	// reached. Session is the session that takes the claim, not a
	// condition. Queuing it cancels the queued operations of the other
	// sessions on the claim (see EnqueueClaimOp).
	ClaimOpAdopt = "adopt"
)

const (
	claimQueueVersion = 1
	// claimQueueMax bounds the file: a queue this long is a bug somewhere,
	// and the oldest operations are the least likely to still apply.
	claimQueueMax = 1000
	// claimQueueLockWait is how long a caller waits for the lock of the file
	// (held for a read and a write of a small file, never during git).
	claimQueueLockWait = 10 * time.Second
)

// ErrInvalidClaimOp is returned when a queued operation is incomplete.
var ErrInvalidClaimOp = errors.New("invalid claim operation")

// ClaimOp is an operation on a claim waiting in the queue.
type ClaimOp struct {
	// ID identifies the operation in the file (filled by EnqueueClaimOp).
	ID   string `json:"id"`
	Kind string `json:"kind"`
	// Project and TicketID designate the claim.
	Project  string `json:"project"`
	TicketID string `json:"ticket"`
	// ClaimedBy is the member that must still own the claim (required).
	ClaimedBy string `json:"claimed_by"`
	// Session, when set, is the session that must still work on the claim.
	// A claim without session does not match an operation that has one.
	// For a ClaimOpAdopt it is the session that takes the claim (required).
	Session string `json:"session,omitempty"`
	// Status is the new status of a ClaimOpStatus.
	Status string `json:"status,omitempty"`
	// AnyTransition lets a ClaimOpStatus skip the transition rules (a ticket
	// closed elsewhere: in_progress → done), like UpdateClaimStatusFromTracker.
	AnyTransition bool `json:"any_transition,omitempty"`
	// Label is the label of a ClaimOpLabel.
	Label string `json:"label,omitempty"`
	// QueuedAt is when the operation was queued; a touch renews the lease
	// from this time.
	QueuedAt time.Time `json:"queued_at"`
}

// claimQueueFile is the content of .git/oh-claims-queue.json.
type claimQueueFile struct {
	Version int       `json:"version"`
	Ops     []ClaimOp `json:"ops"`
}

// ClaimQueueCorruptError reports a queue file that cannot be read. The file
// is set aside (SetAside) and not deleted: its operations are not applied,
// but they are not lost either.
type ClaimQueueCorruptError struct {
	Path     string
	SetAside string
	Cause    error
}

func (e *ClaimQueueCorruptError) Error() string {
	msg := fmt.Sprintf("claims queue %s is unreadable (%v)", e.Path, e.Cause)
	if e.SetAside != "" {
		msg += fmt.Sprintf(": set aside as %s, its operations were not applied", e.SetAside)
	}
	return msg
}

func (e *ClaimQueueCorruptError) Unwrap() error { return e.Cause }

// ErrClaimQueueNewer is returned when the queue was written by a newer oh.
var ErrClaimQueueNewer = errors.New("claims queue written by a newer oh")

func (r *Repo) claimQueuePath() string {
	return filepath.Join(r.path, ".git", "oh-claims-queue.json")
}

// lockClaimQueue takes the lock of the queue file (not the lock of the
// clone: the queue is read and written while git is busy).
func (r *Repo) lockClaimQueue(ctx context.Context) (func(), error) {
	if !r.IsCloned() {
		return nil, ErrNotCloned
	}
	lctx, cancel := context.WithTimeout(ctx, claimQueueLockWait)
	defer cancel()
	unlock, err := filelock.LockContext(lctx, r.claimQueuePath()+".lock")
	if err != nil {
		return nil, fmt.Errorf("locking the claims queue: %w", err)
	}
	return unlock, nil
}

// ClaimQueue returns the queued operations, oldest first. A file that cannot
// be read gives a *ClaimQueueCorruptError (nothing is moved).
func (r *Repo) ClaimQueue() ([]ClaimOp, error) {
	return r.readClaimQueue(false)
}

// readClaimQueue reads the file; setAside renames an unreadable one so that
// the next write does not overwrite it. The caller holds the queue lock when
// it is going to write.
func (r *Repo) readClaimQueue(setAside bool) ([]ClaimOp, error) {
	path := r.claimQueuePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var f claimQueueFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, r.corruptClaimQueue(path, err, setAside)
	}
	switch {
	case f.Version > claimQueueVersion:
		return nil, fmt.Errorf("%w: version %d, this oh reads %d (%s)", ErrClaimQueueNewer, f.Version, claimQueueVersion, path)
	case f.Version != claimQueueVersion:
		return nil, r.corruptClaimQueue(path, fmt.Errorf("version %d", f.Version), setAside)
	}
	return f.Ops, nil
}

func (r *Repo) corruptClaimQueue(path string, cause error, setAside bool) error {
	e := &ClaimQueueCorruptError{Path: path, Cause: cause}
	if setAside {
		dest := fmt.Sprintf("%s.corrupt-%s", path, time.Now().UTC().Format("20060102T150405.000Z"))
		if err := os.Rename(path, dest); err != nil {
			e.Cause = fmt.Errorf("%w; and it could not be set aside: %v", cause, err)
		} else {
			e.SetAside = dest
		}
		slog.Warn("teamstate.claims_queue.corrupt", "path", path, "set_aside", e.SetAside, "error", cause)
	}
	return e
}

func (r *Repo) writeClaimQueue(ops []ClaimOp) error {
	path := r.claimQueuePath()
	if len(ops) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := json.MarshalIndent(claimQueueFile{Version: claimQueueVersion, Ops: ops}, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, append(data, '\n'), 0o600)
}

func (op *ClaimOp) validate() error {
	switch op.Kind {
	case ClaimOpRelease, ClaimOpTouch:
	case ClaimOpAdopt:
		if op.Session == "" {
			return fmt.Errorf("%w: adopt operation without session", ErrInvalidClaimOp)
		}
	case ClaimOpStatus:
		if op.Status == "" {
			return fmt.Errorf("%w: status operation without status", ErrInvalidClaimOp)
		}
	case ClaimOpLabel:
		if op.Label == "" {
			return fmt.Errorf("%w: label operation without label", ErrInvalidClaimOp)
		}
	default:
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidClaimOp, op.Kind)
	}
	if _, err := SafeName(op.Project); err != nil {
		return fmt.Errorf("%w: project: %v", ErrInvalidClaimOp, err)
	}
	if _, err := SafeName(op.TicketID); err != nil {
		return fmt.Errorf("%w: ticket: %v", ErrInvalidClaimOp, err)
	}
	if op.ClaimedBy == "" {
		return fmt.Errorf("%w: claimed_by is required (an operation without owner would apply to any claim)", ErrInvalidClaimOp)
	}
	return nil
}

// sameClaim is true for two operations on the same claim of the same owner.
func (op *ClaimOp) sameClaim(o *ClaimOp) bool {
	return op.Project == o.Project && op.TicketID == o.TicketID && op.ClaimedBy == o.ClaimedBy
}

// sameTarget is true for two operations of the same kind on the same claim
// of the same owner and session.
func (op *ClaimOp) sameTarget(o *ClaimOp) bool {
	return op.Kind == o.Kind && op.Project == o.Project && op.TicketID == o.TicketID &&
		op.ClaimedBy == o.ClaimedBy && op.Session == o.Session
}

// EnqueueClaimOp adds an operation to the queue. A touch replaces the queued
// touch of the same claim (only the latest matters); any other operation
// identical to a queued one is not added twice. It does not wait for git
// and works offline. An unreadable queue file is set aside (logged) and a
// new queue is started, so that the operation is not refused.
//
// A claim that a session is going to take over (ClaimOpAdopt) is no longer
// the business of the other sessions: queuing the adoption cancels the
// queued operations of every other session on the claim (they were decided
// for a claim the new session now works on, and replayed they would release
// it), and an operation of another session queued after an adoption is not
// added. Both are logged.
func (r *Repo) EnqueueClaimOp(op ClaimOp) error {
	if err := op.validate(); err != nil {
		return err
	}
	unlock, err := r.lockClaimQueue(context.Background())
	if err != nil {
		return err
	}
	defer unlock()
	ops, err := r.readClaimQueue(true)
	var corrupt *ClaimQueueCorruptError
	if err != nil && !errors.As(err, &corrupt) {
		return err
	}
	if op.QueuedAt.IsZero() {
		op.QueuedAt = time.Now().UTC()
	}
	if op.ID == "" {
		op.ID = newClaimOpID()
	}
	kept := ops[:0:0]
	for _, o := range ops {
		switch {
		case op.Kind == ClaimOpAdopt && op.sameClaim(&o) && o.Session != op.Session:
			slog.Info("teamstate.claims_queue.superseded", "op", o.Kind, "project", o.Project, "ticket", o.TicketID, "session", o.Session, "adopted_by", op.Session)
		case op.Kind != ClaimOpAdopt && o.Kind == ClaimOpAdopt && op.sameClaim(&o) && o.Session != op.Session:
			slog.Info("teamstate.claims_queue.not_queued", "op", op.Kind, "project", op.Project, "ticket", op.TicketID, "session", op.Session, "adopted_by", o.Session)
			return nil
		case op.Kind == ClaimOpTouch && op.sameTarget(&o):
			// replaced by the new one
		case op.sameTarget(&o) && op.Status == o.Status && op.Label == o.Label && op.AnyTransition == o.AnyTransition:
			return nil // already queued
		default:
			kept = append(kept, o)
		}
	}
	if len(kept) >= claimQueueMax {
		return fmt.Errorf("claims queue full (%d operations): %s", len(kept), r.claimQueuePath())
	}
	return r.writeClaimQueue(append(kept, op))
}

func newClaimOpID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// FlushClaimQueue applies the queued operations, in order, in a single
// commit on top of the latest remote state (one pull, one push), then empties
// the queue. applied counts the operations whose conditions held (including
// those that found the claim already in the wanted state), dropped the ones
// that were abandoned: claim gone, owner or session changed, status not
// allowed, unknown kind. Each dropped operation is logged.
//
// It is idempotent: an operation replayed after a crash between the push and
// the emptying of the file changes nothing. When the remote cannot be reached
// (or the push fails), the operations stay in the file untouched and the
// error is returned; operations queued while it ran are kept too. An
// unreadable queue file is set aside and reported as *ClaimQueueCorruptError.
func (r *Repo) FlushClaimQueue(ctx context.Context) (applied, dropped int, err error) {
	unlock, err := r.lockClaimQueue(ctx)
	if err != nil {
		return 0, 0, err
	}
	ops, err := r.readClaimQueue(true)
	unlock()
	if err != nil || len(ops) == 0 {
		return 0, 0, err
	}

	err = r.Transact(ctx, func(_ context.Context, tx *Tx) (TxResult, error) {
		applied, dropped = 0, 0 // the transaction may run again
		cfg, cfgErr := tx.Config()
		if cfgErr != nil {
			cfg = &TeamConfig{}
		}
		var changed int
		owners := map[string]bool{}
		for i := range ops {
			ok, didChange, reason := r.applyClaimOp(tx, cfg, &ops[i])
			if !ok {
				dropped++
				slog.Warn("teamstate.claims_queue.dropped", "op", ops[i].Kind, "project", ops[i].Project,
					"ticket", ops[i].TicketID, "member", ops[i].ClaimedBy, "session", ops[i].Session, "reason", reason)
				continue
			}
			applied++
			if didChange {
				changed++
				owners[ops[i].ClaimedBy] = true
			}
		}
		who := "several members"
		if len(owners) == 1 {
			for m := range owners {
				who = m
			}
		}
		return TxResult{Message: fmt.Sprintf("claims-queue: %s applies %d operation(s)", who, changed)}, nil
	})
	if err != nil {
		return 0, 0, err
	}

	// Remove what was processed; what was queued meanwhile stays.
	unlock, lockErr := r.lockClaimQueue(ctx)
	if lockErr != nil {
		return applied, dropped, fmt.Errorf("claims applied, queue not emptied (they will be replayed without effect): %w", lockErr)
	}
	defer unlock()
	current, readErr := r.readClaimQueue(true)
	var corrupt *ClaimQueueCorruptError
	if readErr != nil && !errors.As(readErr, &corrupt) {
		return applied, dropped, readErr
	}
	done := make(map[string]bool, len(ops))
	for _, o := range ops {
		done[o.ID] = true
	}
	remaining := slices.DeleteFunc(slices.Clone(current), func(o ClaimOp) bool { return done[o.ID] })
	if err := r.writeClaimQueue(remaining); err != nil {
		return applied, dropped, fmt.Errorf("claims applied, queue not emptied (they will be replayed without effect): %w", err)
	}
	return applied, dropped, nil
}

// applyClaimOp applies op in the transaction. ok: the conditions held;
// changed: a file was written or removed; reason: why it was dropped.
func (r *Repo) applyClaimOp(tx *Tx, cfg *TeamConfig, op *ClaimOp) (ok, changed bool, reason string) {
	if err := op.validate(); err != nil {
		return false, false, err.Error()
	}
	c, err := r.getClaim(op.Project, op.TicketID)
	switch {
	case errors.Is(err, ErrClaimNotFound):
		return false, false, "claim gone"
	case err != nil:
		return false, false, err.Error()
	}
	if c.ClaimedBy != op.ClaimedBy {
		return false, false, fmt.Sprintf("claim now owned by %q", c.ClaimedBy)
	}
	if op.Kind != ClaimOpAdopt && op.Session != "" && c.Session != op.Session {
		return false, false, fmt.Sprintf("claim now on session %q", c.Session)
	}
	rel := r.claimRelPath(op.Project, op.TicketID)

	write := func() (bool, bool, string) {
		data, err := marshalClaim(c)
		if err != nil {
			return false, false, "marshaling claim: " + err.Error()
		}
		if err := tx.WriteFile(rel, data); err != nil {
			return false, false, "writing claim: " + err.Error()
		}
		return true, true, ""
	}

	switch op.Kind {
	case ClaimOpRelease:
		if err := tx.Remove(rel); err != nil {
			return false, false, "removing claim: " + err.Error()
		}
		slog.Info("teamstate.claim.release", "project", op.Project, "ticket", op.TicketID, "queued", true)
		return true, true, ""

	case ClaimOpStatus:
		statuses := r.boardStatuses
		if len(statuses) == 0 {
			statuses = cfg.Board.AllStatuses()
		}
		if !IsValidBoardStatus(op.Status, statuses) {
			return false, false, fmt.Sprintf("%v: %q", ErrInvalidStatus, op.Status)
		}
		if c.Status == op.Status {
			return true, false, ""
		}
		if !op.AnyTransition && !IsValidBoardTransition(c.Status, op.Status) {
			return false, false, fmt.Sprintf("%v: from %q to %q", ErrInvalidTransition, c.Status, op.Status)
		}
		c.Status = op.Status
		c.LastActivity = InstantOf(time.Now())
		return write()

	case ClaimOpLabel:
		if slices.Contains(c.Labels, op.Label) {
			return true, false, ""
		}
		c.Labels = append(c.Labels, op.Label)
		c.LastActivity = InstantOf(time.Now())
		return write()

	case ClaimOpAdopt:
		if !adoptInto(c, cfg, op.Session, time.Now().UTC()) {
			return true, false, ""
		}
		slog.Info("teamstate.claim.adopt", "project", op.Project, "ticket", op.TicketID, "member", op.ClaimedBy, "session", op.Session, "queued", true)
		return write()

	case ClaimOpTouch:
		if !leaseApplies(c, cfg) {
			return false, false, "no lease on this claim"
		}
		at := op.QueuedAt
		if at.IsZero() {
			at = time.Now()
		}
		if end := c.LeaseUntil.Time(); !end.IsZero() && end.Sub(at) > cfg.Claim.LeaseDuration()/2 {
			return true, false, "" // still long, or renewed since
		}
		c.LeaseUntil = leaseEnd(at, cfg)
		return write()
	}
	return false, false, "unknown kind " + op.Kind
}

// HeartbeatResult is what one beat of the claims heartbeat did.
type HeartbeatResult struct {
	// Touched are the claims whose lease was renewed in the commit.
	Touched []ClaimRef
	// Queued is the number of renewals put in the queue because the commit
	// failed (remote unreachable, clone busy).
	Queued int
	// Applied and Dropped are the result of FlushClaimQueue.
	Applied, Dropped int
}

// Heartbeat is the signal of life of the claims of a session: it renews
// their leases (TouchClaims, one commit, none while the leases are long)
// then applies the queue (FlushClaimQueue). When the renewal fails, it is
// queued (one touch per claim) to be replayed by a later beat, and the queue
// is not flushed in the same breath if the cause is the network or a busy
// clone. The errors are the ones of the two steps, joined.
func (r *Repo) Heartbeat(ctx context.Context, refs []ClaimRef, member, session string, now time.Time) (HeartbeatResult, error) {
	var res HeartbeatResult
	var errs []error

	touched, err := r.TouchClaims(ctx, refs, member, session, now)
	res.Touched = touched
	if err != nil {
		errs = append(errs, err)
		if ctx.Err() == nil && !errors.Is(err, ErrNotCloned) {
			res.Queued = r.queueTouches(refs, member, session, now)
		}
		if errors.Is(err, ErrOffline) || errors.Is(err, ErrBusy) {
			return res, errors.Join(errs...)
		}
	}

	applied, dropped, err := r.FlushClaimQueue(ctx)
	res.Applied, res.Dropped = applied, dropped
	if err != nil {
		errs = append(errs, err)
	}
	return res, errors.Join(errs...)
}

// queueTouches queues a touch for each claim that TouchClaims would have
// renewed; it returns how many were queued. Without member nothing is
// queued: a touch needs the owner it is conditioned on.
func (r *Repo) queueTouches(refs []ClaimRef, member, session string, now time.Time) int {
	if member == "" {
		return 0
	}
	r.mu.RLock()
	due, _ := r.leasesToRenew(refs, member, session, now)
	r.mu.RUnlock()
	n := 0
	for _, c := range due {
		op := ClaimOp{
			Kind: ClaimOpTouch, Project: c.Project, TicketID: c.TicketID,
			ClaimedBy: c.ClaimedBy, Session: c.Session, QueuedAt: now.UTC(),
		}
		if err := r.EnqueueClaimOp(op); err != nil {
			slog.Warn("teamstate.claims_queue.enqueue_failed", "ticket", c.TicketID, "error", err)
			continue
		}
		n++
	}
	return n
}

// QueuedAdoption is the session a queued adoption is going to give the claim
// to ("" and false when none is queued).
func (r *Repo) QueuedAdoption(project, ticketID, member string) (string, bool) {
	ops, err := r.ClaimQueue()
	if err != nil {
		return "", false
	}
	probe := ClaimOp{Project: project, TicketID: ticketID, ClaimedBy: member}
	session, ok := "", false
	for i := range ops {
		if ops[i].Kind == ClaimOpAdopt && ops[i].sameClaim(&probe) {
			session, ok = ops[i].Session, true
		}
	}
	return session, ok
}

// QueueAdoptClaim queues the adoption of the claim the member holds by a
// session (AdoptClaim for a remote that cannot be reached), after checking
// in the clone that the claim exists and is the member's. queued is false,
// without error, when the claim is somebody else's; a claim that is not in
// the clone gives ErrClaimNotFound.
func (r *Repo) QueueAdoptClaim(project, ticketID, member, session string) (queued bool, err error) {
	if member == "" || session == "" {
		return false, nil
	}
	if _, err := SafeName(project); err != nil {
		return false, fmt.Errorf("invalid project name: %w", err)
	}
	if _, err := SafeName(ticketID); err != nil {
		return false, fmt.Errorf("invalid ticket ID: %w", err)
	}
	r.mu.RLock()
	c, err := r.getClaim(project, ticketID)
	r.mu.RUnlock()
	if err != nil {
		return false, err
	}
	if c.ClaimedBy != member {
		return false, nil
	}
	op := ClaimOp{Kind: ClaimOpAdopt, Project: project, TicketID: ticketID, ClaimedBy: member, Session: session}
	if err := r.EnqueueClaimOp(op); err != nil {
		return false, err
	}
	return true, nil
}

// AdoptClaimDurable is AdoptClaim that does not lose the adoption when the
// remote cannot be reached or the clone stays busy: the adoption is queued
// (ClaimOpAdopt) and applied by the next FlushClaimQueue, before anything
// the previous session of the claim had queued (that is cancelled). queued
// tells that it was. Online, the operations the other sessions of the claim
// had queued are cancelled as well, since the claim is now on this session.
func (r *Repo) AdoptClaimDurable(ctx context.Context, project, ticketID, member, session string) (adopted, queued bool, err error) {
	adopted, err = r.AdoptClaim(ctx, project, ticketID, member, session)
	switch {
	case err == nil:
		r.supersedeClaimOps(project, ticketID, member, session)
		return adopted, false, nil
	case errors.Is(err, ErrOffline), errors.Is(err, ErrBusy), errors.Is(err, context.DeadlineExceeded):
		queued, qerr := r.QueueAdoptClaim(project, ticketID, member, session)
		if qerr != nil {
			return false, false, errors.Join(err, qerr)
		}
		return false, queued, nil
	}
	return false, false, err
}

// supersedeClaimOps removes the queued operations of the sessions other than
// session on the claim of member. The queue is left alone when none matches.
func (r *Repo) supersedeClaimOps(project, ticketID, member, session string) {
	probe := ClaimOp{Project: project, TicketID: ticketID, ClaimedBy: member}
	unlock, err := r.lockClaimQueue(context.Background())
	if err != nil {
		return
	}
	defer unlock()
	ops, err := r.readClaimQueue(false)
	if err != nil || len(ops) == 0 {
		return
	}
	kept := slices.DeleteFunc(slices.Clone(ops), func(o ClaimOp) bool {
		drop := o.sameClaim(&probe) && o.Session != session
		if drop {
			slog.Info("teamstate.claims_queue.superseded", "op", o.Kind, "project", o.Project, "ticket", o.TicketID, "session", o.Session, "adopted_by", session)
		}
		return drop
	})
	if len(kept) == len(ops) {
		return
	}
	if err := r.writeClaimQueue(kept); err != nil {
		slog.Warn("teamstate.claims_queue.supersede_failed", "project", project, "ticket", ticketID, "error", err)
	}
}
