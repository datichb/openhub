package runsvc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/daemon/api"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/filelock"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// Session deletion (`oh session delete`, launch with OnExistingDelete): the
// session and its work go for good; its remote branch is never touched.

// DeleteOptions tune a deletion.
type DeleteOptions struct {
	// ProjectPath is the project directory ("" = the main worktree of the
	// session's repository).
	ProjectPath string
	// DiscardBaseChanges discards the uncommitted changes of a session that
	// worked in the project directory (kept otherwise).
	DiscardBaseChanges bool
}

// DeletePlan is what deleting a session loses (confirmation).
type DeletePlan struct {
	SessionID  string          `json:"session_id"`
	State      domain.RunState `json:"state"`
	LaunchPath string          `json:"path,omitempty"`
	// InBase: the session worked in the project directory (no worktree to
	// remove; its changes are discarded only with DiscardBaseChanges).
	InBase bool `json:"in_base"`
	// Worktree is the separate worktree removed ("" = none).
	Worktree string `json:"worktree,omitempty"`
	// SharedWith are the other open sessions working in that worktree: it
	// is then kept, with its branch.
	SharedWith []string `json:"shared_with,omitempty"`
	// Branch is the local branch deleted ("" = none); RemoteBranch tells
	// that a remote branch of that name exists (kept).
	Branch       string `json:"branch,omitempty"`
	RemoteBranch bool   `json:"remote_branch,omitempty"`
	// UnpushedCommits are the commits of the session found on no remote
	// branch (lost with the branch).
	UnpushedCommits []string `json:"unpushed_commits,omitempty"`
	// ChangedFiles are the uncommitted changes of its directory.
	ChangedFiles []string `json:"changed_files,omitempty"`
	// SessionDir is ~/.oh/sessions/<id>; GroupData the tool data of its
	// server group, removed when no other session uses the group.
	SessionDir string `json:"session_dir,omitempty"`
	GroupData  string `json:"group_data,omitempty"`
	// OpenDecisions are the decisions still waiting in the session.
	OpenDecisions int `json:"open_decisions"`
}

// Losses reports whether the deletion loses work (commits or uncommitted
// changes), the changes of the project directory being discarded or not.
func (p *DeletePlan) Losses(discardBase bool) bool {
	if p.InBase {
		return discardBase && len(p.ChangedFiles) > 0
	}
	if p.Worktree == "" || len(p.SharedWith) > 0 {
		return false
	}
	return len(p.UnpushedCommits) > 0 || len(p.ChangedFiles) > 0
}

// DeleteReport is what a deletion did.
type DeleteReport struct {
	SessionID       string   `json:"session_id"`
	Stopped         bool     `json:"stopped"`
	WorktreeRemoved string   `json:"worktree_removed,omitempty"`
	BranchDeleted   string   `json:"branch_deleted,omitempty"`
	RemoteBranch    bool     `json:"remote_branch_kept,omitempty"`
	Discarded       bool     `json:"base_changes_discarded,omitempty"`
	BaseChangesKept bool     `json:"base_changes_kept,omitempty"`
	WorktreeKept    string   `json:"worktree_kept,omitempty"` // shared with other sessions
	GroupData       string   `json:"group_data_removed,omitempty"`
	Errors          []string `json:"errors,omitempty"`
}

// ErrDeleteUnsupported is returned when the session store cannot delete rows.
var ErrDeleteUnsupported = errors.New("the session store cannot delete sessions")

// PlanDelete lists what deleting a session loses. Nothing is changed.
func (s *Service) PlanDelete(ctx context.Context, sessionID string, opts DeleteOptions) (*DeletePlan, error) {
	sess, err := s.Session(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	p := &DeletePlan{SessionID: sess.ID, State: sess.State, LaunchPath: sess.LaunchPath}
	if s.SessionsDir != "" {
		if d := filepath.Join(s.SessionsDir, sess.ID); dirExists(d) {
			p.SessionDir = d
		}
	}
	if s.Decisions != nil {
		if open, err := s.Decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: sess.ID}); err == nil {
			p.OpenDecisions = len(open)
		}
	}
	others, _ := s.Sessions.List(ctx, sess.ProjectID)
	if sess.GroupKey != "" && s.ServersDir != "" {
		used := false
		for _, o := range others {
			if o.ID != sess.ID && o.GroupKey == sess.GroupKey {
				used = true
				break
			}
		}
		if d := filepath.Join(s.ServersDir, sess.GroupKey); !used && dirExists(d) {
			p.GroupData = d
		}
	}
	dir := sess.LaunchPath
	if dir == "" || !dirExists(dir) || !worktree.IsGitRepo(dir) || sess.Runtime == "remote" {
		return p, nil
	}
	main := opts.ProjectPath
	if main == "" {
		main = mainWorktree(dir)
	}
	p.InBase = samePath(dir, main)
	if lines, err := gitLines(dir, "status", "--porcelain", "--untracked-files=all"); err == nil {
		for _, l := range lines {
			if len(l) > 3 {
				p.ChangedFiles = append(p.ChangedFiles, l[3:])
			}
		}
	}
	if p.InBase {
		return p, nil
	}
	p.Worktree = filepath.Clean(dir)
	for _, o := range others {
		if o.ID != sess.ID && openState(o.State) && o.LaunchPath != "" && samePath(o.LaunchPath, dir) {
			p.SharedWith = append(p.SharedWith, o.ID)
		}
	}
	branch := sess.WorktreeBranch
	if cur, err := worktree.CurrentBranch(dir); err == nil && !strings.HasPrefix(cur, "(detached)") && cur != "" {
		branch = cur
	}
	if branch != "" && branch != worktree.DetectBaseBranch(main) {
		p.Branch = branch
		if refs, err := gitLines(main, "for-each-ref", "--format=%(refname)", "refs/remotes/*/"+branch); err == nil && len(refs) > 0 {
			p.RemoteBranch = true
		}
	}
	if remotes, err := gitLines(dir, "remote"); err == nil && len(remotes) == 0 {
		p.UnpushedCommits, _ = gitLines(dir, "log", "--oneline", "--no-decorate", worktree.DetectBaseBranch(main)+"..HEAD")
	} else {
		args := []string{"log", "--oneline", "--no-decorate", "HEAD", "--not", "--remotes"}
		if branch != "" {
			args = append(args, "--exclude="+branch)
		}
		p.UnpushedCommits, _ = gitLines(dir, append(args, "--branches")...)
	}
	return p, nil
}

// DeleteSession deletes a session for good: it is stopped (team claims
// released), its separate worktree removed (kept when other open sessions
// use it), its local branch deleted (never the remote one), the changes of
// the project directory discarded with DiscardBaseChanges only, then its
// record, decisions, folder and, when no other session uses its server
// group, the tool data of the group.
func (s *Service) DeleteSession(ctx context.Context, sessionID string, opts DeleteOptions) (*DeleteReport, error) {
	del, ok := s.Sessions.(domain.SessionDeleter)
	if !ok {
		return nil, ErrDeleteUnsupported
	}
	plan, err := s.PlanDelete(ctx, sessionID, opts)
	if err != nil {
		return nil, err
	}
	rep := &DeleteReport{SessionID: sessionID, RemoteBranch: plan.RemoteBranch}
	if !terminal(plan.State) || plan.State == domain.RunCompleted {
		if err := s.StopSession(ctx, sessionID); err != nil {
			return rep, fmt.Errorf("stopping the session: %w", err)
		}
		rep.Stopped = true
	}
	fail := func(what string, err error) { rep.Errors = append(rep.Errors, what+": "+err.Error()) }
	switch {
	case plan.Worktree != "" && len(plan.SharedWith) > 0:
		rep.WorktreeKept = plan.Worktree
	case plan.Worktree != "" && !dirExists(plan.Worktree):
		rep.WorktreeRemoved = plan.Worktree // a temporary worktree, removed with the stop of its session
	case plan.Worktree != "":
		main := opts.ProjectPath
		if main == "" {
			main = mainWorktree(plan.Worktree)
		}
		if err := worktree.Remove(main, plan.Worktree, true); err != nil {
			fail("worktree", err)
		} else {
			rep.WorktreeRemoved = plan.Worktree
			if plan.Branch != "" && !strings.HasPrefix(plan.Branch, "-") {
				if _, err := gitOutput(main, "branch", "-D", "--", plan.Branch); err != nil {
					fail("branch", err)
				} else {
					rep.BranchDeleted = plan.Branch
				}
			}
		}
	case plan.InBase && len(plan.ChangedFiles) > 0:
		if !opts.DiscardBaseChanges {
			rep.BaseChangesKept = true
			break
		}
		if _, err := gitOutput(plan.LaunchPath, "reset", "--hard", "-q"); err != nil {
			fail("reset", err)
		} else if _, err := gitOutput(plan.LaunchPath, "clean", "-fdq"); err != nil {
			fail("clean", err)
		} else {
			rep.Discarded = true
		}
	}
	if err := del.Delete(ctx, sessionID); err != nil {
		return rep, fmt.Errorf("deleting the session record: %w", err)
	}
	if plan.SessionDir != "" {
		if err := os.RemoveAll(plan.SessionDir); err != nil {
			fail("session folder", err)
		}
	}
	if plan.GroupData != "" {
		if err := s.removeGroupData(ctx, plan.GroupData); err != nil {
			fail("server group", err)
		} else {
			rep.GroupData = plan.GroupData
		}
	}
	return rep, nil
}

// removeGroupData removes the folder of a server group whose server is not
// running, with its lock held.
func (s *Service) removeGroupData(ctx context.Context, dir string) error {
	gk := filepath.Base(dir)
	if srv, err := s.Servers.Get(ctx, gk); err == nil && srv.Status == domain.ServerReady && filelock.ProcessAlive(srv.PID) {
		return errors.New("its server is running")
	}
	unlock, err := s.lockGroup(ctx, gk)
	if err != nil {
		return err
	}
	defer unlock()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	lock := filepath.Clean(api.GroupLockPath(s.ServersDir, gk))
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if filepath.Clean(p) == lock {
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	_ = s.Servers.Delete(ctx, gk)
	return nil
}

// mainWorktree is the main working tree of the repository of dir.
func mainWorktree(dir string) string {
	common, err := gitOutput(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return dir
	}
	return filepath.Dir(strings.TrimSpace(common))
}

func samePath(a, b string) bool {
	ra, err1 := filepath.EvalSymlinks(a)
	rb, err2 := filepath.EvalSymlinks(b)
	if err1 != nil || err2 != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
