package launch

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitIdentityEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(dir, "none"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	run := func(args ...string) {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		require.NoError(t, err, string(out))
	}
	run("init", "-q")
	assert.Empty(t, gitIdentityEnv(dir))
	run("config", "user.name", "Ada Lovelace")
	run("config", "user.email", "ada@example.com")
	assert.Equal(t, map[string]string{
		"GIT_AUTHOR_NAME": "Ada Lovelace", "GIT_COMMITTER_NAME": "Ada Lovelace",
		"GIT_AUTHOR_EMAIL": "ada@example.com", "GIT_COMMITTER_EMAIL": "ada@example.com",
	}, gitIdentityEnv(dir))
}
