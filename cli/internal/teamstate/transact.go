package teamstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/workflow"
)

// Publication transactions (O14): a change that depends on the latest state
// of the team-state (next version number, revalidation) is rebuilt from
// scratch when another member pushed first, instead of being rebased.

var (
	// ErrOffline is returned when the team-state remote cannot be reached:
	// nothing is committed, the caller may queue the operation.
	ErrOffline = errors.New("team-state remote unreachable")
	// ErrPushRejected is returned when every attempt was overtaken by a
	// concurrent push.
	ErrPushRejected = errors.New("team-state push rejected after retries")
)

// maxTxAttempts bounds the rebuild loop of Transact.
const maxTxAttempts = 4

// Tx gives a Transact build function access to the repo while the write
// lock is held (the public read methods would deadlock).
type Tx struct {
	r       *Repo
	written map[string]bool // repo-relative paths written or removed
}

// Path is the repo root.
func (tx *Tx) Path() string { return tx.r.path }

// Repo is the repo of the transaction; only lock-free methods and the
// helpers of Tx may be used from the build function.
func (tx *Tx) Repo() *Repo { return tx.r }

// ReadFile reads a repo-relative file.
func (tx *Tx) ReadFile(rel string) ([]byte, error) {
	return os.ReadFile(filepath.Join(tx.r.path, rel))
}

// WriteFile writes a repo-relative file (parents created) and records it.
func (tx *Tx) WriteFile(rel string, data []byte) error {
	tx.written[rel] = true
	return writeRel(tx.r.path, rel, data)
}

// Remove deletes a repo-relative file (missing is fine) and records it.
func (tx *Tx) Remove(rel string) error {
	err := os.Remove(filepath.Join(tx.r.path, rel))
	switch {
	case err == nil:
		tx.written[rel] = true
	case !os.IsNotExist(err):
		return err
	}
	return nil
}

// Touch records files changed by other helpers (lock, events).
func (tx *Tx) Touch(rels ...string) {
	for _, r := range rels {
		tx.written[r] = true
	}
}

// Lock reads workflows.lock.
func (tx *Tx) Lock() (*WorkflowLock, error) { return tx.r.readWorkflowLock() }

// WriteLock writes workflows.lock.
func (tx *Tx) WriteLock(l *WorkflowLock) error {
	tx.Touch(WorkflowsLockFile)
	return tx.r.WriteWorkflowLockLocal(l)
}

// Seal is SealWorkflowLocal within the transaction.
func (tx *Tx) Seal(scope WorkflowScope, id string, meta LockMeta) (LockEntry, error) {
	e, files, err := tx.r.SealWorkflowLocal(scope, id, meta)
	tx.Touch(files...)
	return e, err
}

// LoadWorkflowScopes loads the published workflows of scopes (integrity
// checked) into cat.
func (tx *Tx) LoadWorkflowScopes(cat *workflow.MemCatalog, scopes ...WorkflowScope) workflow.Diagnostics {
	lock, err := tx.r.readWorkflowLock()
	if err != nil {
		return workflow.Diagnostics{integrityDiag(DiagWorkflowLockInvalid, filepath.Join(tx.r.path, WorkflowsLockFile), err)}
	}
	var diags workflow.Diagnostics
	for _, s := range scopes {
		diags = append(diags, tx.r.loadPublished(cat, s, lock)...)
	}
	return diags
}

// CheckPublish is Repo.CheckPublish within the transaction.
func (tx *Tx) CheckPublish(member string) error {
	cfg, err := tx.r.loadConfig()
	if err != nil {
		return err
	}
	mf, err := tx.r.readMembersFile()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return cfg.Governance.CanPublish(member, func(id string) bool {
		if mf == nil {
			return false
		}
		_, ok := mf.Members[id]
		return ok
	})
}

// AppendEvent writes an event (committed with the transaction).
func (tx *Tx) AppendEvent(e Event) error {
	rel, err := tx.r.appendEventLocal(e)
	if err != nil {
		return err
	}
	tx.Touch(rel)
	return nil
}

// TxResult is what a Transact build function returns.
type TxResult struct {
	Message string // commit message
}

// Transact runs build under the write lock on top of the latest remote
// state: pull → build → commit → push. When the push is rejected (another
// member pushed first), the local commit is dropped and the whole cycle
// runs again, so build must recompute everything from disk (version
// numbers, validation). A build error restores the files it touched.
//
// A remote that cannot be reached gives ErrOffline with nothing committed.
// On a repo without remote (solo), the commit stays local.
func (r *Repo) Transact(ctx context.Context, build func(ctx context.Context, tx *Tx) (TxResult, error)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.IsCloned() {
		return ErrNotCloned
	}
	for attempt := range maxTxAttempts {
		if err := r.pull(ctx); err != nil {
			return classifyRemoteError(r.remote, err)
		}
		tx := &Tx{r: r, written: map[string]bool{}}
		res, err := build(ctx, tx)
		if err != nil {
			r.restore(ctx, tx)
			return err
		}
		files := tx.files()
		if len(files) == 0 {
			return nil
		}
		if _, err := r.git(ctx, r.path, append([]string{"add", "-A", "--"}, files...)...); err != nil {
			r.restore(ctx, tx)
			return fmt.Errorf("staging files: %w", err)
		}
		if _, err := r.git(ctx, r.path, "diff", "--cached", "--quiet"); err == nil {
			return nil // nothing changed
		}
		if _, err := r.git(ctx, r.path, "commit", "-m", res.Message); err != nil {
			_, _ = r.git(ctx, r.path, "reset", "-q", "--", ".")
			r.restore(ctx, tx)
			return fmt.Errorf("committing: %w", err)
		}
		if r.beforePush != nil {
			r.beforePush()
		}
		pushErr := r.push(ctx)
		if pushErr == nil {
			return nil
		}
		// Drop our commit whatever the reason: it is rebuilt or queued.
		if _, err := r.git(ctx, r.path, "reset", "-q", "--hard", "HEAD~1"); err != nil {
			return fmt.Errorf("dropping the local commit after %v: %w", pushErr, err)
		}
		if !isRejectedPush(pushErr) {
			return classifyRemoteError(r.remote, pushErr)
		}
		slog.Debug("teamstate.transact.retry", "attempt", attempt+1, "error", pushErr)
		time.Sleep(retryDelay * time.Duration(attempt+1))
	}
	return ErrPushRejected
}

func (tx *Tx) files() []string {
	out := make([]string, 0, len(tx.written))
	for f := range tx.written {
		out = append(out, f)
	}
	return out
}

// restore puts back the files a failed build touched: tracked files from
// HEAD, new files removed.
func (r *Repo) restore(ctx context.Context, tx *Tx) {
	for f := range tx.written {
		if _, err := r.git(ctx, r.path, "cat-file", "-e", "HEAD:"+filepath.ToSlash(f)); err == nil {
			_, _ = r.git(ctx, r.path, "checkout", "HEAD", "--", f)
		} else {
			_ = os.Remove(filepath.Join(r.path, f))
		}
	}
}

func isRejectedPush(err error) bool {
	msg := strings.ToLower(err.Error())
	for _, p := range []string{"rejected", "non-fast-forward", "fetch first", "failed to push some refs"} {
		if strings.Contains(msg, p) && !isNetworkError(err) {
			return true
		}
	}
	return false
}

// isNetworkError reports a remote that cannot be reached (no network, host
// down), as opposed to an authentication or repository error.
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, p := range []string{
		"could not resolve host", "connection refused", "network is unreachable",
		"no route to host", "timed out", "temporary failure in name resolution",
		"failed to connect", "couldn't connect", "connection reset", "name or service not known",
	} {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}

func classifyRemoteError(remote string, err error) error {
	switch {
	case isAuthError(err):
		return fmt.Errorf("%s", authErrorMessage(remote))
	case isNetworkError(err):
		return fmt.Errorf("%w: %v", ErrOffline, err)
	}
	return err
}

// ---------------------------------------------------------------------------
// Offline queue
// ---------------------------------------------------------------------------

// QueuedOp is a team-state operation waiting for the network (O14). It is
// replayed through the whole cycle (revalidation, next version).
type QueuedOp struct {
	Kind     string    `json:"kind"` // publish | restore | archive
	Scope    string    `json:"scope"`
	ID       string    `json:"id"`
	Member   string    `json:"member"`
	Message  string    `json:"message,omitempty"`
	Version  int       `json:"version,omitempty"` // restore
	QueuedAt time.Time `json:"queued_at"`
}

// queuePath keeps the queue inside .git: local to the clone, never pushed.
func (r *Repo) queuePath() string { return filepath.Join(r.path, ".git", "oh-workflow-queue.json") }

// Queue returns the queued operations, oldest first.
func (r *Repo) Queue() ([]QueuedOp, error) {
	data, err := os.ReadFile(r.queuePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ops []QueuedOp
	if err := json.Unmarshal(data, &ops); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", r.queuePath(), err)
	}
	return ops, nil
}

// Enqueue adds op, replacing a queued operation on the same workflow.
func (r *Repo) Enqueue(op QueuedOp) error {
	ops, err := r.Queue()
	if err != nil {
		return err
	}
	if op.QueuedAt.IsZero() {
		op.QueuedAt = time.Now().UTC()
	}
	kept := ops[:0]
	for _, o := range ops {
		if o.Scope != op.Scope || o.ID != op.ID {
			kept = append(kept, o)
		}
	}
	return r.writeQueue(append(kept, op))
}

// Dequeue removes the queued operation on scope/id.
func (r *Repo) Dequeue(scope, id string) error {
	ops, err := r.Queue()
	if err != nil {
		return err
	}
	kept := ops[:0]
	for _, o := range ops {
		if o.Scope != scope || o.ID != id {
			kept = append(kept, o)
		}
	}
	return r.writeQueue(kept)
}

func (r *Repo) writeQueue(ops []QueuedOp) error {
	if len(ops) == 0 {
		if err := os.Remove(r.queuePath()); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := json.MarshalIndent(ops, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.queuePath(), data, 0o644)
}

// ParseScope parses WorkflowScope.String() ("team", "project:<p>").
func ParseScope(s string) (WorkflowScope, error) {
	if s == "team" {
		return TeamScope(), nil
	}
	if p, ok := strings.CutPrefix(s, "project:"); ok && p != "" {
		sc := ProjectScope(p)
		return sc, sc.check()
	}
	return WorkflowScope{}, fmt.Errorf("invalid workflow scope %q", s)
}
