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
type Repo struct {
	mu     sync.RWMutex
	path   string // local clone path (e.g. ~/.oh/team-state/)
	remote string // Git remote URL
}

// NewRepo creates a Repo instance. It does NOT clone or validate the repo.
func NewRepo(remote, localPath string) *Repo {
	return &Repo{
		remote: remote,
		path:   localPath,
	}
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
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.push(ctx)
}

// push is the internal (unlocked) push implementation.
func (r *Repo) push(ctx context.Context) error {
	if !r.IsCloned() {
		return ErrNotCloned
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

// CommitAndPush stages the given files, commits with the message, and pushes.
// If push fails due to conflict, it retries with pull --rebase up to maxPushRetries.
// After a successful push it performs a best-effort pull to pick up any commits
// pushed concurrently by teammates.
//
// Pass "." as a file to stage all changes (new files, modifications, deletions).
//
// Acquires the write lock for the entire operation.
func (r *Repo) CommitAndPush(ctx context.Context, msg string, files ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.commitAndPush(ctx, msg, files...)
}

// withWriteLock executes fn while holding the write lock for the entire
// pull → mutate → commitAndPush cycle. This eliminates the race window that
// exists when Pull() and CommitAndPush() are called as separate locked operations.
//
// fn receives the context and may call internal unlocked helpers:
// r.getClaim, r.commitAndPush, r.pull, etc. It MUST NOT call the public
// locked methods (Pull, CommitAndPush) as that would deadlock.
//
// A best-effort pull is performed before fn to ensure the working tree is fresh.
// ErrNotCloned from pull is silently ignored (repo may be used locally without remote).
func (r *Repo) withWriteLock(ctx context.Context, fn func(ctx context.Context) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Best-effort pull to ensure fresh working tree.
	if err := r.pull(ctx); err != nil && !errors.Is(err, ErrNotCloned) {
		return err
	}

	return fn(ctx)
}

// commitAndPush is the internal (unlocked) implementation of CommitAndPush.
func (r *Repo) commitAndPush(ctx context.Context, msg string, files ...string) error {
	if !r.IsCloned() {
		return ErrNotCloned
	}

	// Stage files
	args := append([]string{"add"}, files...)
	if _, err := r.git(ctx, r.path, args...); err != nil {
		return fmt.Errorf("staging files: %w", err)
	}

	// Check if the index has anything staged.
	// git diff --cached --quiet exits 0 if nothing is staged, 1 if there are staged changes.
	// We use this instead of "git status --porcelain" because status includes untracked
	// files which are NOT staged — causing a false "there is something to commit" result.
	_, err := r.git(ctx, r.path, "diff", "--cached", "--quiet")
	if err == nil {
		// Exit code 0 → index is clean, nothing staged → nothing to commit
		return nil
	}
	// Any error from diff --cached means there are staged changes → proceed to commit

	// Commit
	if _, err := r.git(ctx, r.path, "commit", "-m", msg); err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	// Push with retry on conflict
	for attempt := range maxPushRetries {
		pushErr := r.push(ctx)
		if pushErr == nil {
			// Best-effort pull to pick up concurrent commits from teammates.
			// Errors are intentionally ignored: the local state is already consistent
			// after our successful push, and a failed pull is non-critical here.
			_ = r.pull(ctx)
			slog.Debug("teamstate.commitAndPush", "msg", msg, "path", r.path)
			return nil
		}

		// Auth / permission errors are permanent — retrying won't help.
		if isAuthError(pushErr) {
			return fmt.Errorf("push échoué — %s", authErrorMessage(r.remote))
		}

		// Last attempt — return the real push error, not a generic message.
		if attempt == maxPushRetries-1 {
			slog.Warn("teamstate.push.exhausted", "attempts", maxPushRetries, "path", r.path, "error", pushErr)
			return fmt.Errorf("push échoué après %d tentatives : %w", maxPushRetries, pushErr)
		}

		slog.Warn("teamstate.push.retry", "attempt", attempt+1, "path", r.path, "error", pushErr)

		// Non-fast-forward conflict — pull --rebase and retry.
		if pullErr := r.pull(ctx); pullErr != nil {
			if isAuthError(pullErr) {
				return fmt.Errorf("pull échoué — %s", authErrorMessage(r.remote))
			}
			return fmt.Errorf("rebasing before retry: %w", pullErr)
		}

		time.Sleep(retryDelay * time.Duration(1<<uint(attempt)))
	}

	return fmt.Errorf("%w: push échoué après %d tentatives (conflit persistant)", ErrSyncConflict, maxPushRetries)
}

// InitStructure creates the base directory structure in the repo if missing.
// This is called after clone to ensure the expected layout exists.
func (r *Repo) InitStructure(ctx context.Context) error {
	dirs := []string{
		"projects",
		"wiki",
		"wiki/.pending",
		"reports",
	}
	for _, d := range dirs {
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
		"GIT_SSH_COMMAND=ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return string(out), nil
}
