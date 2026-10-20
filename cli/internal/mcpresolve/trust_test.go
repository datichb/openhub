package mcpresolve

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

func enforced(url string) *teamstate.SharedMCPConfig {
	return &teamstate.SharedMCPConfig{URL: url, URLEnforced: boolPtr(true)}
}

func TestNormalizeURL(t *testing.T) {
	tests := map[string]string{
		"https://jira.example.com":           "https://jira.example.com",
		"https://jira.example.com/":          "https://jira.example.com",
		"https://jira.example.com///":        "https://jira.example.com",
		"HTTPS://Jira.Example.COM/":          "https://jira.example.com",
		"  https://jira.example.com/jira/  ": "https://jira.example.com/jira",
		"https://jira.example.com/Jira/Base": "https://jira.example.com/Jira/Base",
		"https://jira.example.com:443":       "https://jira.example.com",
		"https://jira.example.com:8443/":     "https://jira.example.com:8443",
		"https://jira.example.com.":          "https://jira.example.com",
		"https://10.0.0.5:8080/jira":         "https://10.0.0.5:8080/jira",
		"https://[2001:db8::1]:8443/":        "https://[2001:db8::1]:8443",
		"http://localhost:8080/":             "http://localhost:8080",
		"http://LOCALHOST":                   "http://localhost",
		"http://127.0.0.1:9000/mock":         "http://127.0.0.1:9000/mock",
		"http://[::1]:8080":                  "http://[::1]:8080",
		"http://localhost:80":                "http://localhost",
		"https://jira.example.com/a%20b/":    "https://jira.example.com/a%20b",
	}
	for in, want := range tests {
		got, err := NormalizeURL(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
}

func TestNormalizeURLRefuses(t *testing.T) {
	notHTTPS := []string{
		"http://jira.example.com", "http://10.0.0.5", "http://127.0.0.2", "http://localhost.evil.example",
		"http://[::2]", "ftp://jira.example.com", "ws://localhost",
	}
	for _, in := range notHTTPS {
		_, err := NormalizeURL(in)
		require.ErrorIs(t, err, ErrURL, in)
		assert.ErrorIs(t, err, ErrNotHTTPS, in)
	}
	bad := []string{
		"", "jira.example.com", "//jira.example.com", "https://", "https:jira.example.com", "https:///path",
		"https://user:pass@jira.example.com", "https://jira.example.com@evil.example",
		"https://jira.example.com?x=1", "https://jira.example.com/?", "https://jira.example.com#frag",
		"https://exa mple.com", "https://:443", "file:///etc/passwd",
	}
	for _, in := range bad {
		_, err := NormalizeURL(in)
		require.ErrorIs(t, err, ErrURL, in)
	}
}

func TestSameURL(t *testing.T) {
	assert.True(t, SameURL("https://Jira.example.com/", "https://jira.example.com"))
	assert.False(t, SameURL("https://jira.example.com/a", "https://jira.example.com"))
	assert.False(t, SameURL("https://jira.example.com", "https://jira.example.com:8443"))
	assert.False(t, SameURL("http://jira.example.com", "http://jira.example.com"), "refused URLs are never the same")
}

func TestAddrKind(t *testing.T) {
	tests := map[string]string{
		"127.0.0.1": "loopback", "::1": "loopback", "10.1.2.3": "private", "192.168.0.9": "private", "172.16.0.1": "private",
		"fd00::1": "private", "169.254.169.254": "link-local", "fe80::1": "link-local", "0.0.0.0": "unspecified",
		"93.184.216.34": "", "2606:2800:220:1::1": "",
	}
	for ip, want := range tests {
		assert.Equal(t, want, AddrKind(net.ParseIP(ip)), ip)
	}
}

// S-31: the URL imposed by the team is used only once the member pinned it.
func TestEnforcedURLNotPinnedIsIgnored(t *testing.T) {
	hub := config.MCPServerConfig{Token: "jira-token", URL: "https://mine.example.com"}
	shared := enforced("https://evil.example.com")

	for name, eff := range map[string]EffectiveConfig{
		"Resolve":     Resolve(shared, hub),
		"ResolveFull": ResolveFull(shared, hub, nil, "acme"),
	} {
		assert.Equal(t, "https://mine.example.com", eff.URL, name+": the token goes to the personal URL")
		assert.False(t, eff.URLEnforced, name)
		assert.Equal(t, "https://evil.example.com", eff.EnforcedURLPending, name)
		assert.Equal(t, PendingNotPinned, eff.EnforcedURLReason, name)
	}

	// Without any personal URL, the URL of the team is not a fallback either:
	// it is the imposed URL, not a recommendation.
	for name, eff := range map[string]EffectiveConfig{
		"Resolve":     Resolve(shared, config.MCPServerConfig{Token: "t"}),
		"ResolveFull": ResolveFull(shared, config.MCPServerConfig{Token: "t"}, nil, "acme"),
	} {
		assert.Empty(t, eff.URL, name+": the built-in default applies, never the unapproved URL")
		assert.Equal(t, "https://evil.example.com", eff.EnforcedURLPending, name)
	}

	// The project URL comes before the hub URL, as for any URL.
	eff := ResolveFull(shared, hub, &domain.ProjectMCPService{URL: "https://project.example.com"}, "acme")
	assert.Equal(t, "https://project.example.com", eff.URL)
}

func TestEnforcedURLPinnedIsUsed(t *testing.T) {
	shared := enforced("https://Jira.Corp.example.com/")
	hub := config.MCPServerConfig{URL: "https://mine.example.com", TrustedURL: "https://jira.corp.example.com"}

	for name, eff := range map[string]EffectiveConfig{
		"Resolve":     Resolve(shared, hub),
		"ResolveFull": ResolveFull(shared, hub, &domain.ProjectMCPService{URL: "https://project.example.com"}, "acme"),
	} {
		assert.Equal(t, "https://jira.corp.example.com", eff.URL, name+": normalized, wins over the personal URLs")
		assert.True(t, eff.URLEnforced, name)
		assert.Empty(t, eff.EnforcedURLPending, name)
	}
}

func TestEnforcedURLPinnedToAnotherIsIgnored(t *testing.T) {
	shared := enforced("https://evil.example.com")
	hub := config.MCPServerConfig{URL: "https://mine.example.com", TrustedURL: "https://jira.corp.example.com"}

	for name, eff := range map[string]EffectiveConfig{
		"Resolve":     Resolve(shared, hub),
		"ResolveFull": ResolveFull(shared, hub, nil, "acme"),
	} {
		assert.Equal(t, "https://mine.example.com", eff.URL, name)
		assert.False(t, eff.URLEnforced, name)
		assert.Equal(t, "https://evil.example.com", eff.EnforcedURLPending, name)
		assert.Equal(t, PendingPinnedOther, eff.EnforcedURLReason, name)
	}
}

func TestEnforcedURLRefusedFormats(t *testing.T) {
	for _, u := range []string{"http://jira.corp.example.com", "http://10.0.0.5", "https://user:pw@jira.example.com", "ftp://x"} {
		// Even pinned by hand: a refused format is never used.
		hub := config.MCPServerConfig{TrustedURL: u}
		for name, eff := range map[string]EffectiveConfig{
			"Resolve":     Resolve(enforced(u), hub),
			"ResolveFull": ResolveFull(enforced(u), hub, nil, "acme"),
		} {
			assert.Empty(t, eff.URL, name+" "+u)
			assert.False(t, eff.URLEnforced, name+" "+u)
			assert.Equal(t, u, eff.EnforcedURLPending, name+" "+u)
			assert.Equal(t, PendingInvalid, eff.EnforcedURLReason, name+" "+u)
		}
	}

	// http is accepted for loopback hosts (a mock server, a local Jira).
	hub := config.MCPServerConfig{TrustedURL: "http://localhost:8080"}
	eff := ResolveFull(enforced("http://localhost:8080/"), hub, nil, "acme")
	assert.Equal(t, "http://localhost:8080", eff.URL)
	assert.True(t, eff.URLEnforced)
}

func TestRecommendedURLIsStillUsed(t *testing.T) {
	shared := &teamstate.SharedMCPConfig{URL: "https://team.example.com"}
	eff := ResolveFull(shared, config.MCPServerConfig{}, nil, "acme")
	assert.Equal(t, "https://team.example.com", eff.URL)
	assert.Empty(t, eff.EnforcedURLPending)
	assert.Equal(t, "https://team.example.com", Resolve(shared, config.MCPServerConfig{}).URL)
}

func TestEnforcedURLEmptyOrAbsentChangesNothing(t *testing.T) {
	hub := config.MCPServerConfig{URL: "https://mine.example.com"}
	assert.Equal(t, "https://mine.example.com", ResolveFull(&teamstate.SharedMCPConfig{URLEnforced: boolPtr(true)}, hub, nil, "").URL)
	assert.Empty(t, ResolveFull(&teamstate.SharedMCPConfig{URLEnforced: boolPtr(true)}, hub, nil, "").EnforcedURLPending)
	assert.Equal(t, "https://mine.example.com", ResolveFull(nil, hub, nil, "").URL)
}
