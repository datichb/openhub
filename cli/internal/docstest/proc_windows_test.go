//go:build windows

package docstest

import "os/exec"

func setProcessGroup(*exec.Cmd) {}
