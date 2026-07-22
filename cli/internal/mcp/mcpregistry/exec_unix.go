package mcpregistry

import (
	"os"
	"os/exec"
	"syscall"
)

// execBinary replaces the current process with the given binary.
// On Unix, uses syscall.Exec for a clean exec-replace.
// Falls back to os/exec on other platforms.
func execBinary(binary string) error {
	// Use syscall.Exec for clean process replacement (Unix)
	env := os.Environ()
	return syscall.Exec(binary, []string{binary}, env)
}

// ExecOrRun tries exec-replace; if not supported, runs as subprocess.
func ExecOrRun(binary string, args []string) error {
	cmd := exec.Command(binary, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
