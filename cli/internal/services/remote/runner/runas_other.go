//go:build !unix

package runner

import "os/exec"

// runAs is not supported: remote jobs run on Linux runners.
func runAs(*exec.Cmd, *User) {}
