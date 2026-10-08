package gitutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FileChange is a file changed since a commit.
type FileChange struct {
	File      string
	Status    string // added | modified | deleted
	Additions int
	Deletions int
}

// WorkDiff is what a working tree changed since a commit.
type WorkDiff struct {
	Files []FileChange
	Patch string
}

// ErrUnknownRef is returned when the starting commit is not in the
// repository any more (garbage collected).
var ErrUnknownRef = errors.New("unknown starting commit")

// DiffSince returns the changes of the working tree at dir since ref:
// commits made since, uncommitted changes and untracked files (not ignored).
// The index, the branches and the working tree are not touched: the
// untracked files are added to a temporary copy of the index.
func DiffSince(dir, ref string) (*WorkDiff, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := gitOut(ctx, dir, nil, "rev-parse", "--verify", "-q", ref+"^{commit}"); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownRef, ref)
	}
	index, err := gitOut(ctx, dir, nil, "rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp("", "oh-index-*")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if data, err := os.ReadFile(strings.TrimSpace(index)); err == nil {
		_, _ = tmp.Write(data)
	}
	_ = tmp.Close()
	env := []string{"GIT_INDEX_FILE=" + tmpPath}
	if _, err := gitOut(ctx, dir, env, "add", "-A", "--", "."); err != nil {
		return nil, err
	}
	return diff(ctx, dir, env, "--cached", ref)
}

// diff reads the numstat, the statuses and the patch of `git diff args…`.
func diff(ctx context.Context, dir string, env []string, args ...string) (*WorkDiff, error) {
	run := func(mode ...string) (string, error) {
		full := append(append([]string{"diff", "--no-renames"}, mode...), args...)
		return gitOut(ctx, dir, env, append(full, "--")...)
	}
	numstat, err := run("--numstat")
	if err != nil {
		return nil, err
	}
	names, err := run("--name-status")
	if err != nil {
		return nil, err
	}
	patch, err := run()
	if err != nil {
		return nil, err
	}
	status := map[string]string{}
	for _, l := range strings.Split(names, "\n") {
		if code, file, ok := strings.Cut(l, "\t"); ok {
			status[file] = statusName(code)
		}
	}
	out := &WorkDiff{Patch: patch}
	for _, l := range strings.Split(numstat, "\n") {
		parts := strings.SplitN(l, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		add, _ := strconv.Atoi(parts[0]) // "-" for a binary file
		del, _ := strconv.Atoi(parts[1])
		out.Files = append(out.Files, FileChange{File: parts[2], Status: status[parts[2]], Additions: add, Deletions: del})
	}
	return out, nil
}

// RefBefore returns the last commit of rev ("" = HEAD) made before t (""
// when none): the starting point of a session that did not record one.
func RefBefore(dir, rev string, t time.Time) string {
	if rev == "" {
		rev = "HEAD"
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	out, err := gitOut(ctx, dir, nil, "rev-list", "-1", "--before="+strconv.FormatInt(t.Unix(), 10), rev, "--")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// DiffRange returns the committed changes from ref to the branch to (the
// session branch is no longer checked out in its directory).
func DiffRange(dir, ref, to string) (*WorkDiff, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := gitOut(ctx, dir, nil, "rev-parse", "--verify", "-q", ref+"^{commit}"); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownRef, ref)
	}
	if _, err := gitOut(ctx, dir, nil, "rev-parse", "--verify", "-q", "refs/heads/"+to); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownRef, to)
	}
	return diff(ctx, dir, nil, ref, to)
}

// BaseRef returns the commit a branch started from: the merge-base of HEAD
// with the base branch of the repository ("" when unknown).
func BaseRef(dir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	out, err := gitOut(ctx, dir, nil, "merge-base", DetectBaseBranch(dir, ""), "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func statusName(code string) string {
	switch {
	case strings.HasPrefix(code, "A"):
		return "added"
	case strings.HasPrefix(code, "D"):
		return "deleted"
	}
	return "modified"
}

func gitOut(ctx context.Context, dir string, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = filepath.Clean(dir)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return string(out), nil
}
