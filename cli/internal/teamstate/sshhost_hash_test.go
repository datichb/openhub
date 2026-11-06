package teamstate

import (
	"context"
	"encoding/base64"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// HashKnownHosts: the entry oh writes is the one OpenSSH writes. Nothing here
// talks to a network: a fake ssh -G, throwaway keys, ssh-keygen on temporary
// files when it is installed.

// Two entries as `ssh-keygen -H` of OpenSSH 10.3 wrote them for a plain
// file: the name of the port 22 and the [host]:port of another port.
const (
	hashedVectorHost = "|1|Kh/XsxGBxEu8lkenaCsq2nCObI0=|3PkS82q9pLQQYT8YM3WoOOoISCI="
	hashedVectorPort = "|1|I8j+jqiQrJoXUUSQnq53b2mX62E=|z2L5p7ORSb89XANVwFf/Gj0xsws="
)

func saltOf(t *testing.T, hashed string) []byte {
	t.Helper()
	parts := strings.Split(hashed, "|")
	if len(parts) != 4 || parts[1] != "1" {
		t.Fatalf("%q is not a hashed name", hashed)
	}
	salt, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	return salt
}

func TestHashedHostNameIsTheOneOfOpenSSH(t *testing.T) {
	for name, want := range map[string]string{
		"git.example.test":        hashedVectorHost,
		"[git.example.test]:2222": hashedVectorPort,
	} {
		salt := saltOf(t, want)
		if len(salt) != hostSaltLen {
			t.Errorf("the salt of OpenSSH has %d bytes, hostSaltLen is %d", len(salt), hostSaltLen)
		}
		if got := hashedHostName(salt, name); got != want {
			t.Errorf("hashedHostName(%q) = %q, want %q", name, got, want)
		}
		if !hashedMatch(want, name) {
			t.Errorf("hashedMatch must read what OpenSSH wrote for %q", name)
		}
	}
	if hashedMatch(hashedVectorHost, "[git.example.test]:2222") || hashedMatch(hashedVectorPort, "git.example.test") {
		t.Error("the entry of a port is not the entry of the other")
	}
	if hashedMatch(hashedVectorHost, "other.example.test") {
		t.Error("another name must not match")
	}
}

func TestNewHashedHostNameDrawsASaltEachTime(t *testing.T) {
	a, err := newHashedHostName("git.example.test")
	if err != nil {
		t.Fatal(err)
	}
	b, err := newHashedHostName("git.example.test")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Error("two entries of the same name share their salt")
	}
	for _, h := range []string{a, b} {
		if got := len(saltOf(t, h)); got != hostSaltLen {
			t.Errorf("salt of %d bytes, want %d", got, hostSaltLen)
		}
		if !hashedMatch(h, "git.example.test") || hashedMatch(h, "other.example.test") {
			t.Errorf("%q must match its own name only", h)
		}
	}
}

// What ssh-keygen -H writes for a plain file has the hash oh computes.
func TestHashedHostNameMatchesSSHKeygen(t *testing.T) {
	keygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		t.Skip("ssh-keygen not installed")
	}
	path := filepath.Join(t.TempDir(), "known_hosts")
	names := []string{"git.example.test", "[git.example.test]:2222", "2001:db8::1", "[2001:db8::1]:2222", "gitlab-internal"}
	var plain strings.Builder
	for _, n := range names {
		plain.WriteString(n + " ssh-ed25519 " + testKeyED25519 + "\n")
	}
	if err := os.WriteFile(path, []byte(plain.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(keygen, "-H", "-f", path).CombinedOutput(); err != nil {
		t.Skipf("ssh-keygen -H failed: %v %s", err, out)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != len(names) {
		t.Fatalf("ssh-keygen wrote %d lines, want %d: %q", len(lines), len(names), data)
	}
	for i, name := range names {
		hashed := strings.Fields(lines[i])[0]
		if got := hashedHostName(saltOf(t, hashed), name); got != hashed {
			t.Errorf("hashedHostName(%q) = %q, ssh-keygen wrote %q", name, got, hashed)
		}
	}
}

func hashedSSHFixture(t *testing.T, hashknownhosts string) (home string, tools *fakeSSHTools) {
	t.Helper()
	home, tools = sshSandbox(t)
	tools.gErr = nil
	tools.gOut = "hostname git.example.test\nport 22\nuserknownhostsfile ~/.ssh/known_hosts\nglobalknownhostsfile none\nhashknownhosts " + hashknownhosts + "\n"
	return home, tools
}

func TestTrustHashesTheNameWhenHashKnownHostsIsOn(t *testing.T) {
	_, tools := hashedSSHFixture(t, "yes")
	ctx := context.Background()
	h := UnknownSSHHost(ctx, "git@git.example.test:team/state.git")
	if h == nil || !h.Hashed {
		t.Fatalf("host = %+v, want unknown with HashKnownHosts on", h)
	}
	tools.scan = "git.example.test ssh-rsa " + testKeyRSA + "\ngit.example.test ssh-ed25519 " + testKeyED25519 + "\n"
	keys, err := h.Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Trust(keys); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(h.WriteTo)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "git.example.test") {
		t.Errorf("the name must not be in clear in %q", data)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("known_hosts = %q, want one line per key", data)
	}
	var hashes []string
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 3 || !strings.HasPrefix(fields[0], "|1|") || fields[1] != keys[i].Type || fields[2] != keys[i].blob {
			t.Errorf("line %d = %q, want |1|salt|hash type key", i, line)
		}
		if !hashedMatch(fields[0], "git.example.test") {
			t.Errorf("line %d does not hash git.example.test", i)
		}
		hashes = append(hashes, fields[0])
	}
	if hashes[0] == hashes[1] {
		t.Error("each line has a salt of its own")
	}
	if UnknownSSHHost(ctx, "git@git.example.test:team/state.git") != nil {
		t.Error("a hashed entry makes the server known")
	}
	if UnknownSSHHost(ctx, "git@GIT.example.test:team/state.git") != nil {
		t.Error("the case of the host does not matter")
	}
	assertSSHKeygenFinds(t, h.WriteTo, "git.example.test", true)
	assertSSHKeygenFinds(t, h.WriteTo, "other.example.test", false)
	assertXCryptoKnows(t, h.WriteTo, "git.example.test:22", testKeyED25519, true)
	assertXCryptoKnows(t, h.WriteTo, "other.example.test:22", testKeyED25519, false)
}

func TestTrustHashesTheNameOfAPort(t *testing.T) {
	_, tools := hashedSSHFixture(t, "yes")
	ctx := context.Background()
	h := UnknownSSHHost(ctx, "ssh://git@git.example.test:2222/team/state.git")
	if h == nil || !h.Hashed || h.Name != "[git.example.test]:2222" {
		t.Fatalf("host = %+v", h)
	}
	tools.scan = "[git.example.test]:2222 ssh-ed25519 " + testKeyED25519 + "\n"
	keys, err := h.Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Trust(keys); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(h.WriteTo)
	if strings.Contains(string(data), "git.example.test") || !strings.HasPrefix(string(data), "|1|") {
		t.Errorf("known_hosts = %q, want a hashed name", data)
	}
	if !hashedMatch(strings.Fields(string(data))[0], "[git.example.test]:2222") {
		t.Error("the hash is the one of [host]:port")
	}
	if UnknownSSHHost(ctx, "ssh://git@git.example.test:2222/team/state.git") != nil {
		t.Error("the hashed [host]:port makes the server known")
	}
	if UnknownSSHHost(ctx, "git@git.example.test:team/state.git") == nil {
		t.Error("the entry of port 2222 does not cover port 22")
	}
	assertSSHKeygenFinds(t, h.WriteTo, "[git.example.test]:2222", true)
	assertSSHKeygenFinds(t, h.WriteTo, "git.example.test", false)
	assertXCryptoKnows(t, h.WriteTo, "git.example.test:2222", testKeyED25519, true)
}

func TestTrustWritesInClearWhenHashKnownHostsIsOff(t *testing.T) {
	for _, setting := range []string{"no", "false"} {
		_, tools := hashedSSHFixture(t, setting)
		ctx := context.Background()
		h := UnknownSSHHost(ctx, "git@git.example.test:team/state.git")
		if h == nil || h.Hashed {
			t.Fatalf("hashknownhosts %s: host = %+v, want unknown and not hashed", setting, h)
		}
		tools.scan = "git.example.test ssh-ed25519 " + testKeyED25519 + "\n"
		keys, _ := h.Scan(ctx)
		if err := h.Trust(keys); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(h.WriteTo)
		if want := "git.example.test ssh-ed25519 " + testKeyED25519 + "\n"; string(data) != want {
			t.Errorf("hashknownhosts %s: known_hosts = %q, want %q", setting, data, want)
		}
	}
}

// Without an answer of ssh -G the setting is unknown: the name stays in clear
// (the behaviour before HashKnownHosts was read).
func TestTrustWritesInClearWhenSSHDoesNotAnswer(t *testing.T) {
	_, tools := sshSandbox(t)
	ctx := context.Background()
	h := UnknownSSHHost(ctx, "git@git.example.test:team/state.git")
	if h == nil || h.Hashed {
		t.Fatalf("host = %+v", h)
	}
	tools.scan = "git.example.test ssh-ed25519 " + testKeyED25519 + "\n"
	keys, _ := h.Scan(ctx)
	if err := h.Trust(keys); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(h.WriteTo)
	if !strings.HasPrefix(string(data), "git.example.test ssh-ed25519 ") {
		t.Errorf("known_hosts = %q", data)
	}
}

// A server already in known_hosts under a hashed name is known, whatever the
// form (host, [host]:port, case of the host), and is not added a second time.
func TestHashedEntriesAreFoundAndNotDuplicated(t *testing.T) {
	home, tools := hashedSSHFixture(t, "yes")
	ctx := context.Background()
	path := filepath.Join(home, ".ssh", "known_hosts")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	content := "# comment\n" + hashedVectorHost + " ssh-ed25519 " + testKeyED25519 + "\n" +
		hashedVectorPort + " ssh-ed25519 " + testKeyED25519 + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	for remote, known := range map[string]bool{
		"git@git.example.test:team/state.git":            true,
		"git@Git.Example.Test:team/state.git":            true,
		"ssh://git@git.example.test:2222/team/state.git": true,
		"ssh://git@git.example.test:2200/team/state.git": false,
	} {
		if got := UnknownSSHHost(ctx, remote) == nil; got != known {
			t.Errorf("%s: known = %v, want %v", remote, got, known)
		}
	}
	other := tools.gOut
	tools.gOut = strings.Replace(other, "git.example.test", "other.example.test", 1)
	if UnknownSSHHost(ctx, "git@other.example.test:team/state.git") == nil {
		t.Error("another server is not in the file")
	}
	tools.gOut = other

	// the server is known by its ed25519 key only: trusting the keys it presents
	// adds the rsa one, and the ed25519 one is not repeated
	h, _ := ResolveSSHHost(ctx, "git@git.example.test:team/state.git")
	tools.scan = "git.example.test ssh-ed25519 " + testKeyED25519 + "\ngit.example.test ssh-rsa " + testKeyRSA + "\n"
	keys, _ := h.Scan(ctx)
	if err := h.Trust(keys); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if got := strings.Count(string(data), "ssh-ed25519"); got != 2 {
		t.Errorf("the ed25519 key was added again: %d entries in %q", got, data)
	}
	if got := strings.Count(string(data), "ssh-rsa"); got != 1 {
		t.Errorf("the rsa key must be added once: %d entries in %q", got, data)
	}
	if !strings.HasPrefix(string(data), content) {
		t.Errorf("the entries that were there must stay as they were: %q", data)
	}
	before := string(data)
	if err := h.Trust(keys); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(path); string(after) != before {
		t.Errorf("trusting what is known changes nothing, got %q", after)
	}
}

func TestManualCommandsHashTheName(t *testing.T) {
	h := &SSHHost{Host: "git.example.test", Port: "2222", User: "git", WriteTo: "/k/known_hosts", Hashed: true}
	sshCmd, keyscanCmd := h.ManualCommands()
	if sshCmd != "ssh -p 2222 -T git@git.example.test" {
		t.Errorf("ssh command = %q", sshCmd)
	}
	if want := "ssh-keyscan -H -p 2222 git.example.test >> "; !strings.HasPrefix(keyscanCmd, want) {
		t.Errorf("keyscan command = %q, want it to start with %q", keyscanCmd, want)
	}
	h.Hashed = false
	if _, keyscanCmd = h.ManualCommands(); strings.Contains(keyscanCmd, "-H") {
		t.Errorf("keyscan command = %q, the name is in clear", keyscanCmd)
	}
}

func assertSSHKeygenFinds(t *testing.T, path, name string, want bool) {
	t.Helper()
	keygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		return
	}
	err = exec.Command(keygen, "-F", name, "-f", path).Run()
	if got := err == nil; got != want {
		t.Errorf("ssh-keygen -F %q: found = %v, want %v", name, got, want)
	}
}

// golang.org/x/crypto/ssh/knownhosts is another reader of the format: it must
// see the entry too.
func assertXCryptoKnows(t *testing.T, path, address, keyBlob string, want bool) {
	t.Helper()
	check, err := knownhosts.New(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(keyBlob)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.ParsePublicKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	err = check(address, &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 22}, key)
	if got := err == nil; got != want {
		t.Errorf("x/crypto knownhosts for %q: known = %v (%v), want %v", address, got, err, want)
	}
}
