package checkpoint

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// R6, CP-R3: the control substitution in the daemon, and every attempt to
// get around it.

var allPause = func() map[string]string {
	return map[string]string{"manuel": "pause", "semi-auto": "pause", "auto": "pause"}
}

func fanoutParentWF() *sessionspec.WorkflowRuntime {
	return &sessionspec.WorkflowRuntime{ID: "implement-spec", DefaultMode: "semi-auto",
		Checkpoints: []sessionspec.CheckpointDef{
			{ID: "cp-plan", Mandatory: true, Behaviors: allPause()},
			{ID: "cp-merge", Mandatory: true, Behaviors: allPause(), Unlocks: []string{sessionspec.UnlockMerge},
				Verify: &sessionspec.VerifyDef{Cmd: []string{"make", "check"}, Expect: "pass", On: sessionspec.VerifyOnIntegration}},
			{ID: "cp-final", Behaviors: map[string]string{"manuel": "pause", "semi-auto": "pause", "auto": "auto"}},
		},
		Fanout: &sessionspec.FanoutDef{Child: "ticket", Tickets: "tickets", Parallel: 1, Merge: "cp-merge", Delegates: []string{"cp-2"}},
	}
}

func fanoutChildWF() *sessionspec.WorkflowRuntime {
	return &sessionspec.WorkflowRuntime{ID: "ticket", DefaultMode: "semi-auto",
		Checkpoints: []sessionspec.CheckpointDef{
			{ID: "cp-1", Behaviors: map[string]string{"manuel": "pause", "semi-auto": "auto", "auto": "auto"}},
			{ID: "cp-2", Mandatory: true, Behaviors: allPause(), Unlocks: []string{sessionspec.UnlockCommit, sessionspec.UnlockClose},
				Verify: &sessionspec.VerifyDef{Cmd: []string{"go", "test"}, Expect: "pass"}},
		},
	}
}

type fanoutFixture struct {
	svc            *Service
	ctx            context.Context
	childDir, intg string
	heads          map[string]string
	mu             sync.Mutex
	execDirs       []string
	exit           int
	integrated     int
	integrateErr   error
}

func (f *fanoutFixture) run(_ context.Context, dir string, _, _ []string, _ *TailBuffer) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.execDirs = append(f.execDirs, dir)
	return f.exit, nil
}

func parentRecord(child string) *domain.SessionFanout {
	return &domain.SessionFanout{Role: domain.RoleParent, Experimental: true, Status: domain.FanoutRunning,
		Children: []domain.FanoutChild{{Ticket: "ev-1", Session: child, State: domain.ChildRunning}}}
}

func newFanoutFixture(t *testing.T) *fanoutFixture {
	t.Helper()
	svc, ctx := newTestService(t)
	f := &fanoutFixture{svc: svc, ctx: ctx, childDir: t.TempDir(), intg: t.TempDir(), heads: map[string]string{}}
	f.heads[f.intg] = "i1"
	f.heads[f.childDir] = "c1"
	svc.Exec = f.run
	svc.Fingerprint = func(_ context.Context, dir string) string { return "fp-" + dir }
	svc.Head = func(_ context.Context, dir string) (string, bool, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		h, ok := f.heads[dir]
		if !ok {
			return "", false, errors.New("unexpected dir " + dir)
		}
		return h, false, nil
	}
	svc.Integrate = func(_ context.Context, sess *domain.Session, cp string) (string, error) {
		f.integrated++
		return "merged by oh", f.integrateErr
	}
	writeBundle(t, svc.BundlesDir, "h_parent", fanoutParentWF())
	writeBundle(t, svc.BundlesDir, "h_child", fanoutChildWF())
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_p", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_parent", Mode: "semi-auto", State: domain.RunIdle, LaunchPath: t.TempDir(), Tickets: []string{"ev-1"}, Fanout: parentRecord("ses_c")}))
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_c", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g2",
		BundleHash: "h_child", Mode: "semi-auto", State: domain.RunActive, LaunchPath: f.childDir, ParentSessionID: "ses_p",
		Fanout: &domain.SessionFanout{Role: domain.RoleChild, Parent: "ses_p", Ticket: "ev-1", Branch: "oh/fanout/p/ev-1"}}))
	return f
}

func (f *fanoutFixture) setParent(t *testing.T, mut func(*domain.Session)) {
	t.Helper()
	p, err := f.svc.Sessions.Get(f.ctx, "ses_p")
	require.NoError(t, err)
	mut(p)
	require.NoError(t, f.svc.Sessions.Update(f.ctx, p))
	require.NoError(t, f.svc.Sessions.(domain.SessionFanoutStore).SetSessionFanout(f.ctx, "ses_p", p.Fanout))
}

func (f *fanoutFixture) evaluate(t *testing.T, session, cp string) string {
	t.Helper()
	eff, err := f.svc.Evaluate(f.ctx, session, bundle.CheckpointAction(), map[string]any{"id": cp})
	require.NoError(t, err)
	return eff
}

func TestFanoutDelegatedCheckpoint(t *testing.T) {
	f := newFanoutFixture(t)
	assert.Equal(t, "allow", f.evaluate(t, "ses_c", "cp-2"), "cp-2 of the child does not pause")
	d, auto, _, err := f.svc.Asked(f.ctx, "ses_c", "ses_c", adapters.PendingDecision{ID: "perm1", Call: &adapters.ToolCall{Input: map[string]any{"id": "cp-2"}}})
	require.NoError(t, err)
	assert.True(t, auto)
	assert.Nil(t, d)
	st, err := f.svc.Status(f.ctx, "ses_c")
	require.NoError(t, err)
	assert.Equal(t, "auto", st.Checkpoints[1].Behavior)
	assert.Subset(t, st.LockedOps, []string{"push", "merge", "close"})
	require.NotNil(t, st.Fanout)
	assert.Equal(t, "ses_p", st.Fanout.Parent)

	res, err := f.svc.Reached(f.ctx, "ses_c", Call{ID: "cp-2"})
	require.NoError(t, err)
	assert.Equal(t, []string{"commit", "close"}, res.Unlocked)
	assert.Contains(t, f.execDirs, f.childDir, "the verification of the child still runs, in its worktree")
	cs, err := f.svc.State(f.ctx, "ses_c")
	require.NoError(t, err)
	var passed *domain.CheckpointEvent
	for i, e := range cs.Timeline {
		if e.Kind == domain.CheckpointPassed && e.ID == "cp-2" {
			passed = &cs.Timeline[i]
		}
	}
	require.NotNil(t, passed)
	assert.Equal(t, TimelineDelegated, passed.By, "traced")
	assert.Contains(t, passed.Message, "cp-merge")
	assert.Contains(t, passed.Message, "ses_p")

	// cp-1 is not delegated: it keeps its behavior (pause in manuel).
	require.NoError(t, f.svc.Sessions.Update(f.ctx, func() *domain.Session {
		c, _ := f.svc.Sessions.Get(f.ctx, "ses_c")
		c.Mode = "manuel"
		return c
	}()))
	assert.Equal(t, "ask", f.evaluate(t, "ses_c", "cp-1"))
	assert.Equal(t, "allow", f.evaluate(t, "ses_c", "cp-2"))
}

func TestFanoutDelegatedVerificationStillRefuses(t *testing.T) {
	f := newFanoutFixture(t)
	f.exit = 1
	_, err := f.svc.Reached(f.ctx, "ses_c", Call{ID: "cp-2"})
	var ve *VerifyError
	require.ErrorAs(t, err, &ve, "a delegated checkpoint is never passed on a failed verification")
	cs, _ := f.svc.State(f.ctx, "ses_c")
	assert.False(t, cs.HasPassed("cp-2"))
}

// Every way to get the substitution without its conditions keeps the pause.
func TestFanoutDelegationBypass(t *testing.T) {
	cases := map[string]func(t *testing.T, f *fanoutFixture){
		"launched with --parent only": func(t *testing.T, f *fanoutFixture) {
			c, _ := f.svc.Sessions.Get(f.ctx, "ses_c")
			require.NoError(t, f.svc.Sessions.(domain.SessionFanoutStore).SetSessionFanout(f.ctx, c.ID, nil))
		},
		"child record pointing to itself": func(t *testing.T, f *fanoutFixture) {
			require.NoError(t, f.svc.Sessions.(domain.SessionFanoutStore).SetSessionFanout(f.ctx, "ses_c",
				&domain.SessionFanout{Role: domain.RoleChild, Parent: "ses_c", Ticket: "ev-1"}))
		},
		"merger role": func(t *testing.T, f *fanoutFixture) {
			require.NoError(t, f.svc.Sessions.(domain.SessionFanoutStore).SetSessionFanout(f.ctx, "ses_c",
				&domain.SessionFanout{Role: domain.RoleMerger, Parent: "ses_p", Ticket: "ev-1"}))
		},
		"another ticket": func(t *testing.T, f *fanoutFixture) {
			require.NoError(t, f.svc.Sessions.(domain.SessionFanoutStore).SetSessionFanout(f.ctx, "ses_c",
				&domain.SessionFanout{Role: domain.RoleChild, Parent: "ses_p", Ticket: "ev-2"}))
		},
		"not listed by the parent": func(t *testing.T, f *fanoutFixture) {
			f.setParent(t, func(p *domain.Session) { p.Fanout = parentRecord("ses_other") })
		},
		"flag off at launch": func(t *testing.T, f *fanoutFixture) {
			f.setParent(t, func(p *domain.Session) { p.Fanout.Experimental = false })
		},
		"parent not a fan-out": func(t *testing.T, f *fanoutFixture) {
			f.setParent(t, func(p *domain.Session) { p.Fanout.Role = domain.RoleChild })
		},
		"children done": func(t *testing.T, f *fanoutFixture) {
			f.setParent(t, func(p *domain.Session) { p.Fanout.Status = domain.FanoutReady })
		},
		"child stopped": func(t *testing.T, f *fanoutFixture) {
			f.setParent(t, func(p *domain.Session) { p.Fanout.Children[0].State = domain.ChildFailed })
		},
		"parent ended": func(t *testing.T, f *fanoutFixture) {
			f.setParent(t, func(p *domain.Session) { p.State = domain.RunCompleted })
		},
		"parent bundle without delegation": func(t *testing.T, f *fanoutFixture) {
			wf := fanoutParentWF()
			wf.Fanout.Delegates = nil
			f.rebundle(t, wf)
		},
		"merge checkpoint not mandatory": func(t *testing.T, f *fanoutFixture) {
			wf := fanoutParentWF()
			wf.Checkpoints[1].Mandatory = false
			f.rebundle(t, wf)
		},
		"merge checkpoint not verified": func(t *testing.T, f *fanoutFixture) {
			wf := fanoutParentWF()
			wf.Checkpoints[1].Verify = nil
			f.rebundle(t, wf)
		},
		"merge verified in the session": func(t *testing.T, f *fanoutFixture) {
			wf := fanoutParentWF()
			wf.Checkpoints[1].Verify.On = ""
			f.rebundle(t, wf)
		},
		"merge passed by the auto mode": func(t *testing.T, f *fanoutFixture) {
			wf := fanoutParentWF()
			wf.Checkpoints[1].Behaviors["auto"] = "auto"
			f.rebundle(t, wf)
		},
		"merge unlocks nothing": func(t *testing.T, f *fanoutFixture) {
			wf := fanoutParentWF()
			wf.Checkpoints[1].Unlocks = nil
			f.rebundle(t, wf)
		},
		"parent without fan-out in its bundle": func(t *testing.T, f *fanoutFixture) {
			wf := fanoutParentWF()
			wf.Fanout = nil
			f.rebundle(t, wf)
		},
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFanoutFixture(t)
			mut(t, f)
			assert.Equal(t, "ask", f.evaluate(t, "ses_c", "cp-2"))
			_, err := f.svc.Reached(f.ctx, "ses_c", Call{ID: "cp-2"})
			assert.ErrorIs(t, err, ErrNotApproved, "never passed without the user")
		})
	}
}

func (f *fanoutFixture) rebundle(t *testing.T, wf *sessionspec.WorkflowRuntime) {
	t.Helper()
	writeBundle(t, f.svc.BundlesDir, "h_parent2", wf)
	f.setParent(t, func(p *domain.Session) { p.BundleHash = "h_parent2" })
}

// No session of a fan-out pushes, merges or closes, even with cp-2 passed.
func TestFanoutChildRestrictions(t *testing.T) {
	f := newFanoutFixture(t)
	for _, id := range []string{"ses_c", "ses_p"} {
		rules, err := f.svc.Rules(f.ctx, id)
		require.NoError(t, err)
		for _, cmd := range []string{"git push origin HEAD", "git merge oh/fanout/p/integration", "git pull", "bd close ev-1", "git update-ref refs/heads/oh/fanout/p/integration HEAD"} {
			assert.Equal(t, sessionspec.EffectDeny, bundle.ShellEffect(rules, cmd), "%s: %s", id, cmd)
		}
	}
	_, err := f.svc.Reached(f.ctx, "ses_c", Call{ID: "cp-2"})
	require.NoError(t, err)
	var le *LockError
	require.ErrorAs(t, f.svc.GuardClose(f.ctx, "ses_c", f.childDir), &le, "oh closes the tickets it merged")
	assert.Contains(t, le.Msg, "oh")
	rules, _ := f.svc.Rules(f.ctx, "ses_p")
	assert.Equal(t, sessionspec.EffectDeny, bundle.ShellEffect(rules, "git commit -m x"), "the parent never commits")
}

func (f *fanoutFixture) ready(t *testing.T) {
	t.Helper()
	f.setParent(t, func(p *domain.Session) {
		p.Fanout.Status = domain.FanoutReady
		p.Fanout.Children[0].State = domain.ChildMerged
		p.Fanout.Integration = &domain.FanoutIntegration{Branch: "oh/fanout/p/integration", Path: f.intg, Base: "b0", Head: "i1"}
	})
}

func TestFanoutMergeCheckpoint(t *testing.T) {
	f := newFanoutFixture(t)
	// Asked while the children run: no decision, the call is refused with
	// the reason.
	assert.Equal(t, "allow", f.evaluate(t, "ses_p", "cp-merge"))
	d, auto, _, err := f.svc.Asked(f.ctx, "ses_p", "ses_p", adapters.PendingDecision{ID: "perm2", Call: &adapters.ToolCall{Input: map[string]any{"id": "cp-merge"}}})
	require.NoError(t, err)
	assert.True(t, auto)
	assert.Nil(t, d)
	_, err = f.svc.Reached(f.ctx, "ses_p", Call{ID: "cp-merge"})
	var le *LockError
	require.ErrorAs(t, err, &le)
	assert.Equal(t, 0, f.integrated)

	f.ready(t)
	assert.Equal(t, "ask", f.evaluate(t, "ses_p", "cp-merge"), "pauses in every mode")
	_, err = f.svc.Reached(f.ctx, "ses_p", Call{ID: "cp-merge"})
	assert.ErrorIs(t, err, ErrNotApproved)

	// The verification runs on the integration branch, and oh merges.
	res, err := reachValidated(t, f.svc, f.ctx, "ses_p", "cp-merge")
	require.NoError(t, err)
	assert.Contains(t, f.execDirs, f.intg)
	assert.Equal(t, 1, f.integrated)
	assert.Contains(t, res.Message, "merged by oh")
	cs, _ := f.svc.State(f.ctx, "ses_p")
	assert.True(t, cs.HasPassed("cp-merge"))
	_, open := cs.Unlocks[sessionspec.UnlockMerge]
	assert.False(t, open, "no merge window for the agents")
}

func TestFanoutMergeCheckpointGuards(t *testing.T) {
	t.Run("integration moved outside oh", func(t *testing.T) {
		f := newFanoutFixture(t)
		f.ready(t)
		f.heads[f.intg] = "forged"
		_, err := reachValidated(t, f.svc, f.ctx, "ses_p", "cp-merge")
		var le *LockError
		require.ErrorAs(t, err, &le)
		assert.Contains(t, le.Msg, "forged")
		assert.Equal(t, 0, f.integrated)
	})
	t.Run("failed verification", func(t *testing.T) {
		f := newFanoutFixture(t)
		f.ready(t)
		f.exit = 1
		_, err := reachValidated(t, f.svc, f.ctx, "ses_p", "cp-merge")
		var ve *VerifyError
		require.ErrorAs(t, err, &ve)
		assert.Equal(t, 0, f.integrated)
	})
	t.Run("never overridden", func(t *testing.T) {
		f := newFanoutFixture(t)
		f.ready(t)
		tool := &fakeTool{}
		d := domain.Decision{ID: "d1", Kind: domain.DecisionCheckpoint, SessionID: "ses_p", Payload: domain.DecisionPayload{Data: map[string]any{DataCheckpoint: "cp-merge"}}}
		err := f.svc.Resolve(f.ctx, tool, d, ChoiceOverride, "")
		require.Error(t, err)
		assert.Empty(t, tool.replies)
		assert.True(t, f.svc.Card(f.ctx, "ses_p", "cp-merge").NoOverride, "the card offers no override")
	})
	t.Run("merge by oh failed", func(t *testing.T) {
		f := newFanoutFixture(t)
		f.ready(t)
		f.integrateErr = errors.New("conflict with the branch of the session")
		_, err := reachValidated(t, f.svc, f.ctx, "ses_p", "cp-merge")
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "conflict"))
		cs, _ := f.svc.State(f.ctx, "ses_p")
		assert.False(t, cs.HasPassed("cp-merge"))
	})
	t.Run("no integration in the daemon", func(t *testing.T) {
		f := newFanoutFixture(t)
		f.ready(t)
		f.svc.Integrate = nil
		_, err := reachValidated(t, f.svc, f.ctx, "ses_p", "cp-merge")
		var le *LockError
		require.ErrorAs(t, err, &le)
	})
	t.Run("already merged", func(t *testing.T) {
		f := newFanoutFixture(t)
		f.ready(t)
		f.setParent(t, func(p *domain.Session) { p.Fanout.Status = domain.FanoutMerged })
		_, err := reachValidated(t, f.svc, f.ctx, "ses_p", "cp-merge")
		var le *LockError
		require.ErrorAs(t, err, &le)
	})
}

// R6: cp-merge refused with an instruction asks oh for a fix round on the
// integration branch, with what failed in its verification; a plain
// refusal, another checkpoint or a workflow without fix round does not.
func TestFanoutMergeRefusedAsksForAFix(t *testing.T) {
	type call struct{ instruction, failure string }
	setup := func(t *testing.T, fix bool) (*fanoutFixture, *[]call) {
		f := newFanoutFixture(t)
		wf := fanoutParentWF()
		if fix {
			wf.Fanout.Fix, wf.Fanout.FixInput = "quick", "request"
		}
		f.rebundle(t, wf)
		f.ready(t)
		var calls []call
		f.svc.RequestFix = func(_ context.Context, sess *domain.Session, wf *sessionspec.WorkflowRuntime, instruction, failure string) string {
			calls = append(calls, call{instruction, failure})
			return "FIX ROUND"
		}
		return f, &calls
	}
	merge := domain.Decision{ID: "d1", Kind: domain.DecisionCheckpoint, SessionID: "ses_p", Payload: domain.DecisionPayload{Data: map[string]any{DataCheckpoint: "cp-merge"}}}

	f, calls := setup(t, true)
	f.exit = 1
	_, err := reachValidated(t, f.svc, f.ctx, "ses_p", "cp-merge")
	var ve *VerifyError
	require.ErrorAs(t, err, &ve, "the verification on the integration branch fails")
	tool := &fakeTool{}
	require.NoError(t, f.svc.Resolve(f.ctx, tool, merge, ChoiceFix, "corrige le test"))
	require.Len(t, *calls, 1)
	assert.Equal(t, "corrige le test", (*calls)[0].instruction)
	assert.NotEmpty(t, (*calls)[0].failure, "what failed goes to the fix round")
	require.Len(t, tool.steers, 1)
	assert.Contains(t, tool.steers[0], "FIX ROUND")
	assert.Equal(t, 0, f.integrated)

	f, calls = setup(t, true)
	tool = &fakeTool{}
	require.NoError(t, f.svc.Resolve(f.ctx, tool, merge, ChoiceReject, ""))
	assert.Empty(t, *calls, "a refusal without instruction asks for nothing")

	f, calls = setup(t, false)
	require.NoError(t, f.svc.Resolve(f.ctx, &fakeTool{}, merge, ChoiceFix, "corrige"))
	assert.Empty(t, *calls, "no fix round declared")

	f, calls = setup(t, true)
	plan := merge
	plan.Payload = domain.DecisionPayload{Data: map[string]any{DataCheckpoint: "cp-plan"}}
	require.NoError(t, f.svc.Resolve(f.ctx, &fakeTool{}, plan, ChoiceFix, "corrige"))
	assert.Empty(t, *calls, "only the merge checkpoint")
}

// A fixer is never a child: its commit checkpoint pauses even when the
// parent delegates it, and it never pushes, merges nor closes.
func TestFanoutFixerNotDelegated(t *testing.T) {
	f := newFanoutFixture(t)
	wf := fanoutParentWF()
	wf.Fanout.Fix, wf.Fanout.FixInput = "quick", "request"
	f.rebundle(t, wf)
	require.NoError(t, f.svc.Sessions.(domain.SessionFanoutStore).SetSessionFanout(f.ctx, "ses_c",
		&domain.SessionFanout{Role: domain.RoleFixer, Parent: "ses_p", Ticket: "ev-1", Branch: "oh/fanout/p/integration"}))
	f.setParent(t, func(p *domain.Session) {
		p.Fanout.Fixes = []domain.FanoutFix{{Round: 1, Session: "ses_c", State: domain.ChildRunning}}
	})
	assert.Equal(t, "ask", f.evaluate(t, "ses_c", "cp-2"))
	_, err := f.svc.Reached(f.ctx, "ses_c", Call{ID: "cp-2"})
	assert.ErrorIs(t, err, ErrNotApproved, "the user validates the fix")
	rules, err := f.svc.Rules(f.ctx, "ses_c")
	require.NoError(t, err)
	for _, cmd := range []string{"git push origin HEAD", "git merge main", "bd close ev-1", "git branch -f oh/fanout/p/integration HEAD~1"} {
		assert.Equal(t, sessionspec.EffectDeny, bundle.ShellEffect(rules, cmd), cmd)
	}
	assert.NotEqual(t, sessionspec.EffectDeny, bundle.ShellEffect(bundle.FanoutRoleRules(bundle.FanoutFixer), "git commit -m fix"),
		"the fixer commits its fix (after its own commit checkpoint)")
}
