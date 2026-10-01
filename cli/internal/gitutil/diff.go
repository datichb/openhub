// Package gitutil provides lightweight Git helpers for policies and other CLI features.
package gitutil

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"
)

const (
	// maxDiffLines caps the number of added lines returned to prevent OOM on huge diffs.
	maxDiffLines = 10_000
	// maxFileSize is the maximum file size (in bytes) we read for pattern scanning.
	maxFileSize = 1 * 1024 * 1024 // 1 MB
	// gitTimeout is the default timeout for git subprocesses.
	gitTimeout = 10 * time.Second
)

// DetectBaseBranch returns the base/trunk branch for the repo at dir.
// It checks (in order): configuredBase override, symbolic-ref of origin/HEAD,
// local "main", local "master". Defaults to "main".
func DetectBaseBranch(dir, configuredBase string) string {
	if configuredBase != "" {
		return configuredBase
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try symbolic-ref of origin/HEAD.
	cmd := exec.CommandContext(ctx, "git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil {
		ref := strings.TrimSpace(string(out))
		parts := strings.Split(ref, "/")
		if len(parts) > 0 && parts[len(parts)-1] != "" {
			return parts[len(parts)-1]
		}
	}

	// Fallback: check if "main" exists.
	cmd = exec.CommandContext(ctx, "git", "rev-parse", "--verify", "refs/heads/main")
	cmd.Dir = dir
	if err := cmd.Run(); err == nil {
		return "main"
	}

	// Fallback: check if "master" exists.
	cmd = exec.CommandContext(ctx, "git", "rev-parse", "--verify", "refs/heads/master")
	cmd.Dir = dir
	if err := cmd.Run(); err == nil {
		return "master"
	}

	return "main"
}

// DiffAddedLines returns the added lines ('+' prefix stripped) between the
// merge-base of baseBranch and HEAD. Returns nil and no error when the diff
// cannot be computed (orphan branch, shallow clone, etc.).
func DiffAddedLines(dir, baseBranch string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	// Find merge-base.
	mb := exec.CommandContext(ctx, "git", "merge-base", baseBranch, "HEAD")
	mb.Dir = dir
	mergeBase, err := mb.Output()
	if err != nil {
		return nil, nil // no merge-base = orphan or shallow clone
	}
	base := strings.TrimSpace(string(mergeBase))

	// Get unified diff (added lines only via parsing).
	diff := exec.CommandContext(ctx, "git", "diff", "--unified=0", "--no-color", base+"..HEAD")
	diff.Dir = dir
	out, err := diff.Output()
	if err != nil {
		return nil, nil
	}

	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024) // 2MB max line
	for scanner.Scan() {
		line := scanner.Text()
		// Keep only added lines (start with '+') but skip diff headers ('+++').
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			lines = append(lines, line[1:]) // strip the leading '+'
			if len(lines) >= maxDiffLines {
				break
			}
		}
	}
	return lines, nil
}

// ModifiedFiles returns the list of files modified between the merge-base of
// baseBranch and HEAD. Deleted files are excluded. Returns nil on error.
func ModifiedFiles(dir, baseBranch string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	// Find merge-base.
	mb := exec.CommandContext(ctx, "git", "merge-base", baseBranch, "HEAD")
	mb.Dir = dir
	mergeBase, err := mb.Output()
	if err != nil {
		return nil, nil
	}
	base := strings.TrimSpace(string(mergeBase))

	// List modified files, excluding deletions.
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "--diff-filter=d", base+"..HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	var files []string
	for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

// TrackedFiles returns all tracked files in the repo (respects .gitignore).
// Uses `git ls-files` instead of filesystem walk.
func TrackedFiles(dir string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "ls-files")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	var files []string
	for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

// HasTestFiles checks if any of the given file paths look like test files
// based on common multi-language naming conventions.
func HasTestFiles(files []string) bool {
	for _, f := range files {
		base := f
		if idx := strings.LastIndex(f, "/"); idx >= 0 {
			base = f[idx+1:]
		}
		lower := strings.ToLower(base)

		// Go: *_test.go
		if strings.HasSuffix(lower, "_test.go") {
			return true
		}
		// JS/TS: *.test.js, *.test.ts, *.spec.js, *.spec.ts (and jsx/tsx)
		if strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.") {
			return true
		}
		// Python: test_*.py, *_test.py
		if strings.HasSuffix(lower, ".py") && (strings.HasPrefix(lower, "test_") || strings.HasSuffix(lower, "_test.py")) {
			return true
		}
		// Java: *Test.java, *Tests.java
		if strings.HasSuffix(base, "Test.java") || strings.HasSuffix(base, "Tests.java") {
			return true
		}
		// Ruby: *_spec.rb, *_test.rb
		if strings.HasSuffix(lower, "_spec.rb") || strings.HasSuffix(lower, "_test.rb") {
			return true
		}
	}
	return false
}

// IsBinary returns true if the content appears to contain null bytes (simple heuristic).
func IsBinary(data []byte) bool {
	// Check first 8KB for null bytes.
	limit := 8192
	if len(data) < limit {
		limit = len(data)
	}
	for i := 0; i < limit; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}
