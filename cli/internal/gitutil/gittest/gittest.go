// Package gittest helps the tests that run git.
package gittest

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Base is the git configuration the tests can count on, whatever the
// machine: a CI runner has no identity (git then refuses to commit: « empty
// ident name ») and no init.defaultBranch (a new repository starts on
// master, while the tests push and clone main). Signing is off.
const Base = "[user]\n\tname = oh tests\n\temail = tests@oh.invalid\n[init]\n\tdefaultBranch = main\n" +
	"[commit]\n\tgpgsign = false\n[tag]\n\tgpgsign = false\n"

// Config turns the automatic maintenance off. Since git 2.47, a commit, a
// fetch or a push (on both sides) starts `git maintenance run --auto
// --detach`, which outlives the command and creates objects/maintenance.lock
// in the repository: on a loaded machine, after the end of the test, while
// t.TempDir removes it (« unlinkat …/objects: directory not empty »).
const Config = "[maintenance]\n\tauto = false\n[receive]\n\tautogc = false\n[gc]\n\tauto = 0\n"

// Defaults is the whole global git configuration of the tests: Base then
// Config. It does not include the user's own: a test must not depend on
// the identity or the defaults of the machine it runs on.
func Defaults() string { return Base + Config }

// Main runs the tests of a package with Defaults() as the global git
// configuration (GIT_CONFIG_GLOBAL, whatever it was); it is the body of
// TestMain.
func Main(m *testing.M) int {
	dir, err := os.MkdirTemp("", "oh-gittest-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gittest:", err)
		return m.Run()
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(path, []byte(Defaults()), 0o600); err != nil {
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
