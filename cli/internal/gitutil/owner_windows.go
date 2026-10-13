//go:build windows

package gitutil

import (
	"errors"
	"os"
	"os/exec"
)

// statOwner: no unix owner (remote jobs run on Linux).
func statOwner(string) (uint32, bool) { return 0, false }

// runAsOwner refuses another account.
func runAsOwner(*exec.Cmd, Owner) error {
	return errors.New("git: running as another account is not supported on Windows")
}

func openNoBlock(name string) (*os.File, error) { return os.Open(name) }
