//go:build windows

package checkpoint

import (
	"errors"
	"os/exec"
)

// setProcessGroup keeps the default (the process is killed on timeout).
func setProcessGroup(*exec.Cmd) {}

// runAs refuses another account (remote jobs run on Linux).
func runAs(_ *exec.Cmd, acct *Account) error {
	if acct == nil {
		return nil
	}
	return errors.New("verification: running as another account is not supported on Windows")
}
