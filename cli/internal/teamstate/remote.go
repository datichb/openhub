package teamstate

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// RemoteMismatchError: the folder of a clone holds the clone of another
// repository than the one asked for. It is never reused: the wizards say which
// folder and which repositories, and leave the choice to the user.
type RemoteMismatchError struct {
	Path string // the folder of the clone
	Want string // the repository asked for
	Have string // the origin of the clone ("" when it has none)
}

func (e *RemoteMismatchError) Error() string {
	have := e.Have
	if have == "" {
		have = i18n.T("v532w.clone.no_origin")
	}
	return i18n.Tf("v532w.clone.other_remote", e.Path, have, e.Want)
}

// CheckRemote verifies that an existing clone is the clone of the repository
// of r: its origin is that repository (the same host and path, whatever the
// scheme, the .git suffix or the user). A repository made without a remote
// (a solo team-state) or not cloned yet always passes.
func (r *Repo) CheckRemote() error {
	if r.remote == "" || !r.IsCloned() {
		return nil
	}
	have, _ := originURL(r.path)
	if have != "" && SameRemote(have, r.remote) {
		return nil
	}
	return &RemoteMismatchError{Path: r.path, Want: r.remote, Have: have}
}

// originURL reads the URL of the origin of the clone at path from
// .git/config (no git process).
func originURL(path string) (string, bool) {
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
				return strings.TrimSpace(v), true
			}
		}
	}
	return "", false
}

// SameRemote reports whether a and b designate the same repository: same host
// and same path, whatever the scheme (https, ssh, scp-like), the user, the
// .git suffix, the trailing slash or the case of the host. Local paths are
// compared as cleaned paths.
func SameRemote(a, b string) bool {
	ka, kb := remoteKey(a), remoteKey(b)
	return ka != "" && ka == kb
}

func remoteKey(remote string) string {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return ""
	}
	host, path := "", ""
	switch {
	case strings.Contains(remote, "://"):
		u, err := url.Parse(remote)
		if err != nil || u.Host == "" {
			return filepath.Clean(remote)
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
		return filepath.Clean(remote)
	}
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	return host + "/" + path
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
