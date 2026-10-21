package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/testenv"
)

func TestAnalyzeEnvAllow(t *testing.T) {
	environ := []string{
		"PATH=/bin", "HOME=/h", "PWD=/x", "SHLVL=1",
		"GEMINI_API_KEY=g-secret", "GH_TOKEN=gh-secret", "MYCO_REGION=eu", "MYCO_API_TOKEN=t-secret", "SSH_AUTH_SOCK=/a",
		"ANTHROPIC_API_KEY=a-secret",
	}
	t.Run("nothing added", func(t *testing.T) {
		f := analyzeEnvAllow(nil, environ)
		assert.ElementsMatch(t, []string{"GEMINI_API_KEY", "GH_TOKEN", "MYCO_REGION", "MYCO_API_TOKEN", "SSH_AUTH_SOCK", "ANTHROPIC_API_KEY"}, f.Held,
			"names of the variables held back, without the terminal noise")
		assert.Empty(t, f.SecretLooks)
		assert.Empty(t, f.SecretPassed)
	})
	t.Run("secret-looking names added", func(t *testing.T) {
		f := analyzeEnvAllow([]string{"GEMINI_API_KEY", "SSH_AUTH_SOCK", "OTHER_PASSWORD", "NPM_SECRET"}, environ)
		assert.ElementsMatch(t, []string{"GEMINI_API_KEY", "OTHER_PASSWORD", "NPM_SECRET"}, f.SecretLooks)
		assert.NotContains(t, f.Held, "GEMINI_API_KEY")
		assert.NotContains(t, f.Held, "SSH_AUTH_SOCK")
		assert.Contains(t, f.Held, "GH_TOKEN")
	})
	t.Run("a pattern lets a secret of the environment through", func(t *testing.T) {
		f := analyzeEnvAllow([]string{"MYCO_*"}, environ)
		assert.Equal(t, []string{"MYCO_API_TOKEN"}, f.SecretPassed)
		assert.NotContains(t, f.Held, "MYCO_REGION")
		assert.Empty(t, f.SecretLooks, "the entry itself is not a secret name")
	})
	t.Run("invalid and managed entries", func(t *testing.T) {
		f := analyzeEnvAllow([]string{"*", "ANTHROPIC_API_KEY", "FINE_ONE"}, environ)
		assert.Equal(t, []string{"*"}, f.Invalid)
		assert.Equal(t, []string{"ANTHROPIC_API_KEY"}, f.Managed)
		assert.Contains(t, f.Held, "ANTHROPIC_API_KEY", "removed anyway")
		assert.Empty(t, f.SecretLooks)
	})
	t.Run("values never appear", func(t *testing.T) {
		f := analyzeEnvAllow([]string{"MYCO_*"}, environ)
		for _, n := range append(f.Held, f.SecretPassed...) {
			assert.NotContains(t, n, "secret")
		}
	})
}

func TestEnvAllowDoctorChecks(t *testing.T) {
	i18ntest.Set(t, "en")
	t.Setenv("GEMINI_API_KEY", "g-secret")
	checks := envAllowDoctorChecks(t.Context())
	if assert.NotEmpty(t, checks) {
		assert.True(t, checks[0].OK)
		assert.Contains(t, checks[0].Detail, "base list")
	}
	var held string
	for _, c := range checks {
		assert.NotContains(t, c.Detail, "g-secret", "values are never shown")
		if c.Warn == false && c.Detail != checks[0].Detail {
			held = c.Detail
		}
	}
	assert.Contains(t, held, "GEMINI_API_KEY")
	assert.Contains(t, held, "oh config set execution.env_allow")
}

// Regression test of the leak of 2026-09-29 (the Bedrock key of the
// environment of oh read by an agent with `env`): with fake credentials of
// every kind in the environment, `oh doctor` says what a wide env_allow does.
// A pattern that lets through a secret oh does not manage (GH_TOKEN,
// GEMINI_API_KEY…) is the documented choice of the user, and the doctor warns;
// the credentials oh manages are removed whatever the pattern, and the doctor
// says so; the suffix patterns are not patterns, and the doctor says so too.
// Neither the names held back nor the warnings ever show a value.
func TestEnvAllowDoctorWarnsOfWhatAWidePatternLetsThrough(t *testing.T) {
	i18ntest.Set(t, "en")
	secrets := testenv.SetFakeSecrets(t)
	environ := os.Environ()

	t.Run("the wide list of the audit", func(t *testing.T) {
		f := analyzeEnvAllow([]string{"AWS_*", "*_TOKEN", "*_KEY"}, environ)
		assert.Equal(t, []string{"*_TOKEN", "*_KEY"}, f.Invalid, "suffix patterns match nothing")
		assert.Empty(t, f.SecretPassed, "AWS_* lets through nothing of the fake credentials: they are managed by oh")
		for _, n := range []string{"AWS_BEARER_TOKEN_BEDROCK", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "GH_TOKEN", "GEMINI_API_KEY", "OH_EVAL_BEDROCK_TOKEN", "OH_E2E_BEDROCK_TOKEN"} {
			assert.Contains(t, f.Held, n, "still held back")
		}
	})

	t.Run("a valid wide pattern", func(t *testing.T) {
		f := analyzeEnvAllow([]string{"GH_*", "GEMINI_*", "OH_*", "AWS_*", "GITHUB_TOKEN"}, environ)
		assert.Equal(t, []string{"GEMINI_API_KEY", "GH_TOKEN", "OH_E2E_BEDROCK_TOKEN", "OH_EVAL_BEDROCK_TOKEN"}, f.SecretPassed,
			"the secrets oh does not manage, that the patterns let through")
		assert.Equal(t, []string{"GITHUB_TOKEN"}, f.Managed)
		for _, n := range []string{"AWS_BEARER_TOKEN_BEDROCK", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "GITHUB_TOKEN", "GITLAB_TOKEN", "ANTHROPIC_API_KEY", "OPENAI_API_KEY"} {
			assert.Contains(t, f.Held, n, "removed whatever the pattern")
		}
	})

	t.Run("the checks of oh doctor", func(t *testing.T) {
		a := &app.App{Config: &config.Config{}}
		a.Config.Execution.EnvAllow = []string{"GH_*", "GEMINI_*", "AWS_*", "*_TOKEN", "GITLAB_TOKEN"}
		application = a
		t.Cleanup(func() { application = nil })

		var warnings []string
		for _, c := range envAllowDoctorChecks(t.Context()) {
			assert.Empty(t, testenv.LeaksIn(secrets, c.Name+" "+c.Detail), "values are never shown")
			if c.Warn {
				warnings = append(warnings, c.Detail)
			}
		}
		require.Len(t, warnings, 3)
		all := strings.Join(warnings, "\n")
		assert.Contains(t, all, "entries ignored")
		assert.Contains(t, all, "*_TOKEN")
		assert.Contains(t, all, "named like secrets: GEMINI_API_KEY, GH_TOKEN", "the user is told which secrets the patterns let through")
		assert.Contains(t, all, "managed by oh")
		assert.Contains(t, all, "GITLAB_TOKEN")
	})
}
