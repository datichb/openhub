package teamstate

import (
	"context"
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
func (r *Repo) EnsureReady(ctx context.Context) error {
	if !r.IsCloned() {
		return r.Clone(ctx)
	}
	return r.Pull(ctx)
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
func (r *Repo) CommitAndPush(ctx context.Context, msg string, files ...string) error {
	if !r.IsCloned() {
		return ErrNotCloned
	}

	// Stage files
	args := append([]string{"add"}, files...)
	if _, err := r.git(ctx, r.path, args...); err != nil {
		return fmt.Errorf("staging files: %w", err)
	}

	// Check if there's anything to commit
	status, _ := r.git(ctx, r.path, "status", "--porcelain")
	if strings.TrimSpace(status) == "" {
		return nil // nothing to commit
	}

	// Commit
	if _, err := r.git(ctx, r.path, "commit", "-m", msg); err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	// Push with retry on conflict
	for attempt := range maxPushRetries {
		err := r.Push(ctx)
		if err == nil {
			return nil
		}

		// If last attempt, fail
		if attempt == maxPushRetries-1 {
			return ErrSyncConflict
		}

		// Rebase and retry
		if pullErr := r.Pull(ctx); pullErr != nil {
			return fmt.Errorf("rebasing before retry: %w", pullErr)
		}

		time.Sleep(retryDelay)
	}

	return ErrSyncConflict
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

// git executes a git command and returns the combined output.
func (r *Repo) git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return string(out), nil
}
