package teamstate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	maxPushRetries = 3
	retryDelay     = 500 * time.Millisecond
)

// Repo manages the local clone of the team-state Git repository.
//
// mu is a read-write mutex that serialises all git operations (Clone, Pull,
// Push, CommitAndPush) against concurrent filesystem reads (ListClaims,
// ListMembers, …).  Git operations acquire the write lock; read methods
// acquire the read lock so that multiple reads can proceed in parallel but
// never overlap with a rebase/checkout that could leave files in a
// transitional state.
//
// The git operations also hold the write lock of the clone (lock.go): an
// in-process lock shared by every Repo of the same path, and the file lock
// .git/oh.lock shared with the other oh processes (N15).
type Repo struct {
	mu            sync.RWMutex
	path          string   // local clone path (e.g. ~/.oh/team-state/)
	remote        string   // Git remote URL
	boardStatuses []string // custom board column IDs (set via SetBoardStatuses)
	// beforePush runs in Transact between the commit and the push (tests:
	// simulate a concurrent push).
	beforePush func()
}

// NewRepo creates a Repo instance. It does NOT clone or validate the repo.
func NewRepo(remote, localPath string) *Repo {
	return &Repo{
		remote: remote,
		path:   localPath,
	}
}

// SetBoardStatuses configures the valid board statuses for custom column support.
// When set, UpdateClaimStatus and UpdateClaimStatusFromTracker accept these
// statuses in addition to the built-in 6 defaults.
// Call this after loading the team config BoardConfig.AllStatuses().
func (r *Repo) SetBoardStatuses(statuses []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.boardStatuses = statuses
}

// Path returns the local filesystem path to the team-state repo.
func (r *Repo) Path() string {
	return r.path
}

// Remote returns the configured Git remote URL.
func (r *Repo) Remote() string {
	return r.remote
}

// IsCloned returns true if the local directory exists and is a Git repo.
func (r *Repo) IsCloned() bool {
	info, err := os.Stat(filepath.Join(r.path, ".git"))
	return err == nil && info.IsDir()
}

// EnsureReady clones the repo if absent, otherwise pulls latest changes.
// If the repo was just cloned it is already up-to-date — no pull is needed.
//
// Error classification on Pull:
//   - Auth error (missing/invalid credentials) → fatal error with actionable message
//   - Other error (network, rebase conflict)   → *PullWarning (non-fatal, local usable)
func (r *Repo) EnsureReady(ctx context.Context) error {
	if !r.IsCloned() {
		return r.Clone(ctx)
	}
	if err := r.Pull(ctx); err != nil {
		if isAuthError(err) {
			return fmt.Errorf("%s", authErrorMessage(r.remote))
		}
		return &PullWarning{Cause: err}
	}
	return nil
}

// PullWarning is returned by EnsureReady when the repo is already cloned but
// a pull fails. The local content is still usable — callers should surface
// this as a warning, not abort the operation.
type PullWarning struct {
	Cause error
}

func (w *PullWarning) Error() string {
	return fmt.Sprintf("unable to sync with remote (using local content): %v", w.Cause)
}

func (w *PullWarning) Unwrap() error { return w.Cause }

// IsPullWarning reports whether err is a PullWarning.
func IsPullWarning(err error) bool {
	var pw *PullWarning
	return errors.As(err, &pw)
}

// Clone performs the initial clone of the remote repository.
// Acquires the write lock for the duration of the git operation.
func (r *Repo) Clone(ctx context.Context) error {
	if r.activeTx(ctx) != nil {
		return nil // within a transaction the clone exists
	}
	release, err := r.lockGit(ctx)
	if err != nil {
		return err
	}
	defer release()
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.clone(ctx)
}

// clone is the internal (unlocked) clone implementation.
func (r *Repo) clone(ctx context.Context) error {
	if r.IsCloned() {
		return nil
	}
	parent := filepath.Dir(r.path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}
	slog.Debug("teamstate.clone.start", "url", r.remote, "path", r.path)
	start := time.Now()
	_, err := r.git(ctx, parent, "clone", r.remote, r.path)
	if err != nil {
		elapsed := time.Since(start)
		slog.Warn("teamstate.clone.failed", "url", r.remote, "path", r.path, "error", err, "duration", elapsed)
		if isAuthError(err) {
			return fmt.Errorf("clone failed — %s", authErrorMessage(r.remote))
		}
		return fmt.Errorf("cloning team-state repo: %w", err)
	}
	elapsed := time.Since(start)
	slog.Debug("teamstate.clone.done", "path", r.path, "duration", elapsed)
	return nil
}

// Pull fetches and rebases on the remote branch.
// Acquires the write lock so that in-flight file reads are not interrupted
// by a rebase that rewrites working-tree files.
func (r *Repo) Pull(ctx context.Context) error {
	if r.activeTx(ctx) != nil {
		return nil // the transaction pulled when it started
	}
	release, err := r.lockGit(ctx)
	if err != nil {
		return err
	}
	defer release()
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pull(ctx)
}

// pull is the internal (unlocked) pull implementation, intended to be called
// from within methods that already hold the write lock (e.g. CommitAndPush).
func (r *Repo) pull(ctx context.Context) error {
	if !r.IsCloned() {
		return ErrNotCloned
	}
	if r.localOnly(ctx) {
		return nil
	}
	start := time.Now()
	_, err := r.git(ctx, r.path, "pull", "--rebase", "--autostash")
	if err != nil {
		slog.Warn("teamstate.pull.failed", "path", r.path, "duration", time.Since(start), "error", err)
		return fmt.Errorf("pulling team-state: %w", err)
	}
	slog.Debug("teamstate.pull", "path", r.path, "duration", time.Since(start))
	return nil
}

// Push pushes local commits to the remote.
// Acquires the write lock.
func (r *Repo) Push(ctx context.Context) error {
	if r.activeTx(ctx) != nil {
		return nil // the transaction pushes when it ends
	}
	release, err := r.lockGit(ctx)
	if err != nil {
		return err
	}
	defer release()
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.push(ctx)
}

// push is the internal (unlocked) push implementation.
func (r *Repo) push(ctx context.Context) error {
	if !r.IsCloned() {
		return ErrNotCloned
	}
	if r.localOnly(ctx) {
		return nil
	}
	start := time.Now()
	_, err := r.git(ctx, r.path, "push")
	if err != nil {
		slog.Debug("teamstate.push.failed", "path", r.path, "duration", time.Since(start), "error", err)
		return fmt.Errorf("pushing team-state: %w", err)
	}
	slog.Debug("teamstate.push", "path", r.path, "duration", time.Since(start))
	return nil
}

// CommitAndPush commits files the caller already wrote, and pushes. It is a
// transaction (Transact): pull, commit, push; a rejected push is retried
// on top of the remote (the files are kept, rebased by the pull). "." is
// refused (ErrStageAll): the caller lists its files, never another
// writer's. Inside a transaction (ctx of Transact or WithWriteLock), the
// files join it.
func (r *Repo) CommitAndPush(ctx context.Context, msg string, files ...string) error {
	if err := checkFiles(files); err != nil {
		return err
	}
	if tx := r.activeTx(ctx); tx != nil {
		tx.Touch(files...)
		tx.addMessage(msg)
		return nil
	}
	return r.transact(ctx, true, func(_ context.Context, tx *Tx) (TxResult, error) {
		tx.Touch(files...)
		return TxResult{Message: msg}, nil
	})
}

// WithWriteLock runs fn as a transaction (Transact): on top of the latest
// remote state, its files committed and pushed together, fn run again when
// another member pushed first (fn recomputes from disk). fn writes files
// with the internal helpers and declares them with commitAndPush, which only
// records them. Within fn, ctx carries the transaction: the public methods
// of the repo join it (no second lock, no second pull).
func (r *Repo) WithWriteLock(ctx context.Context, fn func(ctx context.Context) error) error {
	return r.withWriteLock(ctx, fn)
}

// withWriteLock is the internal implementation.
func (r *Repo) withWriteLock(ctx context.Context, fn func(ctx context.Context) error) error {
	if r.activeTx(ctx) != nil {
		return fn(ctx)
	}
	if !r.IsCloned() {
		// Not a repository: fn runs (its validation errors first), its
		// commit gives ErrNotCloned.
		release, err := r.lockGit(ctx)
		if err != nil {
			return err
		}
		defer release()
		return fn(ctx)
	}
	return r.transact(ctx, false, func(ctx context.Context, tx *Tx) (TxResult, error) {
		return TxResult{}, fn(ctx)
	})
}

// commitAndPush declares the files of a transaction (called from
// WithWriteLock callbacks).
func (r *Repo) commitAndPush(ctx context.Context, msg string, files ...string) error {
	if err := checkFiles(files); err != nil {
		return err
	}
	tx := r.activeTx(ctx)
	if tx == nil {
		if !r.IsCloned() {
			return ErrNotCloned
		}
		return errNoTransaction
	}
	tx.Touch(files...)
	tx.addMessage(msg)
	return nil
}

// ErrStageAll: "." (or an empty list) was given as the files of a commit;
// the caller must list its own files (another writer's files would be
// committed with them, N15).
var ErrStageAll = errors.New("team-state: commit every change (\".\") is refused, list the files")

var errNoTransaction = errors.New("team-state: commit outside a transaction")

func checkFiles(files []string) error {
	for _, f := range files {
		if c := filepath.Clean(f); c == "." || c == "" || c == "/" || c == "*" {
			return ErrStageAll
		}
	}
	return nil
}

// localOnly reports whether the repo has no git remote (solo team-state):
// pull and push are then no-ops and commits stay local. It reads
// .git/config directly (no git process on the pull/push path).
func (r *Repo) localOnly(ctx context.Context) bool {
	if data, err := os.ReadFile(filepath.Join(r.path, ".git", "config")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "[remote ") {
				return false
			}
		}
		return true
	}
	out, err := r.git(ctx, r.path, "remote")
	return err == nil && strings.TrimSpace(out) == ""
}

// structureDirs are the folders of the layout of a team-state (each one holds
// a .gitkeep so that git tracks it empty). catalog/ is the catalogue of team
// bricks (ADR-054), shared like the rest.
var structureDirs = []string{
	"projects",
	"wiki",
	"wiki/.pending",
	"reports",
	"workflows/published",
	"workflows/drafts",
	"workflows/prompts",
	"workflows/history",
	"catalog/agents",
	"catalog/skills",
}

// InitStructure creates the base directory structure in the repo if missing.
// This is called after clone to ensure the expected layout exists.
func (r *Repo) InitStructure(ctx context.Context) error {
	for _, d := range structureDirs {
		full := filepath.Join(r.path, d)
		if err := os.MkdirAll(full, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", d, err)
		}
		// Add .gitkeep for empty directories
		gitkeep := filepath.Join(full, ".gitkeep")
		if _, err := os.Stat(gitkeep); os.IsNotExist(err) {
			if err := os.WriteFile(gitkeep, nil, 0o644); err != nil {
				return fmt.Errorf("creating .gitkeep in %s: %w", d, err)
			}
		}
	}
	return nil
}

// EnsureStructure is InitStructure followed by the commit of the .gitkeep
// files that git does not track yet (one commit "team: init structure", pushed
// with the other writes of the team-state): the folders of the layout, the
// catalogue of bricks catalog/ (ADR-054) included, are part of what the team
// shares and must not stay untracked in the clone (A5). It does nothing more
// when everything is tracked. A repository that is not a clone only gets the
// folders; a commit that cannot be made (offline, lock busy) is logged and
// left for the next call, the layout being on disk anyway.
func (r *Repo) EnsureStructure(ctx context.Context) error {
	if err := r.InitStructure(ctx); err != nil {
		return err
	}
	if !r.IsCloned() {
		return nil
	}
	missing := r.untrackedKeeps(ctx)
	if len(missing) == 0 {
		return nil
	}
	if err := r.CommitAndPush(ctx, "team: init structure", missing...); err != nil {
		slog.Warn("teamstate.structure.commit_failed", "error", err)
	}
	return nil
}

// untrackedKeeps lists the .gitkeep files of the layout that git does not
// track (paths relative to the repository).
func (r *Repo) untrackedKeeps(ctx context.Context) []string {
	var keeps []string
	for _, d := range structureDirs {
		keeps = append(keeps, filepath.ToSlash(filepath.Join(d, ".gitkeep")))
	}
	out, err := r.git(ctx, r.path, append([]string{"ls-files", "-z", "--"}, keeps...)...)
	if err != nil {
		return nil
	}
	tracked := map[string]bool{}
	for _, f := range strings.Split(out, "\x00") {
		tracked[f] = true
	}
	var missing []string
	for _, k := range keeps {
		if !tracked[k] {
			missing = append(missing, k)
		}
	}
	return missing
}

// HasConfig returns true if config.toml exists in the team-state repo.
func (r *Repo) HasConfig() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, err := os.Stat(filepath.Join(r.path, "config.toml"))
	return err == nil
}

// HasPolicies returns true if policies.toml exists in the team-state repo.
func (r *Repo) HasPolicies() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, err := os.Stat(filepath.Join(r.path, "policies.toml"))
	return err == nil
}

// HasMember returns true if the given member ID exists in members.toml.
func (r *Repo) HasMember(id string) bool {
	// ListMembers acquires the read lock itself.
	members, err := r.ListMembers()
	if err != nil {
		return false
	}
	for _, m := range members {
		if m.ID == id {
			return true
		}
	}
	return false
}

// git executes a git command and returns the combined output.
// GIT_TERMINAL_PROMPT=0 prevents git from blocking on interactive credential
// prompts when the process has no TTY (e.g. running as a TUI background task).
// GIT_SSH_COMMAND with BatchMode=yes makes SSH fail immediately instead of
// waiting for a passphrase or host-key confirmation.
func (r *Repo) git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_SSH_COMMAND=ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15 -o ServerAliveInterval=15 -o ServerAliveCountMax=2",
		// A stalled HTTP transfer gives up instead of holding the lock.
		"GIT_HTTP_LOW_SPEED_LIMIT=1000",
		"GIT_HTTP_LOW_SPEED_TIME=30",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return string(out), nil
}
