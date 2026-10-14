package checkpoint

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// E9 (R8): cp-release unlocks commit, tag and mr; the tag waits for the
// release commit, and the tag and mr windows close at the next mandatory
// checkpoint (CP-R8) or when cp-release is asked again.
func TestReleaseWindows(t *testing.T) {
	svc, ctx := newTestService(t)
	wf := &sessionspec.WorkflowRuntime{ID: "release", DefaultMode: "manuel",
		Checkpoints: []sessionspec.CheckpointDef{
			{ID: "cp-release", Label: map[string]string{"": "Release"}, Behaviors: map[string]string{"manuel": "pause"}, Mandatory: true,
				Unlocks: []string{sessionspec.UnlockCommit, sessionspec.UnlockTag, sessionspec.UnlockMR}},
			{ID: "cp-after", Label: map[string]string{"": "Après"}, Behaviors: map[string]string{"manuel": "pause"}, Mandatory: true},
		}}
	writeBundle(t, svc.BundlesDir, "h_release", wf)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_r", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_release", Mode: "manuel", State: domain.RunActive, LaunchPath: "/work"}))
	h := &fakeHead{head: "c1"}
	svc.Head = h.get
	tags := &fakeTags{fp: "t0"}
	svc.Tags = tags.get

	tag := sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: "git tag*", Effect: sessionspec.EffectDeny}
	mr := sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: "glab mr create*", Effect: sessionspec.EffectDeny}
	commit := sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: "git commit*", Effect: sessionspec.EffectDeny}
	rules := func() []sessionspec.PermissionRule {
		r, err := svc.Rules(ctx, "ses_r")
		require.NoError(t, err)
		return r
	}
	assert.Subset(t, rules(), []sessionspec.PermissionRule{commit, tag, mr}, "before cp-release")

	_, err := reachValidated(t, svc, ctx, "ses_r", "cp-release")
	require.NoError(t, err)
	assert.NotContains(t, rules(), commit)
	assert.NotContains(t, rules(), mr)
	assert.Contains(t, rules(), tag, "the tag waits for the release commit")

	h.head = "c2"
	_, _, err = svc.OnCall(ctx, "ses_r", adapters.ToolCall{ID: "s1", Action: sessionspec.ActionShell, Status: adapters.CallCalled, Input: map[string]any{"command": "git commit -m 'chore(release): v1.3.0'"}})
	require.NoError(t, err)
	changed, _, err := svc.OnCall(ctx, "ses_r", adapters.ToolCall{ID: "s1", Action: sessionspec.ActionShell, Status: adapters.CallOK})
	require.NoError(t, err)
	assert.True(t, changed, "committed: the rules change")
	assert.NotContains(t, rules(), tag, "committed: the tag opens")
	assert.Contains(t, rules(), commit)

	// p4-debts: the first tag closes the tag window, as the commit closes
	// the commit window; a call that leaves the tags as they were does not.
	shell(t, svc, ctx, "ses_r", "s2", "git log -1")
	assert.NotContains(t, rules(), tag, "no tag yet: the window stays open")
	tags.fp = "t1"
	assert.True(t, shell(t, svc, ctx, "ses_r", "s3", "git tag -a v1.3.0 -m 'v1.3.0'"), "tagged: the rules change")
	assert.Contains(t, rules(), tag, "tagged: the window closes")
	assert.NotContains(t, rules(), mr, "mr keeps its own window")
	st, _ := svc.State(ctx, "ses_r")
	assert.NotContains(t, st.Unlocks, sessionspec.UnlockTag)
	assert.Equal(t, domain.CheckpointWindow, st.Timeline[len(st.Timeline)-1].Kind)
	assert.Equal(t, sessionspec.UnlockTag, st.Timeline[len(st.Timeline)-1].Message)

	// Asked again on the release commit (nothing new to commit): the tag
	// does not wait for a commit that will never come.
	_, err = reachValidated(t, svc, ctx, "ses_r", "cp-release")
	require.NoError(t, err)
	assert.NotContains(t, rules(), tag, "passed again on the release commit: the tag stays open")
	assert.NotContains(t, rules(), commit)

	_, err = reachValidated(t, svc, ctx, "ses_r", "cp-after")
	require.NoError(t, err)
	assert.Subset(t, rules(), []sessionspec.PermissionRule{tag, mr}, "the next mandatory checkpoint closes them")
	st, _ = svc.State(ctx, "ses_r")
	assert.NotContains(t, st.Unlocks, sessionspec.UnlockTag)
	assert.NotContains(t, st.Unlocks, sessionspec.UnlockMR)
}

type fakeTags struct {
	fp  string
	err error
}

func (f *fakeTags) get(_ context.Context, dir string) (string, error) {
	if dir != "/work" {
		return "", errors.New("unexpected dir " + dir)
	}
	return f.fp, f.err
}

// shell reports a shell call of the session, run to the end.
func shell(t *testing.T, svc *Service, ctx context.Context, sessionID, id, cmd string) (changed bool) {
	t.Helper()
	_, _, err := svc.OnCall(ctx, sessionID, adapters.ToolCall{ID: id, Action: sessionspec.ActionShell, Status: adapters.CallCalled, Input: map[string]any{"command": cmd}})
	require.NoError(t, err)
	changed, _, err = svc.OnCall(ctx, sessionID, adapters.ToolCall{ID: id, Action: sessionspec.ActionShell, Status: adapters.CallOK})
	require.NoError(t, err)
	return changed
}

// p4-debts: a checkpoint that unlocks the tag alone: a failed tag leaves
// the window open, the first tag closes it, asking the checkpoint again
// opens it; a git read that fails closes it (fails closed); tags that
// could not be read when the window opened leave it to the next
// checkpoint.
func TestTagWindowClosesAfterFirstTag(t *testing.T) {
	svc, ctx := newTestService(t)
	wf := &sessionspec.WorkflowRuntime{ID: "tagonly", DefaultMode: "manuel",
		Checkpoints: []sessionspec.CheckpointDef{
			{ID: "cp-tag", Label: map[string]string{"": "Tag"}, Behaviors: map[string]string{"manuel": "pause"}, Mandatory: true,
				Unlocks: []string{sessionspec.UnlockTag}},
		}}
	writeBundle(t, svc.BundlesDir, "h_tag", wf)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_t", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_tag", Mode: "manuel", State: domain.RunActive, LaunchPath: "/work"}))
	h := &fakeHead{head: "c1"}
	svc.Head = h.get
	tags := &fakeTags{fp: "t0"}
	svc.Tags = tags.get
	tag := sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: "git tag*", Effect: sessionspec.EffectDeny}
	rules := func() []sessionspec.PermissionRule {
		r, err := svc.Rules(ctx, "ses_t")
		require.NoError(t, err)
		return r
	}
	_, err := reachValidated(t, svc, ctx, "ses_t", "cp-tag")
	require.NoError(t, err)
	assert.NotContains(t, rules(), tag, "no commit to wait for: the tag opens")

	assert.False(t, shell(t, svc, ctx, "ses_t", "s1", "git tag -a v1 -m x"), "the tag failed: nothing changed")
	assert.NotContains(t, rules(), tag)
	tags.fp = "t1"
	assert.True(t, shell(t, svc, ctx, "ses_t", "s2", "git tag -a v1 -m x"))
	assert.Contains(t, rules(), tag, "first tag: the window closes")

	_, err = reachValidated(t, svc, ctx, "ses_t", "cp-tag")
	require.NoError(t, err)
	assert.NotContains(t, rules(), tag, "asked again: the window opens again")
	tags.err = errors.New("git refuses")
	assert.True(t, shell(t, svc, ctx, "ses_t", "s3", "ls"))
	assert.Contains(t, rules(), tag, "git cannot be read: the window closes")

	_, err = reachValidated(t, svc, ctx, "ses_t", "cp-tag")
	require.NoError(t, err)
	assert.NotContains(t, rules(), tag, "tags unknown at the opening")
	tags.err, tags.fp = nil, "t2"
	assert.False(t, shell(t, svc, ctx, "ses_t", "s4", "git tag v2"))
	assert.NotContains(t, rules(), tag, "tags unknown at the opening: the window waits for the next checkpoint")
}

func TestGitTags(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	git("init", "-q")
	git("commit", "-q", "--allow-empty", "-m", "a")
	none, err := GitTags(context.Background(), dir)
	require.NoError(t, err)
	assert.NotEmpty(t, none)
	git("tag", "-a", "v1", "-m", "v1")
	one, err := GitTags(context.Background(), dir)
	require.NoError(t, err)
	assert.NotEqual(t, none, one, "a new tag changes the fingerprint")
	git("commit", "-q", "--allow-empty", "-m", "b")
	same, _ := GitTags(context.Background(), dir)
	assert.Equal(t, one, same, "a commit leaves it")
	git("tag", "-d", "v1")
	gone, _ := GitTags(context.Background(), dir)
	assert.Equal(t, none, gone)
	_, err = GitTags(context.Background(), t.TempDir())
	assert.Error(t, err, "not a repository")
}
