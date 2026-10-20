package envfilter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func names(kept []string) []string {
	var out []string
	for _, kv := range kept {
		for i := range kv {
			if kv[i] == '=' {
				out = append(out, kv[:i])
				break
			}
		}
	}
	return out
}

func TestSecretOutsideTheListIsNotPassed(t *testing.T) {
	environ := []string{
		"PATH=/usr/bin", "HOME=/home/u", "LANG=fr_FR.UTF-8", "LC_ALL=C", "TZ=UTC", "TMPDIR=/tmp",
		"GEMINI_API_KEY=secret1", "GH_TOKEN=secret2", "NPM_TOKEN=secret3", "SLACK_BOT_TOKEN=secret4",
		"DATABASE_URL=postgres://u:p@h/db", "AWS_SECRET_ACCESS_KEY=secret5", "MY_SERVICE_PASSWORD=secret6",
		"SSH_AUTH_SOCK=/tmp/agent.sock",
	}
	kept, dropped := Split(environ, nil)
	assert.ElementsMatch(t, []string{"PATH", "HOME", "LANG", "LC_ALL", "TZ", "TMPDIR"}, names(kept))
	assert.ElementsMatch(t, []string{"GEMINI_API_KEY", "GH_TOKEN", "NPM_TOKEN", "SLACK_BOT_TOKEN", "DATABASE_URL",
		"AWS_SECRET_ACCESS_KEY", "MY_SERVICE_PASSWORD", "SSH_AUTH_SOCK"}, dropped, "the names only, never the values")
}

func TestVariablesOfTheBaseList(t *testing.T) {
	for _, n := range []string{
		"PATH", "HOME", "USER", "LOGNAME", "SHELL", "TERM", "LANG", "LC_ALL", "LC_CTYPE", "TMPDIR", "TZ",
		"XDG_CACHE_HOME", "XDG_RUNTIME_DIR", "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "https_proxy", "no_proxy",
		"SSL_CERT_FILE", "NODE_EXTRA_CA_CERTS", "GOPATH", "JAVA_HOME", "OH_HOME",
	} {
		assert.True(t, Allowed(n, nil), n)
	}
	for _, n := range []string{"SSH_AUTH_SOCK", "GEMINI_API_KEY", "GH_TOKEN", "OH_LLM_KEY", "OH_TEAMSTATE_TOKEN", "OH_PASSPHRASE",
		"LCX", "XDGX", "NPM_TOKEN", "GOOGLE_APPLICATION_CREDENTIALS", ""} {
		assert.False(t, Allowed(n, nil), n)
	}
}

func TestEnvAllowAddsAName(t *testing.T) {
	assert.False(t, Allowed("SSH_AUTH_SOCK", nil))
	assert.True(t, Allowed("SSH_AUTH_SOCK", []string{"SSH_AUTH_SOCK"}))
	assert.False(t, Allowed("SSH_AUTH_SOCK_OLD", []string{"SSH_AUTH_SOCK"}), "a name is exact")
	assert.True(t, Allowed("GEMINI_API_KEY", []string{"GEMINI_API_KEY"}), "the user decides")
}

func TestEnvAllowPattern(t *testing.T) {
	extra := []string{"MYCO_*", "JAVA_OPTS"}
	assert.True(t, Allowed("MYCO_REGION", extra))
	assert.True(t, Allowed("MYCO_", extra))
	assert.False(t, Allowed("MYCOX", extra))
	assert.False(t, Allowed("OTHER_MYCO_X", extra))
	assert.True(t, Allowed("JAVA_OPTS", extra))
	assert.False(t, Allowed("JAVA_OPTS2", extra))

	kept, dropped := Split([]string{"MYCO_A=1", "MYCO_B=2", "OTHER=3"}, extra)
	assert.Equal(t, []string{"MYCO_A=1", "MYCO_B=2"}, kept)
	assert.Equal(t, []string{"OTHER"}, dropped)
}

// The credentials oh manages reach the tool only as the session token of the
// proxy, whatever the user lists.
func TestManagedCredentialsStayRemoved(t *testing.T) {
	extra := []string{"ANTHROPIC_API_KEY", "AWS_*", "GITHUB_TOKEN", "OH_PASSPHRASE", "OH_*"}
	for _, n := range []string{"ANTHROPIC_API_KEY", "AWS_BEARER_TOKEN_BEDROCK", "AWS_SECRET_ACCESS_KEY", "AWS_PROFILE",
		"GITHUB_TOKEN", "GITLAB_TOKEN", "OH_PASSPHRASE", "OPENAI_API_KEY"} {
		assert.False(t, Allowed(n, extra), n)
		assert.True(t, HardDenied(n), n)
	}
	assert.True(t, Allowed("AWS_REGION", extra), "AWS_* lets the other AWS variables in")
	assert.True(t, Allowed("OH_SOMETHING", extra))
}

func TestInvalidPatternsMatchNothing(t *testing.T) {
	for _, p := range []string{"*", "A*", "", "a b", "X=Y", "*_KEY", "A**", "1ABC", "A-B"} {
		assert.False(t, ValidPattern(p), "%q", p)
	}
	for _, p := range []string{"FOO", "FOO_*", "AB*", "_X", "ProgramFiles(x86)"} {
		assert.True(t, ValidPattern(p), "%q", p)
	}
	assert.False(t, Allowed("ANYTHING", []string{"*"}), "a bare star does not switch the filter off")
	assert.False(t, Allowed("ANYTHING", []string{"A*"}))
	assert.True(t, Allowed("ANYTHING", []string{"ANY*"}))
}

func TestLooksSecret(t *testing.T) {
	for _, n := range []string{"GEMINI_API_KEY", "GH_TOKEN", "x_secret", "DB_PASSWORD", "TOKEN", "SECRET", "PASSWORD",
		"MY_TOKEN*", "SERVICE_PASSPHRASE", "GOOGLE_CREDENTIALS"} {
		assert.True(t, LooksSecret(n), n)
	}
	for _, n := range []string{"PATH", "MYCO_*", "JAVA_OPTS", "KEYBOARD", "TOKENIZER_URL", "MONKEY", "SSH_AUTH_SOCK"} {
		assert.False(t, LooksSecret(n), n)
	}
}

func TestSplitSkipsMalformedEntries(t *testing.T) {
	kept, dropped := Split([]string{"NOEQUALS", "=C:=C:\\", "=X", "PATH=/bin"}, nil)
	assert.Equal(t, []string{"PATH=/bin"}, kept)
	assert.Empty(t, dropped)
}

func TestBaseNamesAreSortedAndValid(t *testing.T) {
	list := BaseNames()
	require.NotEmpty(t, list)
	assert.Contains(t, list, "LC_*")
	assert.Contains(t, list, "XDG_*")
	assert.Contains(t, list, "PATH")
	for _, n := range list {
		assert.True(t, ValidPattern(n), n)
		assert.False(t, LooksSecret(n), "%s is a secret-looking name in the base list", n)
	}
}
