package workflow

import (
	"io/fs"
	"path"
)

// PreconditionResult is the outcome of one precondition.
type PreconditionResult struct {
	ID    string
	OK    bool
	Label LocalizedText
	// Block: the launch must be refused (on_fail: block).
	Block bool
	// Suggest is the workflow to offer instead (may be empty); Resume
	// offers to come back to this workflow once it has finished.
	Suggest string
	Resume  bool
	// Failure says what failed (R8, E7): set for the git and tool checks,
	// so that the message names the branch, the changes or the tools.
	Failure *PreconditionFailure
}

// Kinds of a failed precondition (PreconditionFailure.Kind).
const (
	FailNotGit  = "not_git"   // the location is not a git repository (or git cannot read it)
	FailDirty   = "dirty"     // uncommitted changes (Dirty)
	FailBranch  = "branch"    // the current branch matches no pattern (Branch, Want)
	FailTools   = "tools"     // commands missing from the PATH (Missing)
	FailPathAny = "path_none" // none of the paths exists
)

// PreconditionFailure details a failed precondition.
type PreconditionFailure struct {
	Kind string
	// Branch is the current branch ("" for a detached HEAD); Want the
	// expected patterns.
	Branch string
	Want   []string
	// Dirty lists the uncommitted changes (`git status --porcelain` lines).
	Dirty []string
	// Missing lists the commands not found.
	Missing []string
	// Err is the git error, for FailNotGit.
	Err string
}

// GitState is what the git checks read from the session location.
type GitState struct {
	// Branch is the current branch ("" for a detached HEAD).
	Branch string
	// Dirty lists the uncommitted changes outside .beads.
	Dirty []string
}

// PreconditionProbe reads, for the preconditions, what is not a file of
// the location. A nil function fails its checks (closed).
type PreconditionProbe struct {
	// Git reads the git state of the location; an error when it is not a
	// git repository or git cannot read it.
	Git func() (GitState, error)
	// LookPath reports whether a command is found in the PATH.
	LookPath func(cmd string) bool
}

// EvaluatePreconditions runs the preconditions of s, in declaration order,
// against the session location fsys (os.DirFS(location)). It reads the file
// system only (the git and tool checks fail); the caller decides how to
// present a failure.
func EvaluatePreconditions(s *Spec, fsys fs.FS) []PreconditionResult {
	return EvaluatePreconditionsWith(s, fsys, PreconditionProbe{})
}

// EvaluatePreconditionsWith runs the preconditions of s with probe for the
// git state (read once, when a precondition needs it) and the tools.
func EvaluatePreconditionsWith(s *Spec, fsys fs.FS, probe PreconditionProbe) []PreconditionResult {
	var (
		git    *GitState
		gitErr error
		read   bool
	)
	gitState := func() (*GitState, error) {
		if !read {
			read = true
			if probe.Git == nil {
				gitErr = errNoGitProbe
			} else if st, err := probe.Git(); err != nil {
				gitErr = err
			} else {
				git = &st
			}
		}
		return git, gitErr
	}
	var out []PreconditionResult
	for _, k := range s.Preconditions.Keys() {
		pc, _ := s.Preconditions.Get(k)
		if pc.Disabled {
			continue
		}
		r := PreconditionResult{ID: k, Label: pc.Label, Block: pc.OnFail == OnFailBlock}
		r.Failure = checkPrecondition(pc.Check, fsys, probe, gitState)
		r.OK = r.Failure == nil
		if pc.Suggest != nil {
			r.Suggest, r.Resume = pc.Suggest.Workflow, pc.Suggest.Resume
		}
		out = append(out, r)
	}
	return out
}

type probeError string

func (e probeError) Error() string { return string(e) }

const errNoGitProbe = probeError("git state not read")

// checkPrecondition runs the test of c; nil when it holds.
func checkPrecondition(c PreconditionCheck, fsys fs.FS, probe PreconditionProbe, gitState func() (*GitState, error)) *PreconditionFailure {
	if len(c.PathExists) > 0 && !checkPaths(fsys, c.PathExists) {
		return &PreconditionFailure{Kind: FailPathAny}
	}
	if c.GitClean || len(c.Branch) > 0 {
		st, err := gitState()
		if err != nil {
			return &PreconditionFailure{Kind: FailNotGit, Err: err.Error()}
		}
		if c.GitClean && len(st.Dirty) > 0 {
			return &PreconditionFailure{Kind: FailDirty, Dirty: st.Dirty}
		}
		if len(c.Branch) > 0 && !BranchMatches(st.Branch, c.Branch) {
			return &PreconditionFailure{Kind: FailBranch, Branch: st.Branch, Want: c.Branch}
		}
	}
	if len(c.Tools) > 0 {
		var missing []string
		for _, t := range c.Tools {
			if probe.LookPath == nil || !probe.LookPath(t) {
				missing = append(missing, t)
			}
		}
		if len(missing) > 0 {
			return &PreconditionFailure{Kind: FailTools, Missing: missing}
		}
	}
	return nil
}

// BranchMatches reports whether branch matches one of the patterns
// (path.Match: `release/*`); a detached HEAD ("") matches none.
func BranchMatches(branch string, patterns []string) bool {
	if branch == "" {
		return false
	}
	for _, p := range patterns {
		if ok, err := path.Match(p, branch); err == nil && ok {
			return true
		}
	}
	return false
}

// FailedPreconditions returns the failed preconditions of results.
func FailedPreconditions(results []PreconditionResult) []PreconditionResult {
	var out []PreconditionResult
	for _, r := range results {
		if !r.OK {
			out = append(out, r)
		}
	}
	return out
}

func checkPaths(fsys fs.FS, paths []string) bool {
	for _, p := range paths {
		clean := path.Clean(p)
		if !fs.ValidPath(clean) {
			continue
		}
		if _, err := fs.Stat(fsys, clean); err == nil {
			return true
		}
	}
	return false
}
