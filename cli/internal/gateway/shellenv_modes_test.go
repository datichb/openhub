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

// shellFixture is a machine with two bd: the fake bd of oh (shims) and the
// one a user's start-up file puts first (other).
type shellFixture struct {
	root, shims, other, home string
}

func newShellFixture(t *testing.T) shellFixture {
	t.Helper()
	root := t.TempDir()
	f := shellFixture{root: root, shims: filepath.Join(root, "shims"), other: filepath.Join(root, "other"), home: filepath.Join(root, "home")}
	for _, d := range []string{f.shims, f.other, f.home} {
		require.NoError(t, os.MkdirAll(d, 0o755))
	}
	for _, d := range []string{f.shims, f.other} {
		require.NoError(t, os.WriteFile(filepath.Join(d, "bd"), []byte("#!/bin/sh\n"), 0o755))
	}
	return f
}

func (f shellFixture) want() string { return filepath.Join(f.shims, "bd") }

// startup returns the variables of oh's shell start-up for the machine env.
func (f shellFixture) startup(t *testing.T, machine map[string]string) map[string]string {
	t.Helper()
	got, err := ShellStartup(filepath.Join(f.root, "run", "shell"), []string{f.shims}, func(k string) string { return machine[k] })
	require.NoError(t, err)
	return got
}

func merge(maps ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

func (f shellFixture) userFile(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("export PATH=\""+f.other+":$PATH\"\nexport USER_FILE_READ=1\n"), 0o644))
}

func run(t *testing.T, sh string, env map[string]string, args ...string) string {
	t.Helper()
	cmd := exec.Command(sh, args...)
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.Output()
	require.NoError(t, err)
	return string(out)
}

// S-15: ENV is read by sh and dash when interactive. The user's ENV puts
// another bd first; oh's ENV file loads it, then puts the fake bd back.
func TestShellStartupControlsENV(t *testing.T) {
	f := newShellFixture(t)
	userENV := filepath.Join(f.home, ".shrc")
	f.userFile(t, userENV)
	machine := map[string]string{"ENV": userENV}
	startup := f.startup(t, machine)
	assert.NotEqual(t, userENV, startup[EnvENV], "the shell reads oh's file")
	assert.Equal(t, userENV, startup[EnvUserENV])
	assert.FileExists(t, startup[EnvENV])

	shells := []string{"/bin/sh"}
	if dash, err := exec.LookPath("dash"); err == nil {
		shells = append(shells, dash)
	}
	for _, sh := range shells {
		t.Run(sh, func(t *testing.T) {
			base := map[string]string{"SHELL": sh, "HOME": f.home, "PATH": f.shims + ":/usr/bin:/bin", "ENV": userENV}
			got, err := CheckShellBD(context.Background(), base, f.want())
			var be *ShellBDError
			require.ErrorAs(t, err, &be, "the user's ENV wins without oh (S-15 reproduced)")
			assert.ErrorIs(t, err, ErrShellBD)
			assert.Equal(t, ShellModeInteractive, be.Mode, "only an interactive sh reads ENV")
			assert.Equal(t, filepath.Join(f.other, "bd"), got)

			env := merge(base, startup)
			got, err = CheckShellBD(context.Background(), env, f.want())
			require.NoError(t, err)
			assert.Equal(t, f.want(), got)
			assert.Equal(t, "1\n", run(t, sh, env, "-i", "-c", "echo $USER_FILE_READ"), "the user's ENV file is still read")
		})
	}
}

// Without a user ENV, oh's file still keeps the fake bd first in an
// interactive sh.
func TestShellStartupENVWithoutUserFile(t *testing.T) {
	f := newShellFixture(t)
	startup := f.startup(t, nil)
	assert.NotContains(t, startup, EnvUserENV)
	env := merge(map[string]string{"SHELL": "/bin/sh", "HOME": f.home, "PATH": f.other + ":" + f.shims + ":/usr/bin:/bin"}, startup)
	assert.Equal(t, f.want()+"\n", run(t, "/bin/sh", env, "-i", "-c", "command -v bd"))
}

// The ENV of oh itself is not taken for the user's (a second start-up).
func TestShellStartupDoesNotChainItself(t *testing.T) {
	f := newShellFixture(t)
	first := f.startup(t, nil)
	again := f.startup(t, first)
	assert.NotContains(t, again, EnvUserENV)
	assert.NotContains(t, again, EnvUserBashEnv)
	assert.NotContains(t, again, EnvUserZDOTDIR)
}

// S-15: bash reads ~/.bashrc when interactive, and nothing redirects it (no
// variable): the check starts bash that way too and sees what ~/.bashrc
// does. The line documented for ~/.bashrc restores the order.
func TestInteractiveBashIsChecked(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not installed")
	}
	f := newShellFixture(t)
	f.userFile(t, filepath.Join(f.home, ".bashrc"))
	startup := f.startup(t, nil)
	env := merge(map[string]string{"SHELL": bash, "HOME": f.home, "PATH": f.shims + ":/usr/bin:/bin"}, startup)

	got, err := CheckShellBD(context.Background(), env, f.want())
	var be *ShellBDError
	require.ErrorAs(t, err, &be)
	assert.Equal(t, ShellModeInteractive, be.Mode, "BASH_ENV covers the commands of the tool, not ~/.bashrc")
	assert.Equal(t, filepath.Join(f.other, "bd"), got)

	// The line of the documentation, at the end of ~/.bashrc.
	rc := filepath.Join(f.home, ".bashrc")
	data, err := os.ReadFile(rc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(rc, append(data, []byte("[ -n \"$OH_PATH_FIRST\" ] && PATH=\"$OH_PATH_FIRST:$PATH\"\n")...), 0o644))
	got, err = CheckShellBD(context.Background(), env, f.want())
	require.NoError(t, err)
	assert.Equal(t, f.want(), got)
}

// zsh when interactive reads .zshrc from ZDOTDIR: oh's file covers it.
func TestInteractiveZshIsControlled(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh not installed")
	}
	f := newShellFixture(t)
	f.userFile(t, filepath.Join(f.home, ".zshrc"))
	startup := f.startup(t, nil)
	base := map[string]string{"SHELL": zsh, "HOME": f.home, "PATH": f.shims + ":/usr/bin:/bin"}
	_, err = CheckShellBD(context.Background(), base, f.want())
	var be *ShellBDError
	require.ErrorAs(t, err, &be, "~/.zshrc puts another bd first without oh")
	assert.Equal(t, ShellModeInteractive, be.Mode)

	got, err := CheckShellBD(context.Background(), merge(base, startup), f.want())
	require.NoError(t, err)
	assert.Equal(t, f.want(), got)
}

// An interactive start that answers nothing proves nothing.
func TestInteractiveProbeWithoutAnswerIsNotAFailure(t *testing.T) {
	f := newShellFixture(t)
	fake := filepath.Join(f.root, "fakeshell")
	script := "#!/bin/sh\ncase \"$1\" in\n-i) exit 1 ;;\n*) echo " + f.want() + " ;;\nesac\n"
	require.NoError(t, os.WriteFile(fake, []byte(script), 0o755))
	got, err := CheckShellBD(context.Background(), map[string]string{"SHELL": fake, "HOME": f.home}, f.want())
	require.NoError(t, err)
	assert.Equal(t, f.want(), got)

	// But a non-interactive start that answers nothing is a failure, as before.
	broken := filepath.Join(f.root, "brokenshell")
	require.NoError(t, os.WriteFile(broken, []byte("#!/bin/sh\nexit 1\n"), 0o755))
	_, err = CheckShellBD(context.Background(), map[string]string{"SHELL": broken, "HOME": f.home}, f.want())
	var be *ShellBDError
	require.ErrorAs(t, err, &be)
	assert.Equal(t, ShellModeCommand, be.Mode)
}

// S-15: fish has no variable for its configuration but XDG_CONFIG_HOME. A
// fish session gets a configuration home of its own: its config.fish loads
// the user's, and its other entries are links to the user's own.
func TestShellStartupFishFiles(t *testing.T) {
	f := newShellFixture(t)
	userXDG := filepath.Join(f.home, ".config")
	require.NoError(t, os.MkdirAll(filepath.Join(userXDG, "fish"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(userXDG, "git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(userXDG, "git", "config"), []byte("[user]\n\tname = U\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(userXDG, "gh"), []byte("x"), 0o644))

	startup := f.startup(t, map[string]string{"SHELL": "/usr/bin/fish", "HOME": f.home})
	mine := startup[EnvXDGConfig]
	require.NotEmpty(t, mine)
	assert.Equal(t, userXDG, startup[EnvUserXDG])
	assert.NotEqual(t, userXDG, mine)
	assert.FileExists(t, filepath.Join(mine, "fish", "config.fish"))
	for _, name := range []string{"git", "gh"} {
		target, err := os.Readlink(filepath.Join(mine, name))
		require.NoError(t, err, name)
		assert.Equal(t, filepath.Join(userXDG, name), target)
	}
	_, err := os.Lstat(filepath.Join(mine, "fish", "conf.d"))
	assert.Error(t, err, "the user's fish folder is not copied or linked: config.fish loads it")
	data, err := os.ReadFile(filepath.Join(mine, "git", "config"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "name = U", "the tools of the session read their configuration through the links")

	// Redone each time: entries removed or added since.
	require.NoError(t, os.Remove(filepath.Join(userXDG, "gh")))
	require.NoError(t, os.MkdirAll(filepath.Join(userXDG, "new"), 0o755))
	f.startup(t, map[string]string{"SHELL": "/usr/bin/fish", "HOME": f.home})
	_, err = os.Lstat(filepath.Join(mine, "gh"))
	assert.Error(t, err)
	_, err = os.Lstat(filepath.Join(mine, "new"))
	assert.NoError(t, err)

	// A configured XDG_CONFIG_HOME of the user is the one that is linked.
	custom := filepath.Join(f.root, "custom-config")
	require.NoError(t, os.MkdirAll(filepath.Join(custom, "nvim"), 0o755))
	startup = f.startup(t, map[string]string{"SHELL": "/opt/homebrew/bin/fish", "HOME": f.home, "XDG_CONFIG_HOME": custom})
	assert.Equal(t, custom, startup[EnvUserXDG])
	_, err = os.Lstat(filepath.Join(startup[EnvXDGConfig], "nvim"))
	assert.NoError(t, err)
	_, err = os.Lstat(filepath.Join(startup[EnvXDGConfig], "git"))
	assert.Error(t, err, "links of another configuration home are gone")
}

// Users of other shells keep their XDG_CONFIG_HOME: nothing is redirected.
func TestShellStartupLeavesXDGAloneForOtherShells(t *testing.T) {
	f := newShellFixture(t)
	for _, sh := range []string{"/bin/zsh", "/bin/bash", "/bin/sh", ""} {
		startup := f.startup(t, map[string]string{"SHELL": sh, "HOME": f.home})
		assert.NotContains(t, startup, EnvXDGConfig, sh)
		assert.NotContains(t, startup, EnvUserXDG, sh)
	}
}

// The fish start-up works with a real fish: the user's conf.d, functions and
// config.fish are loaded, and the fake bd is first, for fish -c and fish -i -c.
func TestShellStartupControlsFish(t *testing.T) {
	fish, err := exec.LookPath("fish")
	if err != nil {
		t.Skip("fish not installed")
	}
	f := newShellFixture(t)
	userXDG := filepath.Join(f.home, ".config")
	userFish := filepath.Join(userXDG, "fish")
	require.NoError(t, os.MkdirAll(filepath.Join(userFish, "conf.d"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(userFish, "functions"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(userXDG, "git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(userXDG, "git", "config"), []byte("[user]\n\tname = U\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(userFish, "conf.d", "10-env.fish"), []byte("set -gx FROM_CONF_D 1\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(userFish, "functions", "hello.fish"), []byte("function hello; echo hello-from-user; end\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(userFish, "config.fish"),
		[]byte("set -gx PATH "+f.other+" $PATH\nset -gx FROM_CONFIG 1\n"), 0o644))

	base := map[string]string{"SHELL": fish, "HOME": f.home, "PATH": f.shims + ":" + filepath.Dir(fish) + ":/usr/bin:/bin"}
	_, err = CheckShellBD(context.Background(), base, f.want())
	var be *ShellBDError
	require.ErrorAs(t, err, &be, "config.fish puts another bd first without oh (S-15 reproduced)")
	assert.Equal(t, ShellModeCommand, be.Mode, "fish reads config.fish even for -c")

	startup := f.startup(t, map[string]string{"SHELL": fish, "HOME": f.home})
	env := merge(base, startup)
	got, err := CheckShellBD(context.Background(), env, f.want())
	require.NoError(t, err)
	assert.Equal(t, f.want(), got)

	out := run(t, fish, env, "-c", "echo $FROM_CONF_D$FROM_CONFIG; hello; cat $XDG_CONFIG_HOME/git/config")
	assert.Contains(t, out, "11", "the user's conf.d and config.fish are loaded")
	assert.Contains(t, out, "hello-from-user", "the user's functions are found")
	assert.Contains(t, out, "name = U", "the other tools find their configuration")
	assert.Equal(t, f.want()+"\n", run(t, fish, env, "-i", "-c", "command -v bd"))
}
