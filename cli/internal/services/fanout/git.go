package fanout

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// Git operations of a fan-out: only oh runs them (the agents of a fan-out
// are refused merges, pushes and the branches of the fan-out).

// BranchPrefix starts every branch of a fan-out (bundle.FanoutRoleRules
// refuses them to the agents).
const BranchPrefix = "oh/fanout/"

// IntegrationBranch is the integration branch of the fan-out of parent.
func IntegrationBranch(parent string) string { return BranchPrefix + shortID(parent) + "/integration" }

// ChildBranch is the branch of the child of ticket.
func ChildBranch(parent, ticket string) string {
	return BranchPrefix + shortID(parent) + "/" + worktree.Slug(ticket)
}

func shortID(id string) string {
	id = strings.TrimPrefix(id, "ses_")
	if len(id) > 10 {
		id = id[len(id)-10:]
	}
	return worktree.Slug(strings.ToLower(id))
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := gitutil.Command(ctx, dir, args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		return out.String(), fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), msg, err)
	}
	return out.String(), nil
}

// Head returns the commit of dir.
func Head(ctx context.Context, dir string) (string, error) {
	out, err := git(ctx, dir, "rev-parse", "--verify", "HEAD")
	return strings.TrimSpace(out), err
}

// BranchHead returns the commit of a branch of the repository of dir.
func BranchHead(ctx context.Context, dir, branch string) (string, error) {
	out, err := git(ctx, dir, "rev-parse", "--verify", "refs/heads/"+branch)
	return strings.TrimSpace(out), err
}

// AddWorktree creates branch at from and its worktree, a sibling directory
// of the project (reused when it already exists on that branch).
func AddWorktree(ctx context.Context, project, branch, from string) (string, error) {
	path := worktree.SiblingPath(project, branch)
	if fi, err := os.Stat(filepath.Join(path, ".git")); err == nil && !fi.IsDir() {
		if cur, err := git(ctx, path, "symbolic-ref", "--short", "HEAD"); err == nil && strings.TrimSpace(cur) == branch {
			return path, nil
		}
		return "", fmt.Errorf("%s exists and is not the worktree of %s", path, branch)
	}
	if _, err := git(ctx, project, "worktree", "add", "-b", branch, path, from); err != nil {
		return "", err
	}
	return path, nil
}

// RemoveWorktree removes a worktree of the fan-out (its branch is kept).
func RemoveWorktree(ctx context.Context, project, path string) error {
	_, err := git(ctx, project, "worktree", "remove", "--force", path)
	return err
}

// CommitsAhead counts the commits of branch that from does not have.
func CommitsAhead(ctx context.Context, dir, from, branch string) (int, error) {
	out, err := git(ctx, dir, "rev-list", "--count", from+".."+branch)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(out))
}

// Dirty reports uncommitted changes in dir outside .beads.
func Dirty(ctx context.Context, dir string) (bool, error) {
	out, err := git(ctx, dir, "status", "--porcelain", "--", ".", ":(exclude).beads")
	return strings.TrimSpace(out) != "", err
}

// ErrConflict is a merge stopped on conflicts (Merge leaves it in progress).
var ErrConflict = errors.New("merge conflict")

// Merge merges branch into the branch checked out in dir, always with a
// merge commit. On conflicts the merge is left in progress and the
// conflicted files are returned with ErrConflict.
func Merge(ctx context.Context, dir, branch, message string) (conflicts []string, err error) {
	if _, err := git(ctx, dir, "merge", "--no-ff", "--no-edit", "-m", message, branch); err != nil {
		files := ConflictedFiles(ctx, dir)
		if len(files) > 0 || MergeInProgress(ctx, dir) {
			return files, ErrConflict
		}
		return nil, err
	}
	return nil, nil
}

// FastForward merges branch into the branch checked out in dir, fast
// forward when it can, else with a merge commit; a conflict is aborted.
func FastForward(ctx context.Context, dir, branch, message string) error {
	if _, err := git(ctx, dir, "merge", "--ff-only", branch); err == nil {
		return nil
	}
	if _, err := Merge(ctx, dir, branch, message); err != nil {
		AbortMerge(ctx, dir)
		return err
	}
	return nil
}

// ConflictedFiles lists the files left in conflict in dir.
func ConflictedFiles(ctx context.Context, dir string) []string {
	out, err := git(ctx, dir, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil
	}
	return strings.Fields(out)
}

// MergeInProgress reports a merge waiting for its commit in dir.
func MergeInProgress(ctx context.Context, dir string) bool {
	_, err := git(ctx, dir, "rev-parse", "-q", "--verify", "MERGE_HEAD")
	return err == nil
}

// AbortMerge abandons the merge in progress in dir.
func AbortMerge(ctx context.Context, dir string) {
	if MergeInProgress(ctx, dir) {
		_, _ = git(ctx, dir, "merge", "--abort")
	}
}

// ResetHard puts the branch of dir back on commit (the integration branch,
// after a merger left it in a wrong state).
func ResetHard(ctx context.Context, dir, commit string) error {
	AbortMerge(ctx, dir)
	_, err := git(ctx, dir, "reset", "--hard", "-q", commit)
	return err
}

// IsAncestor reports whether a is an ancestor of b.
func IsAncestor(ctx context.Context, dir, a, b string) bool {
	_, err := git(ctx, dir, "merge-base", "--is-ancestor", a, b)
	return err == nil
}

// Parents counts the parents of commit.
func Parents(ctx context.Context, dir, commit string) int {
	out, err := git(ctx, dir, "rev-list", "--parents", "-n", "1", commit)
	if err != nil {
		return 0
	}
	return len(strings.Fields(out)) - 1
}

// conflictMarker starts a line left by a conflict.
var conflictMarkers = [][]byte{[]byte("<<<<<<< "), []byte(">>>>>>> "), []byte("=======\n")}

// HasMarkers reports whether one of files (relative to dir) still holds
// conflict markers at the start of a line.
func HasMarkers(dir string, files []string) bool {
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f)))
		if err != nil {
			continue // removed while resolving
		}
		for _, line := range bytes.SplitAfter(data, []byte("\n")) {
			for _, m := range conflictMarkers {
				if bytes.HasPrefix(line, m) {
					return true
				}
			}
		}
	}
	return false
}
