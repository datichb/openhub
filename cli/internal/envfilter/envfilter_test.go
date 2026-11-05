package envfilter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// otherNamesOfManaged are the names, besides the ones oh knows, under which the
// tools read the credentials oh manages: gh (GH_TOKEN, GH_ENTERPRISE_TOKEN,
// GITHUB_ENTERPRISE_TOKEN), glab and python-gitlab (GITLAB_ACCESS_TOKEN,
// GITLAB_PRIVATE_TOKEN, GITLAB_OAUTH_TOKEN), the Anthropic SDK
// (ANTHROPIC_AUTH_TOKEN).
var otherNamesOfManaged = []string{
	"GH_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN",
	"GITLAB_ACCESS_TOKEN", "GITLAB_PRIVATE_TOKEN", "GITLAB_OAUTH_TOKEN",
	"ANTHROPIC_AUTH_TOKEN",
}

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
	extra := []string{"ANTHROPIC_API_KEY", "AWS_*", "GITHUB_TOKEN", "GH_TOKEN", "OH_PASSPHRASE", "OH_*"}
	for _, n := range []string{"ANTHROPIC_API_KEY", "AWS_BEARER_TOKEN_BEDROCK", "AWS_SECRET_ACCESS_KEY", "AWS_PROFILE",
		"GITHUB_TOKEN", "GH_TOKEN", "GITLAB_TOKEN", "OH_PASSPHRASE", "OPENAI_API_KEY"} {
		assert.False(t, Allowed(n, extra), n)
		assert.True(t, HardDenied(n), n)
	}
	assert.True(t, Allowed("AWS_REGION", extra), "AWS_* lets the other AWS variables in")
	assert.True(t, Allowed("OH_SOMETHING", extra))
}

// The tools read a managed credential under more than one name (gh reads
// GH_TOKEN as well as GITHUB_TOKEN, glab GITLAB_ACCESS_TOKEN…): each of the
// names is removed like the one oh knows, by name and by pattern.
func TestOtherNamesOfTheManagedTokensStayRemoved(t *testing.T) {
	patterns := []string{"GH_*", "GITHUB_*", "GITLAB_*", "ANTHROPIC_*"}
	for _, n := range otherNamesOfManaged {
		t.Run(n, func(t *testing.T) {
			assert.True(t, HardDenied(n))
			assert.False(t, FromBase(n))
			assert.False(t, Allowed(n, nil))
			assert.False(t, Allowed(n, []string{n}), "listed by its name")
			assert.True(t, FromExtra(n, []string{n}), "the entry does match: only the hard list holds it back")

			matching := 0
			for _, p := range patterns {
				if FromExtra(n, []string{p}) {
					matching++
					assert.False(t, Allowed(n, []string{p}), "listed by the pattern %s", p)
				}
			}
			assert.Equal(t, 1, matching, "one of the patterns covers the name")
			assert.False(t, Allowed(n, append([]string{n}, patterns...)), "listed every way at once")
		})
	}

	environ := []string{"PATH=/bin", "GH_HOST=github.example.com", "GITLAB_URL=https://gitlab.example.com", "GEMINI_API_KEY=g"}
	for _, n := range otherNamesOfManaged {
		environ = append(environ, n+"=x")
	}
	kept, dropped := Split(environ, append([]string{"GEMINI_API_KEY"}, patterns...))
	assert.ElementsMatch(t, []string{"PATH", "GH_HOST", "GITLAB_URL", "GEMINI_API_KEY"}, names(kept),
		"the names of the tokens go, the other variables of the same families stay as listed")
	assert.ElementsMatch(t, otherNamesOfManaged, dropped)
}

// The families are not denied as a whole: only the credentials, by name. A
// generic name is not GitLab's (glab reads OAUTH_TOKEN, but so do other tools).
func TestOnlyTheNamesOfTheTokensAreHardDenied(t *testing.T) {
	for _, n := range []string{"GH_HOST", "GH_REPO", "GH_PAGER", "GITHUB_ACTIONS", "GITLAB_URL", "GITLAB_HOST", "ANTHROPIC_BASE_URL", "OAUTH_TOKEN"} {
		assert.False(t, HardDenied(n), n)
	}
	assert.True(t, Allowed("GH_HOST", []string{"GH_*"}))
	assert.True(t, Allowed("GITLAB_URL", []string{"GITLAB_*"}))
	assert.False(t, Allowed("GH_HOST", nil))
}

// Windows compares the names without regard to case: gh_token is GH_TOKEN
// there. The test switches the comparison on (it restores it), on any system.
func TestManagedTokensAreRemovedWhateverTheCaseOnWindows(t *testing.T) {
	previous := foldCase
	foldCase = true
	t.Cleanup(func() { foldCase = previous })

	for _, n := range append([]string{"GITHUB_TOKEN", "GITLAB_TOKEN", "ANTHROPIC_API_KEY", "AWS_SECRET_ACCESS_KEY"}, otherNamesOfManaged...) {
		for _, spelling := range []string{n, strings.ToLower(n), cased(n)} {
			assert.True(t, HardDenied(spelling), spelling)
			assert.False(t, Allowed(spelling, nil), spelling)
			for _, listed := range []string{spelling, strings.ToLower(n), strings.ToLower(n[:strings.Index(n, "_")+1]) + "*"} {
				require.True(t, FromExtra(spelling, []string{listed}), "%s is listed as %s", spelling, listed)
				assert.False(t, Allowed(spelling, []string{listed}), "%s listed as %s", spelling, listed)
			}
		}
	}
	for _, n := range []string{"gh_token", "Gh_Token", "github_enterprise_token", "anthropic_auth_token", "gitlab_oauth_token"} {
		assert.True(t, HardDenied(n), n)
	}

	kept, dropped := Split([]string{"Path=C:\\bin", "gh_token=x", "Github_Enterprise_Token=x", "anthropic_auth_token=x", "gh_host=h", "my_var=1"},
		[]string{"gh_token", "GITHUB_*", "ANTHROPIC_AUTH_TOKEN", "gh_host", "MY_VAR"})
	assert.ElementsMatch(t, []string{"Path", "gh_host", "my_var"}, names(kept))
	assert.ElementsMatch(t, []string{"gh_token", "Github_Enterprise_Token", "anthropic_auth_token"}, dropped)
}

// cased changes the case of every other letter: GH_TOKEN becomes Gh_ToKeN.
func cased(s string) string {
	out := []rune(strings.ToLower(s))
	for i, r := range out {
		if i%2 == 0 {
			out[i] = []rune(strings.ToUpper(string(r)))[0]
		}
	}
	return string(out)
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
