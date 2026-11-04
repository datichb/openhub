package teamstate

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
)

// GitCredentials are the user name and the token git is given for an HTTPS
// remote. An empty Token means none.
type GitCredentials struct {
	Username string
	Token    string
}

// CredentialSource gives the credentials oh holds for remote (the keychain of
// oh), or the zero value. It is called before every git operation on an
// HTTPS remote: it must be cheap.
type CredentialSource func(ctx context.Context, remote string) GitCredentials

var credentialSource atomic.Pointer[CredentialSource]

// SetCredentialSource sets where the token of an HTTPS team-state repository
// is read (nil: nowhere, git uses what the machine has). It returns what
// puts the previous source back.
func SetCredentialSource(src CredentialSource) (restore func()) {
	prev := credentialSource.Load()
	if src == nil {
		credentialSource.Store(nil)
	} else {
		credentialSource.Store(&src)
	}
	return func() { credentialSource.Store(prev) }
}

func credentialsFor(ctx context.Context, remote string) GitCredentials {
	src := credentialSource.Load()
	if src == nil || !IsHTTPS(remote) {
		return GitCredentials{}
	}
	return (*src)(ctx, remote)
}

// askpass is a one-shot GIT_ASKPASS: a private directory (0700) holding the
// user name and the token in two files (0600) and a script that prints one
// or the other, according to what git asks. The token is never in a command
// line, in the environment or in a URL; git asks the script only for the
// one operation it is started for, and Close removes the directory.
type askpass struct {
	dir string
	env string
}

// newAskpass writes the askpass of creds. It fails, and the caller goes on
// without (git then uses what the machine has), when the directory or the
// files cannot be written.
func newAskpass(creds GitCredentials) (*askpass, error) {
	dir, err := os.MkdirTemp("", "oh-askpass-")
	if err != nil {
		return nil, err
	}
	a := &askpass{dir: dir}
	user := creds.Username
	if user == "" {
		user = "oauth2"
	}
	userFile, tokenFile := filepath.Join(dir, "user"), filepath.Join(dir, "token")
	script := filepath.Join(dir, "askpass.sh")
	for _, f := range []struct {
		path string
		data string
		mode os.FileMode
	}{
		{userFile, user, 0o600},
		{tokenFile, creds.Token, 0o600},
		{script, askpassScript(userFile, tokenFile), 0o700},
	} {
		if err := os.WriteFile(f.path, []byte(f.data), f.mode); err != nil {
			a.Close()
			return nil, err
		}
	}
	a.env = "GIT_ASKPASS=" + shellPath(script)
	return a, nil
}

// askpassScript answers the user name for « Username for … » and the token
// for everything else. It only reads the two files.
func askpassScript(userFile, tokenFile string) string {
	return "#!/bin/sh\ncase \"$1\" in\n[Uu]sername*) cat " + shellQuote(shellPath(userFile)) + " ;;\n*) cat " + shellQuote(shellPath(tokenFile)) + " ;;\nesac\n"
}

// shellPath is a path as the shell that runs the script reads it: on Windows
// Git for Windows runs it with its own sh, which takes forward slashes.
func shellPath(p string) string {
	if runtime.GOOS == "windows" {
		return filepath.ToSlash(p)
	}
	return p
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// Close removes the directory of the askpass.
func (a *askpass) Close() {
	if a != nil && a.dir != "" {
		_ = os.RemoveAll(a.dir)
	}
}

// networkCommands are the git commands that talk to the remote.
var networkCommands = map[string]bool{"clone": true, "pull": true, "push": true, "fetch": true, "ls-remote": true}

// gitCommand builds `git args…` for an operation on remote (the remote of the
// repository, "" when unknown): no terminal prompt, a non-interactive ssh
// and, when oh holds a token for an HTTPS remote, the askpass that gives it
// to git. cleanup removes the askpass: call it when the command is over.
//
// creds, when not nil, are given to git instead of the ones the source holds
// (a token that is checked before it is kept).
func gitCommand(ctx context.Context, remote, dir string, creds *GitCredentials, sshOpts string, env []string, args ...string) (cmd *exec.Cmd, cleanup func()) {
	given := GitCredentials{}
	if creds != nil {
		given = *creds
	} else {
		given = credentialsFor(ctx, remote)
	}
	cleanup = func() {}
	var askEnv string
	if given.Token != "" {
		if ap, err := newAskpass(given); err != nil {
			slog.Warn("teamstate.askpass.failed", "error", err)
		} else {
			askEnv, cleanup = ap.env, ap.Close
			// The token is oh's, for this operation: no credential helper of the
			// machine gets it to keep (git approves what worked), nor answers
			// before it with an older one.
			args = append([]string{"-c", "credential.helper="}, args...)
		}
	}
	cmd = exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_SSH_COMMAND=ssh -o BatchMode=yes "+sshOpts,
	)
	if networkCommand(args) {
		// The refusals of a server are recognised by their words (IsAuthError):
		// git says them in English, not in the language of the machine.
		cmd.Env = append(cmd.Env, "LC_ALL=C")
	}
	cmd.Env = append(cmd.Env, env...)
	if askEnv != "" {
		cmd.Env = append(cmd.Env, askEnv)
	}
	return cmd, cleanup
}

// networkCommand reports whether args is a git command that talks to the
// remote (past the -c options).
func networkCommand(args []string) bool {
	for i := 0; i < len(args); i++ {
		if args[i] == "-c" {
			i++
			continue
		}
		return networkCommands[args[i]]
	}
	return false
}
