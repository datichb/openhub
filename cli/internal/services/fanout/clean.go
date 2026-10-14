package fanout

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
)

// Clean-up of a fan-out: the worktrees and the branches oh/fanout/… of a
// parent, once its merge checkpoint passed (Integrate) or the parent
// abandoned (stopped for good), and by oh gc for what is left.
//
//   - a worktree goes when no session of the fan-out runs in it any more
//     and nothing is left uncommitted in it (else it stays, traced);
//   - a branch goes when its worktree is gone and each of its commits is on
//     another branch (local or remote): the children first, then the
//     integration branch. A merged fan-out leaves nothing; an abandoned one
//     keeps the integration branch and the children not merged into it;
//   - a branch with commits found nowhere else is deleted only on an
//     explicit request (Discard, `oh fanout clean --discard`), its commit
//     traced in the events of the fan-out (`git branch <branch> <commit>`
//     brings it back while git keeps the objects).

// CleanOptions tune a clean-up.
type CleanOptions struct {
	// Discard also deletes the branches whose commits are on no other
	// branch (explicit, traced).
	Discard bool
	// DryRun only plans.
	DryRun bool
	// Live reports a session still running (its worktree stays; nil =
	// read from the store).
	Live func(ctx context.Context, sessionID string) bool
}

// CleanResult is what a clean-up removed and kept.
type CleanResult struct {
	Worktrees []string `json:"worktrees,omitempty"` // removed (or to remove)
	Branches  []string `json:"branches,omitempty"`  // deleted (or to delete)
	Discarded []string `json:"discarded,omitempty"` // "branch commit": deleted with commits found nowhere else
	Kept      []string `json:"kept,omitempty"`      // "branch: reason"
}

// Empty reports a clean-up with nothing to remove.
func (r CleanResult) Empty() bool {
	return len(r.Worktrees) == 0 && len(r.Branches) == 0 && len(r.Discarded) == 0
}

// Reasons a branch or a worktree of a fan-out stays.
const (
	KeptRunning   = "running"  // a session of the fan-out still runs in it
	KeptDirty     = "dirty"    // uncommitted changes in its worktree
	KeptUnmerged  = "unmerged" // commits found on no other branch
	KeptCheckedIn = "worktree" // its worktree could not be removed
	KeptForeign   = "not-oh"   // the directory is not the worktree of the branch
)

// Cleanable reports whether the fan-out of sess may be cleaned up: its
// merge checkpoint passed, or the parent was stopped for good.
func Cleanable(sess *domain.Session) bool {
	f := sess.Fanout
	if f == nil || f.Role != domain.RoleParent || f.Integration == nil {
		return false
	}
	return f.Status == domain.FanoutMerged || sess.State == domain.RunStopped
}

type cleanEntry struct {
	branch, path string
	sessions     []string
}

// Clean removes what the fan-out of the parent sess left, as the options
// say, and traces it in the record of the fan-out (unless DryRun).
func Clean(ctx context.Context, store Store, sess *domain.Session, opts CleanOptions) (CleanResult, error) {
	var res CleanResult
	f := sess.Fanout
	if f == nil || f.Role != domain.RoleParent {
		return res, ErrNotFanout
	}
	if f.Integration == nil {
		return res, nil
	}
	repo := cleanRepo(sess)
	if repo == "" {
		return res, errors.New("no directory of the repository of the fan-out is left")
	}
	live := opts.Live
	if live == nil {
		live = func(ctx context.Context, id string) bool {
			s, err := store.Get(ctx, id)
			return err == nil && !terminal(s.State)
		}
	}
	var entries []cleanEntry
	var mergers []string // and fixers: they work in the integration worktree
	for _, x := range f.Fixes {
		mergers = append(mergers, x.Session)
	}
	for _, c := range f.Children {
		if c.Merger != "" {
			mergers = append(mergers, c.Merger)
		}
		if c.Branch != "" {
			entries = append(entries, cleanEntry{branch: c.Branch, path: c.Path, sessions: []string{c.Session}})
		}
	}
	ig := f.Integration
	entries = append(entries, cleanEntry{branch: ig.Branch, path: ig.Path, sessions: mergers})

	discarded := map[string]string{}
	for _, e := range entries {
		if !strings.HasPrefix(e.branch, BranchPrefix) {
			continue // never a branch oh did not create
		}
		if kept := cleanWorktree(ctx, repo, e, live, opts.DryRun); kept != "" {
			if kept != "gone" {
				res.Kept = append(res.Kept, e.branch+": "+kept)
				continue
			}
		} else {
			res.Worktrees = append(res.Worktrees, e.path)
		}
		head, err := BranchHead(ctx, repo, e.branch)
		if err != nil {
			continue // already gone
		}
		if !reachableElsewhere(ctx, repo, e.branch) {
			if !opts.Discard {
				res.Kept = append(res.Kept, e.branch+": "+KeptUnmerged)
				continue
			}
			discarded[e.branch] = head
		}
		if !opts.DryRun {
			if _, err := git(ctx, repo, "branch", "-D", e.branch); err != nil {
				res.Kept = append(res.Kept, e.branch+": "+err.Error())
				delete(discarded, e.branch)
				continue
			}
		}
		if h, ok := discarded[e.branch]; ok {
			res.Discarded = append(res.Discarded, e.branch+" "+h)
		} else {
			res.Branches = append(res.Branches, e.branch)
		}
	}
	if opts.DryRun || (res.Empty() && slices.Equal(res.Kept, f.Kept)) {
		return res, nil
	}
	return res, traceClean(ctx, store, sess.ID, res)
}

// cleanWorktree removes the worktree of e: "" when removed, "gone" when
// there is none, else the reason it stays.
func cleanWorktree(ctx context.Context, repo string, e cleanEntry, live func(context.Context, string) bool, dry bool) string {
	if e.path == "" {
		return "gone"
	}
	if _, err := os.Stat(e.path); err != nil {
		_, _ = git(ctx, repo, "worktree", "prune")
		return "gone"
	}
	if cur, err := git(ctx, e.path, "symbolic-ref", "--short", "HEAD"); err != nil || strings.TrimSpace(cur) != e.branch {
		return KeptForeign
	}
	for _, id := range e.sessions {
		if id != "" && live(ctx, id) {
			return KeptRunning
		}
	}
	if dirty, err := Dirty(ctx, e.path); err != nil || dirty || MergeInProgress(ctx, e.path) {
		return KeptDirty
	}
	if dry {
		return ""
	}
	if err := RemoveWorktree(ctx, repo, e.path); err != nil {
		return KeptCheckedIn
	}
	return ""
}

// reachableElsewhere reports whether every commit of branch is on another
// branch, local or remote.
func reachableElsewhere(ctx context.Context, repo, branch string) bool {
	out, err := git(ctx, repo, "rev-list", "refs/heads/"+branch, "--not", "--exclude="+branch, "--branches", "--remotes")
	return err == nil && strings.TrimSpace(out) == ""
}

// cleanRepo is the main worktree of the repository of the fan-out, found
// from a directory of it still there.
func cleanRepo(sess *domain.Session) string {
	dirs := []string{sess.LaunchPath}
	if f := sess.Fanout; f != nil {
		if f.Integration != nil {
			dirs = append(dirs, f.Integration.Path)
		}
		for _, c := range f.Children {
			dirs = append(dirs, c.Path)
		}
	}
	for _, d := range dirs {
		if d == "" {
			continue
		}
		out, err := git(context.Background(), d, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if err != nil {
			continue
		}
		if main := filepath.Dir(strings.TrimSpace(out)); main != "" {
			if _, err := os.Stat(main); err == nil {
				return main
			}
		}
	}
	return ""
}

// traceClean writes the clean-up in the events of the fan-out.
func traceClean(ctx context.Context, store Store, parent string, res CleanResult) error {
	p, err := store.Get(ctx, parent)
	if err != nil || p.Fanout == nil {
		return err
	}
	f := *p.Fanout
	f.Events = append([]domain.FanoutEvent(nil), p.Fanout.Events...)
	if len(res.Worktrees) > 0 || len(res.Branches) > 0 {
		f.AddEvent(domain.FanoutEventCleaned, "", "", fmt.Sprintf("%d worktree(s), %d branch(es): %s",
			len(res.Worktrees), len(res.Branches), strings.Join(res.Branches, ", ")))
	}
	for _, d := range res.Discarded {
		f.AddEvent(domain.FanoutEventDiscarded, "", "", d)
	}
	if !slices.Equal(res.Kept, p.Fanout.Kept) {
		for _, k := range res.Kept {
			f.AddEvent(domain.FanoutEventKept, "", "", k)
		}
	}
	f.Kept = res.Kept
	f.Cleaned = len(res.Kept) == 0
	return store.SetSessionFanout(ctx, parent, &f)
}
