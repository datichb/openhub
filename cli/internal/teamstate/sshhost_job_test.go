package teamstate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// What runs without anybody to ask (the daemon, oh doctor) reports a server
// only when ssh itself would take it for a first contact: it answered `ssh -G`,
// the server has no entry, and no certificate authority covers it.
func TestUnknownSSHHostForJob(t *testing.T) {
	home, tools := sshSandbox(t)
	ctx := context.Background()
	const remote = "git@git.example.test:team/state.git"
	hosts := filepath.Join(home, ".ssh", "known_hosts")

	// ssh cannot say how it resolves the server: names and files are guesses.
	if UnknownSSHHostForJob(ctx, remote) != nil {
		t.Error("ssh -G failed: the answer is not certain, nothing is reported")
	}
	if UnknownSSHHost(ctx, remote) == nil {
		t.Fatal("the assistants still treat it as unknown")
	}

	tools.gErr = nil
	tools.gOut = "hostname git.example.test\nport 22\n"
	h := UnknownSSHHostForJob(ctx, remote)
	if h == nil || !h.Resolved || h.Name != "git.example.test" {
		t.Fatalf("host = %+v, want the unknown server", h)
	}
	if UnknownSSHHostForJob(ctx, "https://git.example.test/team/state.git") != nil || UnknownSSHHostForJob(ctx, "/tmp/state") != nil {
		t.Error("only an SSH remote has a server")
	}

	// A certificate authority for the domain vouches for it.
	if err := os.MkdirAll(filepath.Dir(hosts), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hosts, []byte("@cert-authority *.example.test ssh-ed25519 "+testKeyED25519+"\n@cert-authority *.other.test ssh-ed25519 "+testKeyED25519+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if UnknownSSHHostForJob(ctx, remote) != nil {
		t.Error("a certificate authority covers the server: ssh may accept it, nothing is reported")
	}
	if UnknownSSHHost(ctx, remote) == nil {
		t.Error("the assistants keep asking (the CA lines do not count as a key of the server)")
	}

	// A CA of another domain does not.
	if err := os.WriteFile(hosts, []byte("@cert-authority *.other.test ssh-ed25519 "+testKeyED25519+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if UnknownSSHHostForJob(ctx, remote) == nil {
		t.Error("the CA of another domain says nothing of this server")
	}

	// Known once it has an entry.
	if err := os.WriteFile(hosts, []byte("git.example.test ssh-ed25519 "+testKeyED25519+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if UnknownSSHHostForJob(ctx, remote) != nil {
		t.Error("a known server is not reported")
	}
}
