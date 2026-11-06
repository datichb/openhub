package teamstate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The askpass is for the commands that talk to the remote: a local command
// neither reads the source nor writes a file.
func TestLocalGitCommandsGetNoAskpass(t *testing.T) {
	skipWithoutSh(t)
	left := emptyTmp(t)
	var calls int
	restore := SetCredentialSource(func(_ context.Context, _ string) GitCredentials {
		calls++
		return GitCredentials{Username: "u", Token: "t"}
	})
	defer restore()

	const remote = "https://gitlab.example.com/org/team.git"
	for _, args := range [][]string{{"status"}, {"add", "-A", "--", "x"}, {"commit", "-m", "x"}, {"rev-parse", "HEAD"}, {"remote"}} {
		cmd, cleanup := gitCommand(t.Context(), remote, t.TempDir(), nil, "", nil, args...)
		cleanup()
		assert.Empty(t, lastEnv(cmd.Env, "GIT_ASKPASS"), args[0])
		assert.NotContains(t, cmd.Args, "credential.helper=", args[0])
	}
	assert.Zero(t, calls, "the source is not asked for a local command")
	assert.Empty(t, left())

	cmd, cleanup := gitCommand(t.Context(), remote, t.TempDir(), nil, "", nil, "push")
	defer cleanup()
	assert.NotEmpty(t, lastEnv(cmd.Env, "GIT_ASKPASS"))
	assert.Equal(t, 1, calls)
}

// The askpass of the environment (the credential dialog of an IDE terminal)
// is never run: without a token of oh, git fails at once instead of waiting
// for an answer.
func TestGitCommandIgnoresTheAskpassOfTheEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the askpass script needs a POSIX sh")
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	restore := SetCredentialSource(func(context.Context, string) GitCredentials { return GitCredentials{} })
	defer restore()
	dir := t.TempDir()
	marker := filepath.Join(dir, "asked")
	script := filepath.Join(dir, "askpass.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\ntouch '"+marker+"'\necho x\n"), 0o755))
	t.Setenv("GIT_ASKPASS", script)
	t.Setenv("SSH_ASKPASS", script)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="git"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	remote := srv.URL + "/org/team.git"

	cmd, cleanup := gitCommand(t.Context(), remote, t.TempDir(), nil, "", nil, "ls-remote", remote)
	cleanup()
	assert.Empty(t, lastEnv(cmd.Env, "GIT_ASKPASS"))
	assert.Empty(t, lastEnv(cmd.Env, "SSH_ASKPASS"))

	assert.False(t, RemoteAccessible(t.Context(), remote))
	assert.NoFileExists(t, marker, "the askpass of the environment was run")
}

// lastEnv is the value a command is started with for name (the last entry
// wins), "" when it has none.
func lastEnv(env []string, name string) string {
	val := ""
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, name+"="); ok {
			val = v
		}
	}
	return val
}
