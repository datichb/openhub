// Package gitremote reads Git remote URLs: which repository two spellings
// designate, where its host and path are, what the origin of a clone is. It
// depends on nothing, so that the configuration (which derives the folder and
// the ID of a team from a remote) and teamstate (which checks a clone) share
// one reading.
package gitremote

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Same reports whether a and b designate the same repository: same host and
// same path, whatever the scheme (https, ssh, scp-like), the user, the .git
// suffix, the trailing slash or the case of the host. Local paths are compared
// as cleaned paths.
func Same(a, b string) bool {
	ka, kb := key(a), key(b)
	return ka != "" && ka == kb
}

func key(remote string) string {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return ""
	}
	host, path, ok := Split(remote)
	if !ok {
		return filepath.Clean(remote)
	}
	return host + "/" + path
}

// Split returns the host (lowercase) and the repository path (no leading or
// trailing slash, no .git suffix) of a remote URL or scp-like address. ok is
// false for a local path, which has neither.
func Split(remote string) (host, path string, ok bool) {
	remote = strings.TrimSpace(remote)
	switch {
	case strings.Contains(remote, "://"):
		u, err := url.Parse(remote)
		if err != nil || u.Host == "" {
			return "", "", false
		}
		host, path = strings.ToLower(u.Hostname()), u.Path
	case isSCPLike(remote):
		rest := remote
		if i := strings.Index(rest, "@"); i >= 0 {
			rest = rest[i+1:]
		}
		h, p, _ := strings.Cut(rest, ":")
		host, path = strings.ToLower(h), p
	default:
		return "", "", false
	}
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	return host, path, true
}

// isSCPLike: user@host:path (git's short form for ssh), not a local path nor
// a Windows drive.
func isSCPLike(s string) bool {
	i := strings.Index(s, ":")
	if i <= 1 || strings.ContainsAny(s[:i], `/\`) {
		return false
	}
	return true
}

// Origin reads the URL of the origin of the clone at path from .git/config
// (no git process). ok is false when the file cannot be read; the URL is
// empty when the clone has no origin.
func Origin(path string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(path, ".git", "config"))
	if err != nil {
		return "", false
	}
	inOrigin := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "["):
			inOrigin = strings.EqualFold(strings.Join(strings.Fields(line), " "), `[remote "origin"]`)
		case inOrigin && strings.HasPrefix(line, "url"):
			if k, v, ok := strings.Cut(line, "="); ok && strings.TrimSpace(k) == "url" {
				return ConfigValue(v), true
			}
		}
	}
	return "", false
}

// ConfigGet reads the value of key in section of the .git/config of the clone
// at path (no git process): section is the header as git writes it without the
// brackets (`core`, `remote "origin"`), the section and the key are compared
// without regard to case, and the last value wins, as in git. ok is false when
// the file cannot be read or holds no such key. The files the configuration
// includes are not followed.
func ConfigGet(path, section, key string) (value string, ok bool) {
	data, err := os.ReadFile(filepath.Join(path, ".git", "config"))
	if err != nil {
		return "", false
	}
	want := strings.Join(strings.Fields(section), " ")
	in := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "["):
			header, _, _ := strings.Cut(strings.TrimPrefix(line, "["), "]")
			in = strings.EqualFold(strings.Join(strings.Fields(header), " "), want)
		case in && line != "":
			k, v, found := strings.Cut(line, "=")
			if found && strings.EqualFold(strings.TrimSpace(k), key) {
				value, ok = ConfigValue(v), true
			}
		}
	}
	return value, ok
}

// IsCloneOf reports whether path holds a clone whose origin is the repository
// remote.
func IsCloneOf(path, remote string) bool {
	if info, err := os.Stat(filepath.Join(path, ".git")); err != nil || !info.IsDir() {
		return false
	}
	have, _ := Origin(path)
	return have != "" && Same(have, remote)
}

// ConfigValue decodes a value as git writes it in a config file: quoted
// parts, backslash escapes (a Windows path is written C:\\Users\\…) and an
// inline comment (# or ;) outside quotes.
func ConfigValue(raw string) string {
	var b strings.Builder
	quoted := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch {
		case c == '\\' && i+1 < len(raw):
			i++
			switch raw[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'b':
				b.WriteByte('\b')
			default:
				b.WriteByte(raw[i])
			}
		case c == '"':
			quoted = !quoted
		case (c == '#' || c == ';') && !quoted:
			return strings.TrimSpace(b.String())
		default:
			b.WriteByte(c)
		}
	}
	return strings.TrimSpace(b.String())
}
