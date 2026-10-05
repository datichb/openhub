//go:build windows

package cmd

import (
	"os"
	"os/exec"
)

func execReplaceProcess(bin string, argv, env []string) error {
	c := exec.Command(bin, argv[1:]...)
	c.Stdin, c.Stdout, c.Stderr, c.Env = os.Stdin, os.Stdout, os.Stderr, env
	return c.Run()
}
