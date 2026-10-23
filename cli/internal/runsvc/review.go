package runsvc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// The review of a branch other than the current one: the diff comes from
// the reviewed branch, so `read`, `grep` and `glob` must read its files
// too. The session works in a temporary worktree of the branch, detached
// (the branch is never checked out nor moved), under <hub>/worktrees. The
// session cannot write there: a read-only workflow (risk: read) gets agents
// that edit nothing and run git commands that only read.

// ReadBranchError is a branch to review that does not exist, locally or on
// a remote (nothing is fetched).
type ReadBranchError struct{ Branch string }

func (e *ReadBranchError) Error() string {
	return fmt.Sprintf("the branch to review does not exist, locally or on a remote: %s", e.Branch)
}

func (e *ReadBranchError) Unwrap() error { return worktree.ErrBranchNotFound }

// worktreesRoot is the folder of the temporary worktrees.
func (s *Service) worktreesRoot() string {
	if s.WorktreesDir != "" {
		return s.WorktreesDir
	}
	if s.ServersDir != "" {
		return worktree.TempRoot(filepath.Dir(s.ServersDir))
	}
	return filepath.Join(os.TempDir(), "oh-worktrees")
}

// readLocation is the location of a read-only session that reviews branch:
// the project directory when the branch is checked out there (nothing
// changes), else a temporary worktree to create, detached on the branch.
func (s *Service) readLocation(projectID, projectPath, branch string) (Location, *Warning, error) {
	t, err := s.readTarget(projectPath, branch)
	if err != nil {
		if errors.Is(err, worktree.ErrBranchNotFound) {
			return Location{}, nil, &ReadBranchError{Branch: branch}
		}
		return Location{}, nil, err
	}
	if t.Current {
		return Location{Path: projectPath, Kind: LocationBase}, nil, nil
	}
	path := worktree.NewTempPath(s.worktreesRoot(), projectID, branch)
	loc := Location{Path: path, Kind: LocationWorktree, Branch: branch, Create: true, Temp: true, Ref: t.Ref}
	return loc, &Warning{Code: WarnReviewWorktree, Args: []any{branch, path}}, nil
}

// ReleaseTempWorktree removes the temporary worktree of a session that is
// over (stopped): nothing else holds it, unless another open session of the
// project works there. A session of another kind of location is left
// alone. root is the folder of the temporary worktrees (worktree.TempRoot).
func ReleaseTempWorktree(ctx context.Context, root string, sessions domain.SessionStore, sess domain.Session) {
	if !worktree.IsTemp(root, sess.LaunchPath) {
		return
	}
	if sessions != nil {
		if list, err := sessions.List(ctx, sess.ProjectID); err == nil {
			for _, o := range list {
				if o.ID != sess.ID && openState(o.State) && o.LaunchPath != "" && samePath(o.LaunchPath, sess.LaunchPath) {
					return
				}
			}
		}
	}
	if err := worktree.RemoveDetached(sess.LaunchPath); err != nil {
		slog.Warn("runsvc: temporary worktree not removed", "session", sess.ID, "path", sess.LaunchPath, "error", err)
	}
}

func (s *Service) releaseTempWorktree(ctx context.Context, sess domain.Session) {
	ReleaseTempWorktree(ctx, s.worktreesRoot(), s.Sessions, sess)
}
