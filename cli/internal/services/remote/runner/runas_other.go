//go:build !unix

package runner

import (
	"os"
	"os/exec"
)

// runAs is not supported: remote jobs run on Linux runners.
func runAs(*exec.Cmd, *User) {}

func openNoFollow(p string) (*os.File, error) { return os.Open(p) }

func isSymlinkErr(error) bool { return false }

func ownedBy(_ os.FileInfo, u *User) bool { return u == nil }
