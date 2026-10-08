package runsvc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// Session locations (`oh run --location`, launch form "Emplacement").

// LocationKind is where a session works.
type LocationKind string

const (
	LocationBase     LocationKind = "base"     // the project directory
	LocationWorktree LocationKind = "worktree" // a git worktree (existing or created)
	LocationNew      LocationKind = "new"      // choice only: a new worktree per session
)

// LocationChoice is the location asked for a launch.
type LocationChoice struct {
	Kind LocationKind
	Path string // existing worktree (Kind = worktree)
}

// ParseLocation reads a --location value: "" or "base", "new", or the path
// of an existing worktree.
func ParseLocation(v string) LocationChoice {
	switch v {
	case "", string(LocationBase):
		return LocationChoice{Kind: LocationBase}
	case string(LocationNew):
		return LocationChoice{Kind: LocationNew}
	}
	if abs, err := filepath.Abs(v); err == nil {
		v = abs
	}
	return LocationChoice{Kind: LocationWorktree, Path: v}
}

// Location is the resolved working directory of a planned session.
type Location struct {
	Path   string       `json:"path"`
	Kind   LocationKind `json:"kind"` // base | worktree
	Branch string       `json:"branch,omitempty"`
	// Create means the worktree is created when the session starts.
	Create bool `json:"create,omitempty"`
	// Auto means a worktree was chosen because the requested directory is
	// used by another session that writes (O10) or has uncommitted changes.
	Auto bool `json:"auto,omitempty"`
	// Stash means the uncommitted changes of the directory are stashed when
	// the session starts (DirtyStash).
	Stash bool `json:"stash,omitempty"`
}

// DirtyPolicy is what a writing session does when its directory has
// uncommitted changes (v5 corrections, A18).
type DirtyPolicy string

const (
	// DirtyWorktree (default): the session works in a new worktree; the
	// changes stay untouched in the directory.
	DirtyWorktree DirtyPolicy = ""
	// DirtyAllow: the session works in the directory anyway (--allow-dirty).
	DirtyAllow DirtyPolicy = "allow"
	// DirtyStash: the changes are stashed before the session starts.
	DirtyStash DirtyPolicy = "stash"
)

// ParseDirtyPolicy reads a dirty policy ("", worktree, allow, stash).
func ParseDirtyPolicy(v string) (DirtyPolicy, error) {
	switch v {
	case "", "worktree":
		return DirtyWorktree, nil
	case string(DirtyAllow), string(DirtyStash):
		return DirtyPolicy(v), nil
	}
	return "", fmt.Errorf("runsvc: unknown dirty policy %q", v)
}

// RiskWrites reports whether a workflow risk may modify files: read and plan
// (Beads only) do not; an unknown risk (legacy launch) counts as writing.
func RiskWrites(risk string) bool { return risk != "read" && risk != "plan" }

// WorkBranch is the dedicated branch a session works on: the branch of its
// worktree, or the current branch of the project when it is not the base
// one. "" on the base branch, a detached HEAD or outside git: the agent may
// then offer to create a branch.
func (l Location) WorkBranch(projectPath string) string {
	if l.Kind == LocationWorktree {
		return l.Branch
	}
	cur, err := worktree.CurrentBranch(l.Path)
	if err != nil || cur == "" || strings.HasPrefix(cur, "(detached)") {
		return ""
	}
	if cur == worktree.DetectBaseBranch(projectPath) {
		return ""
	}
	return cur
}

// ErrNotGitRepo is returned when a worktree is needed outside a git repository.
var ErrNotGitRepo = errors.New("the project is not a git repository: worktrees are unavailable")

// ErrNoBranch is returned when a new worktree has no branch name.
var ErrNoBranch = errors.New("a branch name is required for a new worktree")

// busyWriters returns the directories used by open sessions of a project
// that may write (risk other than read; unknown risk counts as writing).
// Sleeping sessions hold their directory too: they resume there, on their
// branch (v5 corrections, A18).
func (s *Service) busyWriters(ctx context.Context, projectID string) map[string]string {
	out := map[string]string{}
	if s.Sessions == nil || projectID == "" {
		return out
	}
	list, err := s.Sessions.List(ctx, projectID)
	if err != nil {
		return out
	}
	for _, o := range list {
		switch o.State {
		case domain.RunPreparing, domain.RunQueued, domain.RunActive, domain.RunWaiting, domain.RunIdle, domain.RunSleeping:
		default:
			continue
		}
		if !RiskWrites(o.WorkflowRisk) || o.LaunchPath == "" {
			continue
		}
		out[filepath.Clean(o.LaunchPath)] = o.ID
	}
	return out
}

// planLocations resolves the location of each session of a run. writes
// tells whether the workflow may modify files (O10 applies to writers);
// branches gives the branch of each session's worktree, if one is needed;
// dirty tells what a writer does in a directory with uncommitted changes.
func (s *Service) planLocations(ctx context.Context, projectID, projectPath string, choice LocationChoice, writes bool, branches []string, dirty DirtyPolicy) ([]Location, []Warning, error) {
	projectPath = filepath.Clean(projectPath)
	busy := map[string]string{}
	if writes {
		busy = s.busyWriters(ctx, projectID)
	}
	isGit := s.isGitRepo(projectPath)
	var (
		out   []Location
		warns []Warning
	)
	taken := map[string]bool{}
	newWorktree := func(i int, auto bool) (Location, error) {
		if !isGit {
			return Location{}, ErrNotGitRepo
		}
		b := ""
		if i < len(branches) {
			b = branches[i]
		}
		if b == "" {
			return Location{}, ErrNoBranch
		}
		p := filepath.Clean(worktree.SiblingPath(projectPath, b))
		loc := Location{Path: p, Kind: LocationWorktree, Branch: b, Auto: auto}
		if st, err := os.Stat(p); err != nil || !st.IsDir() {
			loc.Create = true
		}
		return loc, nil
	}
	n := max(len(branches), 1)
	// Several sessions that write: one worktree each (one ticket, one branch).
	perSession := writes && n > 1 && choice.Kind == LocationBase
	for i := 0; i < n; i++ {
		var (
			loc Location
			err error
		)
		switch {
		case choice.Kind == LocationNew:
			loc, err = newWorktree(i, false)
		case perSession:
			loc, err = newWorktree(i, true)
		case choice.Kind == LocationWorktree:
			st, serr := os.Stat(choice.Path)
			if serr != nil || !st.IsDir() {
				return nil, nil, fmt.Errorf("worktree %s: %w", choice.Path, os.ErrNotExist)
			}
			loc = Location{Path: filepath.Clean(choice.Path), Kind: LocationWorktree}
			if b, berr := worktree.CurrentBranch(loc.Path); berr == nil {
				loc.Branch = b
			}
		default:
			loc = Location{Path: projectPath, Kind: LocationBase}
		}
		if err != nil {
			return nil, nil, err
		}
		if writes && choice.Kind != LocationNew && !perSession {
			if other, ok := busy[loc.Path]; ok || taken[loc.Path] {
				switch {
				case !isGit:
					warns = append(warns, Warning{Code: WarnSharedLocation, Args: []any{loc.Path}})
				default:
					if loc, err = newWorktree(i, true); err != nil {
						return nil, nil, err
					}
					if ok {
						warns = append(warns, Warning{Code: WarnAutoWorktree, Args: []any{other, loc.Path}})
					}
				}
			}
		}
		if writes && !loc.Create && s.isDirty(loc.Path) {
			switch {
			case dirty == DirtyStash:
				loc.Stash = true
				warns = append(warns, Warning{Code: WarnDirtyStash, Args: []any{loc.Path}})
			case dirty == DirtyWorktree && loc.Kind == LocationBase && isGit:
				// The user's changes stay where they are: the session works
				// in a worktree of its branch.
				from := loc.Path
				if loc, err = newWorktree(i, true); err != nil {
					return nil, nil, err
				}
				warns = append(warns, Warning{Code: WarnDirtyWorktree, Args: []any{from, loc.Path}})
			default:
				// --allow-dirty, or an existing worktree chosen explicitly.
				warns = append(warns, Warning{Code: WarnDirty, Args: []any{loc.Path}})
			}
		}
		taken[loc.Path] = true
		out = append(out, loc)
	}
	return out, warns, nil
}

func (s *Service) isGitRepo(p string) bool {
	if s.Git != nil {
		return s.Git.IsRepo(p)
	}
	return worktree.IsGitRepo(p)
}

func (s *Service) isDirty(p string) bool {
	if s.Git != nil {
		return s.Git.IsDirty(p)
	}
	return worktree.IsGitRepo(p) && worktree.IsDirty(p)
}

func (s *Service) createWorktree(projectPath, branch string) (string, error) {
	if s.Git != nil {
		return s.Git.CreateWorktree(projectPath, branch)
	}
	return worktree.ResolveOrCreate(projectPath, branch)
}

func (s *Service) stash(path, message string) (string, error) {
	if s.Git != nil {
		return s.Git.Stash(path, message)
	}
	return worktree.Stash(path, message)
}

// Git abstracts the git operations of the launcher (tests).
type Git interface {
	IsRepo(path string) bool
	IsDirty(path string) bool
	CreateWorktree(projectPath, branch string) (string, error)
	// Stash puts the uncommitted changes (untracked files included) aside
	// and returns the commit of the stash entry.
	Stash(path, message string) (string, error)
}
