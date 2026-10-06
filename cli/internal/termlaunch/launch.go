package termlaunch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Pref selects where an interactive client is opened.
type Pref string

const (
	PrefAuto     Pref = "auto"
	PrefITerm    Pref = "iterm"
	PrefTerminal Pref = "terminal"
	PrefTmux     Pref = "tmux"
)

// ITermStyle selects how iTerm2 opens the client.
type ITermStyle string

const (
	ITermTab    ITermStyle = "tab"
	ITermSplit  ITermStyle = "split"
	ITermWindow ITermStyle = "window"
)

// ErrNoTerminal means no terminal method succeeded; the caller falls back to
// the browser or to running the client in the current terminal.
var ErrNoTerminal = errors.New("no terminal available to open the session")

// Options describes a client to open.
type Options struct {
	Pref       Pref
	ITermStyle ITermStyle
	Dir        string
	Argv       []string // command to run (no secrets: they would appear in the shell history)
	// Env holds variables set for the command only (non-secret, e.g.
	// OH_HOME), written as shell assignments before it.
	Env   map[string]string
	Title string
}

// Method is one way of opening a terminal.
type Method string

const (
	MethodITerm    Method = "iterm"
	MethodTerminal Method = "terminal"
	MethodTmux     Method = "tmux"
)

// Chain returns the methods tried for a preference, in order.
//
// auto: the terminal oh runs in first (iTerm2 or Terminal.app), then the other
// macOS terminal if installed, then tmux when inside a tmux session.
func Chain(p Pref) []Method {
	inTmux := os.Getenv("TMUX") != ""
	switch p {
	case PrefITerm:
		return []Method{MethodITerm}
	case PrefTerminal:
		return []Method{MethodTerminal}
	case PrefTmux:
		return []Method{MethodTmux}
	}
	var out []Method
	if runtime.GOOS == "darwin" {
		if os.Getenv("TERM_PROGRAM") == "Apple_Terminal" {
			out = append(out, MethodTerminal)
			if iTermInstalled() {
				out = append(out, MethodITerm)
			}
		} else {
			if iTermInstalled() || os.Getenv("TERM_PROGRAM") == "iTerm.app" {
				out = append(out, MethodITerm)
			}
			out = append(out, MethodTerminal)
		}
	}
	if inTmux {
		out = append(out, MethodTmux)
	}
	return out
}

func iTermInstalled() bool {
	for _, p := range []string{"/Applications/iTerm.app", os.ExpandEnv("$HOME/Applications/iTerm.app")} {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// Attempt records one tried method.
type Attempt struct {
	Method Method
	Err    error
}

// Launch tries each method of the chain until one succeeds. It returns the
// method used, or ErrNoTerminal with the failed attempts.
func Launch(ctx context.Context, o Options) (Method, []Attempt, error) {
	var attempts []Attempt
	for _, m := range Chain(o.Pref) {
		err := launchWith(ctx, m, o)
		attempts = append(attempts, Attempt{Method: m, Err: err})
		if err == nil {
			return m, attempts, nil
		}
	}
	return "", attempts, ErrNoTerminal
}

func launchWith(ctx context.Context, m Method, o Options) error {
	cmd := ShellCommandEnv(o.Dir, o.Env, o.Argv)
	switch m {
	case MethodITerm:
		return osascript(ctx, iTermScript(cmd, o.ITermStyle))
	case MethodTerminal:
		return osascript(ctx, terminalScript(cmd))
	case MethodTmux:
		// One shell command argument: tmux < 3.0 does not take the command
		// as separate arguments (and `--` is not understood there).
		args := append(tmuxArgs(o), cmd)
		out, err := exec.CommandContext(ctx, "tmux", args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("tmux: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	return fmt.Errorf("unknown method %q", m)
}

// tmuxArgs opens the client in a new tmux window, or in a split pane when
// the style is "split" (the iTerm2 style setting applies to tmux too).
func tmuxArgs(o Options) []string {
	if o.ITermStyle == ITermSplit {
		return []string{"split-window", "-h", "-c", o.Dir}
	}
	args := []string{"new-window", "-c", o.Dir}
	if o.Title != "" {
		args = append(args, "-n", o.Title)
	}
	return args
}

// ShellCommand renders `cd <dir> && exec <argv…>` with POSIX quoting.
func ShellCommand(dir string, argv []string) string { return ShellCommandEnv(dir, nil, argv) }

// ShellCommandEnv renders `cd <dir> && NAME=value… exec <argv…>` with POSIX
// quoting (variables sorted by name; invalid names are skipped).
func ShellCommandEnv(dir string, env map[string]string, argv []string) string {
	q := make([]string, len(argv))
	for i, a := range argv {
		q[i] = shellQuote(a)
	}
	names := make([]string, 0, len(env))
	for k := range env {
		if validEnvName(k) {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	assign := ""
	for _, k := range names {
		assign += k + "=" + shellQuote(env[k]) + " "
	}
	s := assign + "exec " + strings.Join(q, " ")
	if dir != "" {
		s = "cd " + shellQuote(dir) + " && " + s
	}
	return s
}

func validEnvName(k string) bool {
	if k == "" || (k[0] >= '0' && k[0] <= '9') {
		return false
	}
	for _, r := range k {
		if r != '_' && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func shellQuote(s string) string {
	safe := func(r rune) bool {
		return r == '/' || r == '-' || r == '_' || r == '.' || r == '=' || r == ':' || r == '@' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
	}
	if s != "" && strings.IndexFunc(s, func(r rune) bool { return !safe(r) }) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// asString renders an AppleScript string literal.
func asString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func iTermScript(cmd string, style ITermStyle) string {
	switch style {
	case ITermWindow:
		return `tell application "iTerm2"
	activate
	set w to (create window with default profile)
	tell current session of w to write text ` + asString(cmd) + `
end tell`
	case ITermSplit:
		return `tell application "iTerm2"
	activate
	if (count of windows) = 0 then
		set w to (create window with default profile)
		tell current session of w to write text ` + asString(cmd) + `
	else
		tell current session of current window
			set s to (split vertically with default profile)
			tell s to write text ` + asString(cmd) + `
		end tell
	end if
end tell`
	default:
		return `tell application "iTerm2"
	activate
	if (count of windows) = 0 then
		set w to (create window with default profile)
		tell current session of w to write text ` + asString(cmd) + `
	else
		tell current window
			set t to (create tab with default profile)
			tell current session of t to write text ` + asString(cmd) + `
		end tell
	end if
end tell`
	}
}

func terminalScript(cmd string) string {
	return `tell application "Terminal"
	activate
	do script ` + asString(cmd) + `
end tell`
}

// osascript runs a script and waits for it, so that permission errors
// ("not authorized to send Apple events") are detected.
func osascript(ctx context.Context, script string) error {
	if runtime.GOOS != "darwin" {
		return errors.New("osascript requires macOS")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
