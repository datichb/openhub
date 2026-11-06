package gitutil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/datichb/openhub/cli/internal/gitremote"
)

// The ssh command of git. For an SSH remote git runs `ssh`, or what it is told
// to run, in this order: GIT_SSH_COMMAND, core.sshCommand, GIT_SSH. oh needs to
// know the options of that command (which key, which configuration file,
// which known_hosts) to ask `ssh -G` the same questions as git's ssh would
// answer, and to hand the same options to the ssh that its own git processes
// run.
//
// The command itself is never run, and neither is any piece of it: a command
// line is split the way a shell does, and only a short list of ssh options
// (see sshFlags, sshValueOptions and sshOptionKinds) is carried over, rebuilt
// as arguments of `ssh`. Anything else makes oh drop the whole command and say
// so (SSHIgnore): shell syntax (variable, substitution, pipe, glob…), another
// program than ssh, an option that runs something (ProxyCommand, LocalCommand,
// KnownHostsCommand, Include…) or that oh does not understand. Half a command
// would be worse than none. Two options read a file that could run a program
// through the ssh configuration it holds (`Match exec`, ProxyCommand): -F is
// only carried over from the user's environment and global git configuration,
// never from the configuration of a repository, which an agent could have
// written.

// Where an ssh command comes from (SSHCommand.Source).
const (
	SSHSourceCommandEnv = "GIT_SSH_COMMAND"
	SSHSourceConfig     = "core.sshCommand"
	SSHSourceProgramEnv = "GIT_SSH"
)

// Why an ssh command is not followed (SSHIgnore.Code).
const (
	// IgnoreProgram: the program is not ssh (Arg is the program).
	IgnoreProgram = "program"
	// IgnoreShell: shell syntax that oh does not interpret (Arg is it).
	IgnoreShell = "shell"
	// IgnoreQuote: a quote or a backslash left open.
	IgnoreQuote = "quote"
	// IgnoreOption: an option oh does not carry over (Arg is it).
	IgnoreOption = "option"
	// IgnoreUntrusted: an option that only the environment and the global
	// configuration of git may give (Arg is it).
	IgnoreUntrusted = "untrusted"
	// IgnoreRelative: a file named relatively, which ssh would look for from
	// another folder than the one oh asks from (Arg is it).
	IgnoreRelative = "relative"
)

// SSHIgnore is the reason why an ssh command is not followed.
type SSHIgnore struct {
	Code string
	Arg  string
}

func (i *SSHIgnore) String() string {
	switch i.Code {
	case IgnoreProgram:
		return fmt.Sprintf("the program %q is not ssh", i.Arg)
	case IgnoreShell:
		return fmt.Sprintf("shell syntax %q is not interpreted", i.Arg)
	case IgnoreQuote:
		return "a quote or a backslash is left open"
	case IgnoreOption:
		return fmt.Sprintf("the option %q is not carried over", i.Arg)
	case IgnoreUntrusted:
		return fmt.Sprintf("the option %q is only carried over from the environment or the global git configuration", i.Arg)
	case IgnoreRelative:
		return fmt.Sprintf("the file %q is relative", i.Arg)
	}
	return i.Code
}

// SSHCommand is the ssh command git was told to run, as far as oh follows it.
type SSHCommand struct {
	// Source is where git reads its command from (SSHSource…); empty when git
	// runs plain ssh.
	Source string
	// Args are the ssh options of the command that oh carries over, vetted and
	// in order (nil for plain ssh, or when the command is not followed).
	Args []string
	// Ignored is why the command is not followed: oh then runs plain ssh.
	Ignored *SSHIgnore
}

// Followed reports whether the command gave ssh options that oh carries over.
func (c SSHCommand) Followed() bool { return c.Ignored == nil && len(c.Args) > 0 }

// ShellLine is Args as a piece of command line for sh (what GIT_SSH_COMMAND
// holds, and what the user can paste), every argument quoted when it needs to
// be; empty without Args.
func (c SSHCommand) ShellLine() string {
	parts := make([]string, len(c.Args))
	for i, a := range c.Args {
		parts[i] = shellWord(a)
	}
	return strings.Join(parts, " ")
}

func shellWord(s string) string {
	plain := func(r rune) bool {
		return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("_@%+=:,./-", r)
	}
	if s != "" && strings.IndexFunc(s, func(r rune) bool { return !plain(r) }) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// CustomSSH reads the ssh command git would run in the repository at repoDir
// ("": no repository, only the environment and the global configuration are
// read), in the order git does. A repository whose configuration is read is
// untrusted for -F (see above).
func CustomSSH(ctx context.Context, repoDir string) SSHCommand {
	if v := strings.TrimSpace(os.Getenv("GIT_SSH_COMMAND")); v != "" {
		return fromCommandLine(SSHSourceCommandEnv, v, true)
	}
	if repoDir != "" {
		if v, ok := gitremote.ConfigGet(repoDir, "core", "sshCommand"); ok && strings.TrimSpace(v) != "" {
			return fromCommandLine(SSHSourceConfig, v, false)
		}
	}
	if v := globalSSHCommand(ctx); v != "" {
		return fromCommandLine(SSHSourceConfig, v, true)
	}
	// GIT_SSH is a program, not a command line: it takes no options to carry
	// over. Only worth a word when it is not ssh.
	if v := strings.TrimSpace(os.Getenv("GIT_SSH")); v != "" && !isSSHProgram(v) {
		return SSHCommand{Source: SSHSourceProgramEnv, Ignored: &SSHIgnore{Code: IgnoreProgram, Arg: v}}
	}
	return SSHCommand{}
}

// globalSSHCommand reads core.sshCommand of the global configuration of git
// (includes followed), now: the daemon lives longer than a configuration.
func globalSSHCommand(ctx context.Context) string {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, "git", "config", "--global", "--includes", "--get", "core.sshCommand").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ParseSSHCommand reads a command line the way CustomSSH does, for a source
// that is trusted (the environment, the global configuration) or not (the
// configuration of a repository).
func ParseSSHCommand(source, line string, trusted bool) SSHCommand {
	return fromCommandLine(source, line, trusted)
}

func fromCommandLine(source, line string, trusted bool) SSHCommand {
	c := SSHCommand{Source: source}
	args, ign := parseSSHCommandLine(line, trusted)
	c.Args, c.Ignored = args, ign
	return c
}

// isSSHProgram: the file name of p is ssh (or ssh.exe).
func isSSHProgram(p string) bool {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		p = p[i+1:]
	}
	return strings.TrimSuffix(strings.ToLower(p), ".exe") == "ssh"
}

// sshFlags are the options without a value that are carried over.
var sshFlags = map[string]bool{
	"-4": true, "-6": true, "-a": true, "-A": true, "-C": true, "-q": true,
	"-T": true, "-x": true, "-v": true, "-vv": true, "-vvv": true,
}

type optionKind int

const (
	kindYesNo optionKind = iota
	kindEnum
	kindInt
	kindPath
	kindWord
	kindHostsFiles
)

type sshOptionKind struct {
	name string // as ssh spells it
	kind optionKind
	enum []string
}

// sshOptionKinds are the -o options that are carried over (keys in lower
// case): what picks the key and the known_hosts files, how the server is
// checked, and the timeouts. Not here: anything that runs a program or reads
// another configuration (ProxyCommand, ProxyJump, LocalCommand,
// KnownHostsCommand, Include, Match, PKCS11Provider…), which a bastion or a
// special setup needs and oh does not handle.
var sshOptionKinds = map[string]sshOptionKind{
	"userknownhostsfile":    {name: "UserKnownHostsFile", kind: kindHostsFiles},
	"globalknownhostsfile":  {name: "GlobalKnownHostsFile", kind: kindHostsFiles},
	"hostkeyalias":          {name: "HostKeyAlias", kind: kindWord},
	"hashknownhosts":        {name: "HashKnownHosts", kind: kindYesNo},
	"stricthostkeychecking": {name: "StrictHostKeyChecking", kind: kindEnum, enum: []string{"yes", "no", "ask", "off", "accept-new", "true", "false"}},
	"checkhostip":           {name: "CheckHostIP", kind: kindYesNo},
	"updatehostkeys":        {name: "UpdateHostKeys", kind: kindEnum, enum: []string{"yes", "no", "ask", "true", "false"}},
	"identityfile":          {name: "IdentityFile", kind: kindPath},
	"certificatefile":       {name: "CertificateFile", kind: kindPath},
	"identityagent":         {name: "IdentityAgent", kind: kindPath},
	"identitiesonly":        {name: "IdentitiesOnly", kind: kindYesNo},
	"pubkeyauthentication":  {name: "PubkeyAuthentication", kind: kindYesNo},
	"batchmode":             {name: "BatchMode", kind: kindYesNo},
	"port":                  {name: "Port", kind: kindInt},
	"user":                  {name: "User", kind: kindWord},
	"connecttimeout":        {name: "ConnectTimeout", kind: kindInt},
	"serveraliveinterval":   {name: "ServerAliveInterval", kind: kindInt},
	"serveralivecountmax":   {name: "ServerAliveCountMax", kind: kindInt},
}

// parseSSHCommandLine splits line like a shell and carries over the ssh
// options it knows. It runs nothing.
func parseSSHCommandLine(line string, trusted bool) ([]string, *SSHIgnore) {
	words, ign := splitShell(line)
	if ign != nil {
		return nil, ign
	}
	if len(words) == 0 {
		return nil, &SSHIgnore{Code: IgnoreProgram}
	}
	if !isSSHProgram(words[0]) {
		return nil, &SSHIgnore{Code: IgnoreProgram, Arg: words[0]}
	}
	var args []string
	rest := words[1:]
	for i := 0; i < len(rest); i++ {
		w := rest[i]
		if sshFlags[w] {
			args = append(args, w)
			continue
		}
		if len(w) < 2 || w[0] != '-' || !strings.ContainsRune("iploF", rune(w[1])) {
			return nil, &SSHIgnore{Code: IgnoreOption, Arg: w}
		}
		flag, value := w[:2], w[2:]
		if value == "" {
			if i+1 >= len(rest) {
				return nil, &SSHIgnore{Code: IgnoreOption, Arg: w}
			}
			i++
			value = rest[i]
		}
		switch flag {
		case "-F":
			if !trusted {
				return nil, &SSHIgnore{Code: IgnoreUntrusted, Arg: flag}
			}
			if !absoluteFile(value) {
				return nil, &SSHIgnore{Code: IgnoreRelative, Arg: value}
			}
			args = append(args, flag, value)
		case "-i":
			if value == "" {
				return nil, &SSHIgnore{Code: IgnoreOption, Arg: w}
			}
			args = append(args, flag, value)
		case "-p":
			if !validPort(value) {
				return nil, &SSHIgnore{Code: IgnoreOption, Arg: flag + " " + value}
			}
			args = append(args, flag, value)
		case "-l":
			if !validWord(value) {
				return nil, &SSHIgnore{Code: IgnoreOption, Arg: flag + " " + value}
			}
			args = append(args, flag, value)
		case "-o":
			opt, ign := vetOption(value)
			if ign != nil {
				return nil, ign
			}
			args = append(args, flag, opt)
		}
	}
	return args, nil
}

// vetOption checks the « Key=value » (or « Key value ») of -o and returns it
// as ssh reads it back unambiguously: Key=value.
func vetOption(s string) (string, *SSHIgnore) {
	s = strings.TrimSpace(s)
	cut := strings.IndexAny(s, "= \t")
	if cut <= 0 {
		return "", &SSHIgnore{Code: IgnoreOption, Arg: s}
	}
	key := strings.ToLower(s[:cut])
	val := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s[cut:]), "="))
	spec, ok := sshOptionKinds[key]
	if !ok {
		return "", &SSHIgnore{Code: IgnoreOption, Arg: s[:cut]}
	}
	bad := &SSHIgnore{Code: IgnoreOption, Arg: spec.name + "=" + val}
	if spec.kind != kindHostsFiles && len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
		val = val[1 : len(val)-1]
	}
	if val == "" || strings.ContainsAny(val, "\"\x00\r\n") {
		return "", bad
	}
	switch spec.kind {
	case kindYesNo:
		if v := strings.ToLower(val); v != "yes" && v != "no" {
			return "", bad
		}
	case kindEnum:
		found := false
		for _, e := range spec.enum {
			found = found || strings.EqualFold(val, e)
		}
		if !found {
			return "", bad
		}
	case kindInt:
		if n, err := strconv.Atoi(val); err != nil || n < 0 || n > 65535 {
			return "", bad
		}
	case kindWord:
		if !validWord(val) {
			return "", bad
		}
	case kindHostsFiles:
		for _, f := range strings.Fields(val) {
			if !strings.EqualFold(f, "none") && f != os.DevNull && !absoluteFile(f) {
				return "", &SSHIgnore{Code: IgnoreRelative, Arg: f}
			}
		}
	}
	return spec.name + "=" + val, nil
}

func validPort(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n > 0 && n <= 65535
}

func validWord(s string) bool {
	return s != "" && !strings.HasPrefix(s, "-") && !strings.ContainsAny(s, " \t\r\n\x00")
}

// absoluteFile: the file does not depend on the folder ssh runs from (~, %d
// and absolute paths; ssh expands the first two).
func absoluteFile(f string) bool {
	return f == "~" || strings.HasPrefix(f, "~/") || strings.HasPrefix(f, `~\`) || strings.HasPrefix(f, "%d") ||
		filepath.IsAbs(f) || strings.HasPrefix(f, "/")
}

// splitShell splits a command line the way sh does for plain words: blanks
// separate words, '…' and "…" group, a backslash escapes, an unquoted ~ and
// $HOME / ${HOME} are the home folder. Anything else a shell would interpret
// (other variables, substitutions, pipes, redirections, globs, lists,
// comments, newlines) is not understood: the command is not followed.
func splitShell(line string) ([]string, *SSHIgnore) {
	var (
		words  []string
		cur    strings.Builder
		inWord bool
	)
	home, _ := os.UserHomeDir()
	shell := func(what string) *SSHIgnore { return &SSHIgnore{Code: IgnoreShell, Arg: what} }
	quote := &SSHIgnore{Code: IgnoreQuote}
	if !utf8.ValidString(line) {
		return nil, shell("invalid text")
	}
	rs := []rune(line)
	// variable reads $HOME or ${HOME} at rs[i] (the $) and returns the index of
	// its last rune; any other variable is not understood.
	variable := func(i int) (end int, ok bool) {
		rest := string(rs[i+1:])
		switch {
		case strings.HasPrefix(rest, "{HOME}"):
			return i + len("{HOME}"), home != ""
		case strings.HasPrefix(rest, "HOME"):
			if after := []rune(rest[len("HOME"):]); len(after) > 0 && (after[0] == '_' || unicode.IsLetter(after[0]) || unicode.IsDigit(after[0])) {
				return 0, false
			}
			return i + len("HOME"), home != ""
		}
		return 0, false
	}
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch {
		case c == '\n' || c == '\r' || c < 0x20 && c != '\t':
			return nil, shell("control character")
		case c == ' ' || c == '\t':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		case c == '\'':
			inWord = true
			i++
			for ; i < len(rs) && rs[i] != '\''; i++ {
				if rs[i] < 0x20 && rs[i] != '\t' {
					return nil, shell("control character")
				}
				cur.WriteRune(rs[i])
			}
			if i >= len(rs) {
				return nil, quote
			}
		case c == '"':
			inWord = true
			i++
			for ; i < len(rs) && rs[i] != '"'; i++ {
				switch d := rs[i]; {
				case d < 0x20 && d != '\t':
					return nil, shell("control character")
				case d == '`':
					return nil, shell("`")
				case d == '$':
					end, ok := variable(i)
					if !ok {
						return nil, shell("$")
					}
					cur.WriteString(home)
					i = end
				case d == '\\' && i+1 < len(rs) && strings.ContainsRune("$`\"\\", rs[i+1]):
					i++
					cur.WriteRune(rs[i])
				default:
					cur.WriteRune(d)
				}
			}
			if i >= len(rs) {
				return nil, quote
			}
		case c == '\\':
			if i+1 >= len(rs) {
				return nil, quote
			}
			i++
			if rs[i] < 0x20 && rs[i] != '\t' {
				return nil, shell("control character")
			}
			inWord = true
			cur.WriteRune(rs[i])
		case c == '$':
			end, ok := variable(i)
			if !ok {
				return nil, shell("$")
			}
			inWord = true
			cur.WriteString(home)
			i = end
		case c == '~' && !inWord:
			if i+1 < len(rs) && rs[i+1] != '/' && rs[i+1] != ' ' && rs[i+1] != '\t' {
				return nil, shell("~")
			}
			if home == "" {
				return nil, shell("~")
			}
			inWord = true
			cur.WriteString(home)
		case c == '#' && !inWord:
			return nil, shell("#")
		case strings.ContainsRune(";&|<>()`*?[{", c):
			return nil, shell(string(c))
		default:
			inWord = true
			cur.WriteRune(c)
		}
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words, nil
}
