// Package gittest helps the tests that run git.
package gittest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Config is the git configuration of the tests, after the user's: no
// automatic maintenance. Since git 2.47, a commit, a fetch or a push (on
// both sides) starts `git maintenance run --auto --detach`, which outlives
// the command and creates objects/maintenance.lock in the repository: on a
// loaded machine, after the end of the test, while t.TempDir removes it
// (« unlinkat …/objects: directory not empty »).
const Config = "[maintenance]\n\tauto = false\n[receive]\n\tautogc = false\n[gc]\n\tauto = 0\n"

// Main runs the tests of a package with Config as the global git
// configuration (GIT_CONFIG_GLOBAL), which includes the user's own (so
// identity and defaults stay); it is the body of TestMain.
func Main(m *testing.M) int {
	dir, err := os.MkdirTemp("", "oh-gittest-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gittest:", err)
		return m.Run()
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(path, []byte(globalConfig()), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "gittest:", err)
		return m.Run()
	}
	prev, had := os.LookupEnv("GIT_CONFIG_GLOBAL")
	_ = os.Setenv("GIT_CONFIG_GLOBAL", path)
	defer func() {
		if had {
			_ = os.Setenv("GIT_CONFIG_GLOBAL", prev)
		} else {
			_ = os.Unsetenv("GIT_CONFIG_GLOBAL")
		}
	}()
	return m.Run()
}

// globalConfig includes the global files git would read, then Config
// (the last value wins). A missing included file is ignored by git.
func globalConfig() string {
	var b strings.Builder
	include := func(p string) {
		if p != "" {
			fmt.Fprintf(&b, "[include]\n\tpath = %s\n", strings.ReplaceAll(p, `\`, `/`))
		}
	}
	if p := os.Getenv("GIT_CONFIG_GLOBAL"); p != "" {
		include(p)
	} else {
		xdg := os.Getenv("XDG_CONFIG_HOME")
		if home, err := os.UserHomeDir(); err == nil {
			if xdg == "" {
				xdg = filepath.Join(home, ".config")
			}
			include(filepath.Join(xdg, "git", "config"))
			include(filepath.Join(home, ".gitconfig"))
		}
	}
	return b.String() + Config
}
