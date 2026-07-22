//go:build !windows

package opencode

import (
	"os"
	"syscall"
)

// execReplace replaces the current process with the given binary (Unix).
func execReplace(binary string, args []string, env []string) error {
	return syscall.Exec(binary, args, env)
}

// symlinkOrCopy creates a symlink from src to dst (Unix supports symlinks natively).
func symlinkOrCopy(src, dst string) error {
	return os.Symlink(src, dst)
}

// readSymlink returns the target of a symlink.
func readSymlink(path string) (string, error) {
	return os.Readlink(path)
}
