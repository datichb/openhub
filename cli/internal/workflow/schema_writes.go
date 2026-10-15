package workflow

import (
	"path"
	"slices"
	"strings"
)

// Documentation written by a planning agent (R4, E8) and the specs checked
// by oh at a checkpoint.
//
//	agents:
//	  planner: { role: workflow, writes: ["docs/wiki/specs/**"] }
//	checkpoints:
//	  cp-spec:
//	    verify: { spec: { paths: ["docs/wiki/specs/**"] } }
//	  cp-converge:
//	    verify: { spec: { paths: ["docs/wiki/specs/**"], breakdown: true } }
//
// `writes:` opens documentation paths only (WritesRoot, and the root files
// of WritesRootFiles: p6-debts), to an agent whose
// own rules refuse the edit tools: the bundle adds, after its own rules and
// before the guards, the edit of these paths and the commit of these paths
// only (bundle.WritesRules). A layer may only narrow an inherited list: a
// path the parent does not cover, or `writes:` on an agent that had none, is
// a loosening.

// WritesRoot is the folder every `writes:` path lies in: the documentation.
const WritesRoot = "docs/"

// WritesRootFiles are the files at the root of the project a `writes:`
// path may name, besides docs/ (p6-debts): documentation files only, never
// a folder nor code.
var WritesRootFiles = []string{"ONBOARDING.md", "CONVENTIONS.md", "README.md", "CHANGELOG.md"}

// WritesAll is the suffix of a path that covers a whole folder.
const WritesAll = "/**"

// WritesPathProblem returns why p is not a valid `writes:` path ("" when it
// is): a relative path under docs/, a file or a folder ending in /**, no
// other wildcard; or one of WritesRootFiles.
func WritesPathProblem(p string) string {
	clean := path.Clean(p)
	dir := strings.TrimSuffix(p, WritesAll)
	switch {
	case strings.TrimSpace(p) == "":
		return "empty"
	case strings.Contains(p, `\`) || path.IsAbs(p) || clean != p:
		return "not a clean relative path"
	case slices.Contains(WritesRootFiles, p):
		return ""
	case !strings.HasPrefix(p, WritesRoot) || len(p) == len(WritesRoot):
		return "outside " + WritesRoot + " and not a root file of " + strings.Join(WritesRootFiles, ", ")
	case strings.ContainsAny(dir, "*?[]{}"):
		return "wildcard other than a final /**"
	case strings.Contains(p, "/../") || strings.HasSuffix(dir, "/.."):
		return "not a clean relative path"
	}
	return ""
}

// WritesCovered reports whether the `writes:` path p is covered by one of
// the parent paths: the same path, or a path inside a parent folder (/**).
func WritesCovered(p string, parent []string) bool {
	for _, q := range parent {
		if p == q {
			return true
		}
		if dir, ok := strings.CutSuffix(q, WritesAll); ok && strings.HasPrefix(p, dir+"/") {
			return true
		}
	}
	return false
}

// WritesOf returns the `writes:` paths of agent id (nil when it has none or
// is disabled).
func (s *Spec) WritesOf(id string) []string {
	a, ok := s.Agents.Get(id)
	if !ok || a.Role == RoleDisabled {
		return nil
	}
	return a.Writes
}

// SpecCheck (`verify.spec`) is the check of the specs a session wrote,
// run by oh (internal/specdoc): every file the session changed under Paths
// has a glossary, acceptance criteria that each name their test point and,
// with Breakdown, a breakdown whose slices each cover criteria of the spec
// and that covers every criterion (convergence of the tickets and the
// spec). No spec changed under Paths fails the check.
type SpecCheck struct {
	Paths     []string `yaml:"paths,omitempty"`
	Breakdown bool     `yaml:"breakdown,omitempty"`
}

// Covers reports whether c checks at least what parent does.
func (c *SpecCheck) Covers(parent *SpecCheck) bool {
	if parent == nil {
		return true
	}
	return c != nil && containsAll(c.Paths, parent.Paths) && (c.Breakdown || !parent.Breakdown)
}

func (c *SpecCheck) clone() *SpecCheck {
	if c == nil {
		return nil
	}
	return &SpecCheck{Paths: cloneSlice(c.Paths), Breakdown: c.Breakdown}
}
