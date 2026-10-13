package checkpoint

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Test written first (R7, method tdd): at cp-red oh checks that the changes
// since cp-seams touch the test files of the stack (must_touch_tests), and
// what was changed outside them (outside_tests: warn, refuse), then that the
// tests fail for the right reason (expect: fail): the code builds, the
// tests build and run, and one of them fails.
//
//	since cp-seams ── no test file ──────────── refused
//	               ── other files, refuse ───── refused
//	               ── other files, warn ─────── noted on the result
//	commands ── build / typecheck / lint fail ─ refused (not a red)
//	         ── tests do not build, none ran ── refused (not a red)
//	         ── a test fails ────────────────── passed
//	         ── every test passes ───────────── refused

// maxListed bounds the files named in a reason.
const maxListed = 5

// baseHead is the commit of dir ("" = unknown or no commit yet), read as
// the account of the agents in a remote job.
func (s *Service) baseHead(ctx context.Context, dir string) string {
	if dir == "" {
		return ""
	}
	if s.Head != nil {
		head, _, err := s.Head(ctx, dir)
		if err != nil {
			return ""
		}
		return head
	}
	out, err := gitAs(ctx, dir, s.Account, "rev-parse", "--verify", "-q", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// setBase records the commit a later verification counts the changes from
// ("" = unknown: the count starts at the start of the session).
func setBase(st *domain.CheckpointState, id, head string) {
	if head == "" {
		delete(st.Bases, id)
		return
	}
	if st.Bases == nil {
		st.Bases = map[string]string{}
	}
	st.Bases[id] = head
}

// changesFor are the paths the session changed for verification v: since
// the commit recorded at its `since:` checkpoint, else since the start of
// the session.
func (s *Service) changesFor(ctx context.Context, sess *domain.Session, v *sessionspec.VerifyDef) ([]string, error) {
	since := sess.StartRef
	if v.Since != "" {
		if st, err := s.state(ctx, sess.ID); err == nil && st.Bases[v.Since] != "" {
			since = st.Bases[v.Since]
		}
	}
	if s.Changed != nil {
		return s.Changed(ctx, sess.LaunchPath, since)
	}
	return gitChanged(ctx, sess.LaunchPath, since, s.Account)
}

// changesCheck checks the paths changed against must_touch and
// must_touch_tests: a failed result when they are not met (ok false), else
// the note on the files changed outside the tests (outside_tests: warn).
func (s *Service) changesCheck(ctx context.Context, sess *domain.Session, v *sessionspec.VerifyDef) (failed domain.CheckpointVerify, note string, ok bool) {
	if len(v.MustTouch) == 0 && !v.MustTouchTests {
		return domain.CheckpointVerify{}, "", true
	}
	paths, err := s.changesFor(ctx, sess, v)
	if len(v.MustTouch) > 0 && (err != nil || !touches(paths, v.MustTouch)) {
		return domain.CheckpointVerify{Status: domain.VerifyFailed,
			Reason: i18n.Tf("tui.checkpoint.verify.must_touch", strings.Join(v.MustTouch, ", "))}, "", false
	}
	if !v.MustTouchTests {
		return domain.CheckpointVerify{}, "", true
	}
	patterns := bricks.TestFilePatterns(bricks.DetectSignals(sess.LaunchPath))
	tests, outside := splitTests(paths, patterns)
	if err != nil || len(tests) == 0 {
		return domain.CheckpointVerify{Status: domain.VerifyFailed,
			Reason: i18n.Tf("tui.checkpoint.verify.must_touch_tests", sinceText(v), strings.Join(patterns, ", "))}, "", false
	}
	if len(outside) == 0 {
		return domain.CheckpointVerify{}, "", true
	}
	switch v.OutsideTests {
	case sessionspec.OutsideTestsRefuse:
		return domain.CheckpointVerify{Status: domain.VerifyFailed,
			Reason: i18n.Tf("tui.checkpoint.verify.outside_tests_refused", listed(outside))}, "", false
	case sessionspec.OutsideTestsWarn:
		return domain.CheckpointVerify{}, i18n.Tf("tui.checkpoint.verify.outside_tests", listed(outside)), true
	}
	return domain.CheckpointVerify{}, "", true
}

// splitTests separates the test files (patterns) from the other paths.
func splitTests(paths, patterns []string) (tests, outside []string) {
	for _, p := range paths {
		if touches([]string{p}, patterns) {
			tests = append(tests, p)
		} else {
			outside = append(outside, p)
		}
	}
	return tests, outside
}

func sinceText(v *sessionspec.VerifyDef) string {
	if v.Since == "" {
		return i18n.T("tui.checkpoint.verify.since_start")
	}
	return i18n.Tf("tui.checkpoint.verify.since", v.Since)
}

// listed names at most maxListed paths.
func listed(paths []string) string {
	if len(paths) <= maxListed {
		return strings.Join(paths, ", ")
	}
	return fmt.Sprintf("%s… (+%d)", strings.Join(paths[:maxListed], ", "), len(paths)-maxListed)
}

// redOrder puts the commands that build the code (build, typecheck, lint)
// before the tests: with expect: fail they must pass, so that a failure of
// the tests is a red and not a broken build.
func redOrder(runs []bricks.VerifyRun) []bricks.VerifyRun {
	out := slices.Clone(runs)
	slices.SortStableFunc(out, func(a, b bricks.VerifyRun) int { return redRank(a) - redRank(b) })
	return out
}

func redRank(r bricks.VerifyRun) int {
	if isTestRun(r) {
		return 1
	}
	return 0
}

// isTestRun: a test command of the stack, or a command of the workflow
// document (expected to fail with expect: fail).
func isTestRun(r bricks.VerifyRun) bool {
	return r.Kind == "" || r.Kind == bricks.VerifyKindTest
}

// wrongRed is the reason a failure is not the expected red ("" = it is):
// a command that builds the code failed, or the tests did not build or run.
func wrongRed(r bricks.VerifyRun, exit int, out []byte) string {
	if !isTestRun(r) {
		return i18n.Tf("tui.checkpoint.verify.red_build", r.String())
	}
	if why := bricks.RedFailure(r.Args, exit, out); why != "" {
		return i18n.Tf("tui.checkpoint.verify.red_"+why, r.String())
	}
	return ""
}

// withNote adds the note of the changes check to a result.
func withNote(r domain.CheckpointVerify, note string) domain.CheckpointVerify {
	if note == "" {
		return r
	}
	if r.Reason != "" {
		r.Reason += " ; "
	}
	r.Reason += note
	return r
}
