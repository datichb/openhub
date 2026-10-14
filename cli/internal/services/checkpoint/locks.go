package checkpoint

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Operations locked by a checkpoint (`unlocks:`, A17, A36, A38): a
// mandatory checkpoint that is not passed makes the work that depends on it
// impossible, whatever the model does.
//
//	cp passed ── window open (commit, push, close) ── ticket closed ── locked again
//	                                                └ cp asked again ┘
//
// The shell side is held by the session rules (bundle.SessionRules); the
// Beads side by the oh gateway, which asks GuardClose before closing a
// ticket and reports what ran (BeadsDone). A ticket is closed only once the
// session made a non-empty commit since the checkpoint and has nothing left
// to commit.

// Beads operations reported by the gateway.
const (
	BeadsClaim = "claim"
	BeadsClose = "close"
)

// timelineClosed marks a ticket closed in the timeline (not rendered).
const timelineClosed = "closed"

// LockError is an operation refused by the workflow state (message for the agent).
type LockError struct{ Msg string }

func (e *LockError) Error() string { return e.Msg }

func (s *Service) headErr(ctx context.Context, dir string) (head string, dirty bool, err error) {
	if dir == "" {
		return "", false, nil
	}
	h := s.Head
	if h == nil {
		h = GitHead
	}
	if head, dirty, err = h(ctx, dir); err != nil {
		return "", false, err
	}
	return head, dirty, nil
}

// GitHead returns the commit of dir ("" before the first commit) and whether
// it has uncommitted changes outside .beads (Beads writes its own files).
func GitHead(ctx context.Context, dir string) (head string, dirty bool, err error) {
	if err := gitutil.Check(ctx, dir); err != nil {
		return "", false, err
	}
	if _, err = gitutil.Command(ctx, dir, "rev-parse", "--git-dir").Output(); err != nil {
		return "", false, err
	}
	if out, herr := gitutil.Command(ctx, dir, "rev-parse", "--verify", "-q", "HEAD").Output(); herr == nil {
		head = strings.TrimSpace(string(out))
	}
	out, err := gitutil.Command(ctx, dir, "status", "--porcelain", "--", ".", ":(exclude).beads").Output()
	if err != nil {
		return head, false, err
	}
	return head, strings.TrimSpace(string(out)) != "", nil
}

// GuardClose checks that root may close a ticket now: the checkpoint that
// unlocks closing is passed, and the work was committed since (audit v5,
// S-09): HEAD moved from the commit of the checkpoint, on top of it, with
// at least one change to tracked files outside .beads, and nothing left to
// commit. dir is the session location on the machine. nil when the
// workflow locks nothing; any error refuses (the guard fails closed).
func (s *Service) GuardClose(ctx context.Context, root, dir string) error {
	sess, wf, err := s.session(ctx, root)
	if errors.Is(err, ErrNoWorkflow) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := guardFanoutClose(sess); err != nil {
		return err // R6: oh closes the tickets it merged
	}
	c, ok := bundle.UnlockingCheckpoint(wf, sessionspec.UnlockClose)
	if !ok || c.Behavior(sess.Mode) == sessionspec.CheckpointSkip {
		return nil
	}
	st, err := s.state(ctx, root)
	if err != nil {
		return err
	}
	label := c.LabelFor(i18n.Locale())
	u, open := st.Unlocks[sessionspec.UnlockClose]
	if !open {
		return &LockError{Msg: i18n.Tf("cmd.gateway.beads.locked_checkpoint", c.ID, label)}
	}
	if dir == "" {
		dir = sess.LaunchPath
	}
	work := s.Work
	if work == nil {
		work = GitWork
	}
	w, err := work(ctx, dir, u.Head, u.At)
	switch {
	case err != nil:
		return &LockError{Msg: i18n.Tf("cmd.gateway.beads.locked_git", c.ID, label, err)}
	case !w.Committed:
		return &LockError{Msg: i18n.Tf("cmd.gateway.beads.locked_commit", c.ID, label)}
	case w.Dirty:
		return &LockError{Msg: i18n.Tf("cmd.gateway.beads.locked_dirty", c.ID, label)}
	}
	return nil
}

// Work is what a directory committed since a commit.
type Work struct {
	Head  string // current commit
	Dirty bool   // changes not committed, outside .beads
	// Committed: HEAD moved on top of the commit, and the commits since
	// change tracked files outside .beads (an empty commit does not count).
	Committed bool
}

// errUnknownStart: the commit of the checkpoint is not known (git failed
// when it passed) while the repository already had commits.
var errUnknownStart = errors.New("the commit at the checkpoint is not known")

// GitWork tells what dir committed since the commit since ("" = no commit
// when the checkpoint passed at at).
func GitWork(ctx context.Context, dir, since string, at time.Time) (Work, error) {
	head, dirty, err := GitHead(ctx, dir)
	if err != nil {
		return Work{}, err
	}
	w := Work{Head: head, Dirty: dirty}
	if head == "" || head == since {
		return w, nil
	}
	git := func(args ...string) *exec.Cmd { return gitutil.Command(ctx, dir, args...) }
	from := since
	if since == "" {
		// No commit at the checkpoint: every commit must be newer.
		out, err := git("rev-list", "--max-count=1", fmt.Sprintf("--before=%d", at.Add(-time.Second).Unix()), "HEAD").Output()
		if err != nil {
			return w, err
		}
		if strings.TrimSpace(string(out)) != "" {
			return w, errUnknownStart
		}
		empty := git("hash-object", "-t", "tree", "--stdin")
		empty.Stdin = strings.NewReader("")
		out, err = empty.Output()
		if err != nil {
			return w, err
		}
		from = strings.TrimSpace(string(out))
	} else if err := git("merge-base", "--is-ancestor", since, head).Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return w, nil // HEAD is elsewhere: not committed on top of the checkpoint
		}
		return w, err
	}
	err = git("diff", "--quiet", from, head, "--", ".", ":(exclude).beads").Run()
	var exit *exec.ExitError
	switch {
	case err == nil: // no change: empty commits only
	case errors.As(err, &exit) && exit.ExitCode() == 1:
		w.Committed = true
	default:
		return w, err
	}
	return w, nil
}

// BeadsDone records a Beads operation that ran for root (claimed or closed
// tickets). changed reports that the session rules changed (a ticket closed
// locks the operations again).
func (s *Service) BeadsDone(ctx context.Context, root, op string, ids []string) (changed bool, err error) {
	_, wf, err := s.session(ctx, root)
	if errors.Is(err, ErrNoWorkflow) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = s.update(ctx, root, func(st *domain.CheckpointState) error {
		switch op {
		case BeadsClaim:
			n := len(st.Claimed)
			st.Claimed = appendNew(st.Claimed, ids...)
			if len(st.Claimed) == n {
				return errUnchanged
			}
		case BeadsClose:
			st.Closed = appendNew(st.Closed, ids...)
			for _, id := range ids {
				s.record(st, timelineClosed, id, "", "")
				if resetRounds(st, id) && st.Unresolved == "" {
					changed = true // the developer is unlocked (E5)
				}
			}
			if c, ok := bundle.UnlockingCheckpoint(wf, sessionspec.UnlockClose); ok {
				changed = relockOnClose(st, c.ID) || changed
			}
		default:
			return errUnchanged
		}
		return nil
	})
	if errors.Is(err, errUnchanged) {
		return false, nil
	}
	return changed, err
}

func appendNew(list []string, ids ...string) []string {
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" && !containsFold(list, id) {
			list = append(list, id)
		}
	}
	return list
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

// finished reports whether the workflow of a session reached its end: every
// mandatory checkpoint passed, nothing waiting, and either the last
// checkpoint is the latest one passed, or every ticket the session took is
// closed with no other checkpoint passed since the last closing. A session
// whose only passed checkpoint is the last one (plan only) ends too.
func finished(wf *sessionspec.WorkflowRuntime, mode string, st domain.CheckpointState) bool {
	if wf == nil || st.Waiting != "" {
		return false
	}
	last := ""
	missing := false
	for _, c := range wf.Checkpoints {
		if c.Behavior(mode) == sessionspec.CheckpointSkip {
			continue
		}
		if c.Mandatory && !st.HasPassed(c.ID) {
			missing = true
		}
		last = c.ID
	}
	if missing {
		// A session that went straight to its last checkpoint passed nothing
		// else: nothing was unlocked nor done (a plan-only run, sweep
		// dry_run, N7). It ends there.
		return last != "" && len(st.Passed) == 1 && st.HasPassed(last) && !wfMandatory(wf, last)
	}
	lastPass, lastClose := -1, -1
	passedSinceClose := false
	for i, e := range st.Timeline {
		switch e.Kind {
		case domain.CheckpointPassed:
			lastPass = i
			if lastClose >= 0 && !strings.EqualFold(e.ID, last) {
				passedSinceClose = true
			}
		case timelineClosed:
			lastClose, passedSinceClose = i, false
		}
	}
	if last != "" && lastPass >= 0 && strings.EqualFold(st.Timeline[lastPass].ID, last) {
		return true
	}
	if lastClose < 0 || passedSinceClose {
		return false
	}
	for _, id := range st.Claimed {
		if !containsFold(st.Closed, id) {
			return false
		}
	}
	return true
}

func wfMandatory(wf *sessionspec.WorkflowRuntime, id string) bool {
	c, ok := wf.Checkpoint(id)
	return ok && (c.Mandatory || len(c.Unlocks) > 0)
}

// Finish marks the end of the workflow of root when it is reached (called
// when its step ended with nothing waiting) and declares the outputs the
// agent did not: the tickets closed and the branch (read only then, empty =
// unknown). done reports that the session is now completed.
func (s *Service) Finish(ctx context.Context, root string, branch func() string) (done bool, err error) {
	sess, wf, err := s.session(ctx, root)
	if errors.Is(err, ErrNoWorkflow) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var st domain.CheckpointState
	st, err = s.update(ctx, root, func(st *domain.CheckpointState) error {
		if st.Finished != nil || !finished(wf, sess.Mode, *st) {
			return errUnchanged
		}
		now := s.now()
		st.Finished = &now
		return nil
	})
	if errors.Is(err, errUnchanged) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, o := range wf.Outputs {
		if _, ok := sess.Outputs[o.ID]; ok || o.Record != "" {
			continue // declared, or recorded by oh as it happens (R5)
		}
		value := ""
		switch o.Type {
		case "beads-ids":
			value = strings.Join(st.Closed, ",")
		case "branch":
			if branch != nil {
				value = branch()
			}
		}
		if value != "" {
			_ = s.Declare(ctx, root, Output{ID: o.ID, Type: o.Type, Value: value})
		}
	}
	return true, nil
}

// Reopen forgets the end of the workflow of root (the user went on).
func (s *Service) Reopen(ctx context.Context, root string) {
	_, _ = s.update(ctx, root, func(st *domain.CheckpointState) error {
		if st.Finished == nil {
			return errUnchanged
		}
		st.Finished = nil
		return nil
	})
}

// Imitation tells whether a question of root imitates a checkpoint of its
// workflow (its text names a checkpoint id or label): checkpoints only pass
// through workflow_checkpoint. The imitation is recorded in the timeline.
func (s *Service) Imitation(ctx context.Context, root string, p adapters.PendingDecision) (sessionspec.CheckpointDef, bool) {
	if p.Kind != adapters.DecisionQuestion {
		return sessionspec.CheckpointDef{}, false
	}
	_, wf, err := s.session(ctx, root)
	if err != nil || wf == nil {
		return sessionspec.CheckpointDef{}, false
	}
	parts := []string{p.Title, p.Message}
	for _, f := range p.Fields {
		parts = append(parts, f.Title, f.Description)
		for _, o := range f.Options {
			parts = append(parts, o.Label, o.Description)
		}
	}
	c, ok := imitated(wf, strings.Join(parts, "\n"))
	if !ok {
		return c, false
	}
	_, _ = s.update(ctx, root, func(st *domain.CheckpointState) error {
		if n := len(st.Timeline); n > 0 && st.Timeline[n-1].Kind == domain.CheckpointImitated && st.Timeline[n-1].ID == c.ID {
			return errUnchanged
		}
		s.record(st, domain.CheckpointImitated, c.ID, "", p.Title)
		return nil
	})
	return c, true
}

// minLabel is the shortest checkpoint label matched in a question.
const minLabel = 8

func imitated(wf *sessionspec.WorkflowRuntime, text string) (sessionspec.CheckpointDef, bool) {
	lower := strings.ToLower(text)
	for _, c := range wf.Checkpoints {
		if idPattern(c.ID).MatchString(text) {
			return c, true
		}
		for _, l := range c.Label {
			if l = strings.ToLower(strings.TrimSpace(l)); len(l) >= minLabel && strings.Contains(lower, l) {
				return c, true
			}
		}
	}
	return sessionspec.CheckpointDef{}, false
}

// idPattern matches a checkpoint id as a word, separators optional
// ("cp-2", "CP 2", "cp2", "[CP-2]").
func idPattern(id string) *regexp.Regexp {
	words := strings.FieldsFunc(id, func(r rune) bool { return r == '-' || r == '_' })
	for i, w := range words {
		words[i] = regexp.QuoteMeta(w)
	}
	return regexp.MustCompile(`(?i)(^|[^\p{L}\p{N}])` + strings.Join(words, `[-_ ]?`) + `($|[^\p{L}\p{N}])`)
}
