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

// EnsureWorktreeConfig sets up the opencode configuration inside a worktree so
// it shares the deployed agents, skills and MCP servers from the main project,
// while keeping its own session state and plan files.
//
// Layout created in wtPath:
//
//	.opencode/                   ← real directory (not a symlink)
//	  agents  → relProject/.opencode/agents     (symlink)
//	  skills  → relProject/.opencode/skills     (symlink)
//	  servers → relProject/.opencode/servers    (symlink)
//	  node_modules → relProject/.opencode/node_modules (symlink, if present)
//	  dependency-graph.json → relProject/.opencode/dependency-graph.json (symlink, if present)
//	  opencode.json    (copy of main project's .opencode/opencode.json, if present)
//	  package.json     (copy of main project's .opencode/package.json, if present)
//	  bun.lock         (copy of main project's .opencode/bun.lock, if present)
//	  package-lock.json (copy of main project's .opencode/package-lock.json, if present)
//	opencode.json → relProject/opencode.json    (root-level symlink, if present)
//	.beads    → relProject/.beads               (symlink, if .beads/ exists)
//
// Using a real .opencode/ directory (instead of a full directory symlink) prevents
// OpenCode from resolving it to the same physical path as the main project, which
// would cause the runtime to apply subagent_depth constraints as if the worktree
// session were nested inside the parent project's session.
//
// If the main project has not been deployed yet (.opencode/ does not exist),
// EnsureWorktreeConfig returns ErrProjectNotDeployed so the caller can trigger
// a deploy first.
//
// The function is idempotent: existing entries (symlinks, files, or directories)
// are left untouched.
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
	if _, err := os.Stat(mainOpencode); os.IsNotExist(err) {
		return ErrProjectNotDeployed
	}

	// Ensure wtPath exists.
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		return fmt.Errorf("ensuring worktree directory: %w", err)
	}

	// ── .opencode/ ──────────────────────────────────────────────────────────
	//
	// Migrate: if .opencode is an existing symlink (old pattern), remove it so
	// we can replace it with a real directory.
	wtOpencode := filepath.Join(wtPath, ".opencode")
	if isSymlink(wtOpencode) {
		if err := os.Remove(wtOpencode); err != nil {
			return fmt.Errorf("removing legacy .opencode symlink: %w", err)
		}
	}

	// Create a real .opencode/ directory in the worktree.
	if err := os.MkdirAll(wtOpencode, 0o755); err != nil {
		return fmt.Errorf("creating .opencode directory in worktree: %w", err)
	}

	// Compute the relative path from wtOpencode to mainOpencode.
	// Symlinks placed inside wtOpencode must use this base — they are one
	// directory deeper than wtPath, so relProject cannot be reused here.
	relOpencode, err := filepath.Rel(wtOpencode, mainOpencode)
	if err != nil {
		return fmt.Errorf("computing relative path for .opencode internals: %w", err)
	}

	// Symlink sub-directories that are shared and read-only from the worktree's perspective.
	for _, subdir := range []string{"agents", "skills", "servers"} {
		src := filepath.Join(mainOpencode, subdir)
		if _, err := os.Stat(src); err != nil {
			continue // not present in main project — skip
		}
		if err := ensureSymlink(
			filepath.Join(wtOpencode, subdir),
			filepath.Join(relOpencode, subdir),
		); err != nil {
			return fmt.Errorf("symlinking .opencode/%s: %w", subdir, err)
		}
	}

	// Symlink node_modules (heavy — always shared).
	nodeModules := filepath.Join(mainOpencode, "node_modules")
	if _, err := os.Stat(nodeModules); err == nil {
		if err := ensureSymlink(
			filepath.Join(wtOpencode, "node_modules"),
			filepath.Join(relOpencode, "node_modules"),
		); err != nil {
			return fmt.Errorf("symlinking .opencode/node_modules: %w", err)
		}
	}

	// Symlink dependency-graph.json (project-level, read-only from worktree).
	depGraph := filepath.Join(mainOpencode, "dependency-graph.json")
	if _, err := os.Stat(depGraph); err == nil {
		if err := ensureSymlink(
			filepath.Join(wtOpencode, "dependency-graph.json"),
			filepath.Join(relOpencode, "dependency-graph.json"),
		); err != nil {
			return fmt.Errorf("symlinking .opencode/dependency-graph.json: %w", err)
		}
	}

	// Copy files that the worktree needs locally (opencode needs to read them
	// from the working directory, not via realpath resolution).
	for _, fname := range []string{"opencode.json", "package.json", "bun.lock", "package-lock.json"} {
		src := filepath.Join(mainOpencode, fname)
		dst := filepath.Join(wtOpencode, fname)
		if err := copyFileIfAbsent(src, dst); err != nil {
			return fmt.Errorf("copying .opencode/%s: %w", fname, err)
		}
	}

	// ── Root-level opencode.json ─────────────────────────────────────────────
	mainRootConfig := filepath.Join(projectPath, "opencode.json")
	if _, err := os.Stat(mainRootConfig); err == nil {
		if err := ensureSymlink(
			filepath.Join(wtPath, "opencode.json"),
			filepath.Join(relProject, "opencode.json"),
		); err != nil {
			return fmt.Errorf("symlinking opencode.json: %w", err)
		}
	}

	// ── .beads/ ─────────────────────────────────────────────────────────────
	// Shared ticket database — symlink so both the main project and all worktrees
	// see the same tickets.
	mainBeads := filepath.Join(projectPath, ".beads")
	if _, err := os.Stat(mainBeads); err == nil {
		if err := ensureSymlink(
			filepath.Join(wtPath, ".beads"),
			filepath.Join(relProject, ".beads"),
		); err != nil {
			return fmt.Errorf("symlinking .beads: %w", err)
		}
	}

	return nil
}

// isSymlink reports whether path is a symbolic link (does not follow the link).
func isSymlink(path string) bool {
	fi, err := os.Lstat(path)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

// copyFileIfAbsent copies src to dst only if dst does not already exist.
// It is a no-op when src does not exist or dst is already present.
func copyFileIfAbsent(src, dst string) error {
	// Skip if destination already exists (idempotent).
	if _, err := os.Lstat(dst); err == nil {
		return nil
	}
	// Skip if source does not exist.
	srcData, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.WriteFile(dst, srcData, 0o644)
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

// ResyncConfig forces a full refresh of the .opencode/ layout in an existing
// worktree. It removes any current .opencode/ entry (symlink or real directory)
// and re-creates the layout from scratch via EnsureWorktreeConfig.
//
// Use this after redeploying the main project to propagate updated
// opencode.json / package.json copies into the worktree, or to migrate
// a worktree from the legacy (full-symlink) layout to the current one.
func ResyncConfig(wtPath, projectPath string) error {
	wtOpencode := filepath.Join(wtPath, ".opencode")
	if err := os.RemoveAll(wtOpencode); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing existing .opencode: %w", err)
	}
	return EnsureWorktreeConfig(wtPath, projectPath)
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
