// Package sysnotify shows desktop notifications (P3-T13): terminal-notifier
// when installed (a click brings the terminal running oh back), else
// osascript on macOS, notify-send on Linux. Texts must never contain
// sensitive content (prompts, code, secrets).
package sysnotify

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Note is one notification.
type Note struct {
	Title   string
	Message string
	Group   string // replaces the previous notification of the same group (terminal-notifier)
	Sound   bool
}

// ErrUnavailable means no notification backend exists on this machine.
var ErrUnavailable = errors.New("no desktop notification backend")

// Notifier sends notifications.
type Notifier struct {
	// LookPath and Run are overridable in tests.
	LookPath func(string) (string, error)
	Run      func(ctx context.Context, name string, args ...string) error
	GOOS     string
	// Activate is the bundle id of the application brought to front on click
	// (default: the terminal oh runs in, from __CFBundleIdentifier).
	Activate string
}

// New returns a notifier for this machine.
func New() *Notifier {
	return &Notifier{
		LookPath: exec.LookPath,
		Run: func(ctx context.Context, name string, args ...string) error {
			return exec.CommandContext(ctx, name, args...).Run()
		},
		GOOS:     runtime.GOOS,
		Activate: os.Getenv("__CFBundleIdentifier"),
	}
}

// Command returns the command line used for a note (nil when unavailable).
func (n *Notifier) Command(note Note) []string {
	if p, err := n.LookPath("terminal-notifier"); err == nil {
		args := []string{p, "-title", note.Title, "-message", note.Message}
		if note.Group != "" {
			args = append(args, "-group", note.Group)
		}
		if note.Sound {
			args = append(args, "-sound", "default")
		}
		if n.Activate != "" {
			args = append(args, "-activate", n.Activate)
		}
		return args
	}
	switch n.GOOS {
	case "darwin":
		script := "display notification " + asString(note.Message) + " with title " + asString(note.Title)
		if note.Sound {
			script += ` sound name "default"`
		}
		return []string{"osascript", "-e", script}
	case "linux":
		if p, err := n.LookPath("notify-send"); err == nil {
			return []string{p, "--app-name=oh", note.Title, note.Message}
		}
	}
	return nil
}

// Notify shows a note.
func (n *Notifier) Notify(ctx context.Context, note Note) error {
	argv := n.Command(note)
	if argv == nil {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return n.Run(ctx, argv[0], argv[1:]...)
}

func asString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", " ").Replace(s) + `"`
}
