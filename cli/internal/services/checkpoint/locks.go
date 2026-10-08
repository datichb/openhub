package checkpoint

import (
	"context"
	"errors"
	"os/exec"
	"regexp"
	"strings"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
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
// session committed since the checkpoint (or has nothing to commit).

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

func (s *Service) head(ctx context.Context, dir string) (string, bool) {
	if dir == "" {
		return "", false
	}
	h := s.Head
	if h == nil {
		h = GitHead
	}
	head, dirty, err := h(ctx, dir)
	if err != nil {
		return "", false
	}
	return head, dirty
}

// GitHead returns the commit of dir ("" before the first commit) and whether
// it has uncommitted changes outside .beads (Beads writes its own files).
func GitHead(ctx context.Context, dir string) (head string, dirty bool, err error) {
	if _, err = exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--git-dir").Output(); err != nil {
		return "", false, err
	}
	if out, herr := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--verify", "-q", "HEAD").Output(); herr == nil {
		head = strings.TrimSpace(string(out))
	}
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "status", "--porcelain", "--", ".", ":(exclude).beads").Output()
	if err != nil {
		return head, false, err
	}
	return head, strings.TrimSpace(string(out)) != "", nil
}

// GuardClose checks that root may close a ticket now: the checkpoint that
// unlocks closing is passed, and the work was committed since. dir is the
// session location on the machine. nil when the workflow locks nothing.
func (s *Service) GuardClose(ctx context.Context, root, dir string) error {
	sess, wf, err := s.session(ctx, root)
	if errors.Is(err, ErrNoWorkflow) {
		return nil
	}
	if err != nil {
		return err
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
	cur, dirty := s.head(ctx, dir)
	if cur == u.Head && dirty {
		return &LockError{Msg: i18n.Tf("cmd.gateway.beads.locked_commit", c.ID, label)}
	}
	return nil
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
			}
			if c, ok := bundle.UnlockingCheckpoint(wf, sessionspec.UnlockClose); ok {
				changed = relock(st, c.ID)
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
// closed with no other checkpoint passed since the last closing.
func finished(wf *sessionspec.WorkflowRuntime, mode string, st domain.CheckpointState) bool {
	if wf == nil || st.Waiting != "" {
		return false
	}
	last := ""
	for _, c := range wf.Checkpoints {
		if c.Behavior(mode) == sessionspec.CheckpointSkip {
			continue
		}
		if c.Mandatory && !st.HasPassed(c.ID) {
			return false
		}
		last = c.ID
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
		if _, ok := sess.Outputs[o.ID]; ok {
			continue
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
