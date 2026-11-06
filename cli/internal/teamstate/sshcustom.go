package teamstate

import (
	"context"
	"log/slog"
	"sync"

	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// The ssh command of git. git runs the ssh of GIT_SSH_COMMAND, of
// core.sshCommand or of GIT_SSH, with the options (-i, -F, -o
// UserKnownHostsFile…) its owner chose. The git processes of oh and the test
// of the server (`ssh -G`) must both use them, or the known_hosts that oh
// reads and writes is not the one the connection checks: a server confirmed in
// a file ssh does not read would be accepted without a word at the first
// connection. gitutil.CustomSSH reads the command; only the options it vets
// are carried over (gitutil/sshcommand.go), and the command itself is never
// run, only `ssh`.

// SSHHostOption refines how the SSH server of a remote is resolved.
type SSHHostOption func(*sshHostSettings)

type sshHostSettings struct{ repoDir string }

// InRepo names the clone git runs in for the remote: its .git/config may hold
// a core.sshCommand. Without it only the environment and the global
// configuration of git are read (the assistants run before the clone exists).
func InRepo(dir string) SSHHostOption {
	return func(s *sshHostSettings) { s.repoDir = dir }
}

func newSSHHostSettings(opts []SSHHostOption) sshHostSettings {
	var s sshHostSettings
	for _, o := range opts {
		o(&s)
	}
	return s
}

// ignoredSSHSeen holds the ignored commands already logged: a daemon looks at
// the same command every minute.
var ignoredSSHSeen sync.Map

// noteIgnoredSSH logs, once per process and per reason, an ssh command of git
// that oh does not follow (GIT_SSH is told at the lower level: oh never uses
// it, and it is common).
func noteIgnoredSSH(c gitutil.SSHCommand) {
	if c.Ignored == nil {
		return
	}
	key := c.Source + "\x00" + c.Ignored.Code + "\x00" + c.Ignored.Arg
	if _, seen := ignoredSSHSeen.LoadOrStore(key, true); seen {
		return
	}
	level := slog.LevelWarn
	if c.Source == gitutil.SSHSourceProgramEnv {
		level = slog.LevelInfo
	}
	slog.Default().Log(context.Background(), level, "teamstate.ssh.command_ignored", "source", c.Source, "reason", c.Ignored.String())
}

// CustomSSHNote says in one sentence what became of the ssh command of git for
// this server: the options oh carries over, or why it does not follow the
// command. warn is true for the second; empty when git has no custom command.
func (h *SSHHost) CustomSSHNote() (note string, warn bool) {
	c := h.Custom
	switch {
	case c.Ignored != nil && c.Source == gitutil.SSHSourceProgramEnv:
		return i18n.Tf("v55s.ssh.custom.gitssh", c.Ignored.Arg), true
	case c.Ignored != nil:
		return i18n.Tf("v55s.ssh.custom.ignored", c.Source, ignoreReason(c.Ignored)), true
	case c.Followed():
		return i18n.Tf("v55s.ssh.custom.followed", c.Source, c.ShellLine()), false
	}
	return "", false
}

// ignoreReason is why a command is not followed, in the language of the user.
func ignoreReason(i *gitutil.SSHIgnore) string {
	switch i.Code {
	case gitutil.IgnoreProgram:
		return i18n.Tf("v55s.ssh.reason.program", i.Arg)
	case gitutil.IgnoreShell:
		return i18n.Tf("v55s.ssh.reason.shell", i.Arg)
	case gitutil.IgnoreQuote:
		return i18n.T("v55s.ssh.reason.quote")
	case gitutil.IgnoreOption:
		return i18n.Tf("v55s.ssh.reason.option", i.Arg)
	case gitutil.IgnoreUntrusted:
		return i18n.Tf("v55s.ssh.reason.untrusted", i.Arg)
	case gitutil.IgnoreRelative:
		return i18n.Tf("v55s.ssh.reason.relative", i.Arg)
	}
	return i.String()
}
