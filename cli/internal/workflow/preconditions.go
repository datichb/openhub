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
}

// EvaluatePreconditions runs the preconditions of s, in declaration order,
// against the session location fsys (os.DirFS(location)). It reads the file
// system only; the caller decides how to present a failure.
func EvaluatePreconditions(s *Spec, fsys fs.FS) []PreconditionResult {
	var out []PreconditionResult
	for _, k := range s.Preconditions.Keys() {
		pc, _ := s.Preconditions.Get(k)
		if pc.Disabled {
			continue
		}
		r := PreconditionResult{ID: k, Label: pc.Label, Block: pc.OnFail == OnFailBlock, OK: checkPaths(fsys, pc.Check.PathExists)}
		if pc.Suggest != nil {
			r.Suggest, r.Resume = pc.Suggest.Workflow, pc.Suggest.Resume
		}
		out = append(out, r)
	}
	return out
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
