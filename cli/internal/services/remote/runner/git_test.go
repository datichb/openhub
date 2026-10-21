//go:build unix

package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/remote"
)

type gitFixture struct {
	t      *testing.T
	root   string
	origin string
}

func newGitFixture(t *testing.T) *gitFixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	f := &gitFixture{t: t, root: t.TempDir()}
	f.origin = filepath.Join(f.root, "origin.git")
	gittest.Run(t, f.root, "init", "-q", "--bare", "-b", "main", f.origin)
	gittest.Run(t, f.origin, "config", "receive.advertisePushOptions", "true")
	seed := filepath.Join(f.root, "seed")
	gittest.Run(t, f.root, "clone", "-q", f.origin, seed)
	require.NoError(t, os.WriteFile(filepath.Join(seed, ".gitignore"), []byte("build/\n"), 0o644))
	gittest.Run(t, seed, "add", ".gitignore")
	gittest.Commit(t, seed, "README.md", "x", "init")
	gittest.Run(t, seed, "push", "-q", "origin", "HEAD:main")
	return f
}

// git runs a test git command, never affected by the traps of the agent.
func (f *gitFixture) git(dir string, args ...string) string {
	f.t.Helper()
	return gittest.Run(f.t, dir, append([]string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "safe.directory=*"}, args...)...)
}

// PoC S-01 (audit v5): hooks, core.fsmonitor, a clean filter, another
// remote, credential helper, signing program and alternates planted by the
// agent in its repository. None of them runs during Finish, the token is
// never given to them, and the branch goes to the clone URL of the
// manifest.
func TestFinishRunsNothingOfTheAgent(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	g := Git{Token: "project-secret", Scratch: filepath.Join(f.root, "private")}
	work := filepath.Join(f.root, "work")
	base, err := g.Clone(ctx, f.origin, "main", "", "feat/bd-42", work)
	require.NoError(t, err)

	// The agent commits, then plants its traps and leaves work uncommitted.
	require.NoError(t, os.WriteFile(filepath.Join(work, "a.go"), []byte("package a"), 0o644))
	f.git(work, "add", "a.go")
	f.git(work, "commit", "-q", "-m", "agent commit")
	agentHead := f.git(work, "rev-parse", "HEAD")

	marker := filepath.Join(f.root, "pwned")
	trap := filepath.Join(f.root, "trap.sh")
	require.NoError(t, os.WriteFile(trap, []byte("#!/bin/sh\nenv >> "+marker+"\ncat\n"), 0o755))
	hooks := filepath.Join(work, ".git", "hooks")
	for _, h := range []string{"pre-commit", "post-commit", "commit-msg", "prepare-commit-msg", "pre-push", "post-checkout",
		"reference-transaction", "post-index-change", "pre-auto-gc"} {
		require.NoError(t, os.WriteFile(filepath.Join(hooks, h), []byte("#!/bin/sh\nenv >> "+marker+"\n"), 0o755))
	}
	evil := filepath.Join(f.root, "evil.git")
	f.git(f.root, "init", "-q", "--bare", evil)
	for _, kv := range [][2]string{
		{"core.hooksPath", hooks}, {"core.fsmonitor", trap}, {"filter.evil.clean", trap}, {"filter.evil.smudge", trap},
		{"filter.evil.required", "true"}, {"remote.origin.url", evil}, {"remote.origin.pushurl", evil},
		{"credential.helper", "!" + trap}, {"commit.gpgSign", "true"}, {"gpg.program", trap},
		{"core.sshCommand", trap}, {"diff.external", trap}, {"include.path", trap},
	} {
		f.git(work, "config", kv[0], kv[1])
	}
	require.NoError(t, os.WriteFile(filepath.Join(work, ".gitattributes"), []byte("* filter=evil\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(work, ".git", "objects", "info"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(work, ".git", "objects", "info", "alternates"), []byte(filepath.Join(evil, "objects")+"\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(work, "csv.go"), []byte("package x"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(work, "build"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(work, "build", "out.bin"), []byte("ignored"), 0o644))

	res, err := g.Finish(ctx, f.origin, work, base, "main", "feat/bd-42", "oh: ticket bd-42", "Remote oh session")
	require.NoError(t, err)
	_, statErr := os.Stat(marker)
	assert.True(t, errors.Is(statErr, os.ErrNotExist), "a trap of the agent ran during Finish")

	assert.Len(t, res.Commit, 40)
	assert.Equal(t, res.Commit, f.git(f.origin, "rev-parse", "feat/bd-42"), "pushed to the clone URL of the manifest")
	assert.Empty(t, f.git(evil, "for-each-ref"), "nothing pushed to the agent's remote")
	assert.Equal(t, agentHead, f.git(f.origin, "rev-parse", "feat/bd-42~1"), "the agent's commits are kept")
	files := f.git(f.origin, "ls-tree", "-r", "--name-only", "feat/bd-42")
	for _, want := range []string{"a.go", "csv.go", ".gitattributes", "README.md"} {
		assert.Contains(t, files, want)
	}
	assert.NotContains(t, files, "build/out.bin", ".gitignore respected")
	assert.Equal(t, "oh runner", f.git(f.origin, "log", "-1", "--format=%an", "feat/bd-42"))
	entries, _ := os.ReadDir(g.Scratch)
	assert.Empty(t, entries, "private files removed")
}

// Nested repositories and files the agent does not own are left out of the
// commit (root would read them on its behalf).
func TestFinishSkipsUnsafePaths(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	g := Git{Scratch: filepath.Join(f.root, "private")}
	work := filepath.Join(f.root, "work")
	base, err := g.Clone(ctx, f.origin, "main", "", "feat/x", work)
	require.NoError(t, err)
	nested := filepath.Join(work, "vendor", "lib")
	require.NoError(t, os.MkdirAll(filepath.Join(nested, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nested, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(nested, "lib.go"), []byte("package lib"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(work, "ok.go"), []byte("package ok"), 0o644))

	res, err := g.Finish(ctx, f.origin, work, base, "main", "feat/x", "oh: x", "")
	require.NoError(t, err)
	assert.Equal(t, []string{"vendor/lib"}, res.Skipped)
	files := f.git(f.origin, "ls-tree", "-r", "--name-only", "feat/x")
	assert.Contains(t, files, "ok.go")
	assert.NotContains(t, files, "vendor/lib")

	// Owner check: every file belongs to another account than the agent.
	other := Git{Agent: &User{UID: uint32(os.Getuid()) + 1}}
	skipped, err := other.unsafePaths(work)
	require.NoError(t, err)
	assert.Contains(t, skipped, "ok.go")
	assert.Contains(t, skipped, "README.md")
	mine := Git{Agent: &User{UID: uint32(os.Getuid())}}
	skipped, err = mine.unsafePaths(work)
	require.NoError(t, err)
	assert.Equal(t, []string{"vendor/lib"}, skipped)
}

// A .git that is not a directory (gitdir pointer) is refused.
func TestFinishRefusesGitFile(t *testing.T) {
	f := newGitFixture(t)
	g := Git{Scratch: filepath.Join(f.root, "private")}
	work := filepath.Join(f.root, "work")
	base, err := g.Clone(context.Background(), f.origin, "main", "", "feat/x", work)
	require.NoError(t, err)
	require.NoError(t, os.RemoveAll(filepath.Join(work, ".git")))
	require.NoError(t, os.WriteFile(filepath.Join(work, ".git"), []byte("gitdir: "+f.origin+"\n"), 0o644))
	_, err = g.Finish(context.Background(), f.origin, work, base, "main", "feat/x", "oh: x", "")
	require.ErrorContains(t, err, "is not a directory")
}

// The token never appears in a git command line, URL or environment: git
// gets a one-shot GIT_ASKPASS script, removed afterwards.
func TestGitTokenOnlyThroughAskpass(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "calls")
	fake := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(fake, 0o755))
	script := "#!/bin/sh\n{ echo \"ARGS $*\"; env; echo \"USER $(\"$GIT_ASKPASS\" \"Username for 'https://g':\")\"; echo \"PASS $(\"$GIT_ASKPASS\" \"Password for 'https://oauth2@g':\")\"; } >> " + logFile + "\n" +
		"case \"$*\" in *rev-parse*) echo 0123456789012345678901234567890123456789 ;; esac\n"
	require.NoError(t, os.WriteFile(filepath.Join(fake, "git"), []byte(script), 0o755))
	t.Setenv("PATH", fake+":"+os.Getenv("PATH"))
	t.Setenv("GIT_DIR", "/elsewhere") // inherited GIT_* variables are dropped

	g := Git{Token: "project-secret", Scratch: filepath.Join(dir, "private")}
	_, err := g.Clone(context.Background(), "https://g/acme/api.git", "main", "", "feat/x", filepath.Join(dir, "work"))
	require.NoError(t, err)
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	calls := strings.Split(string(data), "ARGS ")
	var clone string
	for _, c := range calls {
		if strings.HasPrefix(c, "-c core.fsmonitor=false -c core.hooksPath="+os.DevNull) && strings.Contains(c, " clone ") &&
			strings.Contains(c, "-c credential.helper= ") && strings.Contains(c, "--upload-pack=git-upload-pack") {
			clone = c
		}
		lines := strings.SplitN(c, "\n", 2)
		assert.NotContains(t, lines[0], "project-secret", "token in a command line")
		for _, l := range strings.Split(c, "\n") {
			if strings.HasPrefix(l, "PASS ") || strings.HasPrefix(l, "USER ") {
				continue
			}
			assert.NotContains(t, l, "project-secret", "token in the environment")
			assert.NotContains(t, l, "GIT_DIR=", "inherited GIT_* variable")
		}
	}
	require.NotEmpty(t, clone, "clone not run with the safe options")
	assert.Contains(t, clone, "GIT_CONFIG_GLOBAL="+os.DevNull)
	assert.Contains(t, clone, "GIT_CONFIG_NOSYSTEM=1")
	assert.Contains(t, clone, "USER oauth2")
	assert.Contains(t, clone, "PASS project-secret")
	entries, _ := os.ReadDir(g.Scratch)
	assert.Empty(t, entries, "askpass removed")
}

// The agent processes are stopped before the final git steps; when that
// fails, nothing is pushed.
func TestRunQuiescesBeforeFinish(t *testing.T) {
	var order []string
	sess := &fakeSession{fd: &fakeDecisions{}}
	repo := &orderRepo{order: &order}
	in, _ := runInput(t, sess, repo, manifest())
	in.Quiesce = func(context.Context) error { order = append(order, "quiesce"); return nil }
	_, err := Run(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, []string{"clone", "quiesce", "finish"}, order)

	order = nil
	in, out := runInput(t, sess, repo, manifest())
	in.Quiesce = func(context.Context) error { return errors.New("uid 10001 still running") }
	_, err = Run(context.Background(), in)
	require.ErrorContains(t, err, "stopping the agent processes")
	assert.Equal(t, []string{"clone"}, order, "nothing pushed")
	assert.Equal(t, remote.OutcomeFailed, readSummary(t, out).Outcome)
}

type orderRepo struct{ order *[]string }

func (r *orderRepo) Clone(context.Context, string, string, string, string, string) (string, error) {
	*r.order = append(*r.order, "clone")
	return "base1", nil
}
func (r *orderRepo) Finish(context.Context, string, string, string, string, string, string, string) (*PushResult, error) {
	*r.order = append(*r.order, "finish")
	return &PushResult{}, nil
}

func TestGitVersionOK(t *testing.T) {
	for out, want := range map[string]bool{
		"git version 2.39.5\n": true, "git version 2.32.0": true, "git version 2.39.3 (Apple Git-146)": true,
		"git version 2.31.1": false, "git version 2.45.2.windows.1": true, "nope": false, "git version 3": true,
	} {
		assert.Equal(t, want, gitVersionOK(out), out)
	}
}
