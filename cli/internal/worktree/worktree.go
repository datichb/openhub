// Package worktree provides git worktree management as reusable utilities.
// Worktrees are created as sibling directories (../reponame-slug) to avoid
// impacting the main project directory.
package worktree

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Entry represents a git worktree.
type Entry struct {
	Path   string
	Branch string
	Head   string
	IsBare bool
}

// CleanupResult holds the outcome of a CleanupMerged operation.
type CleanupResult struct {
	Removed []string // branches whose worktrees were successfully removed
	Skipped []string // branches that are merged but have uncommitted changes (only when force=false)
}

// Slug converts a branch name to a filesystem-safe string.
// Example: "feat/bd-42" → "feat-bd-42"
func Slug(branch string) string {
	slug := strings.NewReplacer("/", "-", " ", "-").Replace(branch)
	// Condense consecutive dashes
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	return slug
}

// SiblingPath returns the expected sibling directory for a branch worktree.
// Given projectPath=/home/user/myrepo and branch="feat/login",
// returns "/home/user/myrepo-feat-login".
func SiblingPath(projectPath, branch string) string {
	parentDir := filepath.Dir(projectPath)
	repoName := filepath.Base(projectPath)
	return filepath.Join(parentDir, repoName+"-"+Slug(branch))
}

// DetectBaseBranch returns the default branch for the given repo.
// Checks: symbolic-ref of origin/HEAD, then falls back to checking
// if "main" or "master" exist.
func DetectBaseBranch(projectPath string) string {
	// Try symbolic-ref
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = projectPath
	if out, err := cmd.Output(); err == nil {
		ref := strings.TrimSpace(string(out))
		// refs/remotes/origin/main → main
		parts := strings.Split(ref, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	// Fallback: check if "main" branch exists
	cmd = exec.Command("git", "rev-parse", "--verify", "refs/heads/main")
	cmd.Dir = projectPath
	if err := cmd.Run(); err == nil {
		return "main"
	}

	// Fallback: check if "master" branch exists
	cmd = exec.Command("git", "rev-parse", "--verify", "refs/heads/master")
	cmd.Dir = projectPath
	if err := cmd.Run(); err == nil {
		return "master"
	}

	return "main" // default
}

// ExistsOnRemote checks whether a branch exists on the remote (origin).
// Returns true if git ls-remote reports the branch ref.
func ExistsOnRemote(projectPath, branch string) bool {
	cmd := exec.Command("git", "ls-remote", "--heads", "origin", branch)
	cmd.Dir = projectPath
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// ResolveOrCreate returns the absolute path of a worktree for the given branch.
// If the worktree already exists (locally registered), it returns the existing path.
// Otherwise, it creates it as a sibling directory.
//
// Branch resolution order:
//  1. Worktree already registered locally → reuse
//  2. Branch exists on remote (origin) → fetch + checkout
//  3. Branch exists locally → create worktree on it
//  4. Neither → create new local branch
func ResolveOrCreate(projectPath, branch string) (string, error) {
	// Validate branch name to prevent git flag injection or path traversal
	if strings.HasPrefix(branch, "-") {
		return "", fmt.Errorf("invalid branch name (cannot start with '-'): %q", branch)
	}
	if strings.Contains(branch, "..") {
		return "", fmt.Errorf("invalid branch name (cannot contain '..'): %q", branch)
	}
	if branch == "" {
		return "", fmt.Errorf("branch name cannot be empty")
	}

	wtPath := SiblingPath(projectPath, branch)

	// Check if worktree already exists at expected path
	if info, err := os.Stat(wtPath); err == nil && info.IsDir() {
		// Verify it's actually a git worktree
		gitDir := filepath.Join(wtPath, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return wtPath, nil // reuse existing
		}
	}

	// Also check if worktree is registered for this branch (different path)
	entries, err := List(projectPath)
	if err == nil {
		for _, e := range entries {
			if e.Branch == branch {
				return e.Path, nil
			}
		}
	}

	// Check if the branch exists on remote — if so, fetch it first so the
	// local worktree tracks the remote work instead of creating a diverging branch.
	if ExistsOnRemote(projectPath, branch) {
		fetchCmd := exec.Command("git", "fetch", "origin", branch+":"+branch)
		fetchCmd.Dir = projectPath
		_ = fetchCmd.Run() // best-effort; if fetch fails we fall through to local creation

		cmd := exec.Command("git", "worktree", "add", wtPath, branch)
		cmd.Dir = projectPath
		if out, err := cmd.CombinedOutput(); err == nil {
			return wtPath, nil
		} else {
			return "", fmt.Errorf("git worktree add (remote branch): %s", strings.TrimSpace(string(out)))
		}
	}

	// Create worktree — try new branch first
	cmd := exec.Command("git", "worktree", "add", "-b", branch, wtPath)
	cmd.Dir = projectPath
	if _, err := cmd.CombinedOutput(); err == nil {
		return wtPath, nil
	}

	// Fallback: branch already exists locally, just create worktree for it
	cmd = exec.Command("git", "worktree", "add", wtPath, branch)
	cmd.Dir = projectPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git worktree add: %s", strings.TrimSpace(string(out)))
	}

	return wtPath, nil
}

// IsMerged checks whether a branch is fully merged into baseBranch.
// Uses `git branch --merged <baseBranch>` and checks if the target branch
// appears in the output.
func IsMerged(projectPath, branch, baseBranch string) (bool, error) {
	cmd := exec.Command("git", "branch", "--merged", baseBranch)
	cmd.Dir = projectPath
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git branch --merged %s: %w", baseBranch, err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Lines look like "  branch-name" or "* current-branch"
		line = strings.TrimPrefix(line, "* ")
		if line == branch {
			return true, nil
		}
	}
	return false, nil
}

// CleanupMerged removes all worktrees whose branches are merged into baseBranch.
//
// When force is false (safe default), worktrees with uncommitted changes are
// skipped and reported in CleanupResult.Skipped. When force is true, removal
// is forced regardless of dirty state (--force flag on git worktree remove).
//
// Returns a CleanupResult with removed and skipped branch names.
func CleanupMerged(projectPath, baseBranch string, force bool) (CleanupResult, error) {
	var result CleanupResult

	entries, err := List(projectPath)
	if err != nil {
		return result, err
	}

	// Get the main worktree path to skip it
	mainPath, _ := filepath.Abs(projectPath)

	for _, e := range entries {
		// Skip the main worktree and bare entries
		if e.IsBare || e.Branch == "" {
			continue
		}
		entryAbs, _ := filepath.Abs(e.Path)
		if entryAbs == mainPath {
			continue
		}
		// Skip the base branch itself
		if e.Branch == baseBranch {
			continue
		}

		merged, err := IsMerged(projectPath, e.Branch, baseBranch)
		if err != nil {
			continue // skip on error
		}
		if !merged {
			continue
		}

		if err := Remove(projectPath, e.Path, force); err != nil {
			// Without --force, a dirty worktree will fail here — report as skipped.
			result.Skipped = append(result.Skipped, e.Branch)
			continue
		}
		result.Removed = append(result.Removed, e.Branch)
	}

	// Prune stale worktree metadata
	_ = Prune(projectPath)

	return result, nil
}

// Prune removes stale worktree administrative metadata (git worktree prune).
func Prune(projectPath string) error {
	cmd := exec.Command("git", "worktree", "prune")
	cmd.Dir = projectPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree prune: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// List returns all worktrees for the given project.
func List(projectPath string) ([]Entry, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = projectPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	var entries []Entry
	var current Entry

	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			if current.Path != "" {
				entries = append(entries, current)
			}
			current = Entry{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "HEAD "):
			current.Head = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			// refs/heads/main → main
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "bare":
			current.IsBare = true
		}
	}
	if current.Path != "" {
		entries = append(entries, current)
	}

	return entries, nil
}

// CurrentBranch returns the current branch name for the given directory.
// If HEAD is detached, it returns "(detached) <short-sha>".
// Returns an empty string and error if not a git repo or git fails.
func CurrentBranch(dir string) (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}

	// Detached HEAD — return short SHA
	cmd = exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = dir
	out, err = cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repo or git failed: %w", err)
	}
	return "(detached) " + strings.TrimSpace(string(out)), nil
}

// IsGitRepo checks if the given path is inside a git repository.
func IsGitRepo(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path
	return cmd.Run() == nil
}

// Remove removes a worktree by path, with optional force.
func Remove(projectPath, wtPath string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, "--", wtPath)
	cmd := exec.Command("git", args...)
	cmd.Dir = projectPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// BranchName formats a ticket ID using the given pattern (e.g. "feat/%s" → "feat/BD-42").
// If pattern is empty or does not contain "%s", falls back to "feat/<ticketID>".
func BranchName(pattern, ticketID string) string {
	if pattern == "" || !strings.Contains(pattern, "%s") {
		return "feat/" + ticketID
	}
	return fmt.Sprintf(pattern, ticketID)
}

// EnsureWorktreeConfig creates symlinks inside wtPath so that it shares the
// hub configuration of the main project at projectPath. This mirrors the
// git worktree pattern: rather than copying or deploying separately, the
// worktree points back to the parent's deployed config.
//
// Symlinks created (all relative):
//
//	<wtPath>/.opencode    → ../<repo>/.opencode
//	<wtPath>/opencode.json → ../<repo>/opencode.json
//
// If the main project has not been deployed yet (.opencode/ does not exist),
// EnsureWorktreeConfig returns ErrProjectNotDeployed so the caller can trigger
// a deploy first.
//
// Existing symlinks or directories are left untouched (idempotent).
var ErrProjectNotDeployed = fmt.Errorf("project has not been deployed yet (.opencode/ missing in main project)")

func EnsureWorktreeConfig(wtPath, projectPath string) error {
	// Compute the relative path from wtPath to projectPath.
	// Worktrees are always siblings: ../reponame relative to wtPath.
	relProject, err := filepath.Rel(wtPath, projectPath)
	if err != nil {
		return fmt.Errorf("computing relative path: %w", err)
	}

	// Check the main project has been deployed.
	mainOpencode := filepath.Join(projectPath, ".opencode")
	mainConfig := filepath.Join(projectPath, "opencode.json")

	if _, err := os.Stat(mainOpencode); os.IsNotExist(err) {
		return ErrProjectNotDeployed
	}

	// Ensure wtPath exists.
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		return fmt.Errorf("ensuring worktree directory: %w", err)
	}

	// Symlink .opencode/
	if err := ensureSymlink(
		filepath.Join(wtPath, ".opencode"),
		filepath.Join(relProject, ".opencode"),
	); err != nil {
		return fmt.Errorf("symlinking .opencode: %w", err)
	}

	// Symlink opencode.json (only if it exists in the main project)
	if _, err := os.Stat(mainConfig); err == nil {
		if err := ensureSymlink(
			filepath.Join(wtPath, "opencode.json"),
			filepath.Join(relProject, "opencode.json"),
		); err != nil {
			return fmt.Errorf("symlinking opencode.json: %w", err)
		}
	}

	return nil
}

// ensureSymlink creates a symlink at linkPath pointing to target if it does
// not already exist. Existing symlinks (even with different targets) are left
// untouched to avoid disrupting intentional overrides.
func ensureSymlink(linkPath, target string) error {
	if _, err := os.Lstat(linkPath); err == nil {
		return nil // already exists (file, dir, or symlink) — leave it
	}
	return os.Symlink(target, linkPath)
}

// BulkCreate creates multiple worktrees sequentially (to avoid .git/index.lock contention).
// Returns a slice of worktree paths in the same order as the input branches.
func BulkCreate(projectPath string, branches []string) ([]string, error) {
	paths := make([]string, 0, len(branches))
	for _, branch := range branches {
		wtPath, err := ResolveOrCreate(projectPath, branch)
		if err != nil {
			return paths, fmt.Errorf("creating worktree for branch %q: %w", branch, err)
		}
		paths = append(paths, wtPath)
	}
	return paths, nil
}
