package cmd

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// R8 (E7): preconditions on the repository and the tools, read by oh at
// launch on the project directory.

// preconditionGitTimeout bounds each git call of the preconditions.
const preconditionGitTimeout = 10 * time.Second

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
