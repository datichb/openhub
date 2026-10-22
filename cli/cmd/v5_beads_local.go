package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/fsutil"
	"github.com/datichb/openhub/cli/internal/gateway"
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/runsvc"
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
// It is rewritten when the oh executable moved. It tells the fake bd the
// daemon socket, where a git hook asks for its hook token (S-06).
func ensureBeadsShim(dir, exe string) error {
	sock := daemon.DefaultPaths().Socket()
	name, content := "bd", "#!/bin/sh\n# oh: Beads goes through the oh gateway (beads.allow of the workflow).\n"+
		beadswire.EnvDaemonSocket+"="+shellQuote(sock)+" exec "+shellQuote(exe)+" "+BeadsShimArg+" \"$@\"\n"
	if runtime.GOOS == "windows" {
		name, content = "bd.cmd", "@echo off\r\nset \""+beadswire.EnvDaemonSocket+"="+sock+"\"\r\n\""+exe+"\" "+BeadsShimArg+" %*\r\n"
	}
	p := filepath.Join(dir, name)
	if cur, err := os.ReadFile(p); err == nil && string(cur) == content {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(p, []byte(content), 0o755)
}

// localShellPath prepares the fake bd and returns the directories to put
// first on the PATH of local sessions.
func localShellPath() ([]string, error) {
	dir := ohShimsDir()
	if err := ensureBeadsShim(dir, sessionspec.OhExecutable()); err != nil {
		return nil, fmt.Errorf(i18n.T("cmd.v5_beads_local.preparing_the_bd_of_local_sessions"), err)
	}
	return []string{dir}, nil
}

// localShellStartup writes oh's shell start-up files of local sessions and
// returns their variables: the user's start-up files cannot put another bd
// before the fake bd (A16).
func localShellStartup(first []string) (map[string]string, error) {
	return gateway.ShellStartup(filepath.Join(config.HubDir(), "run", "shell"), first, os.Getenv)
}

// localBeadsShim is the fake bd local sessions must find first.
func localBeadsShim() string {
	name := "bd"
	if runtime.GOOS == "windows" {
		name = "bd.cmd"
	}
	return filepath.Join(ohShimsDir(), name)
}

// checkLocalShellBD tells what `bd` the shell of a local session runs
// (gateway.ErrShellBD when it is not the fake bd of oh).
func checkLocalShellBD(ctx context.Context) (string, error) {
	first, err := localShellPath()
	if err != nil {
		return "", err
	}
	startup, err := localShellStartup(first)
	if err != nil {
		return "", err
	}
	return gateway.CheckShellBD(ctx, runsvc.LocalSessionEnv(nil, startup, first), localBeadsShim())
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
