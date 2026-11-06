package teamstate

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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

// core.sshCommand of the global configuration, GIT_SSH_COMMAND before it: the
// order of git, less the configuration of a repository, which is not followed.
func TestSSHHostReadsCoreSSHCommandLikeGit(t *testing.T) {
	_, tools := sshSandbox(t)
	global := os.Getenv("GIT_CONFIG_GLOBAL")
	require.NoError(t, os.WriteFile(global, []byte("[core]\n\tsshCommand = ssh -i /keys/global\n"), 0o600))
	tools.gErr = nil
	tools.gOut = "hostname git.example.test\nport 22\n"
	const remote = "git@git.example.test:team/state.git"
	last := func() []string { return tools.calls[len(tools.calls)-1].args }

	require.NotNil(t, UnknownSSHHost(t.Context(), remote))
	assert.Equal(t, []string{"-G", "-i", "/keys/global", "-l", "git", "--", "git.example.test"}, last())

	t.Setenv("GIT_SSH_COMMAND", "ssh -i /keys/env")
	require.NotNil(t, UnknownSSHHost(t.Context(), remote))
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

	// core.sshCommand of the global configuration
	t.Setenv("GIT_SSH_COMMAND", "")
	require.NoError(t, os.WriteFile(os.Getenv("GIT_CONFIG_GLOBAL"), []byte("[core]\n\tsshCommand = ssh -i /keys/global\n"), 0o600))
	cmd, cleanup = gitCommand(t.Context(), sshRemote, t.TempDir(), nil, oh, nil, "pull")
	cleanup()
	assert.Equal(t, "ssh -o BatchMode=yes -i /keys/global "+oh, lastEnv(cmd.Env, "GIT_SSH_COMMAND"))
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

// ── the core.sshCommand of a repository is not followed ─────────────────────

// repoWithSSHCommand makes a real clone whose own configuration (not the global
// one) sets a core.sshCommand that would switch the check of the servers off.
func repoWithSSHCommand(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	clone := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "core.sshCommand", "ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i /keys/repo"},
	} {
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = clone
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	return clone
}

// The options of a core.sshCommand set in the repository reach neither `ssh -G`
// nor the GIT_SSH_COMMAND of a git process of oh: the check of the server and
// the connection both run the ssh command of the user, never the one the
// repository (which something else than the user may have written) asks for.
func TestTheSSHCommandOfARepositoryIsNotFollowed(t *testing.T) {
	skipWithoutSh(t)
	_, tools := sshSandbox(t)
	tools.gErr = nil
	tools.gOut = "hostname git.example.test\nport 22\n"
	clone := repoWithSSHCommand(t)
	const sshRemote = "git@git.example.test:team/state.git"
	const oh = "-o StrictHostKeyChecking=accept-new -o ConnectTimeout=15"

	// the check of the server: plain ssh, and the server is unknown (the
	// repository does not switch the check off)
	h := UnknownSSHHost(t.Context(), sshRemote)
	require.NotNil(t, h, "UserKnownHostsFile=/dev/null of the repository must not make the server unchecked")
	assert.Equal(t, []string{"-G", "-l", "git", "--", "git.example.test"}, sshGArgs(t, tools))
	assert.Equal(t, gitutil.SSHCommand{}, h.Custom)
	assert.False(t, h.Unchecked)

	// the git processes of oh, run in that clone
	for _, args := range [][]string{{"pull", "--rebase"}, {"push"}, {"fetch"}, {"ls-remote", "origin"}} {
		cmd, cleanup := gitCommand(t.Context(), sshRemote, clone, nil, oh, nil, args...)
		cleanup()
		got := lastEnv(cmd.Env, "GIT_SSH_COMMAND")
		assert.Equal(t, "ssh -o BatchMode=yes "+oh, got, args[0])
		assert.NotContains(t, got, "StrictHostKeyChecking=no")
		assert.NotContains(t, got, "/dev/null")
		assert.NotContains(t, got, "/keys/repo")
	}

	// what the user set wins, and the repository still does not count
	t.Setenv("GIT_SSH_COMMAND", "ssh -i /keys/user")
	tools.calls = nil
	require.NotNil(t, UnknownSSHHost(t.Context(), sshRemote))
	assert.Equal(t, []string{"-G", "-i", "/keys/user", "-l", "git", "--", "git.example.test"}, sshGArgs(t, tools))
	cmd, cleanup := gitCommand(t.Context(), sshRemote, clone, nil, oh, nil, "pull")
	cleanup()
	assert.Equal(t, "ssh -o BatchMode=yes -i /keys/user "+oh, lastEnv(cmd.Env, "GIT_SSH_COMMAND"))
}

// The same, with git itself: it would obey the core.sshCommand of this clone
// (a fake ssh on the PATH records what it is started with and never connects),
// and does not, because the GIT_SSH_COMMAND of oh comes first.
func TestGitInACloneDoesNotRunTheSSHCommandOfItsRepository(t *testing.T) {
	skipWithoutSh(t)
	isolateGitSSH(t)
	clone := repoWithSSHCommand(t)
	bin := t.TempDir()
	record := filepath.Join(t.TempDir(), "args")
	script := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done > '" + record + "'\nexit 1\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "ssh"), []byte(script), 0o700))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	const remote = "ssh://git@git.example.test/team/state.git"
	cmd, cleanup := gitCommand(t.Context(), remote, clone, nil, "-o StrictHostKeyChecking=accept-new", nil, "ls-remote", remote)
	defer cleanup()
	out, err := cmd.CombinedOutput()
	require.Error(t, err, "the fake ssh refuses: %s", out)

	data, err := os.ReadFile(record)
	require.NoError(t, err, "ssh was started: %s", out)
	joined := " " + strings.ReplaceAll(strings.TrimSuffix(string(data), "\n"), "\n", " ") + " "
	assert.Contains(t, joined, " -o BatchMode=yes ")
	assert.Contains(t, joined, " -o StrictHostKeyChecking=accept-new ")
	assert.NotContains(t, joined, "StrictHostKeyChecking=no")
	assert.NotContains(t, joined, "/dev/null")
	assert.NotContains(t, joined, "/keys/repo")
}

func TestRepoSSHCommandNote(t *testing.T) {
	i18ntest.Set(t, "en")
	isolateGitSSH(t)
	clone := repoWithSSHCommand(t)
	note := RepoSSHCommandNote(clone)
	assert.Contains(t, note, "core.sshCommand is set in the repository (")
	assert.Contains(t, note, "not followed by oh; put it in the global git configuration or in GIT_SSH_COMMAND")
	assert.NotContains(t, note, "StrictHostKeyChecking", "the command itself is not shown: it could hold a secret")

	assert.Empty(t, RepoSSHCommandNote(t.TempDir()), "a folder without .git/config")
	assert.Empty(t, RepoSSHCommandNote(""), "no clone")
	plain := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(plain, ".git"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(plain, ".git", "config"), []byte("[core]\n\tbare = false\n\tsshCommand =\n"), 0o600))
	assert.Empty(t, RepoSSHCommandNote(plain), "a clone with no (or an empty) core.sshCommand")
}

// recordLog is a slog handler that keeps the records it is given.
type recordLog struct {
	mu      sync.Mutex
	records []slog.Record
}

func (*recordLog) Enabled(context.Context, slog.Level) bool { return true }
func (l *recordLog) Handle(_ context.Context, r slog.Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, r)
	return nil
}
func (l *recordLog) WithAttrs([]slog.Attr) slog.Handler { return l }
func (l *recordLog) WithGroup(string) slog.Handler      { return l }

// of returns the records of message msg about the clone dir.
func (l *recordLog) of(msg, dir string) []slog.Record {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []slog.Record
	for _, r := range l.records {
		if r.Message != msg {
			continue
		}
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "repo" && a.Value.String() == dir {
				out = append(out, r)
			}
			return true
		})
	}
	return out
}

// The git processes of oh say once, in the log (Info), that the repository
// they run in sets a core.sshCommand they do not follow.
func TestGitCommandSaysOnceThatTheSSHCommandOfTheRepositoryIsNotFollowed(t *testing.T) {
	skipWithoutSh(t)
	isolateGitSSH(t)
	clone := repoWithSSHCommand(t)
	other := t.TempDir()
	logs := &recordLog{}
	prev := slog.Default()
	slog.SetDefault(slog.New(logs))
	t.Cleanup(func() { slog.SetDefault(prev) })

	const remote = "git@git.example.test:team/state.git"
	for range 3 {
		_, cleanup := gitCommand(t.Context(), remote, clone, nil, "", nil, "pull")
		cleanup()
		_, cleanup = gitCommand(t.Context(), remote, other, nil, "", nil, "pull")
		cleanup()
	}
	_, cleanup := gitCommand(t.Context(), remote, clone, nil, "", nil, "status") // does not connect
	cleanup()

	got := logs.of("teamstate.ssh.repo_command_ignored", clone)
	require.Len(t, got, 1, "once per clone")
	assert.Equal(t, slog.LevelInfo, got[0].Level)
	assert.Empty(t, logs.of("teamstate.ssh.repo_command_ignored", other), "a clone without core.sshCommand says nothing")
}
