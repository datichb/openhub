package teamstate

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// A team-state repository that was moved or renamed on the server keeps its
// clone: only the URL changes. VerifyRemote checks that the new URL holds the
// same team-state (the history of the clone is part of it), SetRemote points
// the clone at it.

// ErrUnrelatedRemote: the repository does not share the history of the clone —
// it is another team-state, not the same one moved.
var ErrUnrelatedRemote = errors.New("the repository does not share the history of this team-state")

// ErrEmptyRemote: the repository has no commit to compare with.
var ErrEmptyRemote = errors.New("the repository is empty")

// VerifyRemote fetches the head of remote (nothing of the clone changes but
// FETCH_HEAD) and checks that it shares a history with the clone. The access is
// the one oh holds for remote (SetCredentialSource); a refusal is an *AuthError.
// A clone without commit has nothing to compare: it passes.
func (r *Repo) VerifyRemote(ctx context.Context, remote string) error {
	if r.noPath {
		return ErrNoStatePath
	}
	if !r.IsCloned() {
		return ErrNotCloned
	}
	release, err := r.lockGit(ctx)
	if err != nil {
		return err
	}
	defer release()
	if !r.hasCommit(ctx) {
		return nil
	}
	cmd, cleanup := gitCommand(ctx, remote, r.path, nil,
		"-o StrictHostKeyChecking=accept-new -o ConnectTimeout=15", nil,
		"fetch", "--no-tags", "--quiet", "--", remote, "HEAD")
	defer cleanup()
	if out, err := cmd.CombinedOutput(); err != nil {
		wrapped := fmt.Errorf("git fetch: %s: %w", strings.TrimSpace(string(out)), err)
		if authErr := accessError(remote, wrapped); authErr != nil {
			return authErr
		}
		if strings.Contains(string(out), "couldn't find remote ref") {
			return ErrEmptyRemote
		}
		return classifyRemoteError(remote, wrapped)
	}
	if _, err := r.git(ctx, r.path, "merge-base", "HEAD", "FETCH_HEAD"); err != nil {
		return ErrUnrelatedRemote
	}
	return nil
}

// SetRemote points the origin of the clone at remote and makes it the remote of
// the repo.
func (r *Repo) SetRemote(ctx context.Context, remote string) error {
	if r.noPath {
		return ErrNoStatePath
	}
	if !r.IsCloned() {
		return ErrNotCloned
	}
	release, err := r.lockGit(ctx)
	if err != nil {
		return err
	}
	defer release()
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.git(ctx, r.path, "remote", "set-url", "origin", remote); err != nil {
		return fmt.Errorf("changing the remote: %w", err)
	}
	r.remote = remote
	return nil
}
