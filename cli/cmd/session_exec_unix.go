//go:build !windows

package cmd

import "syscall"

func execReplaceProcess(bin string, argv, env []string) error {
	return syscall.Exec(bin, argv, env)
}
