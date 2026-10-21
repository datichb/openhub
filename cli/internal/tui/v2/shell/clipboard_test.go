package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNativeClipboardCmd(t *testing.T) {
	cmd, args := nativeClipboardCmd()

	switch runtime.GOOS {
	case "darwin":
		assert.Equal(t, "pbcopy", cmd, "macOS command")
		assert.Empty(t, args, "macOS arguments")
	case "windows":
		assert.Equal(t, "clip", cmd, "Windows command")
	case "linux":
		// the command depends on the clipboard tools installed
	default:
		assert.Empty(t, cmd, "unsupported platform: no command")
	}
}

// fakeClipboardCommand puts a fake native clipboard command first (and only)
// in PATH: a shell script running body, where %s is the file of the text
// received. It returns that file.
func fakeClipboardCommand(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake clipboard command is a shell script")
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "clipboard.txt")
	script := "#!/bin/sh\n" + fmt.Sprintf(body, "'"+out+"'") + "\n"
	for _, name := range []string{"pbcopy", "xclip", "xsel"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755), "writing the fake %s", name)
	}
	t.Setenv("PATH", dir)
	t.Setenv("WAYLAND_DISPLAY", "")
	return out
}

// The text goes to the standard input of the native command, whatever it
// holds. The command is a script of a temporary PATH: the clipboard of the
// user is never touched.
func TestWriteNative(t *testing.T) {
	out := fakeClipboardCommand(t, "/bin/cat > %s")

	for _, text := range []string{
		"",
		"simple text",
		"multi\nline\ntext",
		strings.Repeat("x", 10_000),
		"unicode: 日本語 émoji 🎉",
		"\x00\x01\x02binary",
	} {
		require.NoError(t, writeNative(text), "writeNative(%.20q)", text)
		got, err := os.ReadFile(out)
		require.NoError(t, err, "reading what the command received")
		assert.Equal(t, text, string(got), "text received by the command")
	}

	t.Run("no command available", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("only Linux depends on the tools found in PATH")
		}
		t.Setenv("PATH", t.TempDir())
		assert.Error(t, writeNative("x"), "no clipboard tool in PATH")
	})
}

// CopyToClipboard also writes an OSC 52 sequence to the terminal: the test
// is skipped when a terminal is attached, to keep the clipboard of the user
// untouched.
func TestCopyToClipboard(t *testing.T) {
	if tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0); err == nil {
		tty.Close()
		t.Skip("a terminal is attached: the OSC 52 sequence would reach the real clipboard")
	}

	t.Run("native command succeeds", func(t *testing.T) {
		out := fakeClipboardCommand(t, "/bin/cat > %s")
		require.NoError(t, CopyToClipboard("hello"), "copy")
		got, err := os.ReadFile(out)
		require.NoError(t, err, "reading what the command received")
		assert.Equal(t, "hello", string(got), "text received")
	})

	t.Run("one strategy is enough", func(t *testing.T) {
		fakeClipboardCommand(t, "exit 1")
		assert.NoError(t, CopyToClipboard("hello"), "the OSC 52 write succeeds when the native command fails")
	})
}
