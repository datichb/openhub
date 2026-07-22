//go:build windows

package opencode

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// execReplace starts the binary as a subprocess and waits for it to complete (Windows).
// Windows does not support syscall.Exec, so we use os/exec and propagate the exit code.
func execReplace(binary string, args []string, env []string) error {
	cmd := exec.Command(binary, args[1:]...) // args[0] is the binary name
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	return cmd.Run()
}

// symlinkOrCopy falls back to file copy on Windows (symlinks require admin privileges).
func symlinkOrCopy(src, dst string) error {
	// Try symlink first (works if developer mode or admin)
	if err := os.Symlink(src, dst); err == nil {
		return nil
	}
	// Fall back to copy
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating destination: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copying: %w", err)
	}
	return nil
}

// readSymlink returns the target of a symlink (or the path itself on Windows if not a symlink).
func readSymlink(path string) (string, error) {
	target, err := os.Readlink(path)
	if err != nil {
		// Not a symlink — on Windows this can happen if we used copy fallback
		return path, nil
	}
	return target, nil
}
