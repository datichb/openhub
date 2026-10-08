package termlaunch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Location is where an interactive client runs: enough to bring its window
// back to the front (A43: one session = one window).
type Location struct {
	Program  string `json:"program,omitempty"`   // $TERM_PROGRAM of the client terminal (iTerm.app, Apple_Terminal…)
	TTY      string `json:"tty,omitempty"`       // terminal device of the client (/dev/ttys003)
	TmuxPane string `json:"tmux_pane,omitempty"` // $TMUX_PANE when the client runs inside tmux
}

// Empty reports whether nothing locates the client.
func (l Location) Empty() bool { return l.TTY == "" && l.TmuxPane == "" }

// ErrNotFound means no window of a running terminal holds the client.
var ErrNotFound = errors.New("no terminal window holds the client")

// Here returns the location of the current process (its standard input).
func Here() Location {
	l := Location{Program: os.Getenv("TERM_PROGRAM")}
	if os.Getenv("TMUX") != "" {
		l.TmuxPane = os.Getenv("TMUX_PANE")
	}
	l.TTY = stdinTTY()
	return l
}

// stdinTTY is a variable for the tests.
var stdinTTY = func() string {
	if runtime.GOOS == "windows" {
		return ""
	}
	c := exec.Command("tty")
	c.Stdin = os.Stdin
	out, err := c.Output()
	if err != nil {
		return ""
	}
	tty := strings.TrimSpace(string(out))
	if !strings.HasPrefix(tty, "/dev/") {
		return ""
	}
	return tty
}

// Focus brings the window (tab, tmux window and pane) holding a client to
// the front. It returns ErrNotFound when no running terminal holds it; it
// never starts a terminal application.
func Focus(ctx context.Context, l Location) error {
	tty := l.TTY
	if l.TmuxPane != "" {
		if err := focusTmux(ctx, l.TmuxPane); err != nil {
			return err
		}
		// The terminal showing the tmux client, best effort.
		tty, _ = tmuxOut(ctx, "display-message", "-p", "-t", l.TmuxPane, "#{client_tty}")
		if tty == "" {
			return nil
		}
		_ = focusTTY(ctx, l.Program, tty)
		return nil
	}
	if tty == "" {
		return ErrNotFound
	}
	return focusTTY(ctx, l.Program, tty)
}

func focusTmux(ctx context.Context, pane string) error {
	if _, err := tmuxOut(ctx, "select-window", "-t", pane); err != nil {
		return ErrNotFound // pane gone (client closed, tmux server stopped)
	}
	_, _ = tmuxOut(ctx, "select-pane", "-t", pane)
	return nil
}

// focusTTY looks for the terminal tab whose device is tty: in the terminal
// of the client first, then in the other macOS terminal.
func focusTTY(ctx context.Context, program, tty string) error {
	if runtime.GOOS != "darwin" {
		return ErrNotFound
	}
	scripts := []string{iTermFocusScript(tty), terminalFocusScript(tty)}
	if program == "Apple_Terminal" {
		scripts[0], scripts[1] = scripts[1], scripts[0]
	}
	for _, s := range scripts {
		out, err := osascriptOut(ctx, s)
		if err == nil && strings.TrimSpace(out) == "ok" {
			return nil
		}
	}
	return ErrNotFound
}

// iTermFocusScript selects the iTerm2 session on tty (window, tab, pane).
// "is running" keeps a closed iTerm2 closed.
func iTermFocusScript(tty string) string {
	return `if application "iTerm2" is running then
	tell application "iTerm2"
		repeat with w in windows
			repeat with t in tabs of w
				repeat with s in sessions of t
					if tty of s is ` + asString(tty) + ` then
						tell w to select
						tell t to select
						tell s to select
						activate
						return "ok"
					end if
				end repeat
			end repeat
		end repeat
	end tell
end if
return "none"`
}

// terminalFocusScript selects the Terminal.app tab on tty.
func terminalFocusScript(tty string) string {
	return `if application "Terminal" is running then
	tell application "Terminal"
		repeat with w in windows
			repeat with t in tabs of w
				if tty of t is ` + asString(tty) + ` then
					set selected of t to true
					set index of w to 1
					activate
					return "ok"
				end if
			end repeat
		end repeat
	end tell
end if
return "none"`
}

// osascriptOut runs a script and returns its result (variable for the tests).
var osascriptOut = func(ctx context.Context, script string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "osascript", "-e", script).Output()
	if err != nil {
		return "", fmt.Errorf("osascript: %w", err)
	}
	return string(out), nil
}

// tmuxOut runs tmux and returns its trimmed output (variable for the tests).
var tmuxOut = func(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tmux", args...).Output()
	if err != nil {
		return "", fmt.Errorf("tmux: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
