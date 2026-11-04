package teamstate

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

func skipWithoutSh(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the askpass script needs a POSIX sh")
	}
	// An askpass of the machine (an editor's, for one) must not answer in place
	// of the one under test, nor wait for a click.
	t.Setenv("GIT_ASKPASS", "")
	t.Setenv("SSH_ASKPASS", "")
	// The credential helper of the machine (the macOS keychain) must neither
	// answer nor keep what the tests give git.
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// credsFor is a source that holds the token for one remote.
func credsFor(remote, user, token string, calls *atomic.Int32) CredentialSource {
	return func(_ context.Context, r string) GitCredentials {
		if calls != nil {
			calls.Add(1)
		}
		if r == remote {
			return GitCredentials{Username: user, Token: token}
		}
		return GitCredentials{}
	}
}

// emptyTmp makes the temporary folder of the test the one the askpass uses and
// reports what is left in it.
func emptyTmp(t *testing.T) func() []string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	return func() []string {
		entries, err := os.ReadDir(tmp)
		require.NoError(t, err)
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		return names
	}
}

// The askpass is private and answers the user for the user prompt and the
// token for the others; the token is only in its file.
func TestAskpassFilesAndAnswers(t *testing.T) {
	skipWithoutSh(t)
	left := emptyTmp(t)
	ap, err := newAskpass(GitCredentials{Username: "bob", Token: "glpat-sekret'quote"})
	require.NoError(t, err)

	dirInfo, err := os.Stat(ap.dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm(), "private folder")
	for name, mode := range map[string]os.FileMode{"user": 0o600, "token": 0o600, "askpass.sh": 0o700} {
		info, err := os.Stat(filepath.Join(ap.dir, name))
		require.NoError(t, err)
		assert.Equal(t, mode, info.Mode().Perm(), name)
	}
	assert.Equal(t, "GIT_ASKPASS="+filepath.Join(ap.dir, "askpass.sh"), ap.env)
	assert.NotContains(t, ap.env, "sekret", "the token is not in the environment entry")
	script, err := os.ReadFile(filepath.Join(ap.dir, "askpass.sh"))
	require.NoError(t, err)
	assert.NotContains(t, string(script), "sekret", "the token is not in the script")

	run := func(prompt string) string {
		out, err := exec.Command(filepath.Join(ap.dir, "askpass.sh"), prompt).Output()
		require.NoError(t, err)
		return string(out)
	}
	assert.Equal(t, "bob", run("Username for 'https://gitlab.example.com': "))
	assert.Equal(t, "glpat-sekret'quote", run("Password for 'https://bob@gitlab.example.com': "))

	ap.Close()
	assert.Empty(t, left(), "the folder is removed")
}

func TestAskpassDefaultsToOauth2(t *testing.T) {
	skipWithoutSh(t)
	emptyTmp(t)
	ap, err := newAskpass(GitCredentials{Token: "t"})
	require.NoError(t, err)
	defer ap.Close()
	out, err := exec.Command(filepath.Join(ap.dir, "askpass.sh"), "Username for 'https://x': ").Output()
	require.NoError(t, err)
	assert.Equal(t, "oauth2", string(out))
}

// A repository that asks for a token: git has none, then gets it from the
// source of oh through the askpass, for the clone, the pull and the push.
func TestTeamStateOverHTTPSWithAToken(t *testing.T) {
	skipWithoutSh(t)
	left := emptyTmp(t)
	bare, seed := gittest.BareClone(t)
	srv := gittest.ServeHTTP(t, bare, "team-state", "oauth2", "glpat-s3cret")
	ctx := t.Context()
	local := filepath.Join(t.TempDir(), "clone")

	t.Run("without the token the access fails as an access problem", func(t *testing.T) {
		err := NewRepo(srv.URL, local).Clone(ctx)
		require.Error(t, err)
		assert.True(t, IsAuthError(err), "%v", err)
		assert.False(t, NewRepo(srv.URL, local).IsCloned())
		assert.True(t, IsAuthError(ProbeRemote(ctx, srv.URL)))
	})

	t.Run("a wrong token is an access problem", func(t *testing.T) {
		restore := SetCredentialSource(credsFor(srv.URL, "oauth2", "nope", nil))
		defer restore()
		err := ProbeRemote(ctx, srv.URL)
		require.Error(t, err)
		assert.True(t, IsAuthError(err), "%v", err)
	})

	restore := SetCredentialSource(credsFor(srv.URL, "oauth2", "glpat-s3cret", nil))
	defer restore()

	t.Run("probe, clone", func(t *testing.T) {
		require.NoError(t, ProbeRemote(ctx, srv.URL))
		assert.True(t, RemoteAccessible(ctx, srv.URL))
		repo := NewRepo(srv.URL, local)
		require.NoError(t, repo.Clone(ctx))
		assert.True(t, repo.IsCloned())
		assert.Positive(t, srv.Accepted(), "the token reached the server")
		assert.Empty(t, left(), "no askpass is left behind")
	})

	t.Run("pull sees a push of a colleague", func(t *testing.T) {
		gittest.Commit(t, seed, "from-alice.txt", "hello\n", "alice")
		gittest.Run(t, seed, "push", "-q")
		repo := NewRepo(srv.URL, local)
		require.NoError(t, repo.EnsureReady(ctx))
		_, err := os.Stat(filepath.Join(local, "from-alice.txt"))
		require.NoError(t, err, "pulled with the token")
	})

	t.Run("push", func(t *testing.T) {
		repo := NewRepo(srv.URL, local)
		require.NoError(t, os.WriteFile(filepath.Join(local, "mine.txt"), []byte("me\n"), 0o644))
		require.NoError(t, repo.CommitAndPush(ctx, "add mine", "mine.txt"))
		gittest.Run(t, seed, "pull", "-q", "--rebase")
		_, err := os.Stat(filepath.Join(seed, "mine.txt"))
		require.NoError(t, err, "the push reached the server")
		assert.Equal(t, 1, srv.Refused(), "the server refused only the wrong token of the earlier subtest")
		assert.Empty(t, left())
	})
}

// A repository that asks nothing, an SSH remote, or no source: the source is
// not asked (or its answer is not used) and git runs as before.
func TestTeamStateWithoutAToken(t *testing.T) {
	skipWithoutSh(t)
	emptyTmp(t)
	bare, _ := gittest.BareClone(t)
	srv := gittest.ServeHTTP(t, bare, "open", "", "")
	var calls atomic.Int32
	restore := SetCredentialSource(func(_ context.Context, _ string) GitCredentials {
		calls.Add(1)
		return GitCredentials{}
	})
	defer restore()

	require.NoError(t, NewRepo(srv.URL, filepath.Join(t.TempDir(), "c")).Clone(t.Context()))
	assert.Positive(t, calls.Load(), "an HTTPS remote asks the source")

	calls.Store(0)
	_ = ProbeRemote(t.Context(), "git@127.0.0.1.invalid:org/team.git")
	assert.Zero(t, calls.Load(), "an SSH remote never asks for a token")

	other := t.TempDir()
	gittest.Run(t, other, "init", "-q", "--bare")
	require.NoError(t, NewRepo(other, filepath.Join(t.TempDir(), "c")).Clone(t.Context()))
	assert.Zero(t, calls.Load(), "a local path never asks for a token")
}

func TestProbeRemoteSaysWhyItFailed(t *testing.T) {
	bare, _ := gittest.BareClone(t)
	ctx := t.Context()
	require.NoError(t, ProbeRemote(ctx, bare))
	err := ProbeRemote(ctx, filepath.Join(t.TempDir(), "absent.git"))
	require.Error(t, err)
	assert.False(t, IsAuthError(err), "a missing repository is not an access problem: %v", err)
	assert.Error(t, ProbeRemote(ctx, " "))
}

func TestIsAuthErrorCoversSSH(t *testing.T) {
	for _, tt := range []struct {
		msg  string
		want bool
	}{
		{"git ls-remote: git@gitlab.example.com: Permission denied (publickey).\nfatal: Could not read from remote repository.", true},
		{"git@gitlab: Permission denied (publickey,keyboard-interactive).", true},
		{"remote: The project you were looking for could not be found or you don't have permission to access it.", true},
		{"fatal: unable to access 'https://x/y.git/': The requested URL returned error: 403", true},
		{"fatal: could not read Username for 'https://x': terminal prompts disabled", true},
		{"ssh: Could not resolve hostname gitlab.example.com: nodename nor servname provided", false},
		{"fatal: unable to access 'https://x/': Could not resolve host: x", false},
	} {
		assert.Equal(t, tt.want, IsAuthError(errors.New(tt.msg)), tt.msg)
	}
	assert.False(t, IsAuthError(nil))
	// the narrow check of the pull is unchanged: an SSH refusal on a pull stays a warning
	assert.False(t, isAuthError(errors.New("Permission denied (publickey)")))
}

func TestSameRemote(t *testing.T) {
	for _, tt := range []struct {
		a, b string
		want bool
	}{
		{"https://gitlab.example.com/org/team.git", "https://gitlab.example.com/org/team", true},
		{"https://GitLab.example.com/org/team.git/", "https://gitlab.example.com/org/team.git", true},
		{"https://user@gitlab.example.com/org/team.git", "https://gitlab.example.com/org/team.git", true},
		{"git@gitlab.example.com:org/team.git", "ssh://git@gitlab.example.com/org/team.git", true},
		{"git@gitlab.example.com:org/team.git", "https://gitlab.example.com/org/team.git", true},
		{"https://gitlab.example.com/org/team.git", "https://gitlab.example.com/org/other.git", false},
		{"https://gitlab.example.com/org/team.git", "https://github.com/org/team.git", false},
		{"/tmp/a/bare", "/tmp/a/bare/", true},
		{"/tmp/a/bare", "/tmp/b/bare", false},
		{"", "", false},
	} {
		assert.Equal(t, tt.want, SameRemote(tt.a, tt.b), "%q ~ %q", tt.a, tt.b)
	}
}

// An existing clone of another repository is refused, not reused; the clone of
// the same repository (written another way) is kept.
func TestEnsureReadyRefusesTheCloneOfAnotherRemote(t *testing.T) {
	bare, clone := gittest.BareClone(t)
	other, _ := gittest.BareClone(t)

	err := NewRepo(other, clone).EnsureReady(t.Context())
	var mismatch *RemoteMismatchError
	require.ErrorAs(t, err, &mismatch)
	assert.Equal(t, clone, mismatch.Path)
	assert.Equal(t, other, mismatch.Want)
	assert.Equal(t, bare, mismatch.Have)
	assert.Contains(t, err.Error(), clone, "the message names the folder")
	assert.Contains(t, err.Error(), bare, "and the repository found there")
	assert.Contains(t, err.Error(), other, "and the one asked for")
	assert.False(t, IsPullWarning(err))

	require.NoError(t, NewRepo(bare, clone).EnsureReady(t.Context()))
	require.NoError(t, NewRepo(bare+"/", clone).EnsureReady(t.Context()), "same repository")
	require.NoError(t, NewRepo("", clone).EnsureReady(t.Context()), "no remote asked: nothing to compare")
}

func TestCheckRemoteOfACloneWithoutOrigin(t *testing.T) {
	dir := gittest.Repo(t)
	assert.NoError(t, NewRepo("", dir).CheckRemote())
	err := NewRepo("https://gitlab.example.com/org/team.git", dir).CheckRemote()
	var mismatch *RemoteMismatchError
	require.ErrorAs(t, err, &mismatch)
	assert.Empty(t, mismatch.Have)
	assert.Contains(t, strings.ToLower(err.Error()), "no remote")
	assert.NoError(t, NewRepo("https://x/y.git", filepath.Join(t.TempDir(), "absent")).CheckRemote(), "not cloned yet")
}

// A token can be checked before it is kept: the probe with credentials given
// does not need the source, and a wrong one is an access problem.
func TestProbeRemoteAsChecksATokenBeforeItIsKept(t *testing.T) {
	skipWithoutSh(t)
	left := emptyTmp(t)
	bare, _ := gittest.BareClone(t)
	srv := gittest.ServeHTTP(t, bare, "team-state", "bob", "good")
	restore := SetCredentialSource(nil)
	defer restore()

	require.NoError(t, ProbeRemoteAs(t.Context(), srv.URL, GitCredentials{Username: "bob", Token: "good"}))
	err := ProbeRemoteAs(t.Context(), srv.URL, GitCredentials{Username: "bob", Token: "bad"})
	require.Error(t, err)
	assert.True(t, IsAuthError(err), "%v", err)
	assert.Empty(t, left())
}
