//go:build windows

package checkpoint

import "os/exec"

// setProcessGroup keeps the default (the process is killed on timeout).
func setProcessGroup(*exec.Cmd) {}
