package launch

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// R8 (E7): preconditions on the repository and the tools, read by oh at
// launch where the session works (p6-debts): the project directory, the
// worktree given with --location, or, for a worktree created at launch
// (--location new), what it will hold: the files of its branch (or of the
// HEAD of the project for a new branch), that branch, nothing uncommitted.

// preconditionGitTimeout bounds each git call of the preconditions.
const preconditionGitTimeout = 10 * time.Second

// preconditionTarget returns where the preconditions of a launch are read:
// the location of its first session (remote: the project, which the job
// clones).
func preconditionTarget(project string, sessions []runsvc.PlannedSession, remote bool) (fs.FS, workflow.PreconditionProbe) {
	if remote || len(sessions) == 0 || sessions[0].Location.Path == "" {
		return os.DirFS(project), preconditionProbe(project)
	}
	loc := sessions[0].Location
	if !loc.Create {
		return os.DirFS(loc.Path), preconditionProbe(loc.Path)
	}
	ref := "HEAD"
	if branchExists(project, loc.Branch) {
		ref = "refs/heads/" + loc.Branch
	}
	probe := preconditionProbe(project)
	probe.Git = func() (workflow.GitState, error) {
		if _, err := preconditionGitState(project); err != nil {
			return workflow.GitState{}, err // not a git repository
		}
		return workflow.GitState{Branch: loc.Branch}, nil // a new worktree: clean, on its branch
	}
	return gitTreeFS(project, ref), probe
}

func branchExists(dir, branch string) bool {
	if branch == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), preconditionGitTimeout)
	defer cancel()
	return gitutil.Command(ctx, dir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch).Run() == nil
}

// gitTreeFS is the files of a commit of dir, for the path checks (empty
// when git cannot read it).
func gitTreeFS(dir, ref string) fs.FS {
	ctx, cancel := context.WithTimeout(context.Background(), preconditionGitTimeout)
	defer cancel()
	out, _ := gitutil.Command(ctx, dir, "ls-tree", "-r", "-t", "--name-only", "-z", ref).Output()
	t := treeFS{".": true}
	for _, name := range strings.Split(string(out), "\x00") {
		if name != "" {
			t[name] = true
		}
	}
	return t
}

// treeFS answers Stat for the paths of a tree (contents are not read).
type treeFS map[string]bool

func (t treeFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) || !t[name] {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return treeFile(path.Base(name)), nil
}

type treeFile string

func (f treeFile) Stat() (fs.FileInfo, error) { return treeInfo(f), nil }
func (treeFile) Read([]byte) (int, error)     { return 0, io.EOF }
func (treeFile) Close() error                 { return nil }

type treeInfo string

func (i treeInfo) Name() string     { return string(i) }
func (treeInfo) Size() int64        { return 0 }
func (treeInfo) Mode() fs.FileMode  { return 0o444 }
func (treeInfo) ModTime() time.Time { return time.Time{} }
func (treeInfo) IsDir() bool        { return false }
func (treeInfo) Sys() any           { return nil }

// preconditionProbe reads the git state of dir (current branch, uncommitted
// changes outside .beads) with the safe git of oh, and looks the tools up
// in the PATH.
func preconditionProbe(dir string) workflow.PreconditionProbe {
	return workflow.PreconditionProbe{
		Git:      func() (workflow.GitState, error) { return preconditionGitState(dir) },
		LookPath: func(cmd string) bool { _, err := exec.LookPath(cmd); return err == nil },
	}
}

func preconditionGitState(dir string) (workflow.GitState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), preconditionGitTimeout)
	defer cancel()
	out, err := gitutil.Command(ctx, dir, "rev-parse", "--is-inside-work-tree").Output()
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		return workflow.GitState{}, errors.New(i18n.T("cmd.run.precondition.not_git_repo"))
	}
	var st workflow.GitState
	if out, err := gitutil.Command(ctx, dir, "symbolic-ref", "--quiet", "--short", "HEAD").Output(); err == nil {
		st.Branch = strings.TrimSpace(string(out))
	}
	out, err = gitutil.Command(ctx, dir, "status", "--porcelain", "--untracked-files=normal", "--", ".", ":(exclude).beads").Output()
	if err != nil {
		return workflow.GitState{}, err
	}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			st.Dirty = append(st.Dirty, strings.TrimSpace(line))
		}
	}
	return st, nil
}

// preconditionDirtyShown is the number of changes named in the message.
const preconditionDirtyShown = 5

// preconditionDetail says what failed ("" for a missing path: the label
// says it).
func preconditionDetail(f *workflow.PreconditionFailure) string {
	if f == nil {
		return ""
	}
	switch f.Kind {
	case workflow.FailNotGit:
		return i18n.Tf("cmd.run.precondition.not_git", f.Err)
	case workflow.FailDirty:
		shown := f.Dirty
		more := ""
		if len(shown) > preconditionDirtyShown {
			shown = shown[:preconditionDirtyShown]
			more = i18n.Tf("cmd.run.precondition.more", len(f.Dirty)-preconditionDirtyShown)
		}
		return i18n.Tf("cmd.run.precondition.dirty", len(f.Dirty), strings.Join(shown, ", ")+more)
	case workflow.FailBranch:
		if f.Branch == "" {
			return i18n.Tf("cmd.run.precondition.detached", strings.Join(f.Want, ", "))
		}
		return i18n.Tf("cmd.run.precondition.branch", f.Branch, strings.Join(f.Want, ", "))
	case workflow.FailTools:
		return i18n.Tf("cmd.run.precondition.tools", strings.Join(f.Missing, ", "))
	}
	return ""
}

// preconditionWarnings evaluates the workflow preconditions on the project
// directory: a failed `block` refuses the launch, a failed `suggest` is a
// warning naming the workflow to run first.
func preconditionWarnings(sp *workflow.Spec, dir string) ([]runsvc.Warning, []Suggestion, error) {
	return preconditionWarningsIn(sp, os.DirFS(dir), preconditionProbe(dir))
}

func preconditionWarningsIn(sp *workflow.Spec, fsys fs.FS, probe workflow.PreconditionProbe) ([]runsvc.Warning, []Suggestion, error) {
	var (
		out         []runsvc.Warning
		suggestions []Suggestion
	)
	for _, r := range workflow.FailedPreconditions(workflow.EvaluatePreconditionsWith(sp, fsys, probe)) {
		label := r.Label.Text(i18n.Locale())
		if label == "" {
			label = r.ID
		}
		if d := preconditionDetail(r.Failure); d != "" {
			label += " — " + d
		}
		switch {
		case r.Block:
			return nil, nil, errors.New(i18n.Tf("cmd.run.precondition_block", label))
		case r.Suggest != "":
			out = append(out, runsvc.Warning{Code: "precondition", Args: []any{label, r.Suggest}})
			suggestions = append(suggestions, Suggestion{Label: label, Workflow: r.Suggest, Resume: r.Resume})
		default:
			out = append(out, runsvc.Warning{Code: "precondition_failed", Args: []any{label}})
		}
	}
	return out, suggestions, nil
}
