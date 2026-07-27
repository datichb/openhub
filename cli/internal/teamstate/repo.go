package teamstate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxPushRetries = 3
	retryDelay     = 500 * time.Millisecond
)

// Repo manages the local clone of the team-state Git repository.
type Repo struct {
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
	return fmt.Sprintf("impossible de synchroniser avec le remote (le contenu local est utilisé) : %v", w.Cause)
}

func (w *PullWarning) Unwrap() error { return w.Cause }

// IsPullWarning reports whether err is a PullWarning.
func IsPullWarning(err error) bool {
	var pw *PullWarning
	return errors.As(err, &pw)
}

// Clone performs the initial clone of the remote repository.
func (r *Repo) Clone(ctx context.Context) error {
	if r.IsCloned() {
		return nil
	}
	parent := filepath.Dir(r.path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}
	_, err := r.git(ctx, parent, "clone", r.remote, r.path)
	if err != nil {
		return fmt.Errorf("cloning team-state repo: %w", err)
	}
	return nil
}

// Pull fetches and rebases on the remote branch.
func (r *Repo) Pull(ctx context.Context) error {
	if !r.IsCloned() {
		return ErrNotCloned
	}
	_, err := r.git(ctx, r.path, "pull", "--rebase", "--autostash")
	if err != nil {
		return fmt.Errorf("pulling team-state: %w", err)
	}
	return nil
}

// Push pushes local commits to the remote.
func (r *Repo) Push(ctx context.Context) error {
	if !r.IsCloned() {
		return ErrNotCloned
	}
	_, err := r.git(ctx, r.path, "push")
	if err != nil {
		return fmt.Errorf("pushing team-state: %w", err)
	}
	return nil
}

// CommitAndPush stages the given files, commits with the message, and pushes.
// If push fails due to conflict, it retries with pull --rebase up to maxPushRetries.
//
// Pass "." as a file to stage all changes (new files, modifications, deletions).
func (r *Repo) CommitAndPush(ctx context.Context, msg string, files ...string) error {
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
		pushErr := r.Push(ctx)
		if pushErr == nil {
			return nil
		}

		// Auth / permission errors are permanent — retrying won't help.
		if isAuthError(pushErr) {
			return fmt.Errorf("push échoué — %s", authErrorMessage(r.remote))
		}

		// Last attempt — return the real push error, not a generic message.
		if attempt == maxPushRetries-1 {
			return fmt.Errorf("push échoué après %d tentatives : %w", maxPushRetries, pushErr)
		}

		// Non-fast-forward conflict — pull --rebase and retry.
		if pullErr := r.Pull(ctx); pullErr != nil {
			if isAuthError(pullErr) {
				return fmt.Errorf("pull échoué — %s", authErrorMessage(r.remote))
			}
			return fmt.Errorf("rebasing before retry: %w", pullErr)
		}

		time.Sleep(retryDelay)
	}

	return fmt.Errorf("push échoué après %d tentatives (conflit persistant)", maxPushRetries)
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
	_, err := os.Stat(filepath.Join(r.path, "config.toml"))
	return err == nil
}

// HasPolicies returns true if policies.toml exists in the team-state repo.
func (r *Repo) HasPolicies() bool {
	_, err := os.Stat(filepath.Join(r.path, "policies.toml"))
	return err == nil
}

// HasMember returns true if the given member ID exists in members.toml.
func (r *Repo) HasMember(id string) bool {
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
