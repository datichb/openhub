package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The ssh command of git. Nothing here runs a command taken from the
// environment or a configuration: the lines are read, and the markers some of
// them would create must not exist afterwards.

func sshHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GIT_SSH_COMMAND", "")
	t.Setenv("GIT_SSH", "")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	require.NoError(t, os.WriteFile(cfg, nil, 0o600))
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	return home
}

func TestSplitShell(t *testing.T) {
	home := sshHome(t)
	tests := []struct {
		line string
		want []string
	}{
		{"ssh", []string{"ssh"}},
		{"  ssh   -p  2222 ", []string{"ssh", "-p", "2222"}},
		{`ssh -i "/a b/key"`, []string{"ssh", "-i", "/a b/key"}},
		{`ssh -i '/a b/key'`, []string{"ssh", "-i", "/a b/key"}},
		{`ssh -i /a\ b/key`, []string{"ssh", "-i", "/a b/key"}},
		{`ssh -o"UserKnownHostsFile=/a b"`, []string{"ssh", "-oUserKnownHostsFile=/a b"}},
		{`ssh -o 'It'\''s'`, []string{"ssh", "-o", "It's"}},
		{`ssh ""`, []string{"ssh", ""}},
		{`ssh -i ~/.ssh/key`, []string{"ssh", "-i", home + "/.ssh/key"}},
		{`ssh -i $HOME/.ssh/key`, []string{"ssh", "-i", home + "/.ssh/key"}},
		{`ssh -i "${HOME}/.ssh/key"`, []string{"ssh", "-i", home + "/.ssh/key"}},
		{`ssh -i "~/.ssh/key"`, []string{"ssh", "-i", "~/.ssh/key"}}, // quoted: the shell does not expand it
		{`ssh -i '$HOME/key'`, []string{"ssh", "-i", "$HOME/key"}},
		{`ssh -o "A=\$x \"q\" \\ \z"`, []string{"ssh", "-o", `A=$x "q" \ \z`}},
		{"ssh\t-4", []string{"ssh", "-4"}},
		{"", nil},
	}
	for _, tt := range tests {
		got, ign := splitShell(tt.line)
		require.Nil(t, ign, tt.line)
		if runtime.GOOS == "windows" {
			continue // the home folder has backslashes there
		}
		assert.Equal(t, tt.want, got, tt.line)
	}
}

func TestSplitShellRefusesWhatAShellWouldInterpret(t *testing.T) {
	sshHome(t)
	tests := []struct {
		line string
		code string
		arg  string
	}{
		{"ssh -i k; touch marker", IgnoreShell, ";"},
		{"ssh -i k && touch marker", IgnoreShell, "&"},
		{"ssh -i k | cat", IgnoreShell, "|"},
		{"ssh -i k > out", IgnoreShell, ">"},
		{"ssh -i k < in", IgnoreShell, "<"},
		{"ssh -i $(touch marker)", IgnoreShell, "$"},
		{"ssh -i `touch marker`", IgnoreShell, "`"},
		{`ssh -i "` + "`touch marker`" + `"`, IgnoreShell, "`"},
		{"ssh -i $KEY", IgnoreShell, "$"},
		{"ssh -i ${KEY}", IgnoreShell, "$"},
		{"ssh -i $HOMEDIR/k", IgnoreShell, "$"},
		{`ssh -i "$KEY"`, IgnoreShell, "$"},
		{"ssh -i ~other/k", IgnoreShell, "~"},
		{"ssh -i /keys/*", IgnoreShell, "*"},
		{"ssh -i /keys/k?", IgnoreShell, "?"},
		{"ssh -i /keys/[ab]", IgnoreShell, "["},
		{"ssh -i /keys/{a,b}", IgnoreShell, "{"},
		{"ssh # comment", IgnoreShell, "#"},
		{"ssh -4\ntouch marker", IgnoreShell, "control character"},
		{"ssh -4\r", IgnoreShell, "control character"},
		{"ssh -i 'a\nb'", IgnoreShell, "control character"},
		{"ssh \xff", IgnoreShell, "invalid text"},
		{`ssh -i "unclosed`, IgnoreQuote, ""},
		{`ssh -i 'unclosed`, IgnoreQuote, ""},
		{`ssh -i trailing\`, IgnoreQuote, ""},
	}
	for _, tt := range tests {
		words, ign := splitShell(tt.line)
		require.NotNil(t, ign, "%q was split into %q", tt.line, words)
		assert.Equal(t, tt.code, ign.Code, tt.line)
		assert.Equal(t, tt.arg, ign.Arg, tt.line)
		assert.Nil(t, words)
	}
}

func TestParseSSHCommandLine(t *testing.T) {
	sshHome(t)
	tests := []struct {
		line string
		want []string
	}{
		{"ssh", nil},
		{"/usr/bin/ssh", nil},
		{`"C:/Windows/System32/OpenSSH/ssh.exe" -4`, []string{"-4"}},
		{"SSH.EXE", nil},
		{"ssh -i /k/id -F /c/config -p 2222 -l deploy", []string{"-i", "/k/id", "-F", "/c/config", "-p", "2222", "-l", "deploy"}},
		{"ssh -i/k/id -p2222 -ldeploy -F/c/config", []string{"-i", "/k/id", "-p", "2222", "-l", "deploy", "-F", "/c/config"}},
		{"ssh -4 -q -vvv -T -a -C -x -6 -v -vv", []string{"-4", "-q", "-vvv", "-T", "-a", "-C", "-x", "-6", "-v", "-vv"}},
		{"ssh -o UserKnownHostsFile=/c/kh", []string{"-o", "UserKnownHostsFile=/c/kh"}},
		{"ssh -ouserknownhostsfile=/c/kh", []string{"-o", "UserKnownHostsFile=/c/kh"}},
		{`ssh -o "UserKnownHostsFile /c/kh /d/kh"`, []string{"-o", "UserKnownHostsFile=/c/kh /d/kh"}},
		{`ssh -o "UserKnownHostsFile = /c/kh"`, []string{"-o", "UserKnownHostsFile=/c/kh"}},
		{"ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no", []string{"-o", "UserKnownHostsFile=/dev/null", "-o", "StrictHostKeyChecking=no"}},
		{"ssh -o UserKnownHostsFile=none", []string{"-o", "UserKnownHostsFile=none"}},
		{"ssh -o UserKnownHostsFile=~/kh", []string{"-o", "UserKnownHostsFile=~/kh"}},
		{"ssh -o UserKnownHostsFile=%d/.ssh/kh", []string{"-o", "UserKnownHostsFile=%d/.ssh/kh"}},
		{"ssh -o HashKnownHosts=yes -o HostKeyAlias=gitlab-internal", []string{"-o", "HashKnownHosts=yes", "-o", "HostKeyAlias=gitlab-internal"}},
		{"ssh -o StrictHostKeyChecking=accept-new -o BatchMode=yes -o IdentitiesOnly=yes", []string{"-o", "StrictHostKeyChecking=accept-new", "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes"}},
		{"ssh -o ConnectTimeout=5 -o Port=2200 -o User=git", []string{"-o", "ConnectTimeout=5", "-o", "Port=2200", "-o", "User=git"}},
		{`ssh -o 'IdentityFile "~/.ssh/key"'`, []string{"-o", "IdentityFile=~/.ssh/key"}},
	}
	for _, tt := range tests {
		args, ign := parseSSHCommandLine(tt.line)
		require.Nil(t, ign, tt.line)
		assert.Equal(t, tt.want, args, tt.line)
	}

	home, _ := os.UserHomeDir()
	args, ign := parseSSHCommandLine("ssh -i ~/.ssh/key")
	require.Nil(t, ign)
	assert.Equal(t, []string{"-i", home + "/.ssh/key"}, args, "the shell would expand the ~")
}

func TestParseSSHCommandLineIgnoresWhatItDoesNotCarryOver(t *testing.T) {
	sshHome(t)
	tests := []struct {
		line string
		code string
		arg  string
	}{
		{"plink -i k", IgnoreProgram, "plink"},
		{"/opt/wrapper/ssh-wrapper -i k", IgnoreProgram, "/opt/wrapper/ssh-wrapper"},
		{"FOO=bar ssh -4", IgnoreProgram, "FOO=bar"},
		{"tsh ssh", IgnoreProgram, "tsh"},
		{"", IgnoreProgram, ""},
		{"ssh -J bastion", IgnoreOption, "-J"},
		{"ssh -o ProxyCommand=nc", IgnoreOption, "ProxyCommand"},
		{"ssh -o 'ProxyJump bastion'", IgnoreOption, "ProxyJump"},
		{"ssh -o LocalCommand=x", IgnoreOption, "LocalCommand"},
		{"ssh -o KnownHostsCommand=x", IgnoreOption, "KnownHostsCommand"},
		{"ssh -o Include=/c/config", IgnoreOption, "Include"},
		{"ssh -o PermitLocalCommand=yes", IgnoreOption, "PermitLocalCommand"},
		{"ssh -o Match", IgnoreOption, "Match"},
		{"ssh -o", IgnoreOption, "-o"},
		{"ssh -o Foo", IgnoreOption, "Foo"},
		{"ssh -o StrictHostKeyChecking=maybe", IgnoreOption, "StrictHostKeyChecking=maybe"},
		{"ssh -o HashKnownHosts=perhaps", IgnoreOption, "HashKnownHosts=perhaps"},
		{"ssh -o ConnectTimeout=soon", IgnoreOption, "ConnectTimeout=soon"},
		{"ssh -o Port=70000", IgnoreOption, "Port=70000"},
		{"ssh -o User=-oProxyCommand=x", IgnoreOption, "User=-oProxyCommand=x"},
		{`ssh -o UserKnownHostsFile="/a b"`, IgnoreRelative, "b"},
		{"ssh -p", IgnoreOption, "-p"},
		{"ssh -p 0", IgnoreOption, "-p 0"},
		{"ssh -p http", IgnoreOption, "-p http"},
		{"ssh -l -oProxyCommand=x", IgnoreOption, "-l -oProxyCommand=x"},
		{"ssh -L 8080:localhost:80", IgnoreOption, "-L"},
		{"ssh -A", IgnoreOption, "-A"},
		{"ssh -X", IgnoreOption, "-X"},
		{"ssh -W host:22", IgnoreOption, "-W"},
		{"ssh -S /tmp/ctl", IgnoreOption, "-S"},
		{"ssh -4q", IgnoreOption, "-4q"},
		{"ssh --", IgnoreOption, "--"},
		{"ssh git.example.test", IgnoreOption, "git.example.test"},
		{"ssh -F relative/config", IgnoreRelative, "relative/config"},
		{"ssh -o UserKnownHostsFile=relative/kh", IgnoreRelative, "relative/kh"},
		{"ssh -o GlobalKnownHostsFile=kh", IgnoreRelative, "kh"},
		{"ssh -i k; touch marker", IgnoreShell, ";"},
	}
	for _, tt := range tests {
		args, ign := parseSSHCommandLine(tt.line)
		require.NotNil(t, ign, "%q gave %q", tt.line, args)
		assert.Equal(t, tt.code, ign.Code, tt.line)
		assert.Equal(t, tt.arg, ign.Arg, tt.line)
		assert.Nil(t, args, tt.line)
		assert.NotEmpty(t, ign.String())
	}
}

// -F names a configuration file of the user (it can hold Match exec or
// ProxyCommand lines, like ~/.ssh/config): it is carried over, as an absolute path.
func TestParseSSHCommandLineCarriesOverF(t *testing.T) {
	sshHome(t)
	args, ign := parseSSHCommandLine("ssh -F /home/me/ssh_config -i /k/id")
	require.Nil(t, ign)
	assert.Equal(t, []string{"-F", "/home/me/ssh_config", "-i", "/k/id"}, args)

	args, ign = parseSSHCommandLine("ssh -F relative/ssh_config")
	require.NotNil(t, ign)
	assert.Equal(t, IgnoreRelative, ign.Code)
	assert.Nil(t, args)
}

func TestShellLineRoundTrips(t *testing.T) {
	sshHome(t)
	c := SSHCommand{Args: []string{"-i", "/a b/it's", "-o", "UserKnownHostsFile=/c/kh", "-p", "2222", "-o", "HostKeyAlias=$x;`y`"}}
	line := c.ShellLine()
	assert.Equal(t, `-i '/a b/it'\''s' -o UserKnownHostsFile=/c/kh -p 2222 -o 'HostKeyAlias=$x;`+"`y`"+`'`, line)

	words, ign := splitShell("ssh " + line)
	require.Nil(t, ign)
	assert.Equal(t, append([]string{"ssh"}, c.Args...), words)

	assert.Empty(t, SSHCommand{}.ShellLine())

	if runtime.GOOS == "windows" {
		t.Skip("sh")
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not found")
	}
	marker := filepath.Join(t.TempDir(), "marker")
	args := []string{"-i", "/a b/it's $(touch " + marker + ")", "-o", "X=`touch " + marker + "`"}
	out, err := exec.Command(sh, "-c", `printf '%s\n' `+SSHCommand{Args: args}.ShellLine()).Output()
	require.NoError(t, err)
	assert.Equal(t, strings.Join(args, "\n")+"\n", string(out), "sh gives the arguments back as they were")
	assert.NoFileExists(t, marker, "quoting must keep the shell from running anything")
}

func TestCustomSSHOrderOfGit(t *testing.T) {
	sshHome(t)
	ctx := t.Context()
	global := os.Getenv("GIT_CONFIG_GLOBAL")
	require.NoError(t, os.WriteFile(global, []byte("[core]\n\tsshCommand = ssh -i /global/key\n"), 0o600))

	// the global configuration
	c := CustomSSH(ctx)
	assert.Equal(t, SSHSourceConfig, c.Source)
	assert.Equal(t, []string{"-i", "/global/key"}, c.Args)
	assert.True(t, c.Followed())

	// GIT_SSH_COMMAND comes before it
	t.Setenv("GIT_SSH_COMMAND", "ssh -i /env/key -4")
	c = CustomSSH(ctx)
	assert.Equal(t, SSHSourceCommandEnv, c.Source)
	assert.Equal(t, []string{"-i", "/env/key", "-4"}, c.Args)

	// GIT_SSH comes after them, and has no options to carry over
	t.Setenv("GIT_SSH_COMMAND", "")
	t.Setenv("GIT_SSH", "/opt/wrapper/my-ssh")
	require.NoError(t, os.WriteFile(global, nil, 0o600))
	c = CustomSSH(ctx)
	assert.Equal(t, SSHSourceProgramEnv, c.Source)
	assert.Empty(t, c.Args)
	require.NotNil(t, c.Ignored)
	assert.Equal(t, IgnoreProgram, c.Ignored.Code)
	assert.Equal(t, "/opt/wrapper/my-ssh", c.Ignored.Arg)
	assert.False(t, c.Followed())

	t.Setenv("GIT_SSH", "/usr/bin/ssh")
	assert.Equal(t, SSHCommand{}, CustomSSH(ctx), "GIT_SSH naming ssh itself says nothing")

	t.Setenv("GIT_SSH", "")
	assert.Equal(t, SSHCommand{}, CustomSSH(ctx), "plain ssh")
}

// The first source found is the only one: a command that is not followed does
// not let git's lower-priority sources through.
func TestCustomSSHStopsAtTheFirstSource(t *testing.T) {
	sshHome(t)
	require.NoError(t, os.WriteFile(os.Getenv("GIT_CONFIG_GLOBAL"), []byte("[core]\n\tsshCommand = ssh -i /global/key\n"), 0o600))
	t.Setenv("GIT_SSH_COMMAND", "plink -i /env/key")
	c := CustomSSH(t.Context())
	assert.Equal(t, SSHSourceCommandEnv, c.Source)
	assert.Empty(t, c.Args)
	require.NotNil(t, c.Ignored)
	assert.Equal(t, "plink", c.Ignored.Arg)
}

// The global configuration is read with its includes.
func TestCustomSSHFollowsTheIncludesOfTheGlobalConfiguration(t *testing.T) {
	sshHome(t)
	dir := t.TempDir()
	included := filepath.Join(dir, "ssh.gitconfig")
	require.NoError(t, os.WriteFile(included, []byte("[core]\n\tsshCommand = ssh -i /included/key\n"), 0o600))
	require.NoError(t, os.WriteFile(os.Getenv("GIT_CONFIG_GLOBAL"), []byte("[include]\n\tpath = "+filepath.ToSlash(included)+"\n"), 0o600))
	c := CustomSSH(t.Context())
	assert.Equal(t, []string{"-i", "/included/key"}, c.Args)
}

// A command line is never run: the markers its shell syntax would create do
// not exist, whatever the source.
func TestCustomSSHNeverRunsTheCommand(t *testing.T) {
	sshHome(t)
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	lines := []string{
		"ssh -i k; touch " + marker,
		"ssh -i $(touch " + marker + ")",
		"ssh -i `touch " + marker + "`",
		"touch " + marker,
		"sh -c 'touch " + marker + "'",
		"ssh -o ProxyCommand='touch " + marker + "'",
		"ssh -o LocalCommand='touch " + marker + "' -o PermitLocalCommand=yes",
	}
	for _, line := range lines {
		t.Setenv("GIT_SSH_COMMAND", line)
		c := CustomSSH(t.Context())
		assert.NotNil(t, c.Ignored, line)
		assert.Empty(t, c.Args, line)
	}
	assert.NoFileExists(t, marker)

	// GIT_SSH is a program: it is not run either
	if runtime.GOOS != "windows" {
		t.Setenv("GIT_SSH_COMMAND", "")
		prog := filepath.Join(dir, "prog")
		require.NoError(t, os.WriteFile(prog, []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o700))
		t.Setenv("GIT_SSH", prog)
		c := CustomSSH(t.Context())
		assert.Equal(t, SSHSourceProgramEnv, c.Source)
		assert.NoFileExists(t, marker)
	}
}

func TestIsSSHProgram(t *testing.T) {
	for p, want := range map[string]bool{
		"ssh": true, "/usr/bin/ssh": true, `C:\Program Files\Git\usr\bin\ssh.exe`: true, "SSH": true,
		"plink": false, "ssh-wrapper": false, "/usr/bin/ssh2": false, "": false, "sshpass": false,
	} {
		assert.Equal(t, want, isSSHProgram(p), p)
	}
}
