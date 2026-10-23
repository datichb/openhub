package tracker

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// scpLike matches the short form of a git remote, user@host:path.
var scpLike = regexp.MustCompile(`^[^/@\s]+@([^/:\s]+):`)

// ServerOf reduces a URL, a git remote or a bare host to the identity of the
// server it designates, the only thing the GitLab token cascade compares:
//
//   - the host, lower case, without user, trailing dot or path;
//   - the port, only when it is not the default of the web (80, 443);
//   - the scheme is ignored (https://h, http://h and h are the same server);
//   - the port of an ssh:// or git:// remote is the port of the git service,
//     not of the API: it is dropped; user@host:path is read as host.
//
// It returns "" when raw designates no server. Two GitLab instances that
// share a host and differ only by a path prefix are the same server.
func ServerOf(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if m := scpLike.FindStringSubmatch(raw); m != nil && !strings.Contains(raw, "://") {
		return cleanHost(m[1])
	}
	scheme := "https"
	rest := raw
	if i := strings.Index(raw, "://"); i >= 0 {
		scheme = strings.ToLower(raw[:i])
		rest = raw[i+3:]
	}
	u, err := url.Parse("https://" + rest)
	if err != nil || u.Hostname() == "" {
		return ""
	}
	host := cleanHost(u.Hostname())
	if host == "" {
		return ""
	}
	if scheme == "ssh" || scheme == "git" {
		return host
	}
	if port := u.Port(); port != "" && port != "80" && port != "443" {
		return host + ":" + port
	}
	return host
}

func cleanHost(h string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h)), ".")
}

// SameServer reports whether a and b designate the same server (ServerOf);
// two empty or unreadable values are not the same server.
func SameServer(a, b string) bool {
	sa := ServerOf(a)
	return sa != "" && sa == ServerOf(b)
}

// ValidateServer checks a value typed for a "server of the token" setting: it
// must designate a server (a URL such as https://gitlab.example.com, or a
// host). An empty value is valid and means "not declared".
func ValidateServer(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	if ServerOf(raw) == "" {
		return fmt.Errorf("tracker: %q does not designate a server (expected https://gitlab.example.com)", raw)
	}
	return nil
}
