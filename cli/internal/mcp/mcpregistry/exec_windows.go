//go:build windows

package mcpregistry

import (
	"os"
	"os/exec"
)

// execBinary starts the binary as a subprocess and waits for it to complete.
// Windows does not support syscall.Exec, so we use os/exec and propagate stdio.
func execBinary(binary string) error {
	cmd := exec.Command(binary)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ExecOrRun runs the binary as a subprocess (Windows — no exec-replace available).
func ExecOrRun(binary string, args []string) error {
	cmd := exec.Command(binary, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
