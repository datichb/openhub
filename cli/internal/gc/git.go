package gc

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/datichb/openhub/cli/internal/worktree"
)

// Git is what the clean-up asks git about the worktrees.
type Git interface {
	// Removable reports a worktree of repo that can go: clean (untracked
	// files included), not locked, every commit of its branch reachable from
	// another branch (local or remote).
	Removable(ctx context.Context, repo, path, branch string) (ok bool, why string)
	// Remove removes the worktree (never forced), then its branch when it
	// is merged into base.
	Remove(ctx context.Context, repo, path, branch, base string) error
}

type realGit struct{}

func gitOut(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(cmd.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func (realGit) Removable(ctx context.Context, repo, path, branch string) (ok bool, why string) {
	if out, err := gitOut(ctx, path, "status", "--porcelain", "--untracked-files=all"); err != nil || out != "" {
		return false, "dirty"
	}
	list, err := gitOut(ctx, repo, "worktree", "list", "--porcelain")
	if err != nil {
		return false, "unknown"
	}
	found := false
	want := ResolvedPath(path)
	for _, block := range strings.Split(list, "\n\n") {
		lines := strings.Split(block, "\n")
		if len(lines) == 0 || ResolvedPath(strings.TrimPrefix(lines[0], "worktree ")) != want {
			continue
		}
		found = true
		for _, l := range lines[1:] {
			if l == "locked" || strings.HasPrefix(l, "locked ") {
				return false, "locked"
			}
		}
	}
	if !found {
		return false, "not a worktree"
	}
	// Commits of the worktree found on no other branch (local or remote).
	own, err := gitOut(ctx, path, "rev-list", "HEAD", "--not", "--exclude="+branch, "--branches", "--remotes")
	if err != nil || own != "" {
		return false, "unpushed"
	}
	return true, ""
}

func (realGit) Remove(ctx context.Context, repo, path, branch, base string) error {
	if err := worktree.Remove(repo, path, false); err != nil {
		return err
	}
	if base == "" {
		base = worktree.DetectBaseBranch(repo)
	}
	if strings.HasPrefix(branch, "-") {
		return nil
	}
	if merged, err := worktree.IsMerged(repo, branch, base); err == nil && merged {
		if out, err := exec.CommandContext(ctx, "git", "-C", repo, "branch", "-d", branch).CombinedOutput(); err != nil {
			return fmt.Errorf("deleting branch %s: %s", branch, strings.TrimSpace(string(out)))
		}
	}
	return nil
}
