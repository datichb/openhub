package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) { os.Exit(Main(m)) }

// The configuration of the tests turns the automatic maintenance off and
// keeps the user's global configuration.
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
	home, _ := os.UserHomeDir()
	if home != "" && !strings.Contains(string(data), filepath.ToSlash(filepath.Join(home, ".gitconfig"))) {
		t.Errorf("the user's configuration is not included:\n%s", data)
	}
}
