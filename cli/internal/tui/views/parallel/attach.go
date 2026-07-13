package parallel

import (
	"fmt"
	"os"
	"os/exec"

	oc "github.com/datichb/openhub/cli/internal/opencode"
)

// AttachToServer launches `opencode attach` to connect to a running server.
// This is a blocking call — returns when the user exits the opencode TUI.
func AttachToServer(port int) error {
	bin, err := oc.FindBinary()
	if err != nil {
		return fmt.Errorf("opencode binary not found: %w", err)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	cmd := exec.Command(bin, "attach", url)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Exit code 0 means user quit normally
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 0 {
			return nil
		}
		return fmt.Errorf("opencode attach failed: %w", err)
	}
	return nil
}
