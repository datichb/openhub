//go:build !windows

package checkpoint

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The account of the agents: the commands and the git reads of a
// verification run as it, with its folders.
func TestVerifyAccount(t *testing.T) {
	acct := &Account{UID: 4242, GID: 4343, Home: "/home/oh"}
	cmd := exec.Command("true")
	setProcessGroup(cmd)
	require.NoError(t, runAs(cmd, acct))
	require.NotNil(t, cmd.SysProcAttr.Credential)
	assert.Equal(t, uint32(4242), cmd.SysProcAttr.Credential.Uid)
	assert.Equal(t, uint32(4343), cmd.SysProcAttr.Credential.Gid)
	assert.Empty(t, cmd.SysProcAttr.Credential.Groups, "no supplementary group of oh")
	assert.True(t, cmd.SysProcAttr.Setpgid, "still its own process group")

	g := gitAs(context.Background(), t.TempDir(), acct, "status")
	assert.Equal(t, uint32(4242), g.SysProcAttr.Credential.Uid)
	assert.Contains(t, g.Env, "HOME=/home/oh")
	assert.Contains(t, g.Env, "GIT_TERMINAL_PROMPT=0", "still the safe git of oh")
	for _, kv := range g.Env {
		assert.False(t, strings.HasPrefix(kv, "HOME=") && kv != "HOME=/home/oh", kv)
	}
	assert.Nil(t, gitAs(context.Background(), t.TempDir(), nil, "status").SysProcAttr, "the account of oh by default")
}

// As root (remote job), the verification really runs as the account.
func TestExecVerifyAsAccount(t *testing.T) {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		t.Skip("needs root on Linux (remote job)")
	}
	dir := t.TempDir()
	require.NoError(t, os.Chmod(filepath.Dir(dir), 0o755)) // t.TempDir: 0700
	require.NoError(t, os.Chmod(dir, 0o777))
	out := &TailBuffer{Max: 1024}
	acct := &Account{UID: 65534, GID: 65534, Home: dir}
	exit, err := execVerify(context.Background(), dir, []string{"id", "-u"}, verifyEnv([]string{"PATH=/usr/bin:/bin"}, acct), out, acct)
	require.NoError(t, err)
	assert.Equal(t, 0, exit)
	assert.Equal(t, "65534", strings.TrimSpace(string(out.Bytes())))
}
