package wiki

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/gitutil"
)

// Changes of the project a check is restricted to (modernisation LD-H,
// LD-D): files given by the caller, the files changed between two commits,
// or the files changed since the last synchronization of the wiki.
//
// The last synchronization is a commit kept in the frontmatter of
// docs/wiki/index.md (`synced:`), written by oh when the checkpoint of
// doc-sync is passed, and committed with the wiki. It lives in the
// repository rather than in the database of oh: it travels with the
// branch, a clone and the other members of a team read the same one, and
// it is always in step with the pages it describes. When it is missing or
// no longer reachable (history rewritten), the last commit that changed
// docs/wiki/ stands for it.

// Origins of a scope (CheckScope.From).
const (
	FromPaths     = "paths"            // files given by the caller
	FromRange     = "range"            // base..head
	FromSync      = "sync"             // since the commit of `synced:`
	FromWikiEdit  = "last_wiki_commit" // since the last commit that changed docs/wiki/
	FromNoHistory = "none"             // the wiki was never committed: nothing to synchronize
)

// Bounds of a scope.
const (
	MaxScopePaths   = 500
	MaxScopeCommits = 100
)

// ErrBadRef: a base or head that is not a commit of the repository.
var ErrBadRef = errors.New("not a commit of the repository")

// CheckOptions restrict a check to a change.
type CheckOptions struct {
	// Paths are project files the change touched (relative to the root).
	Paths []string `json:"paths,omitempty"`
	// Base and Head: the files changed from the merge base of Base and
	// Head to Head; Head "" = the working tree (uncommitted and untracked
	// files included).
	Base string `json:"base,omitempty"`
	Head string `json:"head,omitempty"`
	// SinceSync: Base is the last synchronization of the wiki (doc-sync).
	SinceSync bool `json:"since_sync,omitempty"`
}

// Scoped reports whether the options restrict the check.
func (o CheckOptions) Scoped() bool { return len(o.Paths) > 0 || o.Base != "" || o.SinceSync }

// Commit is a commit of a scope.
type Commit struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
}

// CheckScope is the change a check is restricted to.
type CheckScope struct {
	From string `json:"from"`
	// Base and Head are the commits of the range (Head "" = working tree).
	Base    string   `json:"base,omitempty"`
	Head    string   `json:"head,omitempty"`
	Commits []Commit `json:"commits,omitempty"`
	// MoreCommits are the commits of the range past MaxScopeCommits.
	MoreCommits int `json:"more_commits,omitempty"`
	// Changed are the project files changed, sorted.
	Changed []string `json:"changed"`
	// Updated are the pages of the wiki changed (relative to docs/wiki/).
	Updated []string `json:"updated,omitempty"`
	// Pages are the pages concerned: they cite a changed file, or hold a
	// god node the change touches (relative to docs/wiki/).
	Pages []string `json:"pages,omitempty"`
}

func git(ctx context.Context, root string, args ...string) (string, error) {
	out, err := gitutil.Command(ctx, root, args...).Output()
	return string(out), err
}

// commitOf resolves a ref to a commit (refs starting with a dash refused).
func commitOf(ctx context.Context, root, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.HasPrefix(ref, "-") || strings.ContainsAny(ref, " \t\n\x00") || len(ref) > 250 {
		return "", fmt.Errorf("%q: %w", ref, ErrBadRef)
	}
	out, err := git(ctx, root, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("%q: %w", ref, ErrBadRef)
	}
	return strings.TrimSpace(out), nil
}

// SyncMarker returns the commit of the last synchronization written in the
// index ("" when none).
func SyncMarker(root string) string {
	text, err := ReadPage(root, IndexPage, "")
	if err != nil {
		return ""
	}
	fm, _ := splitFrontmatter(text)
	return strings.TrimSpace(fm.Synced)
}

// SetSyncMarker writes the commit of a synchronization in the frontmatter
// of the index (false: no index, or an index without a readable
// frontmatter).
func SetSyncMarker(root, commit string, now time.Time) (bool, error) {
	text, err := ReadPage(root, IndexPage, "")
	if errors.Is(err, ErrNoPage) || errors.Is(err, ErrNoWiki) {
		return false, nil //nolint:nilerr // no index: nothing to mark
	}
	if err != nil {
		return false, err
	}
	fm, _ := splitFrontmatter(text)
	if !fm.Present || fm.Err != nil {
		return false, nil //nolint:nilerr // a frontmatter oh cannot read is left as it is
	}
	if strings.TrimSpace(fm.Synced) == commit {
		return true, nil
	}
	out := setFrontmatter(text, map[string]string{"synced": commit, "updated": now.Format(time.DateOnly)})
	if out == text {
		return false, nil
	}
	return true, writePage(root, IndexPage, out)
}

// syncBase returns the base of a synchronization and where it comes from.
func syncBase(ctx context.Context, root string) (base, from string) {
	if m := SyncMarker(root); m != "" {
		if c, err := commitOf(ctx, root, m); err == nil {
			if _, err := git(ctx, root, "merge-base", "--is-ancestor", c, "HEAD"); err == nil {
				return c, FromSync
			}
		}
	}
	out, err := git(ctx, root, "log", "-1", "--format=%H", "--", Dir)
	if c := strings.TrimSpace(out); err == nil && c != "" {
		return c, FromWikiEdit
	}
	return "", FromNoHistory
}

// ResolveScope returns the change of the options.
func ResolveScope(ctx context.Context, root string, o CheckOptions) (*CheckScope, error) {
	sc := &CheckScope{From: FromPaths}
	set := map[string]bool{}
	for _, p := range o.Paths {
		if f := projectFile(root, p); f != "" && len(set) < MaxScopePaths {
			set[f] = true
		}
	}
	base, head := o.Base, strings.TrimSpace(o.Head)
	if o.SinceSync {
		base, sc.From = syncBase(ctx, root)
	} else if base != "" {
		sc.From = FromRange
	}
	if base != "" {
		b, err := commitOf(ctx, root, base)
		if err != nil {
			return nil, err
		}
		target := "HEAD"
		if head != "" {
			h, err := commitOf(ctx, root, head)
			if err != nil {
				return nil, err
			}
			target, sc.Head = h, h
		}
		if mb, err := git(ctx, root, "merge-base", b, target); err == nil && strings.TrimSpace(mb) != "" {
			b = strings.TrimSpace(mb)
		}
		sc.Base = b
		files, err := changedFiles(ctx, root, b, sc.Head)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if len(set) < MaxScopePaths {
				set[f] = true
			}
		}
		sc.Commits, sc.MoreCommits = commits(ctx, root, b, target)
	}
	for f := range set {
		sc.Changed = append(sc.Changed, f)
	}
	sort.Strings(sc.Changed)
	if sc.Changed == nil {
		sc.Changed = []string{}
	}
	for _, f := range sc.Changed {
		if rel, ok := strings.CutPrefix(f, Dir+"/"); ok && strings.HasSuffix(strings.ToLower(rel), ".md") {
			sc.Updated = append(sc.Updated, rel)
		}
	}
	return sc, nil
}

// projectFile normalizes a file given by a caller: relative to root, with
// slashes ("" out of the project).
func projectFile(root, p string) string {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	if p == "" || strings.ContainsAny(p, "\x00\n") {
		return ""
	}
	if filepath.IsAbs(p) || path.IsAbs(p) {
		rel, err := filepath.Rel(root, filepath.FromSlash(p))
		if err != nil {
			return ""
		}
		p = filepath.ToSlash(rel)
	}
	p = path.Clean(strings.TrimPrefix(p, "./"))
	if !insideProject(p) || p == ".git" || strings.HasPrefix(p, ".git/") {
		return ""
	}
	return p
}

// changedFiles lists the files changed from base to head (head "": the
// working tree, untracked files included), deleted ones included.
func changedFiles(ctx context.Context, root, base, head string) ([]string, error) {
	args := []string{"diff", "--name-only", "--no-renames", base}
	if head != "" {
		args = append(args, head)
	}
	out, err := git(ctx, root, append(args, "--")...)
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	if head == "" {
		if u, err := git(ctx, root, "ls-files", "-o", "--exclude-standard"); err == nil {
			out += "\n" + u
		}
	}
	var files []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			files = append(files, l)
		}
	}
	return files, nil
}

// commits lists the commits from base to head, newest first, at most
// MaxScopeCommits (and how many more).
func commits(ctx context.Context, root, base, head string) (list []Commit, more int) {
	out, err := git(ctx, root, "log", "--no-merges", "--format=%h%x09%s", base+".."+head)
	if err != nil {
		return nil, 0
	}
	for _, l := range strings.Split(out, "\n") {
		sha, subject, ok := strings.Cut(strings.TrimSpace(l), "\t")
		if !ok {
			continue
		}
		if len(list) == MaxScopeCommits {
			more++
			continue
		}
		list = append(list, Commit{SHA: sha, Subject: oneLine(subject, 120)})
	}
	return list, more
}

// HeadCommit returns the commit of HEAD at root ("" outside a repository).
func HeadCommit(ctx context.Context, root string) string {
	if !gitutil.LooksLikeRepository(root) {
		return ""
	}
	out, err := git(ctx, root, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}
