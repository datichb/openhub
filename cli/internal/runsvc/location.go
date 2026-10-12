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

// locOptions adjust the locations of a run for the existing sessions of
// its tickets (RunRequest.OnExisting).
type locOptions struct {
	// free are directories released before the launch (stopped or deleted
	// session): not busy.
	free map[string]bool
	// fixed are imposed locations by planned session (keep, resume, transfer).
	fixed map[int]Location
	// recreate are worktrees removed before the launch (delete): created again.
	recreate map[string]bool
	// clean are directories whose changes are discarded before the launch.
	clean map[string]bool
	// ticketWorktree is, by planned session, an existing session of its
	// ticket working in a worktree of the project (base chosen anyway).
	ticketWorktree map[int]ExistingSession
}

// planLocations resolves the location of each session of a run. writes
// tells whether the workflow may modify files (O10 applies to writers);
// branches gives the branch of each session's worktree, if one is needed;
// dirty tells what a writer does in a directory with uncommitted changes.
func (s *Service) planLocations(ctx context.Context, projectID, projectPath string, choice LocationChoice, writes bool, branches []string, dirty DirtyPolicy, lo locOptions) ([]Location, []Warning, error) {
	projectPath = filepath.Clean(projectPath)
	busy := map[string]string{}
	if writes {
		busy = s.busyWriters(ctx, projectID)
		for p := range lo.free {
			delete(busy, filepath.Clean(p))
		}
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
		if st, err := os.Stat(p); err != nil || !st.IsDir() || lo.recreate[p] {
			loc.Create = true
		}
		return loc, nil
	}
	// autoWorktree moves a session to its worktree; that one may be used
	// too (another session on the same branch).
	autoWorktree := func(i int) (Location, error) {
		loc, err := newWorktree(i, true)
		if err != nil {
			return loc, err
		}
		if other, ok := busy[loc.Path]; ok && writes {
			warns = append(warns, Warning{Code: WarnWorktreeBusy, Args: []any{other, loc.Path}})
		}
		return loc, nil
	}
	n := max(len(branches), 1)
	// Several sessions that write: one worktree each (one ticket, one branch).
	perSession := writes && n > 1 && choice.Kind == LocationBase
	for i := 0; i < n; i++ {
		if loc, ok := lo.fixed[i]; ok {
			taken[loc.Path] = true
			out = append(out, loc)
			continue
		}
		var (
			loc Location
			err error
		)
		switch {
		case choice.Kind == LocationNew:
			loc, err = newWorktree(i, false)
			if err == nil && writes {
				if other, ok := busy[loc.Path]; ok {
					warns = append(warns, Warning{Code: WarnWorktreeBusy, Args: []any{other, loc.Path}})
				}
			}
		case perSession:
			loc, err = autoWorktree(i)
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
					if loc, err = autoWorktree(i); err != nil {
						return nil, nil, err
					}
					if ok {
						code := WarnAutoWorktree
						if !loc.Create {
							code = WarnAutoWorktreeExisting
						}
						warns = append(warns, Warning{Code: code, Args: []any{other, loc.Path}})
					}
				}
			}
		}
		if writes && !loc.Create && !lo.clean[loc.Path] && s.isDirty(loc.Path) {
			switch {
			case dirty == DirtyStash:
				loc.Stash = true
				warns = append(warns, Warning{Code: WarnDirtyStash, Args: []any{loc.Path}})
			case dirty == DirtyWorktree && loc.Kind == LocationBase && isGit:
				// The user's changes stay where they are: the session works
				// in a worktree of its branch.
				from := loc.Path
				if loc, err = autoWorktree(i); err != nil {
					return nil, nil, err
				}
				warns = append(warns, Warning{Code: WarnDirtyWorktree, Args: []any{from, loc.Path}})
			default:
				// --allow-dirty, or an existing worktree chosen explicitly.
				warns = append(warns, Warning{Code: WarnDirty, Args: []any{loc.Path}})
			}
		}
		if e, ok := lo.ticketWorktree[i]; ok && loc.Kind == LocationBase && filepath.Clean(e.LaunchPath) != loc.Path {
			// The ticket is worked on in its worktree: the base gets a
			// second copy of the work.
			warns = append(warns, Warning{Code: WarnTicketWorktree, Args: []any{e.ID, e.LaunchPath}})
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
