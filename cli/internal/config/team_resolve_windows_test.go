//go:build windows

package config

import "testing"

func TestRepoNameFromRemoteOfAWindowsPath(t *testing.T) {
	for remote, want := range map[string]string{
		`C:\Users\RUNNER~1\AppData\Local\Temp\x\002`: "002",
		`D:\teams\acme.git`:                          "acme",
	} {
		if got := RepoNameFromRemote(remote); got != want {
			t.Errorf("RepoNameFromRemote(%q) = %q, want %q", remote, got, want)
		}
	}
}
