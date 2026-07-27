// Package shell — clipboard integration.
// Implements a two-strategy write pipeline:
//  1. OSC 52 escape sequence — works over SSH, inside tmux (with set-clipboard on),
//     and in any modern terminal emulator (iTerm2, WezTerm, Alacritty, kitty, etc.)
//  2. Native OS command fallback — pbcopy (macOS), wl-copy (Linux/Wayland),
//     xclip (Linux/X11). Used when OSC 52 is not supported or fails silently.
//
// No external dependencies are used. All clipboard tools invoked are standard
// OS utilities distributed with or commonly available on every target platform.
package shell

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// CopyToClipboard writes text to the system clipboard.
// It tries OSC 52 first (terminal-level, works over SSH/tmux), then falls back
// to the native OS clipboard command. Both strategies are attempted in sequence
// so that the text is available in both the terminal clipboard and the OS clipboard
// when possible.
// Returns nil if at least one strategy succeeded.
func CopyToClipboard(text string) error {
	osc52Err := writeOSC52(text)
	nativeErr := writeNative(text)

	if osc52Err == nil || nativeErr == nil {
		return nil
	}
	// Both failed — return the native error as it is the most actionable
	return fmt.Errorf("clipboard: OSC52: %w; native: %v", osc52Err, nativeErr)
}

// writeOSC52 writes text to the clipboard via the OSC 52 terminal escape sequence.
// The sequence is: ESC ] 52 ; c ; <base64-encoded-text> BEL
// This works in any terminal that supports the xterm clipboard extensions:
// iTerm2, WezTerm, Alacritty, kitty, Ghostty, and tmux (with set-clipboard on).
//
// We write to /dev/tty (the controlling terminal) rather than stdout to avoid
// contaminating the TUI screen buffer or pipes.
func writeOSC52(text string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	seq := fmt.Sprintf("\x1b]52;c;%s\x07", encoded)

	// Prefer writing to the actual TTY so the sequence goes directly to the
	// terminal regardless of stdout/stderr redirection.
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		// Fallback to stdout if /dev/tty is unavailable
		_, err = fmt.Fprint(os.Stdout, seq)
		return err
	}
	defer tty.Close()
	_, err = fmt.Fprint(tty, seq)
	return err
}

// writeNative copies text to the OS clipboard via a platform-specific command.
// The text is piped to stdin of the clipboard command so it never appears on
// the command line (avoids shell escaping issues and leaking secrets).
func writeNative(text string) error {
	cmd, args := nativeClipboardCmd()
	if cmd == "" {
		return fmt.Errorf("clipboard: no native clipboard command available on %s", runtime.GOOS)
	}

	c := exec.Command(cmd, args...)
	c.Stdin = strings.NewReader(text)
	var stderr bytes.Buffer
	c.Stderr = &stderr

	if err := c.Run(); err != nil {
		return fmt.Errorf("clipboard: %s failed: %w — %s", cmd, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// nativeClipboardCmd returns the clipboard command and arguments for the
// current platform. Returns ("", nil) if no command is available.
func nativeClipboardCmd() (string, []string) {
	switch runtime.GOOS {
	case "darwin":
		return "pbcopy", nil
	case "linux":
		// Prefer Wayland if the session is running under Wayland
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			if _, err := exec.LookPath("wl-copy"); err == nil {
				return "wl-copy", nil
			}
		}
		// Fall back to xclip
		if _, err := exec.LookPath("xclip"); err == nil {
			return "xclip", []string{"-selection", "clipboard"}
		}
		// Fall back to xsel
		if _, err := exec.LookPath("xsel"); err == nil {
			return "xsel", []string{"--clipboard", "--input"}
		}
	case "windows":
		// clip.exe is bundled with every Windows installation
		return "clip", nil
	}
	return "", nil
}
