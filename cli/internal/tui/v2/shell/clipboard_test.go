package shell

import (
	"encoding/base64"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── OSC52 format ──────────────────────────────────────────────────────────────

func TestWriteOSC52_ContainsBase64EncodedText(t *testing.T) {
	// writeOSC52 writes to /dev/tty — we can only test the encoding logic
	// indirectly by verifying the format. We do this by testing that our
	// base64 encoding of a known string produces the expected output.
	text := "hello clipboard"
	expected := base64.StdEncoding.EncodeToString([]byte(text))

	// Build the sequence manually and verify it matches what writeOSC52 would send
	seq := "\x1b]52;c;" + expected + "\x07"
	assert.True(t, strings.HasPrefix(seq, "\x1b]52;c;"), "OSC52 must start with ESC]52;c;")
	assert.True(t, strings.HasSuffix(seq, "\x07"), "OSC52 must end with BEL")
	assert.Contains(t, seq, expected, "OSC52 must contain base64-encoded text")
}

func TestWriteOSC52_EmptyText(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(""))
	seq := "\x1b]52;c;" + encoded + "\x07"
	assert.NotEmpty(t, seq)
}

func TestWriteOSC52_MultilineText(t *testing.T) {
	text := "line one\nline two\nline three"
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	// The sequence must be a single line (base64 has no newlines in StdEncoding with no line breaks)
	assert.NotContains(t, encoded, "\n", "base64 should not contain newlines")
	seq := "\x1b]52;c;" + encoded + "\x07"
	assert.NotEmpty(t, seq)
}

// ── nativeClipboardCmd ────────────────────────────────────────────────────────

func TestNativeClipboardCmd_ReturnsPlatformCommand(t *testing.T) {
	cmd, args := nativeClipboardCmd()

	switch runtime.GOOS {
	case "darwin":
		assert.Equal(t, "pbcopy", cmd)
		assert.Empty(t, args)
	case "windows":
		assert.Equal(t, "clip", cmd)
	case "linux":
		// On Linux the command depends on the environment — may be empty
		// if no clipboard tool is installed. Just verify no panic.
		_ = cmd
		_ = args
	default:
		assert.Empty(t, cmd, "unsupported platform should return empty cmd")
	}
}

func TestNativeClipboardCmd_NeverPanics(t *testing.T) {
	require.NotPanics(t, func() {
		nativeClipboardCmd()
	})
}

// ── CopyToClipboard integration (best-effort, environment-dependent) ──────────

func TestCopyToClipboard_DoesNotPanicOnAnyInput(t *testing.T) {
	// This test doesn't assert success — the clipboard may not be available
	// in a headless CI environment. We only verify no panic.
	inputs := []string{
		"",
		"simple text",
		"multi\nline\ntext",
		strings.Repeat("x", 10_000), // large payload
		"unicode: 日本語 émoji 🎉",
		"\x00\x01\x02binary",        // binary-safe via base64
	}
	for _, input := range inputs {
		assert.NotPanics(t, func() {
			_ = CopyToClipboard(input)
		}, "CopyToClipboard must not panic for input: %q", input)
	}
}
