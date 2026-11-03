//go:build !windows

package mcpregistry

import (
	"os"
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
