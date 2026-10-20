package envfilter

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/testenv"
)

// Regression test of the leak of 2026-09-29: the Bedrock key was in the
// environment of oh and an agent read it with `env` in its shell. The
// callers of the filter (the tool server of a local session, and through it
// the shells of the agents) split the real environment of the process: so
// does this test, with fake credentials of every kind in it.

func splitProcessEnv(t *testing.T, extra []string) (kept, dropped []string) {
	t.Helper()
	t.Setenv("LANG", "fr_FR.UTF-8")
	return Split(os.Environ(), extra)
}

// No value of a credential reaches the tool, under any name, with the base
// list alone and with the wide env_allow of a user in a hurry. Names with a
// star at the start are not patterns (PREFIX_* only): they match nothing.
func TestNoCredentialValuePassesTheFilter(t *testing.T) {
	secrets := testenv.SetFakeSecrets(t)
	for name, extra := range map[string][]string{
		"base list":             nil,
		"wide env_allow":        {"AWS_*", "*_TOKEN", "*_KEY"},
		"every managed name":    {"AWS_BEARER_TOKEN_BEDROCK", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GITHUB_TOKEN", "GITLAB_TOKEN"},
		"prefixes of managed":   {"AWS_*", "ANTHROPIC_*", "OPENAI_*", "GITHUB_*", "GITLAB_*"},
		"invalid wildcards too": {"*", "**", "*_TOKEN", "*KEY"},
	} {
		t.Run(name, func(t *testing.T) {
			kept, _ := splitProcessEnv(t, extra)
			assert.Empty(t, testenv.Leaks(secrets, kept))
			// A control: what is allowed does pass, with its value.
			assert.Contains(t, kept, "LANG=fr_FR.UTF-8")
		})
	}
}

// The credentials oh manages are removed even when env_allow lists them,
// by name or by prefix.
func TestManagedCredentialsAreRemovedWhateverEnvAllowSays(t *testing.T) {
	secrets := testenv.SetFakeSecrets(t)
	managed := []string{"AWS_BEARER_TOKEN_BEDROCK", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GITHUB_TOKEN", "GITLAB_TOKEN"}
	for _, n := range managed {
		require.True(t, HardDenied(n), n)
	}
	kept, _ := splitProcessEnv(t, append([]string{"AWS_*", "ANTHROPIC_*", "OPENAI_*", "GITHUB_*", "GITLAB_*"}, managed...))
	for _, n := range managed {
		assert.NotContains(t, names(kept), n)
		assert.Empty(t, testenv.Leaks(map[string]string{n: secrets[n]}, kept))
	}
}

// A wide pattern that is valid lets through the secrets oh does not manage
// (GH_TOKEN, GEMINI_API_KEY, the tokens of the model tests): this is the
// documented choice, the user decides. `oh doctor` says it
// (cmd/doctor_env_test.go): the names look like secrets and they are not part
// of the base list. The credentials oh manages stay behind, in the same
// environment.
func TestWideValidPatternLetsTheSecretsOhDoesNotManageThrough(t *testing.T) {
	secrets := testenv.SetFakeSecrets(t)
	kept, _ := splitProcessEnv(t, []string{"GH_*", "GEMINI_*", "OH_*"})

	passed := []string{"GH_TOKEN", "GEMINI_API_KEY", "OH_EVAL_BEDROCK_TOKEN", "OH_E2E_BEDROCK_TOKEN"}
	for _, n := range passed {
		assert.Contains(t, kept, n+"="+secrets[n], "a pattern the user wrote")
		assert.False(t, FromBase(n), n)
		assert.True(t, LooksSecret(n), "%s: what the doctor warns about", n)
	}
	var left []string
	for n := range secrets {
		if HardDenied(n) {
			left = append(left, n)
		}
	}
	assert.ElementsMatch(t, []string{"AWS_BEARER_TOKEN_BEDROCK", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GITHUB_TOKEN", "GITLAB_TOKEN"}, left)
	for _, n := range left {
		assert.NotContains(t, names(kept), n)
	}
	assert.Len(t, testenv.Leaks(secrets, kept), len(passed))
}

// Suffix patterns are not patterns: `*_TOKEN` does not widen the list, and
// the doctor reports it as invalid.
func TestSuffixPatternsAreInvalid(t *testing.T) {
	for _, p := range []string{"*_TOKEN", "*_KEY", "*", "**", "*KEY"} {
		assert.False(t, ValidPattern(p), p)
	}
	assert.True(t, ValidPattern("AWS_*"))
	assert.False(t, FromExtra("GH_TOKEN", []string{"*_TOKEN"}))
}
