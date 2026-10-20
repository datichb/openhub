package mcpresolve

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

// The service token is sent to the URL of the service, so a URL imposed by
// the team (a shared file any member can change) is used only once the
// member approved it (audit v5, S-31): it is pinned in the personal hub.toml
// (`mcp.<service>.trusted_url`, `oh mcp trust`) and used when it equals the
// pinned URL after normalization.

// PendingReason tells why the URL imposed by the team is not used.
type PendingReason string

const (
	// PendingNotPinned: the member never approved a URL for this service.
	PendingNotPinned PendingReason = "not_pinned"
	// PendingPinnedOther: the member approved another URL.
	PendingPinnedOther PendingReason = "pinned_other"
	// PendingInvalid: the URL is not acceptable (not https, credentials…).
	PendingInvalid PendingReason = "invalid"
)

// Reasons a URL is refused (ErrURL wraps one of them).
var (
	// ErrURL is wrapped by every refusal of NormalizeURL.
	ErrURL = errors.New("mcpresolve: URL refused")
	// ErrNotHTTPS: only https is accepted (http for loopback hosts only).
	ErrNotHTTPS = errors.New("https required (http only for localhost, 127.0.0.1 and ::1)")
	// ErrBadURL: not an absolute base URL without credentials, query or fragment.
	ErrBadURL = errors.New("not a valid base URL (absolute, no credentials, no query, no fragment)")
)

func urlErr(reason error) error { return fmt.Errorf("%w: %w", ErrURL, reason) }

// loopbackHosts are the only hosts that may be reached over plain http.
var loopbackHosts = map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true}

// NormalizeURL returns the canonical form of a base URL: scheme and host in
// lower case, default port and trailing slash removed. It refuses what must
// never carry a service token: another scheme than https (http is accepted
// for localhost, 127.0.0.1 and ::1), credentials in the URL, a query, a
// fragment, a missing host.
func NormalizeURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Opaque != "" {
		return "", urlErr(ErrBadURL)
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", urlErr(ErrBadURL)
	}
	switch scheme {
	case "https":
	case "http":
		if !loopbackHosts[host] {
			return "", urlErr(ErrNotHTTPS)
		}
	default:
		return "", urlErr(ErrNotHTTPS)
	}
	port := u.Port()
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		port = ""
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port != "" {
		host += ":" + port
	}
	return scheme + "://" + host + strings.TrimRight(u.EscapedPath(), "/"), nil
}

// SameURL reports whether two URLs are the same once normalized.
func SameURL(a, b string) bool {
	na, errA := NormalizeURL(a)
	nb, errB := NormalizeURL(b)
	return errA == nil && errB == nil && na == nb
}

// AddrKind classifies an address a member should know about before
// approving a URL: "loopback", "private", "link-local" (which holds the
// cloud metadata services) or "unspecified"; "" for a public address.
func AddrKind(ip net.IP) string {
	switch {
	case ip.IsLoopback():
		return "loopback"
	case ip.IsPrivate():
		return "private"
	case ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast():
		return "link-local"
	case ip.IsUnspecified():
		return "unspecified"
	}
	return ""
}

// applyEnforcedURL is the first step of the URL cascade. The URL imposed by
// the team is used (URLEnforced) only when the member pinned it. Otherwise
// it is kept in EnforcedURLPending, for the warnings: used is false and the
// cascade goes on without it, and skipShared tells the later steps to leave
// the URL of the team out (it is the imposed URL, not a recommendation).
func (eff *EffectiveConfig) applyEnforcedURL(shared *teamstate.SharedMCPConfig, trusted string) (used, skipShared bool) {
	if shared == nil || !shared.IsURLEnforced() || shared.URL == "" {
		return false, false
	}
	proposed, err := NormalizeURL(shared.URL)
	if err != nil {
		eff.EnforcedURLPending, eff.EnforcedURLReason = shared.URL, PendingInvalid
		return false, true
	}
	if trusted == "" {
		eff.EnforcedURLPending, eff.EnforcedURLReason = shared.URL, PendingNotPinned
		return false, true
	}
	if pinned, err := NormalizeURL(trusted); err != nil || pinned != proposed {
		eff.EnforcedURLPending, eff.EnforcedURLReason = shared.URL, PendingPinnedOther
		return false, true
	}
	eff.URL, eff.URLEnforced = proposed, true
	return true, false
}
