package worktree

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/gitutil"
)

// Temporary worktrees: a detached checkout of a branch the session reads
// without working on it (the review of a branch other than the current
// one). The branch itself is never checked out, moved or deleted.

// ErrBranchNotFound is returned when a branch exists neither locally nor as
// a remote-tracking branch.
var ErrBranchNotFound = errors.New("branch not found")

// ReadTarget is the branch a session reads: the git reference to check out
// (refs/heads/<branch>, else refs/remotes/<remote>/<branch>) and whether
// the branch is the one checked out at the project path.
type ReadTarget struct {
	Ref     string
	Current bool
}

// Name is the reference without its refs/heads/ or refs/remotes/ prefix:
// the branch, or <remote>/<branch> when only the remote has it.
func (t ReadTarget) Name() string {
	return strings.TrimPrefix(strings.TrimPrefix(t.Ref, "refs/heads/"), "refs/remotes/")
}

// ResolveReadTarget finds the branch to read in the repository of
// projectPath: the local branch, else the remote-tracking branch of origin,
// else of another remote (nothing is fetched). ErrBranchNotFound when there
// is none.
func ResolveReadTarget(projectPath, branch string) (ReadTarget, error) {
	if branch == "" || strings.HasPrefix(branch, "-") || strings.Contains(branch, "..") {
		return ReadTarget{}, fmt.Errorf("%w: %q", ErrBranchNotFound, branch)
	}
	exists := func(ref string) bool {
		return gitutil.Command(context.Background(), projectPath, "rev-parse", "--verify", "-q", "--end-of-options", ref+"^{commit}").Run() == nil
	}
	if ref := "refs/heads/" + branch; exists(ref) {
		cur, err := CurrentBranch(projectPath)
		return ReadTarget{Ref: ref, Current: err == nil && cur == branch}, nil
	}
	out, _ := gitutil.Command(context.Background(), projectPath, "remote").Output()
	remotes := strings.Fields(string(out))
	sort.SliceStable(remotes, func(i, j int) bool { return remotes[i] == "origin" && remotes[j] != "origin" })
	for _, r := range remotes {
		if ref := "refs/remotes/" + r + "/" + branch; exists(ref) {
			return ReadTarget{Ref: ref}, nil
		}
	}
	return ReadTarget{}, fmt.Errorf("%w: %q", ErrBranchNotFound, branch)
}

// AddDetached creates a worktree of the repository of projectPath at wtPath
// with a detached HEAD on ref: no branch is created or checked out.
func AddDetached(projectPath, wtPath, ref string) error {
	if strings.HasPrefix(ref, "-") {
		return fmt.Errorf("invalid reference %q", ref)
	}
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		return err
	}
	out, err := gitutil.Command(context.Background(), projectPath, "worktree", "add", "--detach", "--", wtPath, ref).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add --detach: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// RepoOf returns the main working tree of the repository a linked worktree
// belongs to, read from its .git file ("" when wtPath is not a linked
// worktree or its repository is gone).
func RepoOf(wtPath string) string {
	raw, err := os.ReadFile(filepath.Join(wtPath, ".git"))
	if err != nil {
		return ""
	}
	gitdir, ok := strings.CutPrefix(strings.TrimSpace(string(raw)), "gitdir:")
	if !ok {
		return ""
	}
	gitdir = strings.TrimSpace(gitdir)
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(wtPath, gitdir)
	}
	// <repo>/.git/worktrees/<name>
	common := filepath.Dir(filepath.Dir(filepath.Clean(gitdir)))
	if filepath.Base(common) != ".git" {
		return ""
	}
	repo := filepath.Dir(common)
	if st, err := os.Stat(common); err != nil || !st.IsDir() {
		return ""
	}
	return repo
}

// RemoveDetached removes a temporary worktree: through git when its
// repository is still there (forced: nothing in it is worth keeping), then
// the folder itself, and the administrative files of the repository are
// pruned.
func RemoveDetached(wtPath string) error {
	repo := RepoOf(wtPath)
	if repo != "" {
		_ = Remove(repo, wtPath, true) // the folder is removed below anyway
	}
	err := os.RemoveAll(wtPath)
	if repo != "" {
		_ = Prune(repo)
	}
	return err
}

// Temporary worktrees live in <hub>/worktrees/<project>/<branch>-<id>, one
// folder per launch: the end of a session removes its own, and oh gc the
// ones a killed session left.

// TempRoot is the folder of the temporary worktrees of a hub directory.
func TempRoot(hubDir string) string { return filepath.Join(hubDir, "worktrees") }

// NewTempPath returns a new path under root (TempRoot) for the temporary
// worktree of a branch of a project (nothing is created).
func NewTempPath(root, projectID, branch string) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	name := Slug(branch)
	if name == "" {
		name = "branch"
	}
	project := Slug(projectID)
	if project == "" {
		project = "project"
	}
	return filepath.Join(root, project, name+"-"+hex.EncodeToString(b[:]))
}

// IsTemp reports whether path is a temporary worktree of root: a folder
// two levels below it (<project>/<worktree>), never root or a project
// folder itself.
func IsTemp(root, path string) bool {
	if root == "" || path == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return false
	}
	return len(strings.Split(rel, string(filepath.Separator))) == 2
}
