package teamstate

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
