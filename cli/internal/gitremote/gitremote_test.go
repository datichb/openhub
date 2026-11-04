package gitremote

import (
	"os"
	"path/filepath"
	"testing"
)

// git writes the URL of a local path with its backslashes escaped (a Windows
// path is C:\\Users\\…): Origin reads the value git means, not its escaping.
func TestConfigValueDecodesWhatGitWrites(t *testing.T) {
	for raw, want := range map[string]string{
		` https://gitlab.example.com/acme/state.git`: "https://gitlab.example.com/acme/state.git",
		` C:\\Users\\me\\state.git`:                  `C:\Users\me\state.git`,
		` "/path with space/state.git"`:              "/path with space/state.git",
		` /srv/state.git # the team`:                 "/srv/state.git",
		` "/srv/a#b.git"`:                            "/srv/a#b.git",
	} {
		if got := ConfigValue(raw); got != want {
			t.Errorf("ConfigValue(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestSplit(t *testing.T) {
	for remote, want := range map[string][2]string{
		"git@gitlab.com:acme/team-state.git":         {"gitlab.com", "acme/team-state"},
		"https://GitLab.com/acme/sub/team-state.git": {"gitlab.com", "acme/sub/team-state"},
		"ssh://git@host:2222/acme/ts/":               {"host", "acme/ts"},
		"git@host:repo.git":                          {"host", "repo"},
	} {
		host, path, ok := Split(remote)
		if !ok || host != want[0] || path != want[1] {
			t.Errorf("Split(%q) = %q, %q, %v, want %v", remote, host, path, ok, want)
		}
	}
	for _, local := range []string{"/srv/git/team.git", `C:\teams\acme.git`, "relative/dir", ""} {
		if _, _, ok := Split(local); ok {
			t.Errorf("Split(%q): a local path has neither host nor path", local)
		}
	}
}

func TestSame(t *testing.T) {
	if !Same("git@gitlab.com:acme/ts.git", "https://gitlab.com/acme/ts") {
		t.Error("the scheme and the .git suffix do not make another repository")
	}
	if Same("git@gitlab.com:acme/ts.git", "git@gitlab.com:beta/ts.git") {
		t.Error("two groups are two repositories")
	}
	if Same("", "") {
		t.Error("nothing is not a repository")
	}
}

func TestIsCloneOf(t *testing.T) {
	dir := t.TempDir()
	if IsCloneOf(dir, "git@gitlab.com:acme/ts.git") {
		t.Fatal("a folder without .git is no clone")
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if IsCloneOf(dir, "git@gitlab.com:acme/ts.git") {
		t.Fatal("a clone without origin is the clone of nothing")
	}
	cfg := "[remote \"origin\"]\n\turl = https://gitlab.com/acme/ts.git\n"
	if err := os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsCloneOf(dir, "git@gitlab.com:acme/ts.git") {
		t.Error("the same repository over another scheme is the same clone")
	}
	if IsCloneOf(dir, "git@gitlab.com:beta/ts.git") {
		t.Error("another group is another repository")
	}
}
