//go:build unix

package theme

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/creack/pty"
	"github.com/stretchr/testify/require"
)

const confirmHelperEnv = "OH_TEST_CONFIRM_HELPER"

func TestMain(m *testing.M) {
	if os.Getenv(confirmHelperEnv) == "1" {
		ok := true // as oh run --recap
		if err := NewForm(huh.NewGroup(huh.NewConfirm().Title("Go?").Value(&ok))).Run(); err != nil {
			fmt.Println("ERR", err)
			os.Exit(1)
		}
		fmt.Println("RESULT", ok)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// Enter alone validates a confirmation (oh run --recap) under a
// pseudo-terminal (v5 finalisation, Q3-8). At start-up, Bubble Tea makes
// termenv query the terminal (background OSC 11, then the cursor position):
// a terminal answers at once; a bare pseudo-terminal does not, and termenv
// reads the keyboard for up to termenv.OSCTimeout (5 s), swallowing a key
// typed meanwhile. Scripts must answer the cursor query or wait for the
// question, as this test does.
func TestConfirmEnterUnderPTY(t *testing.T) {
	if testing.Short() {
		t.Skip("pseudo-terminal")
	}
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), confirmHelperEnv+"=1", "TERM=xterm-256color", "COLORFGBG=")
	f, err := pty.Start(cmd)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() })

	var out bytes.Buffer
	buf := make([]byte, 4096)
	answered, sent := false, false
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(out.String(), "RESULT") {
		_ = f.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, _ := f.Read(buf)
		out.Write(buf[:n])
		s := out.String()
		if !answered && strings.Contains(s, "\x1b[6n") {
			_, _ = f.Write([]byte("\x1b[1;1R")) // as a terminal: cursor position report
			answered = true
		}
		if !sent && strings.Contains(s, "Yes") {
			_, _ = f.Write([]byte("\r"))
			sent = true
		}
	}
	require.True(t, sent, "question never shown: %q", out.String())
	require.Contains(t, out.String(), "RESULT true", "Enter validates the default (yes)")
}
