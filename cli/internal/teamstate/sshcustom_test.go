package teamstate

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

	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// The ssh command of git (GIT_SSH_COMMAND, core.sshCommand, GIT_SSH): the test
// of the server asks `ssh -G` with its options, and the git processes of oh
// run ssh with them. No command is run (a marker file would show it), no
// network is reached (a fake ssh -G, a fake ssh that only records its
// arguments), and the configuration of ssh is a temporary file given with -F:
// the real ~/.ssh/config never leaks in.

func sshGArgs(t *testing.T, tools *fakeSSHTools) []string {
	t.Helper()
	calls := tools.called("ssh")
	require.Len(t, calls, 1, "ssh -G is asked once")
	return calls[0].args
}

func TestSSHHostAsksSSHWithTheOptionsOfGit(t *testing.T) {
	home, tools := sshSandbox(t)
	custom := filepath.Join(home, "custom", "known_hosts")
	// forward slashes: a backslash is an escape in the command line of git
	t.Setenv("GIT_SSH_COMMAND", "ssh -i /keys/work -F /conf/ssh_config -o UserKnownHostsFile="+filepath.ToSlash(custom)+" -p 2200 -4")
	tools.gErr = nil
	tools.gOut = "hostname real.example.test\nport 2222\nuserknownhostsfile " + custom + "\nglobalknownhostsfile none\n"

	h := UnknownSSHHost(t.Context(), "ssh://deploy@alias:2222/team/state.git")
	require.NotNil(t, h)
	assert.Equal(t, []string{
		"-G", "-i", "/keys/work", "-F", "/conf/ssh_config", "-o", "UserKnownHostsFile=" + filepath.ToSlash(custom), "-p", "2200", "-4",
		"-p", "2222", "-l", "deploy", "--", "alias",
	}, sshGArgs(t, tools), "the options of git, then the port and the user of the URL, which win")
	assert.Equal(t, gitutil.SSHSourceCommandEnv, h.Custom.Source)
	assert.True(t, h.Custom.Followed())
	assert.Equal(t, custom, h.WriteTo, "the known_hosts of the command is the one that is read and written")
	assert.Equal(t, "[real.example.test]:2222", h.Name)

	tools.scan = "[real.example.test]:2222 ssh-ed25519 " + testKeyED25519 + "\n"
	keys, err := h.Scan(t.Context())
	require.NoError(t, err)
	require.NoError(t, h.Trust(keys))
	data, err := os.ReadFile(custom)
	require.NoError(t, err)
	assert.Contains(t, string(data), "[real.example.test]:2222 ssh-ed25519 ")
	assert.NoFileExists(t, filepath.Join(home, ".ssh", "known_hosts"), "the default file is not the one ssh reads")
	tools.calls = nil
	assert.Nil(t, UnknownSSHHost(t.Context(), "ssh://deploy@alias:2222/team/state.git"))
}

// core.sshCommand of the global configuration, then of the clone;
// GIT_SSH_COMMAND before both: the order of git.
func TestSSHHostReadsCoreSSHCommandLikeGit(t *testing.T) {
	_, tools := sshSandbox(t)
	global := os.Getenv("GIT_CONFIG_GLOBAL")
	require.NoError(t, os.WriteFile(global, []byte("[core]\n\tsshCommand = ssh -i /keys/global\n"), 0o600))
	clone := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(clone, ".git"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(clone, ".git", "config"), []byte("[core]\n\tsshCommand = ssh -i /keys/clone\n"), 0o600))
	tools.gErr = nil
	tools.gOut = "hostname git.example.test\nport 22\n"
	const remote = "git@git.example.test:team/state.git"
	last := func() []string { return tools.calls[len(tools.calls)-1].args }

	// no clone given (the assistants, before the clone exists): the global one
	require.NotNil(t, UnknownSSHHost(t.Context(), remote))
	assert.Equal(t, []string{"-G", "-i", "/keys/global", "-l", "git", "--", "git.example.test"}, last())

	// the configuration of the clone comes first
	require.NotNil(t, UnknownSSHHost(t.Context(), remote, InRepo(clone)))
	assert.Equal(t, []string{"-G", "-i", "/keys/clone", "-l", "git", "--", "git.example.test"}, last())

	t.Setenv("GIT_SSH_COMMAND", "ssh -i /keys/env")
	require.NotNil(t, UnknownSSHHost(t.Context(), remote, InRepo(clone)))
	assert.Equal(t, []string{"-G", "-i", "/keys/env", "-l", "git", "--", "git.example.test"}, last())
}

func TestSSHHostWithoutACustomCommandAsksPlainSSH(t *testing.T) {
	_, tools := sshSandbox(t)
	tools.gErr = nil
	tools.gOut = "hostname git.example.test\nport 22\n"
	h := UnknownSSHHost(t.Context(), "ssh://git.example.test:2222/team/state.git")
	require.NotNil(t, h)
	assert.Equal(t, []string{"-G", "-p", "2222", "--", "git.example.test"}, sshGArgs(t, tools))
	assert.Equal(t, gitutil.SSHCommand{}, h.Custom)
	note, warn := h.CustomSSHNote()
	assert.Empty(t, note)
	assert.False(t, warn)
}

// A command that cannot be read with certainty is dropped as a whole, said, and
// never run: plain ssh is asked.
func TestSSHHostIgnoresACommandItCannotRead(t *testing.T) {
	i18ntest.Set(t, "en")
	_, tools := sshSandbox(t)
	marker := filepath.Join(t.TempDir(), "marker")
	tools.gErr = nil
	tools.gOut = "hostname git.example.test\nport 22\n"
	for line, want := range map[string]string{
		"ssh -i /keys/work; touch " + marker:             `the shell syntax ";" is not interpreted`,
		"touch " + marker:                                `the program "touch" is not ssh`,
		"ssh -o ProxyCommand='touch " + marker + "'":     `the option "ProxyCommand" is not carried over`,
		"ssh -F /tmp/ssh_config $(touch " + marker + ")": `the shell syntax "$" is not interpreted`,
		"ssh -i 'unclosed":                               "a quote or a backslash is left open",
		"ssh -o UserKnownHostsFile=relative/known_hosts": `the file "relative/known_hosts" is a relative path`,
	} {
		tools.calls = nil
		t.Setenv("GIT_SSH_COMMAND", line)
		h := UnknownSSHHost(t.Context(), "git@git.example.test:team/state.git")
		require.NotNil(t, h, line)
		assert.Equal(t, []string{"-G", "-l", "git", "--", "git.example.test"}, sshGArgs(t, tools), line)
		note, warn := h.CustomSSHNote()
		assert.True(t, warn, line)
		assert.Contains(t, note, "The SSH command of git (GIT_SSH_COMMAND) is not followed", line)
		assert.Contains(t, note, want, line)
	}
	assert.NoFileExists(t, marker, "the command is never run")
}

func TestSSHHostNoteOfTheCommandItFollows(t *testing.T) {
	i18ntest.Set(t, "en")
	_, tools := sshSandbox(t)
	tools.gErr = nil
	tools.gOut = "hostname git.example.test\nport 22\n"
	t.Setenv("GIT_SSH_COMMAND", "ssh -i '/keys/my key' -o StrictHostKeyChecking=yes")
	h := UnknownSSHHost(t.Context(), "git@git.example.test:team/state.git")
	require.NotNil(t, h)
	note, warn := h.CustomSSHNote()
	assert.False(t, warn)
	assert.Equal(t, "SSH options of git (GIT_SSH_COMMAND) carried over: -i '/keys/my key' -o StrictHostKeyChecking=yes", note)
}

// GIT_SSH is a program: no option to carry over, said when it is not ssh.
func TestSSHHostSaysWhenGitSSHIsNotSSH(t *testing.T) {
	i18ntest.Set(t, "en")
	_, tools := sshSandbox(t)
	tools.gErr = nil
	tools.gOut = "hostname git.example.test\nport 22\n"

	t.Setenv("GIT_SSH", "/opt/tools/ssh-wrapper")
	h := UnknownSSHHost(t.Context(), "git@git.example.test:team/state.git")
	require.NotNil(t, h)
	assert.Equal(t, []string{"-G", "-l", "git", "--", "git.example.test"}, sshGArgs(t, tools))
	note, warn := h.CustomSSHNote()
	assert.True(t, warn)
	assert.Contains(t, note, "GIT_SSH (/opt/tools/ssh-wrapper) is not used")

	t.Setenv("GIT_SSH", "/usr/bin/ssh")
	h = UnknownSSHHost(t.Context(), "git@git.example.test:team/state.git")
	require.NotNil(t, h)
	note, warn = h.CustomSSHNote()
	assert.Empty(t, note, "GIT_SSH naming ssh says nothing")
	assert.False(t, warn)
}

func TestManualCommandsCarryTheOptionsOfGit(t *testing.T) {
	_, tools := sshSandbox(t)
	tools.gErr = nil
	tools.gOut = "hostname git.example.test\nport 22\nuserknownhostsfile /custom/kh\n"
	t.Setenv("GIT_SSH_COMMAND", "ssh -i '/keys/my key' -o UserKnownHostsFile=/custom/kh")
	h := UnknownSSHHost(t.Context(), "ssh://git@git.example.test:2222/x.git")
	require.NotNil(t, h)
	sshCmd, keyscanCmd := h.ManualCommands()
	assert.Equal(t, "ssh -i '/keys/my key' -o UserKnownHostsFile=/custom/kh -p 2222 -T git@git.example.test", sshCmd,
		"by hand, ssh must check and write the file oh reads")
	assert.Equal(t, "ssh-keyscan -p 2222 git.example.test >> "+displayPath(filepath.FromSlash("/custom/kh")), keyscanCmd)
}

// The real ssh -G, on a configuration of the test given with -F by the ssh
// command of git: the alias, the port, the known_hosts file and HashKnownHosts
// come from that file only.
func TestSSHHostWithTheRealSSHAndAnFOptionOfGit(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh not installed")
	}
	if runtime.GOOS == "windows" {
		t.Skip("the quoting of the test is the one of a POSIX path")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	isolateGitSSH(t)
	t.Setenv("SSH_AUTH_SOCK", "")

	dir := t.TempDir()
	custom := filepath.Join(dir, "custom_known_hosts")
	conf := filepath.Join(dir, "ssh_config")
	require.NoError(t, os.WriteFile(conf, []byte("Host alias\n  HostName real.example.test\n  Port 2222\n  User deploy\n"+
		"  UserKnownHostsFile "+custom+"\n  GlobalKnownHostsFile none\n  HashKnownHosts yes\n"), 0o600))
	t.Setenv("GIT_SSH_COMMAND", "ssh -F '"+conf+"' -i "+filepath.Join(dir, "key"))

	// the real ssh -G; ssh-keyscan stays a fake: nothing reaches a network
	scan := &fakeSSHTools{scan: "[real.example.test]:2222 ssh-ed25519 " + testKeyED25519 + "\n"}
	t.Cleanup(SetSSHToolRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "ssh-keyscan" {
			return scan.run(ctx, name, args...)
		}
		return runSSHTool(ctx, name, args...)
	}))

	h := UnknownSSHHost(t.Context(), "git@alias:team/state.git")
	require.NotNil(t, h)
	assert.True(t, h.Resolved)
	assert.Equal(t, "real.example.test", h.Host)
	assert.Equal(t, "2222", h.Port)
	assert.Equal(t, "[real.example.test]:2222", h.Name)
	assert.Equal(t, custom, h.WriteTo)
	assert.True(t, h.Hashed)

	keys, err := h.Scan(t.Context())
	require.NoError(t, err)
	require.NoError(t, h.Trust(keys))
	data, err := os.ReadFile(custom)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(data), "|1|"), "HashKnownHosts of the file the command names: %q", data)
	assert.NotContains(t, string(data), "real.example.test")
	assert.Nil(t, UnknownSSHHost(t.Context(), "git@alias:team/state.git"))
	assertSSHKeygenFinds(t, custom, "[real.example.test]:2222", true)
}

// ── the git processes of oh ─────────────────────────────────────────────────

func TestGitCommandHandsTheOptionsOfGitToSSH(t *testing.T) {
	skipWithoutSh(t)
	isolateGitSSH(t)
	const sshRemote = "git@git.example.test:team/state.git"
	const oh = "-o StrictHostKeyChecking=accept-new -o ConnectTimeout=15"

	// nothing custom: the command of oh, byte for byte
	cmd, cleanup := gitCommand(t.Context(), sshRemote, t.TempDir(), nil, oh, nil, "pull")
	cleanup()
	assert.Equal(t, "ssh -o BatchMode=yes "+oh, lastEnv(cmd.Env, "GIT_SSH_COMMAND"))

	t.Setenv("GIT_SSH_COMMAND", "ssh -i '/keys/my key' -o UserKnownHostsFile=/custom/kh -o StrictHostKeyChecking=yes -o BatchMode=no")
	want := "ssh -o BatchMode=yes -i '/keys/my key' -o UserKnownHostsFile=/custom/kh -o StrictHostKeyChecking=yes -o BatchMode=no " + oh
	for _, args := range [][]string{{"pull", "--rebase"}, {"push"}, {"fetch"}, {"clone", "x", "y"}, {"ls-remote", "x"}} {
		cmd, cleanup = gitCommand(t.Context(), sshRemote, t.TempDir(), nil, oh, nil, args...)
		cleanup()
		assert.Equal(t, want, lastEnv(cmd.Env, "GIT_SSH_COMMAND"), args[0])
	}

	// not for what does not connect, nor for a remote that is not SSH
	cmd, cleanup = gitCommand(t.Context(), sshRemote, t.TempDir(), nil, oh, nil, "status")
	cleanup()
	assert.Equal(t, "ssh -o BatchMode=yes "+oh, lastEnv(cmd.Env, "GIT_SSH_COMMAND"))
	for _, remote := range []string{"https://gitlab.example.com/org/team.git", "/srv/git/team.git"} {
		cmd, cleanup = gitCommand(t.Context(), remote, t.TempDir(), nil, oh, nil, "pull")
		cleanup()
		assert.Equal(t, "ssh -o BatchMode=yes "+oh, lastEnv(cmd.Env, "GIT_SSH_COMMAND"), remote)
	}

	// a command oh cannot read gives plain ssh
	t.Setenv("GIT_SSH_COMMAND", "ssh -i /keys/work; touch "+filepath.Join(t.TempDir(), "marker"))
	cmd, cleanup = gitCommand(t.Context(), sshRemote, t.TempDir(), nil, oh, nil, "pull")
	cleanup()
	assert.Equal(t, "ssh -o BatchMode=yes "+oh, lastEnv(cmd.Env, "GIT_SSH_COMMAND"))

	// the core.sshCommand of the clone is read for the commands run in it
	t.Setenv("GIT_SSH_COMMAND", "")
	clone := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(clone, ".git"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(clone, ".git", "config"), []byte("[core]\n\tsshCommand = ssh -i /keys/clone\n"), 0o600))
	cmd, cleanup = gitCommand(t.Context(), sshRemote, clone, nil, oh, nil, "pull")
	cleanup()
	assert.Equal(t, "ssh -o BatchMode=yes -i /keys/clone "+oh, lastEnv(cmd.Env, "GIT_SSH_COMMAND"))
}

// git itself, through its shell, gives ssh the options once each, with the
// quoting kept: a fake ssh on the PATH records what it is started with and
// never connects.
func TestGitRunsSSHWithTheOptionsOfTheUserAndOfOh(t *testing.T) {
	skipWithoutSh(t)
	isolateGitSSH(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	bin := t.TempDir()
	record := filepath.Join(t.TempDir(), "args")
	script := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done > '" + record + "'\nexit 1\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "ssh"), []byte(script), 0o700))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	key := filepath.Join(t.TempDir(), "it's a key")
	t.Setenv("GIT_SSH_COMMAND", `ssh -i "`+key+`" -o UserKnownHostsFile=/custom/kh -p 2200`)

	const remote = "ssh://git@git.example.test:2222/team/state.git"
	cmd, cleanup := gitCommand(t.Context(), remote, t.TempDir(), nil, "-o StrictHostKeyChecking=accept-new", nil, "ls-remote", remote)
	defer cleanup()
	out, err := cmd.CombinedOutput()
	require.Error(t, err, "the fake ssh refuses: %s", out)

	data, err := os.ReadFile(record)
	require.NoError(t, err, "ssh was started: %s", out)
	// ssh -o BatchMode=yes -i KEY -o UserKnownHostsFile=… -p 2200 -o StrictHostKeyChecking=accept-new
	// (git's own) -p 2222 git@host git-upload-pack '…'
	joined := " " + strings.ReplaceAll(strings.TrimSuffix(string(data), "\n"), "\n", " ") + " "
	for _, want := range []string{
		" -o BatchMode=yes ", " -i " + key + " ", " -o UserKnownHostsFile=/custom/kh ", " -o StrictHostKeyChecking=accept-new ", " -p 2222 ", " git@git.example.test ",
	} {
		assert.Contains(t, joined, want)
	}
	assert.Less(t, strings.Index(joined, " BatchMode=yes "), strings.Index(joined, " UserKnownHostsFile=/custom/kh "), "BatchMode of oh first")
	assert.Less(t, strings.Index(joined, " -p 2200 "), strings.Index(joined, " -p 2222 "), "the port of the URL, which git adds last, wins")
}
