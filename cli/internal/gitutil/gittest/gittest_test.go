package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) { os.Exit(Main(m)) }

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// The configuration of the tests turns the automatic maintenance off and
// stands on its own: it does not include the one of the user.
func TestConfig(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	get := func(key string) string {
		out, _ := exec.Command("git", "config", "--global", "--get", key).Output()
		return strings.TrimSpace(string(out))
	}
	if got := get("maintenance.auto"); got != "false" {
		t.Errorf("maintenance.auto = %q", got)
	}
	if got := get("receive.autogc"); got != "false" {
		t.Errorf("receive.autogc = %q", got)
	}
	data, err := os.ReadFile(os.Getenv("GIT_CONFIG_GLOBAL"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "[include]") {
		t.Errorf("the configuration of the user is included:\n%s", data)
	}
}

// What the tests count on is there even on a machine with no git
// configuration at all (a CI runner): a commit works with no identity in the
// environment, and a new repository starts on main.
func TestBase(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "")
		if err := os.Unsetenv(k); err != nil {
			t.Fatal(err)
		}
	}
	repo := filepath.Join(t.TempDir(), "repo")
	gitOut(t, t.TempDir(), "init", "-q", repo)
	if got := gitOut(t, repo, "symbolic-ref", "--short", "HEAD"); got != "main" {
		t.Errorf("new repository on %q, want main", got)
	}
	gitOut(t, repo, "commit", "-q", "--allow-empty", "-m", "x")
	if got := gitOut(t, repo, "log", "-1", "--format=%an <%ae>"); got != "oh tests <tests@oh.invalid>" {
		t.Errorf("author = %q", got)
	}
	bare := filepath.Join(t.TempDir(), "bare")
	gitOut(t, t.TempDir(), "init", "-q", "--bare", bare)
	if got := gitOut(t, bare, "symbolic-ref", "--short", "HEAD"); got != "main" {
		t.Errorf("new bare repository on %q, want main", got)
	}
}
