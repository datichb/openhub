//go:build !windows

package gateway

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A16: the user's start-up files put another bd first; oh's start-up files
// load them, then put the fake bd back first (zsh and bash).
func TestShellStartupKeepsTheFakeBdFirst(t *testing.T) {
	root := t.TempDir()
	shims, other, home := filepath.Join(root, "shims"), filepath.Join(root, "other"), filepath.Join(root, "home")
	for _, d := range []string{shims, other, home} {
		require.NoError(t, os.MkdirAll(d, 0o755))
	}
	for _, d := range []string{shims, other} {
		require.NoError(t, os.WriteFile(filepath.Join(d, "bd"), []byte("#!/bin/sh\n"), 0o755))
	}
	userBashEnv := filepath.Join(home, ".bashenv")
	for _, f := range []string{filepath.Join(home, ".zshenv"), userBashEnv} {
		require.NoError(t, os.WriteFile(f, []byte("export PATH=\""+other+":$PATH\"\nexport USER_FILE_READ=1\n"), 0o644))
	}
	want := filepath.Join(shims, "bd")
	machine := map[string]string{"BASH_ENV": userBashEnv}
	startup, err := ShellStartup(filepath.Join(root, "run", "shell"), []string{shims}, func(k string) string { return machine[k] })
	require.NoError(t, err)
	assert.Equal(t, userBashEnv, startup[EnvUserBashEnv])
	assert.Equal(t, shims, startup[EnvPathFirst])
	info, err := os.Stat(filepath.Join(root, "run", "shell", "zsh", ".zshenv"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	for _, shell := range []string{"zsh", "bash"} {
		sh, err := exec.LookPath(shell)
		if err != nil {
			continue
		}
		t.Run(shell, func(t *testing.T) {
			base := map[string]string{"SHELL": sh, "HOME": home, "PATH": shims + ":/usr/bin:/bin"}
			if shell == "bash" {
				base["BASH_ENV"] = userBashEnv
			}
			got, err := CheckShellBD(context.Background(), base, want)
			assert.ErrorIs(t, err, ErrShellBD, "the user's file wins without oh (A16 reproduced)")
			assert.Equal(t, filepath.Join(other, "bd"), got)

			env := map[string]string{}
			for k, v := range base {
				env[k] = v
			}
			for k, v := range startup {
				env[k] = v
			}
			got, err = CheckShellBD(context.Background(), env, want)
			require.NoError(t, err)
			assert.Equal(t, want, got)
			cmd := exec.Command(sh, "-c", "echo $USER_FILE_READ")
			for k, v := range env {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
			out, err := cmd.Output()
			require.NoError(t, err)
			assert.Equal(t, "1\n", string(out), "the user's start-up file is still read")
		})
	}
	_, err = CheckShellBD(context.Background(), map[string]string{"SHELL": "/bin/sh", "PATH": "/nonexistent"}, want)
	assert.ErrorIs(t, err, ErrShellBD, "no bd at all")
}
