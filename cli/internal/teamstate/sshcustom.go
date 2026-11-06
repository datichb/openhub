package teamstate

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/datichb/openhub/cli/internal/gitremote"
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
//
// Only the sources of the user are read: GIT_SSH_COMMAND and the global
// configuration of git. The core.sshCommand of a repository, the clone of a
// team included, is not followed (its .git/config is not the user's alone, and
// its options decide which server keys are accepted): it is said, not obeyed.

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
	case gitutil.IgnoreRelative:
		return i18n.Tf("v55s.ssh.reason.relative", i.Arg)
	}
	return i.String()
}

// repoSSHCommandSet reports whether the .git/config of the clone at dir sets a
// core.sshCommand (read without a git process).
func repoSSHCommandSet(dir string) bool {
	if dir == "" {
		return false
	}
	v, ok := gitremote.ConfigGet(dir, "core", "sshCommand")
	return ok && strings.TrimSpace(v) != ""
}

// RepoSSHCommandNote says that the clone at dir sets a core.sshCommand of its
// own, which oh does not follow, and where to put it instead (empty when the
// clone sets none). It is not an error: git outside oh obeys it, and the git
// processes of oh run the ssh command of the user, whatever the repository says.
func RepoSSHCommandNote(dir string) string {
	if !repoSSHCommandSet(dir) {
		return ""
	}
	return i18n.Tf("v55s.ssh.repo_command", displayPath(dir))
}

// repoSSHCommandSeen holds the clones already logged.
var repoSSHCommandSeen sync.Map

// noteRepoSSHCommand logs, once per process and per clone, a core.sshCommand
// that the .git/config of dir sets and oh does not follow.
func noteRepoSSHCommand(ctx context.Context, dir string) {
	if !repoSSHCommandSet(dir) {
		return
	}
	if _, seen := repoSSHCommandSeen.LoadOrStore(dir, true); seen {
		return
	}
	slog.Default().Log(ctx, slog.LevelInfo, "teamstate.ssh.repo_command_ignored", "repo", dir,
		"hint", "core.sshCommand set in the repository is not followed: put it in the global git configuration or in GIT_SSH_COMMAND")
}
