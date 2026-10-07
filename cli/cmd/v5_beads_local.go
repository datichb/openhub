package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// BeadsShimArg, as the first argument, makes oh act as the fake bd (main
// dispatches it before any initialization). Local sessions find it first
// on their PATH as `bd`, so that Beads goes through the gateway and the
// workflow beads.allow in every runtime, as in containers (QB1).
const BeadsShimArg = "__bd"

// ohShimsDir holds the `bd` of local sessions.
func ohShimsDir() string { return filepath.Join(config.HubDir(), "run", "bin") }

// ensureBeadsShim writes the `bd` of local sessions in dir, calling exe.
// It is rewritten when the oh executable moved.
func ensureBeadsShim(dir, exe string) error {
	name, content := "bd", "#!/bin/sh\n# oh: Beads goes through the oh gateway (beads.allow of the workflow).\nexec "+shellQuote(exe)+" "+BeadsShimArg+" \"$@\"\n"
	if runtime.GOOS == "windows" {
		name, content = "bd.cmd", "@echo off\r\n\""+exe+"\" "+BeadsShimArg+" %*\r\n"
	}
	p := filepath.Join(dir, name)
	if cur, err := os.ReadFile(p); err == nil && string(cur) == content {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".bd-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p)
}

// localShellPath prepares the fake bd and returns the directories to put
// first on the PATH of local sessions.
func localShellPath() ([]string, error) {
	dir := ohShimsDir()
	if err := ensureBeadsShim(dir, sessionspec.OhExecutable()); err != nil {
		return nil, fmt.Errorf("preparing the bd of local sessions: %w", err)
	}
	return []string{dir}, nil
}

// realBeadsBinary is the bd run by the gateway: the first one on the PATH
// that is not the fake bd of local sessions ("" = none).
func realBeadsBinary() string {
	shims, _ := filepath.Abs(ohShimsDir())
	for _, d := range filepath.SplitList(os.Getenv("PATH")) {
		if abs, err := filepath.Abs(d); err != nil || abs == shims {
			continue
		}
		for _, name := range []string{"bd", "bd.exe"} {
			p := filepath.Join(d, name)
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				if _, err := exec.LookPath(p); err == nil {
					return p
				}
			}
		}
	}
	return ""
}
