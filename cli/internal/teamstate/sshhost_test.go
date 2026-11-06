package teamstate

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // builds a hashed known_hosts entry the way OpenSSH does
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// The first contact with an SSH server: nothing here runs a real ssh or
// ssh-keyscan against a network (a fake runner, or fake binaries on a PATH of
// their own), and HOME is a temporary directory: the real ~/.ssh is never read
// or written.

const (
	// a real ed25519 and a real rsa public key (throwaway, generated for the tests)
	testKeyED25519 = "AAAAC3NzaC1lZDI1NTE5AAAAIMe2oLQFSPbU1ZXsHBUxEcjB6QExxb9fTzDW8/Z/Plf7"
	testKeyRSA     = "AAAAB3NzaC1yc2EAAAADAQABAAABAQC5wZI1LWkx2IA0DsIA33GDh6UHjfF4IQ6F9nGi/8PtXyGxLBqLWDXyjv0VzQpJSMvqPbxby4UaU/HA6uxKHBRsvcNIAMDPxp4ubfi/0gMmYJcxIc/fm7McUaLY7Yi+z95bXs9hl/LwRSjLvQBvrSLpsn5S61+fWh4PwITJhRKVQoTa7t2OtEFrJZZCt8KYFAdCe6o7Hexm3zcG2XENo6Q1JddY/hC4DorrymTmkceoodUTf2RZAHUxdIyeeC0bNJABkH3gxnLepTVdUANff2/dgPcCc2rPRN1Kl7fUXkt4zk7zOKy3TGqDUVuvFZc7Al9f8fHbvx59wvD2jrq0VtDd"
)

type toolCall struct {
	name string
	args []string
}

// fakeSSHTools replaces the runner of ssh and ssh-keyscan: gOut is what `ssh -G`
// prints (gErr: it fails), scan what ssh-keyscan prints.
type fakeSSHTools struct {
	mu    sync.Mutex
	calls []toolCall
	gOut  string
	gErr  error
	scan  string
	scErr error
}

func (f *fakeSSHTools) run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, toolCall{name, args})
	switch name {
	case "ssh":
		return []byte(f.gOut), f.gErr
	case "ssh-keyscan":
		return []byte(f.scan), f.scErr
	}
	return nil, fmt.Errorf("unexpected tool %s", name)
}

func (f *fakeSSHTools) called(name string) []toolCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []toolCall
	for _, c := range f.calls {
		if c.name == name {
			out = append(out, c)
		}
	}
	return out
}

// sshSandbox gives the test a HOME of its own and the fake tools.
func sshSandbox(t *testing.T) (home string, tools *fakeSSHTools) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	tools = &fakeSSHTools{gErr: errors.New("no ssh")}
	t.Cleanup(SetSSHToolRunner(tools.run))
	return home, tools
}

func TestParseSSHRemote(t *testing.T) {
	tests := []struct {
		remote, user, host, port string
		ok                       bool
	}{
		{"git@gitlab.example.test:team/state.git", "git", "gitlab.example.test", "", true},
		{"gitlab.example.test:team/state.git", "", "gitlab.example.test", "", true},
		{"ssh://git@gitlab.example.test/team/state.git", "git", "gitlab.example.test", "", true},
		{"ssh://git@gitlab.example.test:2222/team/state.git", "git", "gitlab.example.test", "2222", true},
		{"git+ssh://git@h.example.test:2200/x.git", "git", "h.example.test", "2200", true},
		{"ssh://git@[::1]:2222/x.git", "git", "::1", "2222", true},
		{"git@[2001:db8::1]:team/x.git", "git", "2001:db8::1", "", true},
		{"https://gitlab.example.test/team/state.git", "", "", "", false},
		{"http://gitlab.example.test/x.git", "", "", "", false},
		{"file:///srv/git/x.git", "", "", "", false},
		{"/srv/git/x.git", "", "", "", false},
		{"./relative/x.git", "", "", "", false},
		{"../a:b/x.git", "", "", "", false},
		{"", "", "", "", false},
		{"ssh://-oProxyCommand=evil/x.git", "", "", "", false},
		{"-oProxyCommand=evil:x.git", "", "", "", false},
	}
	for _, tt := range tests {
		user, host, port, ok := parseSSHRemote(tt.remote)
		if ok != tt.ok || user != tt.user || host != tt.host || port != tt.port {
			t.Errorf("parseSSHRemote(%q) = %q %q %q %v, want %q %q %q %v",
				tt.remote, user, host, port, ok, tt.user, tt.host, tt.port, tt.ok)
		}
	}
}

func TestUnknownSSHHostIgnoresRemotesThatAreNotSSH(t *testing.T) {
	sshSandbox(t)
	for _, remote := range []string{"https://gitlab.example.test/x.git", "/tmp/bare.git", ""} {
		if h := UnknownSSHHost(context.Background(), remote); h != nil {
			t.Errorf("UnknownSSHHost(%q) = %+v, want nil", remote, h)
		}
	}
}

func TestUnknownSSHHostThenTrusted(t *testing.T) {
	home, tools := sshSandbox(t)
	ctx := context.Background()
	remote := "git@git.example.test:team/state.git"

	h := UnknownSSHHost(ctx, remote)
	if h == nil {
		t.Fatal("a host absent from known_hosts must be unknown")
	}
	if h.Name != "git.example.test" || h.Display() != "git.example.test" {
		t.Errorf("name = %q, display = %q", h.Name, h.Display())
	}
	if want := filepath.Join(home, ".ssh", "known_hosts"); h.WriteTo != want {
		t.Errorf("WriteTo = %q, want %q", h.WriteTo, want)
	}

	tools.scan = "git.example.test ssh-rsa " + testKeyRSA + "\n# comment\ngit.example.test ssh-ed25519 " + testKeyED25519 + "\n"
	keys, err := h.Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0].Type != "ssh-ed25519" || keys[1].Type != "ssh-rsa" {
		t.Fatalf("keys = %+v, want ed25519 then rsa", keys)
	}
	if !strings.HasPrefix(keys[0].Fingerprint, "SHA256:") || strings.Contains(keys[0].Fingerprint, "=") {
		t.Errorf("fingerprint = %q, want SHA256:<base64 without padding>", keys[0].Fingerprint)
	}

	if err := h.Trust(keys); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(h.WriteTo)
	if err != nil {
		t.Fatal(err)
	}
	if want := "git.example.test ssh-ed25519 " + testKeyED25519 + "\ngit.example.test ssh-rsa " + testKeyRSA + "\n"; string(data) != want {
		t.Errorf("known_hosts = %q, want %q", data, want)
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(h.WriteTo); fi.Mode().Perm() != 0o600 {
			t.Errorf("known_hosts mode = %v, want 0600", fi.Mode().Perm())
		}
		if fi, _ := os.Stat(filepath.Dir(h.WriteTo)); fi.Mode().Perm() != 0o700 {
			t.Errorf(".ssh mode = %v, want 0700", fi.Mode().Perm())
		}
	}
	if UnknownSSHHost(ctx, remote) != nil {
		t.Error("a trusted host is known: nothing more to ask")
	}
	if got := tools.called("ssh-keyscan"); len(got) != 1 {
		t.Errorf("ssh-keyscan called %d times, want 1 (the known test is offline)", len(got))
	}
}

func TestSSHHostScanAsksTheRightPort(t *testing.T) {
	_, tools := sshSandbox(t)
	ctx := context.Background()
	h := UnknownSSHHost(ctx, "ssh://git@git.example.test:2222/team/state.git")
	if h == nil {
		t.Fatal("unknown host expected")
	}
	if h.Name != "[git.example.test]:2222" || h.Display() != "git.example.test:2222" {
		t.Errorf("name = %q, display = %q", h.Name, h.Display())
	}
	tools.scan = "[git.example.test]:2222 ssh-ed25519 " + testKeyED25519 + "\n"
	keys, err := h.Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	scan := tools.called("ssh-keyscan")
	if len(scan) != 1 || !strings.Contains(strings.Join(scan[0].args, " "), "-p 2222") || scan[0].args[len(scan[0].args)-1] != "git.example.test" {
		t.Errorf("ssh-keyscan args = %v, want -p 2222 and the host last", scan)
	}
	if err := h.Trust(keys); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(h.WriteTo)
	if !strings.HasPrefix(string(data), "[git.example.test]:2222 ssh-ed25519 ") {
		t.Errorf("known_hosts = %q, want the [host]:port name", data)
	}
	// the default port of the same host is another entry
	if UnknownSSHHost(ctx, "git@git.example.test:team/state.git") == nil {
		t.Error("the entry of port 2222 does not cover port 22")
	}
	if UnknownSSHHost(ctx, "ssh://git@git.example.test:2222/team/state.git") != nil {
		t.Error("the entry of port 2222 covers port 2222")
	}
	sshCmd, keyscanCmd := h.ManualCommands()
	if sshCmd != "ssh -p 2222 -T git@git.example.test" || !strings.HasPrefix(keyscanCmd, "ssh-keyscan -p 2222 git.example.test >> ") {
		t.Errorf("manual commands = %q / %q", sshCmd, keyscanCmd)
	}
}

func TestSSHHostIPv6(t *testing.T) {
	sshSandbox(t)
	ctx := context.Background()
	h := UnknownSSHHost(ctx, "ssh://git@[2001:db8::1]:2222/x.git")
	if h == nil || h.Name != "[2001:db8::1]:2222" || h.Display() != "[2001:db8::1]:2222" {
		t.Fatalf("host = %+v", h)
	}
	h = UnknownSSHHost(ctx, "git@[2001:db8::1]:x.git")
	if h == nil || h.Name != "2001:db8::1" {
		t.Fatalf("host = %+v, want the bare address on port 22", h)
	}
}

func TestKnownHostsPatterns(t *testing.T) {
	home, _ := sshSandbox(t)
	ctx := context.Background()
	salt := []byte("0123456789abcdefghij")
	mac := hmac.New(sha1.New, salt)
	mac.Write([]byte("hashed.example.test"))
	hashed := "|1|" + base64.StdEncoding.EncodeToString(salt) + "|" + base64.StdEncoding.EncodeToString(mac.Sum(nil))

	ssh := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(ssh, 0o700); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"# plain.example.test ssh-ed25519 " + testKeyED25519,
		"alone.example.test ssh-ed25519 " + testKeyED25519,
		"multi1.example.test,multi2.example.test ssh-ed25519 " + testKeyED25519,
		"*.wild.example.test,!bad.wild.example.test ssh-ed25519 " + testKeyED25519,
		hashed + " ssh-ed25519 " + testKeyED25519,
		"@revoked revoked.example.test ssh-ed25519 " + testKeyED25519,
		"@cert-authority ca.example.test ssh-ed25519 " + testKeyED25519,
		"[port.example.test]:2200 ssh-ed25519 " + testKeyED25519,
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(ssh, "known_hosts"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		remote string
		known  bool
	}{
		{"git@plain.example.test:x.git", false}, // commented out
		{"git@alone.example.test:x.git", true},
		{"git@ALONE.example.test:x.git", true},
		{"git@multi2.example.test:x.git", true},
		{"git@a.wild.example.test:x.git", true},
		{"git@bad.wild.example.test:x.git", false},
		{"git@hashed.example.test:x.git", true},
		{"git@other-hashed.example.test:x.git", false},
		{"git@revoked.example.test:x.git", false},
		{"git@ca.example.test:x.git", false},
		{"ssh://git@port.example.test:2200/x.git", true},
		{"git@port.example.test:x.git", false},
	}
	for _, tt := range tests {
		if got := UnknownSSHHost(ctx, tt.remote) == nil; got != tt.known {
			t.Errorf("%s: known = %v, want %v", tt.remote, got, tt.known)
		}
	}
}

func TestSSHHostFollowsTheSSHConfig(t *testing.T) {
	home, tools := sshSandbox(t)
	ctx := context.Background()
	custom := filepath.Join(home, "elsewhere", "hosts")
	tools.gErr = nil
	tools.gOut = strings.Join([]string{
		"user git",
		"hostname real.example.test",
		"port 2222",
		"hostkeyalias",
		"userknownhostsfile " + custom + " ~/.ssh/known_hosts",
		"globalknownhostsfile none",
		"proxyjump none",
	}, "\n")
	h := UnknownSSHHost(ctx, "git@alias:team/state.git")
	if h == nil {
		t.Fatal("unknown host expected")
	}
	if h.Host != "real.example.test" || h.Port != "2222" || h.Name != "[real.example.test]:2222" {
		t.Errorf("host = %+v: the alias must be resolved", h)
	}
	if h.WriteTo != custom {
		t.Errorf("WriteTo = %q, want the first UserKnownHostsFile %q", h.WriteTo, custom)
	}
	ssh := tools.called("ssh")
	if len(ssh) != 1 || ssh[0].args[0] != "-G" || ssh[0].args[len(ssh[0].args)-1] != "alias" {
		t.Errorf("ssh -G call = %v", ssh)
	}

	tools.scan = "[real.example.test]:2222 ssh-ed25519 " + testKeyED25519 + "\n"
	keys, err := h.Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if scan := tools.called("ssh-keyscan"); !strings.Contains(strings.Join(scan[0].args, " "), "-p 2222") || scan[0].args[len(scan[0].args)-1] != "real.example.test" {
		t.Errorf("the real host and port are scanned, got %v", scan)
	}
	if err := h.Trust(keys); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(custom); err != nil {
		t.Errorf("the configured known_hosts must be written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".ssh", "known_hosts")); !os.IsNotExist(err) {
		t.Error("the default known_hosts must stay untouched when the config names another")
	}
	if UnknownSSHHost(ctx, "git@alias:team/state.git") != nil {
		t.Error("a host trusted in the configured file is known")
	}
}

func TestSSHHostKeyAlias(t *testing.T) {
	_, tools := sshSandbox(t)
	tools.gErr = nil
	tools.gOut = "hostname 10.0.0.5\nport 22\nhostkeyalias gitlab-internal\nuserknownhostsfile ~/.ssh/known_hosts\n"
	h := UnknownSSHHost(context.Background(), "git@internal:x.git")
	if h == nil || h.Name != "gitlab-internal" || h.Host != "10.0.0.5" {
		t.Fatalf("host = %+v: ssh looks the alias up, and connects to the address", h)
	}
	tools.scan = "10.0.0.5 ssh-ed25519 " + testKeyED25519 + "\n"
	keys, err := h.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Trust(keys); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(h.WriteTo)
	if !strings.HasPrefix(string(data), "gitlab-internal ssh-ed25519 ") {
		t.Errorf("known_hosts = %q, want the entry under the alias", data)
	}
}

// ssh does not add the port to a HostKeyAlias: the entry is under the alias.
func TestSSHHostKeyAliasHasNoPort(t *testing.T) {
	_, tools := sshSandbox(t)
	tools.gErr = nil
	tools.gOut = "hostname 10.0.0.5\nport 2222\nhostkeyalias Gitlab-Internal\nuserknownhostsfile ~/.ssh/known_hosts\n"
	h := UnknownSSHHost(context.Background(), "ssh://git@internal:2222/x.git")
	if h == nil || h.Name != "gitlab-internal" || h.Host != "10.0.0.5" || h.Port != "2222" {
		t.Fatalf("host = %+v, want the alias alone as the name", h)
	}
	tools.scan = "[10.0.0.5]:2222 ssh-ed25519 " + testKeyED25519 + "\n"
	keys, err := h.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Trust(keys); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(h.WriteTo)
	if !strings.HasPrefix(string(data), "gitlab-internal ssh-ed25519 ") {
		t.Errorf("known_hosts = %q, want the entry under the alias without the port", data)
	}
	if UnknownSSHHost(context.Background(), "ssh://git@internal:2222/x.git") != nil {
		t.Error("the entry under the alias makes the server known")
	}
}

func TestSSHHostWithTheCheckTurnedOff(t *testing.T) {
	_, tools := sshSandbox(t)
	tools.gErr = nil
	tools.gOut = "hostname git.example.test\nport 22\nuserknownhostsfile /dev/null\nstricthostkeychecking false\n"
	if h := UnknownSSHHost(context.Background(), "git@git.example.test:x.git"); h != nil {
		t.Errorf("UserKnownHostsFile /dev/null is the user's choice: nothing to confirm, got %+v", h)
	}
}

func TestSSHHostThroughAProxyCannotBeScanned(t *testing.T) {
	_, tools := sshSandbox(t)
	tools.gErr = nil
	tools.gOut = "hostname git.example.test\nport 22\nproxyjump bastion.example.test\nuserknownhostsfile ~/.ssh/known_hosts\n"
	h := UnknownSSHHost(context.Background(), "git@git.example.test:x.git")
	if h == nil || !h.Proxied {
		t.Fatalf("host = %+v, want unknown and proxied", h)
	}
	_, err := h.Scan(context.Background())
	var scanErr *HostScanError
	if !errors.As(err, &scanErr) {
		t.Fatalf("err = %v, want a *HostScanError", err)
	}
	if got := len(tools.called("ssh-keyscan")); got != 0 {
		t.Errorf("a proxied host is not scanned directly, ssh-keyscan called %d times", got)
	}
	if !strings.Contains(err.Error(), "ssh -T git@git.example.test") {
		t.Errorf("the error must give the command to run: %v", err)
	}
}

func TestSSHHostScanFailures(t *testing.T) {
	_, tools := sshSandbox(t)
	ctx := context.Background()
	h := UnknownSSHHost(ctx, "git@git.example.test:x.git")

	tools.scErr = errors.New("exit status 1")
	if _, err := h.Scan(ctx); err == nil || !strings.Contains(err.Error(), "exit status 1") {
		t.Errorf("a failed scan must say why, got %v", err)
	}
	tools.scErr, tools.scan = nil, "# git.example.test:22 SSH-2.0-OpenSSH\n"
	if _, err := h.Scan(ctx); err == nil {
		t.Error("a scan without any key is a failure")
	}
	tools.scan = "git.example.test ssh-ed25519 not-base64!!\n"
	if _, err := h.Scan(ctx); err == nil {
		t.Error("a key that is not base64 is ignored, and the scan fails")
	}
}

func TestUnknownHostErrorGivesTheCommands(t *testing.T) {
	home, _ := sshSandbox(t)
	h := UnknownSSHHost(context.Background(), "ssh://deploy@git.example.test:2222/x.git")
	err := error(&UnknownHostError{Host: h})
	for _, want := range []string{
		"git.example.test:2222",
		"ssh -p 2222 -T deploy@git.example.test",
		"ssh-keyscan -p 2222 git.example.test >> ",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q must contain %q", err, want)
		}
	}
	if AsUnknownHostError(fmt.Errorf("wrapped: %w", err)) == nil {
		t.Error("AsUnknownHostError must see through wrapping")
	}
	if strings.Contains(err.Error(), home) {
		t.Errorf("the home must be shortened to ~ in %q", err)
	}
}

func TestTrustKeepsWhatKnownHostsHolds(t *testing.T) {
	home, _ := sshSandbox(t)
	path := filepath.Join(home, ".ssh", "known_hosts")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("old.example.test ssh-ed25519 "+testKeyED25519), 0o600); err != nil { // no final newline
		t.Fatal(err)
	}
	h := &SSHHost{Name: "new.example.test", WriteTo: path}
	if err := h.Trust(parseScan([]byte("new.example.test ssh-ed25519 " + testKeyED25519))); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	want := "old.example.test ssh-ed25519 " + testKeyED25519 + "\nnew.example.test ssh-ed25519 " + testKeyED25519 + "\n"
	if string(data) != want {
		t.Errorf("known_hosts = %q, want %q", data, want)
	}
}

// The fingerprint is computed in Go; ssh-keygen says the same.
func TestFingerprintMatchesSSHKeygen(t *testing.T) {
	keygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		t.Skip("ssh-keygen not installed")
	}
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "k")
	if out, err := exec.Command(keygen, "-q", "-t", "ed25519", "-N", "", "-f", keyFile).CombinedOutput(); err != nil {
		t.Skipf("ssh-keygen cannot generate a key: %v %s", err, out)
	}
	pub, err := os.ReadFile(keyFile + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(pub))
	keys := parseScan([]byte("host " + fields[0] + " " + fields[1] + "\n"))
	if len(keys) != 1 {
		t.Fatalf("keys = %+v", keys)
	}
	out, err := exec.Command(keygen, "-lf", keyFile+".pub").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), keys[0].Fingerprint) {
		t.Errorf("ssh-keygen says %q, computed %q", out, keys[0].Fingerprint)
	}
}

// The real runner: fake ssh and ssh-keyscan scripts on a PATH of their own.
func TestSSHHostWithFakeBinaries(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	bin := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	write("ssh", "echo hostname scripted.example.test\necho port 2200\necho userknownhostsfile "+filepath.Join(home, "kh")+"\n")
	write("ssh-keyscan", "echo \"[scripted.example.test]:2200 ssh-ed25519 "+testKeyED25519+"\"\n")
	t.Setenv("PATH", bin)
	t.Cleanup(SetSSHToolRunner(nil))

	ctx := context.Background()
	h := UnknownSSHHost(ctx, "git@alias:x.git")
	if h == nil || h.Name != "[scripted.example.test]:2200" {
		t.Fatalf("host = %+v", h)
	}
	keys, err := h.Scan(ctx)
	if err != nil || len(keys) != 1 {
		t.Fatalf("keys = %v, err = %v", keys, err)
	}
	if err := h.Trust(keys); err != nil {
		t.Fatal(err)
	}
	if UnknownSSHHost(ctx, "git@alias:x.git") != nil {
		t.Error("the host must be known once trusted")
	}
}

func TestSSHHostWithoutSSHKeyscan(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("PATH", t.TempDir()) // nothing on it
	t.Cleanup(SetSSHToolRunner(nil))
	h := UnknownSSHHost(context.Background(), "git@git.example.test:x.git")
	if h == nil {
		t.Fatal("without ssh the host is read from the URL: unknown")
	}
	_, err := h.Scan(context.Background())
	var scanErr *HostScanError
	if !errors.As(err, &scanErr) || !strings.Contains(err.Error(), "ssh-keyscan") {
		t.Errorf("err = %v, want a *HostScanError that names ssh-keyscan", err)
	}
}
