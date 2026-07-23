// Package termlaunch opens a new terminal window/tab and runs a command in a
// given directory. It detects the user's terminal emulator via $TERM_PROGRAM
// and uses the appropriate mechanism:
//
//   - Apple_Terminal → osascript (Terminal.app)
//   - iTerm.app      → osascript (iTerm2)
//   - fallback       → open -a Terminal <dir>  (macOS generic)
//
// The call is non-blocking: it returns as soon as the new window/tab is
// requested. On non-macOS systems the fallback runs the command in the same
// terminal via os/exec (blocking).
package termlaunch

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// OpenInNewTerminal opens a new terminal window/tab, sets the working directory
// to dir, and runs command. Returns nil on success.
func OpenInNewTerminal(dir, command string) error {
	if runtime.GOOS != "darwin" {
		// Non-macOS fallback: run inline (blocks).
		cmd := exec.Command("sh", "-c", command)
		cmd.Dir = dir
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	termProg := os.Getenv("TERM_PROGRAM")
	switch termProg {
	case "iTerm.app":
		return openITerm(dir, command)
	case "Apple_Terminal":
		return openAppleTerminal(dir, command)
	default:
		// Unknown terminal: try Apple_Terminal as best-effort.
		return openAppleTerminal(dir, command)
	}
}

// Detect returns the detected terminal name for display purposes.
func Detect() string {
	if runtime.GOOS != "darwin" {
		return "system terminal"
	}
	switch os.Getenv("TERM_PROGRAM") {
	case "iTerm.app":
		return "iTerm2"
	case "Apple_Terminal":
		return "Terminal.app"
	default:
		return "Terminal.app"
	}
}

func runOsascript(script string) error {
	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("osascript: %w", err)
	}
	// Detach: we don't wait for the script to complete.
	// Terminal.app / iTerm2 will handle the rest asynchronously.
	go func() { _ = cmd.Wait() }()
	return nil
}
