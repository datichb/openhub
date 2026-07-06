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

// ResolveOrCreate returns the absolute path of a worktree for the given branch.
// If the worktree already exists, it returns the existing path.
// Otherwise, it creates it as a sibling directory.
// It tries creating a new branch (-b) first, then falls back to using an existing branch.
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

	// Create worktree — try new branch first
	cmd := exec.Command("git", "worktree", "add", "-b", branch, wtPath)
	cmd.Dir = projectPath
	if _, err := cmd.CombinedOutput(); err == nil {
		return wtPath, nil
	}

	// Fallback: branch already exists, just create worktree for it
	cmd = exec.Command("git", "worktree", "add", wtPath, branch)
	cmd.Dir = projectPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git worktree add: %s", strings.TrimSpace(string(out)))
	}

	return wtPath, nil
}

// EnsureExclude ensures that sibling worktree directories don't pollute
// the main repo's git status. Since we use sibling dirs, this is a no-op
// in practice (siblings are outside the project tree). Kept for API
// completeness and future use.
func EnsureExclude(projectPath string) error {
	// Sibling directories are outside the project tree, so they don't
	// appear in `git status`. No .git/info/exclude manipulation needed.
	// This function exists for semantic clarity and future extensibility.
	_ = projectPath
	return nil
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
// Returns the list of removed branch names.
func CleanupMerged(projectPath, baseBranch string) ([]string, error) {
	entries, err := List(projectPath)
	if err != nil {
		return nil, err
	}

	// Get the main worktree path to skip it
	mainPath, _ := filepath.Abs(projectPath)

	var removed []string
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

		// Remove the worktree
		cmd := exec.Command("git", "worktree", "remove", "--force", e.Path)
		cmd.Dir = projectPath
		if err := cmd.Run(); err != nil {
			continue // skip failed removals
		}
		removed = append(removed, e.Branch)
	}

	// Prune stale worktree metadata
	cmd := exec.Command("git", "worktree", "prune")
	cmd.Dir = projectPath
	_ = cmd.Run()

	return removed, nil
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
