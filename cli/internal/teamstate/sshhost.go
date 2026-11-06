package teamstate

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// The first contact with an SSH server. git and ssh run with
// StrictHostKeyChecking=accept-new: a server the machine does not know yet is
// accepted, and remembered, the first time it answers, silently. The team
// assistants (create, join, `oh teams add`, `oh teams set <id> repo`,
// `oh team promote --remote`) do not leave that to chance: before the first
// connection to a server absent from known_hosts they show its fingerprints
// (ssh-keyscan) and ask for a confirmation; a confirmed server is written to
// the known_hosts that ssh reads, a refused one stops the command. Without a
// terminal nothing is asked: the command is refused with what to run.
//
// The test is offline and cheap (known_hosts is read, `ssh -G` resolves the
// alias and the files); only the scan, done once the user may be asked, talks
// to the server.

// sshHostTimeout bounds `ssh -G` and the scan of a server.
const sshHostTimeout = 10 * time.Second

// sshToolRunner runs ssh / ssh-keyscan and returns their standard output (a
// variable for the tests: nothing of them touches the network).
var sshToolRunner = runSSHTool

func runSSHTool(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

// SetSSHToolRunner replaces how ssh and ssh-keyscan are run (nil: really run
// them); it returns what puts the previous runner back. For the tests.
func SetSSHToolRunner(run func(ctx context.Context, name string, args ...string) ([]byte, error)) (restore func()) {
	prev := sshToolRunner
	if run == nil {
		run = runSSHTool
	}
	sshToolRunner = run
	return func() { sshToolRunner = prev }
}

// SSHHost is the SSH server of a remote, as ssh sees it once ~/.ssh/config has
// been applied.
type SSHHost struct {
	// Host and Port are where ssh connects (Port "" is 22); User is the user of
	// the URL ("git" when it has none).
	Host, Port, User string
	// Name is how known_hosts names the server: the host, or [host]:port.
	Name string
	// Files are the known_hosts files ssh reads, the ones of the user first;
	// WriteTo is the one a confirmed server is added to.
	Files   []string
	WriteTo string
	// Proxied: the connection goes through a ProxyJump or a ProxyCommand, which
	// a direct scan cannot follow.
	Proxied bool
	// Unchecked: the user turned the host check off for this server
	// (UserKnownHostsFile none): there is nothing to confirm.
	Unchecked bool
	// Resolved: `ssh -G` answered, so Host, Port, Name and Files are what ssh
	// uses (not only what the URL says).
	Resolved bool
	// Custom is the ssh command of git of the user (GIT_SSH_COMMAND,
	// core.sshCommand of the global configuration, GIT_SSH) read for this
	// server: the options of it that `ssh -G` was asked with, or why it is not
	// followed (sshcustom.go).
	Custom gitutil.SSHCommand
	// Hashed: HashKnownHosts is on for this server (the default of Debian and
	// Ubuntu), so ssh writes the name of a server hashed (|1|salt|hash) and a
	// server added by oh is written the same way.
	Hashed bool
}

// Display is the server as the user knows it: host, or host:port.
func (h *SSHHost) Display() string {
	if h.Port == "" {
		return h.Host
	}
	return net.JoinHostPort(h.Host, h.Port)
}

// HostKey is a key a server presented: its type and its SHA256 fingerprint
// (the format of `ssh-keygen -lf`).
type HostKey struct {
	Type        string
	Fingerprint string
	blob        string // base64 of the key
}

// UnknownSSHHost returns the SSH server of remote when the machine does not
// know it yet (absent from known_hosts), nil when remote is not SSH, the
// server is known or the user turned the check off. It reads files and
// runs `ssh -G`; it does not talk to the server.
func UnknownSSHHost(ctx context.Context, remote string) *SSHHost {
	h, ok := ResolveSSHHost(ctx, remote)
	if !ok || h.Unchecked || h.Known() {
		return nil
	}
	return h
}

// UnknownSSHHostForJob is UnknownSSHHost for what runs without anybody to ask
// (the daemon, `oh doctor`): the server is reported only when it is certain
// that ssh would take it for a first contact. It stays nil when ssh could not
// be asked how it resolves the server (`ssh -G` failed: the names and files
// are guesses) and when a certificate authority of known_hosts could vouch for
// it (ssh then accepts a signed host key with no entry of its own).
func UnknownSSHHostForJob(ctx context.Context, remote string) *SSHHost {
	h := UnknownSSHHost(ctx, remote)
	if h == nil || !h.Resolved || h.VouchedByCA() {
		return nil
	}
	return h
}

// ResolveSSHHost reads the SSH server out of remote (`ssh://[user@]host[:port]/…`
// or `[user@]host:path`) and applies ~/.ssh/config to it (`ssh -G`: HostName,
// Port, HostKeyAlias, UserKnownHostsFile, ProxyJump) and the ssh command of git
// (GIT_SSH_COMMAND, core.sshCommand of the global configuration: `ssh -G` is
// asked with the same options as git's ssh would get, so the known_hosts that
// is read is the one the connection checks). ok is false for a remote that is
// not SSH.
func ResolveSSHHost(ctx context.Context, remote string) (h *SSHHost, ok bool) {
	user, host, port, ok := parseSSHRemote(remote)
	if !ok {
		return nil, false
	}
	h = &SSHHost{Host: host, Port: normalizePort(port), User: firstOr(user, "git")}
	alias := ""
	userFiles := []string{filepath.Join("~", ".ssh", "known_hosts"), filepath.Join("~", ".ssh", "known_hosts2")}
	var globalFiles []string
	if runtime.GOOS != "windows" {
		globalFiles = []string{"/etc/ssh/ssh_known_hosts", "/etc/ssh/ssh_known_hosts2"}
	}

	h.Custom = gitutil.CustomSSH(ctx)
	noteIgnoredSSH(h.Custom)
	args := append([]string{"-G"}, h.Custom.Args...)
	if port != "" {
		args = append(args, "-p", port)
	}
	if user != "" {
		args = append(args, "-l", user)
	}
	cctx, cancel := context.WithTimeout(ctx, sshHostTimeout)
	defer cancel()
	if out, err := sshToolRunner(cctx, "ssh", append(args, "--", host)...); err == nil {
		h.Resolved = true
		cfg := parseSSHConfig(out)
		if v := cfg["hostname"]; len(v) > 0 {
			h.Host = v[0]
		}
		if h.Port == "" && len(cfg["port"]) > 0 {
			h.Port = normalizePort(cfg["port"][0])
		}
		if v := cfg["hostkeyalias"]; len(v) > 0 && v[0] != "none" {
			alias = v[0]
		}
		if v := cfg["hashknownhosts"]; len(v) > 0 && strings.EqualFold(v[0], "yes") {
			h.Hashed = true
		}
		if v := cfg["userknownhostsfile"]; len(v) > 0 {
			userFiles = v
		}
		if v := cfg["globalknownhostsfile"]; len(v) > 0 {
			globalFiles = v
		}
		for _, key := range []string{"proxyjump", "proxycommand"} {
			if v := cfg[key]; len(v) > 0 && !strings.EqualFold(v[0], "none") {
				h.Proxied = true
			}
		}
	}

	if alias != "" {
		// ssh looks the server up under the alias as it is: the port is not
		// part of it (get_hostfile_hostname_ipaddr, sshconnect.c).
		h.Name = strings.ToLower(alias)
	} else {
		h.Name = knownHostsName(h.Host, h.Port)
	}
	disabled := true
	for _, f := range userFiles {
		if isNoFile(f) {
			continue
		}
		disabled = false
		if p := expandHostsFile(f, h.Host); p != "" {
			h.Files = append(h.Files, p)
			if h.WriteTo == "" {
				h.WriteTo = p
			}
		}
	}
	h.Unchecked = disabled
	for _, f := range globalFiles {
		if p := expandHostsFile(f, h.Host); p != "" && !isNoFile(f) {
			h.Files = append(h.Files, p)
		}
	}
	if h.WriteTo == "" {
		if home, err := os.UserHomeDir(); err == nil {
			h.WriteTo = filepath.Join(home, ".ssh", "known_hosts")
			h.Files = append([]string{h.WriteTo}, h.Files...)
		}
	}
	return h, true
}

// Known reports whether one of the known_hosts files has an entry for the
// server.
func (h *SSHHost) Known() bool {
	for _, f := range h.Files {
		if hostsFileKnows(f, h.Name) {
			return true
		}
	}
	return false
}

// VouchedByCA reports whether a `@cert-authority` line of a known_hosts file
// covers the server: ssh trusts a host key signed by that authority without
// any entry for the server itself.
func (h *SSHHost) VouchedByCA() bool {
	for _, f := range h.Files {
		if hostsFileHasCA(f, h.Name) {
			return true
		}
	}
	return false
}

// Scan asks the server for its keys (ssh-keyscan) and returns them, the best
// type first. It is the only network call of the confirmation.
func (h *SSHHost) Scan(ctx context.Context) ([]HostKey, error) {
	if h.Proxied {
		return nil, &HostScanError{Host: h, Reason: i18n.T("v54a2.host.scan_proxied")}
	}
	cctx, cancel := context.WithTimeout(ctx, sshHostTimeout)
	defer cancel()
	args := []string{"-T", "5", "-t", "ed25519,ecdsa,rsa"}
	if h.Port != "" {
		args = append(args, "-p", h.Port)
	}
	out, err := sshToolRunner(cctx, "ssh-keyscan", append(args, "--", h.Host)...)
	keys := parseScan(out)
	if len(keys) == 0 {
		reason := i18n.T("v54a2.host.scan_silent")
		switch {
		case errors.Is(err, exec.ErrNotFound):
			reason = i18n.T("v54a2.host.scan_missing")
		case err != nil:
			reason = err.Error()
		}
		return nil, &HostScanError{Host: h, Reason: reason}
	}
	return keys, nil
}

// Trust adds the keys to the known_hosts file of the user (the one ssh
// writes), under the name ssh looks the server up with: written in clear, or
// hashed (|1|salt|hash, a salt of its own for each key) when HashKnownHosts is
// on, as ssh does. A key the file already has for the server, hashed or not,
// is not added again.
func (h *SSHHost) Trust(keys []HostKey) error {
	if h.WriteTo == "" || len(keys) == 0 {
		return errors.New("no known_hosts file to write")
	}
	if err := os.MkdirAll(filepath.Dir(h.WriteTo), 0o700); err != nil {
		return err
	}
	existing, _ := os.ReadFile(h.WriteTo)
	var sb strings.Builder
	if len(existing) > 0 && !bytes.HasSuffix(existing, []byte("\n")) {
		sb.WriteString("\n")
	}
	added := 0
	for _, k := range keys {
		if hostsDataHasKey(existing, h.Name, k) {
			continue
		}
		name := h.Name
		if h.Hashed {
			hashed, err := newHashedHostName(h.Name)
			if err != nil {
				return err
			}
			name = hashed
		}
		fmt.Fprintf(&sb, "%s %s %s\n", name, k.Type, k.blob)
		added++
	}
	if added == 0 {
		return nil
	}
	f, err := os.OpenFile(h.WriteTo, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(sb.String()); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// ManualCommands are what the user can run to know the server by hand, after
// checking its fingerprint: ssh asks, and remembers; ssh-keyscan appends
// without asking.
func (h *SSHHost) ManualCommands() (sshCmd, keyscanCmd string) {
	portFlag := ""
	if h.Port != "" {
		portFlag = "-p " + h.Port + " "
	}
	hashFlag := ""
	if h.Hashed {
		hashFlag = "-H " // the name written hashed, as ssh does with HashKnownHosts
	}
	// the options of the ssh command of git, so that ssh checks and writes the
	// known_hosts file oh reads
	custom := ""
	if line := h.Custom.ShellLine(); line != "" {
		custom = line + " "
	}
	sshCmd = "ssh " + custom + portFlag + "-T " + h.User + "@" + h.Host
	keyscanCmd = "ssh-keyscan " + hashFlag + portFlag + h.Host + " >> " + displayPath(h.WriteTo)
	return sshCmd, keyscanCmd
}

// UnknownHostError is the refusal of a command that cannot ask: the server is
// not in known_hosts, and nobody can confirm its fingerprint.
type UnknownHostError struct{ Host *SSHHost }

func (e *UnknownHostError) Error() string {
	sshCmd, keyscanCmd := e.Host.ManualCommands()
	return i18n.Tf("v54a2.host.unknown", e.Host.Display(), sshCmd, keyscanCmd)
}

// AsUnknownHostError returns the *UnknownHostError inside err (nil when none).
func AsUnknownHostError(err error) *UnknownHostError {
	var target *UnknownHostError
	if errors.As(err, &target) {
		return target
	}
	return nil
}

// HostScanError is a server whose keys could not be read.
type HostScanError struct {
	Host   *SSHHost
	Reason string
}

func (e *HostScanError) Error() string {
	sshCmd, keyscanCmd := e.Host.ManualCommands()
	return i18n.Tf("v54a2.host.scan_failed", e.Host.Display(), e.Reason, sshCmd, keyscanCmd)
}

// ── parsing ──────────────────────────────────────────────────────────────────

// parseSSHRemote reads the user, host and port out of an SSH remote: the URL
// forms (ssh://, git+ssh://, ssh+git://) and the scp-like `[user@]host:path`
// (an IPv6 address is then in brackets). A local path, an HTTP(S) URL or a
// host that would be taken for an option is not an SSH remote.
func parseSSHRemote(remote string) (user, host, port string, ok bool) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return "", "", "", false
	}
	if i := strings.Index(remote, "://"); i >= 0 {
		switch strings.ToLower(remote[:i]) {
		case "ssh", "git+ssh", "ssh+git":
		default:
			return "", "", "", false
		}
		u, err := url.Parse(remote)
		if err != nil || u.Hostname() == "" || strings.HasPrefix(u.Hostname(), "-") {
			return "", "", "", false
		}
		return u.User.Username(), u.Hostname(), u.Port(), true
	}
	slash := strings.Index(remote, "/")
	rest := remote
	if at := strings.Index(remote, "@"); at >= 0 && (slash < 0 || at < slash) {
		user, rest = remote[:at], remote[at+1:]
	}
	switch {
	case strings.HasPrefix(rest, "["):
		end := strings.Index(rest, "]:")
		if end < 0 {
			return "", "", "", false
		}
		host = rest[1:end]
	default:
		colon := strings.Index(rest, ":")
		if colon <= 0 || strings.Contains(rest[:colon], "/") {
			return "", "", "", false
		}
		host = rest[:colon]
	}
	if host == "" || strings.HasPrefix(host, "-") || (len(host) == 1 && user == "" && runtime.GOOS == "windows") {
		return "", "", "", false // an option, or a Windows drive letter
	}
	return user, host, "", true
}

func normalizePort(p string) string {
	if p == "22" {
		return ""
	}
	return p
}

func firstOr(s, def string) string {
	if s != "" {
		return s
	}
	return def
}

// knownHostsName is how known_hosts names a server: the host, [host]:port off
// the port 22 (an IPv6 address is bare on the port 22).
func knownHostsName(host, port string) string {
	host = strings.ToLower(host)
	if port == "" {
		return host
	}
	return "[" + host + "]:" + port
}

// parseSSHConfig reads the output of `ssh -G`: one « keyword value… » per
// line, the keywords in lower case.
func parseSSHConfig(out []byte) map[string][]string {
	cfg := map[string][]string{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.ToLower(fields[0])
		if _, seen := cfg[key]; !seen { // the first value wins, as in ssh_config
			cfg[key] = fields[1:]
		}
	}
	return cfg
}

func isNoFile(f string) bool {
	return strings.EqualFold(f, "none") || f == os.DevNull || f == "/dev/null"
}

// expandHostsFile resolves the ~, %d (home), %h (host) and %% of a known_hosts
// file name; "" when it holds another token or the home is unknown.
func expandHostsFile(f, host string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(f, "~") && (len(f) == 1 || f[1] == '/' || f[1] == '\\') {
		if home == "" {
			return ""
		}
		f = home + f[1:]
	}
	var sb strings.Builder
	for i := 0; i < len(f); i++ {
		if f[i] != '%' {
			sb.WriteByte(f[i])
			continue
		}
		i++
		if i >= len(f) {
			return ""
		}
		switch f[i] {
		case '%':
			sb.WriteByte('%')
		case 'd':
			if home == "" {
				return ""
			}
			sb.WriteString(home)
		case 'h':
			sb.WriteString(host)
		default:
			return ""
		}
	}
	return filepath.FromSlash(sb.String())
}

// hostsFileKnows reports whether a known_hosts file has a key entry for name
// (plain, wildcard or hashed pattern). Revoked keys and certificate authorities
// do not count.
func hostsFileKnows(path, name string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	name = strings.ToLower(name)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || strings.HasPrefix(fields[0], "#") || strings.HasPrefix(fields[0], "@") {
			continue
		}
		if patternsMatch(fields[0], name) {
			return true
		}
	}
	return false
}

// hostsFileHasCA reports whether a known_hosts file has a @cert-authority line
// whose patterns match name.
func hostsFileHasCA(path, name string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	name = strings.ToLower(name)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "@cert-authority" {
			continue
		}
		if patternsMatch(fields[1], name) {
			return true
		}
	}
	return false
}

// patternsMatch: the comma-separated host patterns of a known_hosts line match
// name, and none of its !negations does.
func patternsMatch(patterns, name string) bool {
	matched := false
	for _, p := range strings.Split(patterns, ",") {
		negated := strings.HasPrefix(p, "!")
		p = strings.TrimPrefix(p, "!")
		var hit bool
		if strings.HasPrefix(p, "|1|") {
			hit = hashedMatch(p, name)
		} else {
			hit = wildcardMatch(strings.ToLower(p), name)
		}
		switch {
		case hit && negated:
			return false
		case hit:
			matched = true
		}
	}
	return matched
}

// hashedMatch checks a `|1|salt|hash` pattern (HashKnownHosts).
func hashedMatch(pattern, name string) bool {
	parts := strings.Split(pattern, "|")
	if len(parts) != 4 {
		return false
	}
	salt, err1 := base64.StdEncoding.DecodeString(parts[2])
	want, err2 := base64.StdEncoding.DecodeString(parts[3])
	if err1 != nil || err2 != nil {
		return false
	}
	return hmac.Equal(hostMAC(salt, name), want)
}

// wildcardMatch: pattern with * and ? against s.
func wildcardMatch(pattern, s string) bool {
	if pattern == "" {
		return s == ""
	}
	switch pattern[0] {
	case '*':
		for i := 0; i <= len(s); i++ {
			if wildcardMatch(pattern[1:], s[i:]) {
				return true
			}
		}
		return false
	case '?':
		return s != "" && wildcardMatch(pattern[1:], s[1:])
	}
	return s != "" && s[0] == pattern[0] && wildcardMatch(pattern[1:], s[1:])
}

// parseScan reads the output of ssh-keyscan: « host type key » lines, comments
// ignored, one key per type.
func parseScan(out []byte) []HostKey {
	var keys []HostKey
	seen := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		typ, blob := fields[1], fields[2]
		raw, err := base64.StdEncoding.DecodeString(blob)
		if err != nil || seen[typ] {
			continue
		}
		seen[typ] = true
		sum := sha256.Sum256(raw)
		keys = append(keys, HostKey{
			Type:        typ,
			Fingerprint: "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:]),
			blob:        blob,
		})
	}
	sort.SliceStable(keys, func(i, j int) bool { return keyRank(keys[i].Type) < keyRank(keys[j].Type) })
	return keys
}

func keyRank(typ string) int {
	switch {
	case strings.Contains(typ, "ed25519"):
		return 0
	case strings.HasPrefix(typ, "ecdsa"):
		return 1
	}
	return 2
}

// displayPath shortens the home directory of a path to ~.
func displayPath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~/" + filepath.ToSlash(strings.TrimPrefix(p, home+string(filepath.Separator)))
	}
	return p
}
