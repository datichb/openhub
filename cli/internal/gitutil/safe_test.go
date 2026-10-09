package gitutil

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

// trappedRepo returns a repository whose configuration and hooks try to run
// programs (each one creates a file under marks), with an origin remote.
func trappedRepo(t *testing.T) (repo, marks string) {
	t.Helper()
	root := t.TempDir()
	marks = filepath.Join(root, "marks")
	require.NoError(t, os.MkdirAll(marks, 0o755))
	origin := filepath.Join(root, "origin")
	gitIn(t, root, "init", "-q", origin)
	require.NoError(t, os.WriteFile(filepath.Join(origin, "a.txt"), []byte("a\n"), 0o644))
	gitIn(t, origin, "add", ".")
	gitIn(t, origin, "commit", "-qm", "init")
	repo = filepath.Join(root, "repo")
	gitIn(t, root, "clone", "-q", origin, repo)
	mark := func(name string) string { return "touch " + filepath.Join(marks, name) }
	gitIn(t, repo, "config", "core.fsmonitor", mark("fsmonitor"))
	gitIn(t, repo, "config", "diff.external", mark("external"))
	gitIn(t, repo, "config", "remote.origin.uploadpack", mark("uploadpack")+"; git-upload-pack")
	gitIn(t, repo, "config", "core.sshCommand", mark("ssh"))
	gitIn(t, repo, "config", "credential.helper", "!"+mark("credential"))
	gitIn(t, repo, "config", "diff.trap.textconv", mark("textconv"))
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".gitattributes"), []byte("*.txt diff=trap\n"), 0o644))
	for _, h := range []string{"post-checkout", "pre-commit", "reference-transaction", "post-index-change"} {
		require.NoError(t, os.WriteFile(filepath.Join(repo, ".git", "hooks", h), []byte("#!/bin/sh\n"+mark("hook-"+h)+"\n"), 0o755))
	}
	require.NoError(t, os.WriteFile(filepath.Join(repo, "a.txt"), []byte("b\n"), 0o644))
	return repo, marks
}

// N5 (v5 critical): the git calls of oh never run the programs a repository
// configuration names.
func TestSafeGitIgnoresRepoConfig(t *testing.T) {
	repo, marks := trappedRepo(t)
	ctx := context.Background()
	for _, args := range [][]string{
		{"status", "--porcelain"},
		{"diff"},
		{"diff", "HEAD"},
		{"log", "-p", "-1"},
		{"show", "HEAD"},
		{"fetch", "origin"},
		{"ls-remote", "origin"},
		{"switch", "-c", "feat/x"},
		{"worktree", "add", "-q", filepath.Join(filepath.Dir(repo), "wt"), "-b", "wt"},
		{"rev-parse", "HEAD"},
	} {
		out, err := Command(ctx, repo, args...).CombinedOutput()
		require.NoError(t, err, "%v: %s", args, out)
	}
	entries, _ := os.ReadDir(marks)
	var ran []string
	for _, e := range entries {
		ran = append(ran, e.Name())
	}
	assert.Empty(t, ran, "programs of the repository configuration ran on the machine")

	// The raw git runs them (the test traps work).
	_, _ = exec.Command("git", "-C", repo, "status").CombinedOutput()
	entries, _ = os.ReadDir(marks)
	assert.NotEmpty(t, entries, "control: plain git runs the fsmonitor")
}

func TestSafeArgs(t *testing.T) {
	args := SafeArgs("diff", "--stat", "HEAD")
	assert.Contains(t, args, "--no-ext-diff")
	assert.Contains(t, args, "--no-textconv")
	assert.Equal(t, []string{"diff", "--no-ext-diff", "--no-textconv", "--stat", "HEAD"}, args[len(args)-5:])
	args = SafeArgs("fetch", "upstream", "main")
	assert.Equal(t, []string{"fetch", "--upload-pack=git-upload-pack", "upstream", "main"}, args[len(args)-4:])
	args = SafeArgs("push", "origin", "x")
	assert.Equal(t, []string{"push", "--receive-pack=git-receive-pack", "origin", "x"}, args[len(args)-4:])
	assert.NotContains(t, SafeArgs("status"), "--no-ext-diff")
	env := safeEnv([]string{"GIT_DIR=/x", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.pager", "PATH=/bin"})
	assert.Equal(t, []string{"PATH=/bin", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0"}, env)
}

// N5: a change of the repository configuration or hooks after the session
// started is detected.
func TestCheckDetectsTamperedConfig(t *testing.T) {
	SetTrustDir(t.TempDir())
	t.Cleanup(func() { SetTrustDir("") })
	ctx := context.Background()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	gitIn(t, root, "init", "-q", repo)
	require.NoError(t, Check(ctx, repo), "nothing recorded: no check")
	require.NoError(t, Trust(ctx, repo))
	require.NoError(t, Check(ctx, repo))

	gitIn(t, repo, "config", "core.fsmonitor", "touch /tmp/x")
	err := Check(ctx, repo)
	assert.True(t, errors.Is(err, ErrConfigChanged), "config changed")
	var changed *ConfigChangedError
	require.ErrorAs(t, err, &changed)
	require.Len(t, changed.Files, 1, "the file that changed is named")
	assert.Equal(t, "config", filepath.Base(changed.Files[0]))
	assert.NotEmpty(t, changed.Fingerprint)

	require.NoError(t, Trust(ctx, repo))
	hook := filepath.Join(repo, ".git", "hooks", "post-checkout")
	require.NoError(t, os.WriteFile(hook, []byte("#!/bin/sh\n"), 0o755))
	err = Check(ctx, repo)
	assert.True(t, errors.Is(err, ErrConfigChanged), "hook added")
	require.ErrorAs(t, err, &changed)
	require.Len(t, changed.Files, 1)
	assert.Equal(t, "post-checkout", filepath.Base(changed.Files[0]))
	require.NoError(t, Trust(ctx, repo))
	require.NoError(t, os.Remove(hook))
	require.ErrorAs(t, Check(ctx, repo), &changed)
	assert.Equal(t, []string{"post-checkout"}, []string{filepath.Base(changed.Files[0])}, "hook removed")

	// A worktree: its pointer file is covered.
	require.NoError(t, Trust(ctx, repo))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "a"), []byte("a"), 0o644))
	gitIn(t, repo, "add", ".")
	gitIn(t, repo, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "a")
	wt := filepath.Join(root, "wt")
	gitIn(t, repo, "worktree", "add", "-q", wt, "-b", "wt")
	require.NoError(t, Trust(ctx, wt))
	require.NoError(t, os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+filepath.Join(repo, ".git")+"\n"), 0o644))
	assert.Error(t, Check(ctx, wt), "pointer file redirected")
}
